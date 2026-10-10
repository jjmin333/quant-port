package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/enact/quant-port/be/internal/model"
)

/*
종목 타임라인의 재료 — 다섯 표에서 **날짜별로 접어서** 가져온다.

# 접는 것이 핵심이다

삼성전자에 붙은 기사는 15,234건이고 하루 평균 180건이다(실측 2026-09-18). 행을 그대로
가져와 Go에서 세면 한 요청이 수 MB를 읽는다. `GROUP BY` 를 SQL에 맡기면 90일 창이
90행이다.

# 인덱스

뉴스의 종목 필터는 **`MEMBER OF` 라야** `idx_news_symbols_mv`(multi-valued index)를
탄다 — `JSON_CONTAINS`로 쓰면 전체 스캔이다(`news_repo.go`에 같은 주석이 있다).
실측: 90일 창 0.21초, `key: idx_news_symbols_mv`.
*/

// StockTimelineRepo reads the timeline's raw material.
type StockTimelineRepo struct {
	db *sql.DB
}

func NewStockTimelineRepo(db *sql.DB) *StockTimelineRepo {
	return &StockTimelineRepo{db: db}
}

// TimelineDateCount is one day's tally for a lane.
type TimelineDateCount struct {
	Date  string
	Count int
}

// Closes returns the daily closes in the window, oldest first.
func (r *StockTimelineRepo) Closes(
	ctx context.Context, symbol, from, to string,
) ([]model.TimelinePoint, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(trade_date, '%Y-%m-%d'), close_price, change_rate
		  FROM stock_daily_closes
		 WHERE symbol = ? AND trade_date BETWEEN ? AND ?
		 ORDER BY trade_date`, symbol, from, to)
	if err != nil {
		return nil, fmt.Errorf("timeline closes: %w", err)
	}
	defer rows.Close()

	var out []model.TimelinePoint
	for rows.Next() {
		var p model.TimelinePoint
		var rate sql.NullFloat64
		if err := rows.Scan(&p.Date, &p.Close, &rate); err != nil {
			return nil, err
		}
		if rate.Valid {
			v := rate.Float64
			p.ChangeRate = &v
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

/*
NewsCounts — 날짜별 기사 수.

제목을 나르지 않는다. 사용자가 한 날을 펼치면 기존 뉴스 목록 API가 답한다 — 이
응답이 본문까지 실으면 하루 180건 × 90일이 된다.
*/
func (r *StockTimelineRepo) NewsCounts(
	ctx context.Context, symbol, from, to string,
) ([]TimelineDateCount, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	// MEMBER OF 라야 multi-valued index를 탄다 — 머리글 참조.
	rows, err := r.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(published_at, '%Y-%m-%d') d, COUNT(*)
		  FROM news_articles
		 WHERE ? MEMBER OF (symbols_json->'$')
		   AND published_at >= ? AND published_at < DATE_ADD(?, INTERVAL 1 DAY)
		 GROUP BY d`, symbol, from, to)
	if err != nil {
		return nil, fmt.Errorf("timeline news: %w", err)
	}
	defer rows.Close()
	return scanDateCounts(rows)
}

// InsiderDays aggregates insider trades per day into buy/sell share totals.
type TimelineInsiderDay struct {
	Date       string
	Count      int
	BuyShares  float64
	SellShares float64
}

func (r *StockTimelineRepo) InsiderDays(
	ctx context.Context, symbol, from, to string,
) ([]TimelineInsiderDay, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	/*
		`shares_change`의 부호가 방향이다. 매수·매도를 따로 합산해 두면 화면이 순증감과
		함께 「얼마를 사고 얼마를 팔았나」를 말할 수 있다 — 순증감만 주면 1만 주를 사고
		1만 주를 판 날이 아무 일도 없던 날과 같아 보인다.
	*/
	rows, err := r.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(txn_date, '%Y-%m-%d') d,
		       COUNT(*),
		       COALESCE(SUM(CASE WHEN shares_change > 0 THEN shares_change ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN shares_change < 0 THEN -shares_change ELSE 0 END), 0)
		  FROM insider_transactions
		 WHERE symbol = ? AND txn_date BETWEEN ? AND ?
		 GROUP BY d`, symbol, from, to)
	if err != nil {
		return nil, fmt.Errorf("timeline insider: %w", err)
	}
	defer rows.Close()

	var out []TimelineInsiderDay
	for rows.Next() {
		var d TimelineInsiderDay
		if err := rows.Scan(&d.Date, &d.Count, &d.BuyShares, &d.SellShares); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Disclosures returns the filings in the window.
type TimelineDisclosureRow struct {
	Date      string
	Title     string
	ReceiptNo string
	Filer     string
}

func (r *StockTimelineRepo) Disclosures(
	ctx context.Context, symbol, from, to string,
) ([]TimelineDisclosureRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	// 상한을 둔다 — 공시가 하루 수십 건인 종목에서 이 목록이 응답을 지배하지 않도록.
	rows, err := r.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(receipt_date, '%Y-%m-%d'), title,
		       COALESCE(receipt_no, ''), COALESCE(filer, '')
		  FROM stock_disclosures
		 WHERE symbol = ? AND receipt_date BETWEEN ? AND ?
		 ORDER BY receipt_date DESC, id DESC
		 LIMIT 200`, symbol, from, to)
	if err != nil {
		return nil, fmt.Errorf("timeline disclosures: %w", err)
	}
	defer rows.Close()

	var out []TimelineDisclosureRow
	for rows.Next() {
		var d TimelineDisclosureRow
		if err := rows.Scan(&d.Date, &d.Title, &d.ReceiptNo, &d.Filer); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// EarningsDays returns earnings events whose announcement falls in the window.
type TimelineEarningsRow struct {
	Date        string
	FiscalYear  sql.NullInt64
	Period      sql.NullString
	Session     sql.NullString
	DateStatus  sql.NullString
	RevenueAct  sql.NullFloat64
	RevenueEst  sql.NullFloat64
	OpIncomeAct sql.NullFloat64
	EPSAct      sql.NullFloat64
	EPSEst      sql.NullFloat64
	Currency    sql.NullString
}

func (r *StockTimelineRepo) EarningsDays(
	ctx context.Context, symbol, from, to string,
) ([]TimelineEarningsRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(event_date, '%Y-%m-%d'),
		       fiscal_year, fiscal_period, session, date_status,
		       revenue_act, revenue_est, op_income_act, eps_act, eps_est, currency
		  FROM earnings_events
		 WHERE symbol = ? AND event_date BETWEEN ? AND ?
		 ORDER BY event_date`, symbol, from, to)
	if err != nil {
		return nil, fmt.Errorf("timeline earnings: %w", err)
	}
	defer rows.Close()

	var out []TimelineEarningsRow
	for rows.Next() {
		var e TimelineEarningsRow
		if err := rows.Scan(
			&e.Date, &e.FiscalYear, &e.Period, &e.Session, &e.DateStatus,
			&e.RevenueAct, &e.RevenueEst, &e.OpIncomeAct, &e.EPSAct, &e.EPSEst, &e.Currency,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

/*
LaneStarts — 각 레인이 이 종목에 대해 **언제부터 기록돼 있나**.

요청 창과 무관한 전체 기록의 첫 날이다. 이 값이 없으면 화면은 「수집 전」과 「아무 일도
없었다」를 구분할 수 없다 — 이 기능에서 가장 중요한 한 줄이다(`model` 머리글).

다섯 번의 가벼운 MIN 조회다. 하나로 합칠 수도 있지만 표마다 인덱스가 달라 UNION이
오히려 느리고, 각각은 인덱스 첫 행 하나만 읽는다.
*/
func (r *StockTimelineRepo) LaneStarts(ctx context.Context, symbol string) (map[model.TimelineLaneID]string, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	queries := []struct {
		lane  model.TimelineLaneID
		query string
	}{
		{model.TimelineLaneClose, `SELECT DATE_FORMAT(MIN(trade_date), '%Y-%m-%d') FROM stock_daily_closes WHERE symbol = ?`},
		{model.TimelineLaneScore, `SELECT DATE_FORMAT(MIN(trade_date), '%Y-%m-%d') FROM stock_scores WHERE symbol = ?`},
		{model.TimelineLaneEarnings, `SELECT DATE_FORMAT(MIN(event_date), '%Y-%m-%d') FROM earnings_events WHERE symbol = ?`},
		{model.TimelineLaneInsider, `SELECT DATE_FORMAT(MIN(txn_date), '%Y-%m-%d') FROM insider_transactions WHERE symbol = ?`},
		{model.TimelineLaneDisclosure, `SELECT DATE_FORMAT(MIN(receipt_date), '%Y-%m-%d') FROM stock_disclosures WHERE symbol = ?`},
		{model.TimelineLaneNews, `SELECT DATE_FORMAT(MIN(published_at), '%Y-%m-%d') FROM news_articles WHERE ? MEMBER OF (symbols_json->'$')`},
	}

	out := make(map[model.TimelineLaneID]string, len(queries))
	for _, q := range queries {
		var date sql.NullString
		if err := r.db.QueryRowContext(ctx, q.query, symbol).Scan(&date); err != nil {
			return nil, fmt.Errorf("timeline lane start %s: %w", q.lane, err)
		}
		if date.Valid {
			out[q.lane] = date.String
		}
	}
	return out, nil
}

func scanDateCounts(rows *sql.Rows) ([]TimelineDateCount, error) {
	var out []TimelineDateCount
	for rows.Next() {
		var c TimelineDateCount
		if err := rows.Scan(&c.Date, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

/*
TimelineNewsRow — 타임라인에서 하루를 펼쳤을 때 보여 줄 기사 한 줄.

# 왜 뉴스 목록 API를 쓰지 않는가

`NewsStore.List`는 앱에서 가장 손이 많이 간 읽기 경로다 — 15초 TTL 캐시 · 싱글플라이트 ·
만료분 먼저 주고 뒤에서 재구성. 날짜 필터를 더하면 **캐시 키에 차원이 하나 늘고**, 그
키는 거의 재사용되지 않는다(사람마다 다른 날을 편다). 잘 도는 경로에 쓰지 않을 키를
섞는 대신, 타임라인이 자기 조회를 든다.

한 날 · 한 종목이라 `idx_news_symbols_mv` + `published_at` 범위로 끝난다.
*/
type TimelineNewsRow struct {
	ID          string
	Headline    string
	HeadlineKo  sql.NullString
	URL         string
	PublisherID sql.NullString
	Publisher   sql.NullString
	PublishedAt string
	ViewCount   int
}

/*
NewsForDate — 한 날의 기사. 많이 본 순, 같으면 **이른 시각 순**.

# 두 번째 정렬 키가 이른 시각인 이유 (실측 2026-09-18)

처음에는 `view_count DESC, published_at DESC`였다. 그런데 이 DB의 조회수는 사실상
전부 0이라 첫 키가 아무것도 가르지 못하고, 결과가 조용히 **「그날 마지막 20분」**이
됐다 — 005930의 07-31(+21.3%)을 열었더니 23:21~23:40 다섯 건이었다.

하루를 설명하는 기사는 장 전과 장중에 나온다. 그래서 동점일 때는 이른 쪽을 먼저 둔다.
조회수가 실제로 쌓이면 첫 키가 다시 일을 한다 — 규칙을 버리는 대신 순서만 바꿨다.
*/
func (r *StockTimelineRepo) NewsForDate(
	ctx context.Context, symbol, date string, limit int,
) ([]TimelineNewsRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, headline, headline_ko, url, publisher_id, publisher_name,
		       DATE_FORMAT(published_at, '%Y-%m-%d %H:%i'), view_count
		  FROM news_articles
		 WHERE ? MEMBER OF (symbols_json->'$')
		   AND published_at >= ? AND published_at < DATE_ADD(?, INTERVAL 1 DAY)
		 ORDER BY view_count DESC, published_at ASC
		 LIMIT ?`, symbol, date, date, limit)
	if err != nil {
		return nil, fmt.Errorf("timeline news for date: %w", err)
	}
	defer rows.Close()

	var out []TimelineNewsRow
	for rows.Next() {
		var n TimelineNewsRow
		if err := rows.Scan(
			&n.ID, &n.Headline, &n.HeadlineKo, &n.URL,
			&n.PublisherID, &n.Publisher, &n.PublishedAt, &n.ViewCount,
		); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
