package growth

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

/*
앱 표 읽기 — 활성화 지표와 콘텐츠 후보의 재료. **읽기만 한다.**

사용자 표에서는 개수만 센다. 누가 가입했는지·무엇을 담았는지는 꺼내지 않는다.
공개 리포트와 변화 감지는 이미 공개 화면에 나가는 값(제목 · 종목)만 읽는다.
*/
type activity struct {
	db *sql.DB
}

// DayActivity — 하루 사용자 활동(개수). 그날 = 서울 자정~자정.
type DayActivity struct {
	Signups           int `json:"signups"`
	ActiveUsers       int `json:"activeUsers"` // 그날 로그인(리프레시 토큰 발급)한 사용자 수
	FirstHoldingUsers int `json:"firstHoldingUsers"`
	HoldingsAdded     int `json:"holdingsAdded"`
	WatchlistAdded    int `json:"watchlistAdded"`
	ReportsPublished  int `json:"reportsPublished"`
	TotalUsers        int `json:"totalUsers"`
	UsersWithHoldings int `json:"usersWithHoldings"`
}

/*
realUser — 시험 계정을 뺀다(SQL 조건, 별칭 u = users).

2026-09-25 첫 실행에서 「가입 293 (+29,200%)」이 나왔는데 292개가 e2e 계정(`@quant-port.test`,
e2e/src/env.ts의 TEST_EMAIL_DOMAIN)이었다. 리포트 40건도 대부분 그 계정들이 썼다. 도메인 하나만
막지 않고 RFC 2606 예약 도메인(.test · .example · .invalid · .localhost)을 다 뺀다 — 실제로 메일을
받을 수 없는 주소라 사람일 수가 없다.

2026-09-27부터 가입은 이메일이 아니라 아이디라(요청 3) 새 e2e 계정은 도메인이 없다 — 대신
예약 접두사 `qae2e`로 시작한다(e2e/src/data.ts의 TEST_LOGIN_PREFIX). `_`는 LIKE의 한 글자
와일드카드라 접두사에 쓰지 않았다.
*/
const realUser = `u.email NOT LIKE '%.test' AND u.email NOT LIKE '%.example' AND u.email NOT LIKE '%.invalid' AND u.email NOT LIKE '%.localhost' AND u.email NOT LIKE 'qae2e%'`

func (a *activity) day(ctx context.Context, day time.Time) (DayActivity, error) {
	from, to := dayOf(day), dayOf(day).AddDate(0, 0, 1)
	var out DayActivity
	queries := []struct {
		dst  *int
		sql  string
		args []any
	}{
		{&out.Signups, `SELECT COUNT(*) FROM users u WHERE u.created_at >= ? AND u.created_at < ? AND ` + realUser, []any{from, to}},
		{&out.ActiveUsers, `SELECT COUNT(DISTINCT t.user_id) FROM user_refresh_tokens t JOIN users u ON u.id = t.user_id
			WHERE t.created_at >= ? AND t.created_at < ? AND ` + realUser, []any{from, to}},
		{&out.FirstHoldingUsers, `SELECT COUNT(*) FROM (SELECT h.user_id, MIN(h.created_at) m FROM user_holdings h GROUP BY h.user_id) t
			JOIN users u ON u.id = t.user_id WHERE t.m >= ? AND t.m < ? AND ` + realUser, []any{from, to}},
		{&out.HoldingsAdded, `SELECT COUNT(*) FROM user_holdings h JOIN users u ON u.id = h.user_id
			WHERE h.created_at >= ? AND h.created_at < ? AND ` + realUser, []any{from, to}},
		{&out.WatchlistAdded, `SELECT COUNT(*) FROM watchlist_items w JOIN users u ON u.id = w.user_id
			WHERE w.created_at >= ? AND w.created_at < ? AND ` + realUser, []any{from, to}},
		{&out.ReportsPublished, `SELECT COUNT(*) FROM research_calls r JOIN users u ON u.id = r.user_id
			WHERE r.published_at >= ? AND r.published_at < ? AND ` + realUser, []any{from, to}},
		{&out.TotalUsers, `SELECT COUNT(*) FROM users u WHERE u.created_at < ? AND ` + realUser, []any{to}},
		{&out.UsersWithHoldings, `SELECT COUNT(DISTINCT h.user_id) FROM user_holdings h JOIN users u ON u.id = h.user_id
			WHERE h.created_at < ? AND ` + realUser, []any{to}},
	}
	for _, q := range queries {
		if err := a.db.QueryRowContext(ctx, q.sql, q.args...).Scan(q.dst); err != nil {
			return out, err
		}
	}
	return out, nil
}

// Cohort — 한 주에 가입한 사람 중 7일 안에 종목을 하나라도 담은(보유 또는 관심) 비율.
type Cohort struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Signups   int    `json:"signups"`
	Activated int    `json:"activated"`
}

func (a *activity) cohort(ctx context.Context, from, to time.Time) (Cohort, error) {
	out := Cohort{From: ymd(from), To: ymd(to)}
	var activated sql.NullInt64
	err := a.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       SUM(EXISTS(SELECT 1 FROM user_holdings h WHERE h.user_id = u.id AND h.created_at < u.created_at + INTERVAL 7 DAY)
		           OR EXISTS(SELECT 1 FROM watchlist_items w WHERE w.user_id = u.id AND w.created_at < u.created_at + INTERVAL 7 DAY))
		  FROM users u WHERE u.created_at >= ? AND u.created_at < ? AND `+realUser,
		dayOf(from), dayOf(to).AddDate(0, 0, 1)).Scan(&out.Signups, &activated)
	out.Activated = int(activated.Int64)
	return out, err
}

// stockName — 검색어를 종목에 잇는 재료.
type stockName struct {
	Symbol string
	Name   string
}

// stockNames — 이름이 두 글자 이상인 상장 종목. 검색어 안에 이름이 통째로 들어 있으면 그 종목으로 본다.
func (a *activity) stockNames(ctx context.Context) ([]stockName, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT symbol, name FROM stocks
		 WHERE asset_type IN ('domestic', 'us', 'etf') AND delist_date IS NULL AND CHAR_LENGTH(name) >= 2`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []stockName
	for rows.Next() {
		var s stockName
		if err := rows.Scan(&s.Symbol, &s.Name); err != nil {
			return nil, err
		}
		s.Name = strings.TrimSpace(s.Name)
		out = append(out, s)
	}
	return out, rows.Err()
}

// ShareCandidate — 외부 유통 후보(항목 10). 게시는 하지 않는다 — 사람이 고른다.
type ShareCandidate struct {
	Kind   string `json:"kind"` // report | change
	Title  string `json:"title"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

func (a *activity) shareCandidates(ctx context.Context, today time.Time, indexable map[string]bool) ([]ShareCandidate, error) {
	var out []ShareCandidate
	rows, err := a.db.QueryContext(ctx, `
		SELECT r.id, r.title, r.symbol_name, r.view_count FROM research_calls r JOIN users u ON u.id = r.user_id
		 WHERE r.visibility = 'public' AND r.hidden_at IS NULL AND r.retracted_at IS NULL AND r.published_at >= ?
		   AND `+realUser+`
		 ORDER BY r.view_count DESC, r.published_at DESC LIMIT 5`, dayOf(today).AddDate(0, 0, -7))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var title, name string
		var views int
		if err := rows.Scan(&id, &title, &name, &views); err != nil {
			rows.Close()
			return nil, err
		}
		if title == "" {
			title = name + " 리포트"
		}
		out = append(out, ShareCandidate{Kind: "report", Title: title, Path: "/reports/" + strconv.FormatInt(id, 10),
			Reason: "최근 7일 공개 리포트 · 조회 " + strconv.Itoa(views)})
	}
	rows.Close()

	// 가장 최근 감지일의 큰 변화 — 색인 대상 종목만(유입이 닿을 페이지가 있어야 공유가 남는다).
	rows, err = a.db.QueryContext(ctx, `
		SELECT symbol, title, importance FROM stock_change_events
		 WHERE detected_for = (SELECT MAX(detected_for) FROM stock_change_events)
		 ORDER BY importance DESC LIMIT 40`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	picked := 0
	for rows.Next() && picked < 5 {
		var sym, title string
		var imp float64
		if err := rows.Scan(&sym, &title, &imp); err != nil {
			return out, err
		}
		if !indexable[strings.ToUpper(sym)] {
			continue
		}
		picked++
		out = append(out, ShareCandidate{Kind: "change", Title: title, Path: "/stocks/" + sym, Reason: "가장 최근 변화 감지 상위"})
	}
	return out, rows.Err()
}

// upcomingEarnings — 앞으로 7일 안에 실적이 나오는 색인 대상 종목(콘텐츠 후보).
func (a *activity) upcomingEarnings(ctx context.Context, today time.Time, indexable map[string]bool) ([]string, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT DISTINCT symbol FROM earnings_events WHERE event_date BETWEEN ? AND ?`,
		ymd(today), ymd(today.AddDate(0, 0, 7)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var sym string
		if err := rows.Scan(&sym); err != nil {
			return nil, err
		}
		if indexable[strings.ToUpper(sym)] {
			out = append(out, strings.ToUpper(sym))
		}
	}
	return out, rows.Err()
}
