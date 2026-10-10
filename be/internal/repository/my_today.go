package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

/*
홈 「내 종목」 카드의 재료 (2026-09-26 사용자 결정 — 재방문 개선 C).

가입하고 종목을 담아도 홈이 비로그인 화면과 똑같았다. 다시 들어올 이유는 「내 종목에
오늘 무슨 일이 있었나」인데, 그걸 보려면 관심·보유·변화·뉴스 화면을 따로 돌아야 했다.
이 표 셋(관심·보유 · 점수 두 날 · 최근 24시간 기사)을 **한 요청**으로 묶는다.

가격은 여기서 주지 않는다 — 화면이 `useLiveQuotes`로 받아 장중에는 흐르는 값을 얹는다.
실적 일정도 주지 않는다 — 국내 예정일은 읽을 때 추정하는 규칙(`EstimateNextAnnouncement`)이
실적 배치 엔드포인트에 있고, 화면이 이미 그것을 5분 캐시로 부른다.
*/

// MyTodayMaxSymbols caps the card's universe — 관심·보유가 이보다 많으면 관심 순서
// 앞쪽(보유 우선)만 본다. 카드는 여덟 줄을 그리고 나머지는 「외 n종목」으로 적는다.
const MyTodayMaxSymbols = 40

// MyTodaySymbol is one tracked symbol with its standing and today's signals.
type MyTodaySymbol struct {
	Symbol     string `json:"symbol"`
	Name       string `json:"name"`
	AssetType  string `json:"assetType"`
	Held       bool   `json:"held"`
	Watched    bool   `json:"watched"`
	Score      *int   `json:"score,omitempty"`
	ScoreDelta *int   `json:"scoreDelta,omitempty"`
	News24h    int    `json:"news24h"`
	// AIOverview — 저장된 AI 요약 글의 앞부분(최대 300자). 화면은 첫 문장만 쓴다.
	AIOverview string `json:"aiOverview,omitempty"`
	// AIAt — 그 글이 쓰인 날(YYYY-MM-DD).
	AIAt string `json:"aiAt,omitempty"`
}

// MyTodayRepo reads the 「내 종목」 card's rows.
type MyTodayRepo struct{ db *sql.DB }

func NewMyTodayRepo(db *sql.DB) *MyTodayRepo { return &MyTodayRepo{db: db} }

/*
Tracked — 관심·보유 합집합. 보유가 앞, 그다음 관심 목록에서 사용자가 정한 순서.
수량 0인 보유 행은 「판 종목」이라 빼고, 관심에 있으면 관심으로만 남는다.
*/
func (r *MyTodayRepo) Tracked(ctx context.Context, userID int64) ([]MyTodaySymbol, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.symbol, COALESCE(s.name, ''), COALESCE(s.asset_type, ''),
		       MAX(t.held), MAX(t.watched), MIN(t.ord)
		  FROM (
		        SELECT symbol, 0 AS held, 1 AS watched, sort_order AS ord
		          FROM watchlist_items WHERE user_id = ?
		        UNION ALL
		        SELECT symbol, 1, 0, -1
		          FROM user_holdings WHERE user_id = ? AND quantity > 0
		       ) t
		  LEFT JOIN stocks s ON s.symbol = t.symbol
		 GROUP BY t.symbol, s.name, s.asset_type
		 ORDER BY MIN(t.ord), t.symbol
		 LIMIT ?`, userID, userID, MyTodayMaxSymbols)
	if err != nil {
		return nil, fmt.Errorf("my today tracked: %w", err)
	}
	defer rows.Close()
	var out []MyTodaySymbol
	for rows.Next() {
		var (
			it            MyTodaySymbol
			held, watched int
			ord           sql.NullInt64
		)
		if err := rows.Scan(&it.Symbol, &it.Name, &it.AssetType, &held, &watched, &ord); err != nil {
			return nil, err
		}
		it.Held = held == 1
		it.Watched = watched == 1
		out = append(out, it)
	}
	return out, rows.Err()
}

/*
AttachScores — 최신 점수일과 그 전 점수일의 점수를 붙이고 차이를 낸다. 반환은 최신 점수일.

두 날짜는 표 전체의 최근 두 날이다(점수 배치는 전 종목을 한 날짜로 매긴다). 어제 점수가
없는 종목(새로 매겨진 종목)은 차이를 비운다 — 0이라고 적으면 「변화 없음」이라는 다른 말이다.
*/
func (r *MyTodayRepo) AttachScores(ctx context.Context, items []MyTodaySymbol) (string, error) {
	if len(items) == 0 {
		return "", nil
	}
	dateRows, err := r.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(trade_date, '%Y-%m-%d') FROM stock_scores
		 GROUP BY trade_date ORDER BY trade_date DESC LIMIT 2`)
	if err != nil {
		return "", fmt.Errorf("my today score dates: %w", err)
	}
	var dates []string
	for dateRows.Next() {
		var d string
		if err := dateRows.Scan(&d); err != nil {
			dateRows.Close()
			return "", err
		}
		dates = append(dates, d)
	}
	dateRows.Close()
	if len(dates) == 0 {
		return "", nil
	}

	marks := strings.TrimSuffix(strings.Repeat("?,", len(items)), ",")
	dateMarks := strings.TrimSuffix(strings.Repeat("?,", len(dates)), ",")
	args := make([]any, 0, len(items)+len(dates))
	for _, it := range items {
		args = append(args, it.Symbol)
	}
	for _, d := range dates {
		args = append(args, d)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, DATE_FORMAT(trade_date, '%Y-%m-%d'), score
		  FROM stock_scores
		 WHERE symbol IN (`+marks+`) AND trade_date IN (`+dateMarks+`) AND score IS NOT NULL`, args...)
	if err != nil {
		return "", fmt.Errorf("my today scores: %w", err)
	}
	defer rows.Close()
	latest := map[string]int{}
	prev := map[string]int{}
	for rows.Next() {
		var (
			sym, d string
			score  int
		)
		if err := rows.Scan(&sym, &d, &score); err != nil {
			return "", err
		}
		if d == dates[0] {
			latest[sym] = score
		} else {
			prev[sym] = score
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	for i := range items {
		cur, ok := latest[items[i].Symbol]
		if !ok {
			continue
		}
		c := cur
		items[i].Score = &c
		if p, ok := prev[items[i].Symbol]; ok {
			d := cur - p
			items[i].ScoreDelta = &d
		}
	}
	return dates[0], nil
}

/*
AttachNewsCounts — 최근 24시간에 이 종목이 태그된 기사 수.

**날짜 인덱스로 먼저 좁힌다.** 심볼 다중값 인덱스(`idx_news_symbols_mv`)로 가면 인기 종목
(NVDA·삼성전자)의 **모든 기간** 기사를 훑은 뒤 날짜를 거른다 — 실측(09-26) 인기 종목 8개에
0.99초. 24시간 창은 두 시장 합쳐 1,800건 남짓이라 `(market, published_at)`으로 먼저
자르면 0.13초다. 해외 기사는 번역이 붙은 것만 센다 — 뉴스 화면이 보여 주는 기사와 같은 수.
*/
func (r *MyTodayRepo) AttachNewsCounts(ctx context.Context, items []MyTodaySymbol, now time.Time) error {
	if len(items) == 0 {
		return nil
	}
	want := make([]string, 0, len(items))
	index := make(map[string]int, len(items))
	for i, it := range items {
		want = append(want, it.Symbol)
		index[it.Symbol] = i
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		return err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbols_json
		  FROM news_articles FORCE INDEX (idx_news_market_published)
		 WHERE market IN ('domestic', 'us')
		   AND published_at >= ?
		   AND JSON_OVERLAPS(symbols_json->'$', CAST(? AS JSON))
		   AND (market <> 'us' OR COALESCE(headline_ko, '') <> '')`,
		now.Add(-24*time.Hour), string(wantJSON))
	if err != nil {
		return fmt.Errorf("my today news: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return err
		}
		var syms []string
		if json.Unmarshal(raw, &syms) != nil {
			continue
		}
		for _, s := range syms {
			if i, ok := index[strings.ToUpper(s)]; ok {
				items[i].News24h++
			}
		}
	}
	return rows.Err()
}

// myTodayAIOverviewMax caps the stored overview we ship per row — 화면은 첫 문장만 그린다.
const myTodayAIOverviewMax = 300

/*
AttachAIOverviews — 저장된 AI 요약 글(`stock_summary_narratives.overview`)의 앞부분과 쓰인 날.

관심 종목 화면의 「AI 한 줄」 재료다 (2026-09-27 사용자 결정 A). **LLM을 부르지 않는다** —
배치가 이미 써 둔 글을 PK로 읽을 뿐이다. 관심·보유 종목은 AI 요약 배치의 3층 대상이라
대개 일주일 안의 글이 있다. 글이 없는 종목은 비워 두고 화면이 그 줄을 생략한다.
*/
func (r *MyTodayRepo) AttachAIOverviews(ctx context.Context, items []MyTodaySymbol) error {
	if len(items) == 0 {
		return nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(items)), ",")
	args := make([]any, 0, len(items))
	index := make(map[string]int, len(items))
	for i, it := range items {
		args = append(args, strings.ToUpper(it.Symbol))
		index[strings.ToUpper(it.Symbol)] = i
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol,
		       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(payload, '$.overview')), ''),
		       DATE_FORMAT(analyzed_at, '%Y-%m-%d')
		  FROM stock_summary_narratives
		 WHERE symbol IN (`+marks+`)`, args...)
	if err != nil {
		return fmt.Errorf("my today ai overviews: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sym, overview, at string
		if err := rows.Scan(&sym, &overview, &at); err != nil {
			return err
		}
		i, ok := index[strings.ToUpper(sym)]
		if !ok {
			continue
		}
		items[i].AIOverview = clipRunes(strings.TrimSpace(overview), myTodayAIOverviewMax)
		if items[i].AIOverview != "" {
			items[i].AIAt = at
		}
	}
	return rows.Err()
}

// clipRunes cuts s to at most n characters (not bytes — 한글은 3바이트).
func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
