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
변화 이벤트의 재료 조회와 저장. 규칙(문턱·중요도)은 `service/change_detect.go`가 갖고,
여기는 **표에서 후보를 뽑는 일**만 한다. 표를 왜 두는지는 `migrations/080_stock_change_events.sql`.

모든 조회는 기준일 D 하나에 대해 한 번씩이고 종목 단위 반복이 없다.
*/

// ChangeEventRepo owns stock_change_events and the candidate queries.
type ChangeEventRepo struct {
	db *sql.DB
}

func NewChangeEventRepo(db *sql.DB) *ChangeEventRepo { return &ChangeEventRepo{db: db} }

// ChangeUniverseRow is one symbol eligible for the feed.
type ChangeUniverseRow struct {
	Symbol      string
	Name        string
	EnglishName string
	AssetType   string
	MarketCap   float64
	Tracked     bool
}

/*
Universe — 그날 점수가 있는 주식(ETF 제외)과 시가총액, 누가 관심·보유 중인지.

시가총액은 점수 행의 `inputs.marketCap`(종가 × 주식 수)이다 — 따로 계산하지 않는다.
*/
func (r *ChangeEventRepo) Universe(ctx context.Context, tradeDate string) (map[string]ChangeUniverseRow, error) {
	// 지수 추종 상품(ETF·ETN)은 뺀다 — AI 랭킹과 같은 규칙. 국내 ETF·ETN은 asset_type이
	// domestic이라 자산유형만으로는 안 걸러진다(2026-09-23 실측: 채권 ETN이 거래량 270배로 맨 위에 섰다).
	productClause, productArgs := scoreRankableProductClause("st")
	args := append([]any{tradeDate}, productArgs...)
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.symbol, COALESCE(st.name, ''), COALESCE(st.english_name, ''), st.asset_type,
		       COALESCE(CAST(s.inputs->>'$.marketCap' AS DOUBLE), 0),
		       (EXISTS (SELECT 1 FROM watchlist_items w WHERE w.symbol = s.symbol)
		        OR EXISTS (SELECT 1 FROM user_holdings h WHERE h.symbol = s.symbol))
		  FROM stock_scores s
		  JOIN stocks st ON st.symbol = s.symbol
		 WHERE s.trade_date = ? AND st.asset_type IN ('domestic', 'us')`+productClause, args...)
	if err != nil {
		return nil, fmt.Errorf("change universe: %w", err)
	}
	defer rows.Close()
	out := map[string]ChangeUniverseRow{}
	for rows.Next() {
		var u ChangeUniverseRow
		if err := rows.Scan(&u.Symbol, &u.Name, &u.EnglishName, &u.AssetType, &u.MarketCap, &u.Tracked); err != nil {
			return nil, err
		}
		out[u.Symbol] = u
	}
	return out, rows.Err()
}

// PriceSessionRow is one symbol's latest session plus its own recent history.
type PriceSessionRow struct {
	Symbol     string
	Date       string
	ChangeRate float64
	Volume     float64
	StdDev     float64 // of daily change_rate over the prior 90 days
	AvgVolume  float64 // over the prior 20 sessions
}

/*
LatestSessions — 시장별 **가장 최근 세션**의 등락과, 그 종목 자신의 과거 변동성·거래량.

종목마다 마지막 종가 날짜가 아니라 **그 자산유형의 최신 세션일**만 본다. 거래정지로
사흘 전이 마지막인 종목의 사흘 전 급등을 오늘 변화로 올리면 안 된다.
*/
func (r *ChangeEventRepo) LatestSessions(ctx context.Context, tradeDate string) ([]PriceSessionRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH sess AS (
		  SELECT st.asset_type, MAX(c.trade_date) d
		    FROM stock_daily_closes c JOIN stocks st ON st.symbol = c.symbol
		   WHERE c.trade_date BETWEEN DATE_SUB(?, INTERVAL 6 DAY) AND ?
		     AND st.asset_type IN ('domestic', 'us')
		   GROUP BY st.asset_type),
		cur AS (
		  SELECT c.symbol, c.trade_date, c.change_rate, c.volume
		    FROM stock_daily_closes c
		    JOIN stocks st ON st.symbol = c.symbol
		    JOIN sess ON sess.asset_type = st.asset_type AND sess.d = c.trade_date),
		hist AS (
		  SELECT c.symbol, STDDEV_SAMP(c.change_rate) sd, COUNT(*) n
		    FROM stock_daily_closes c JOIN cur ON cur.symbol = c.symbol
		   WHERE c.trade_date >= DATE_SUB(cur.trade_date, INTERVAL 90 DAY) AND c.trade_date < cur.trade_date
		   GROUP BY c.symbol),
		vol AS (
		  SELECT symbol, AVG(volume) av FROM (
		    SELECT c.symbol, c.volume,
		           ROW_NUMBER() OVER (PARTITION BY c.symbol ORDER BY c.trade_date DESC) rn
		      FROM stock_daily_closes c JOIN cur ON cur.symbol = c.symbol
		     WHERE c.trade_date < cur.trade_date AND c.trade_date >= DATE_SUB(cur.trade_date, INTERVAL 45 DAY)
		  ) x WHERE rn <= 20 GROUP BY symbol)
		SELECT cur.symbol, DATE_FORMAT(cur.trade_date, '%Y-%m-%d'),
		       COALESCE(cur.change_rate, 0), COALESCE(cur.volume, 0),
		       COALESCE(hist.sd, 0), COALESCE(vol.av, 0)
		  FROM cur
		  JOIN hist ON hist.symbol = cur.symbol AND hist.n >= 20
		  LEFT JOIN vol ON vol.symbol = cur.symbol`, tradeDate, tradeDate)
	if err != nil {
		return nil, fmt.Errorf("change sessions: %w", err)
	}
	defer rows.Close()
	var out []PriceSessionRow
	for rows.Next() {
		var p PriceSessionRow
		if err := rows.Scan(&p.Symbol, &p.Date, &p.ChangeRate, &p.Volume, &p.StdDev, &p.AvgVolume); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ScoreJumpRow compares two stored score days.
type ScoreJumpRow struct {
	Symbol    string
	Score     int
	PrevScore int
	PrevDate  string
	SameAxes  bool // 두 날 모두 같은 축들이 있었나 — 아니면 축 유무가 만든 변동이다
	// SamePeers — 두 날 지표마다 같은 비교 집단에서 순위를 매겼나(inputs.rankedIn).
	// 섹터 피어가 켜진 날처럼 집단이 바뀌면 회사가 아니라 잣대가 움직인 것이다.
	SamePeers bool
	// SameMethod — 두 날의 산출 방식 판(inputs.method)이 같은가. 판이 없던 옛 행과의
	// 비교는 「다르다」로 본다 — 모르는 것을 같다고 치면 방식 변경일이 변화로 새어 나온다.
	SameMethod bool
}

/*
ScoreJumps — 기준일과 그 직전 점수일의 종합 점수 차가 `minDelta` 이상인 종목.

「같은 축들」은 다섯 축 각각의 유무가 두 날 같은가다. 축이 생기거나 사라져서 바뀐
점수는 회사에 일어난 일이 아니라 데이터에 일어난 일이다(V2_PLAN §0 — AAPL +11 중
+10.75가 가치 축 소멸). 호출자가 그것을 변화에서 뺀다.
*/
func (r *ChangeEventRepo) ScoreJumps(ctx context.Context, tradeDate string, minDelta int) ([]ScoreJumpRow, int, error) {
	var prev sql.NullString
	if err := r.db.QueryRowContext(ctx,
		`SELECT DATE_FORMAT(MAX(trade_date), '%Y-%m-%d') FROM stock_score_history WHERE trade_date < ?`,
		tradeDate).Scan(&prev); err != nil {
		return nil, 0, fmt.Errorf("score jumps prev: %w", err)
	}
	if !prev.Valid {
		return nil, 0, nil
	}
	var compared int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM stock_score_history a
		  JOIN stock_score_history b ON b.symbol = a.symbol AND b.trade_date = ?
		 WHERE a.trade_date = ? AND a.score IS NOT NULL AND b.score IS NOT NULL`,
		prev.String, tradeDate).Scan(&compared); err != nil {
		return nil, 0, fmt.Errorf("score jumps count: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.symbol, a.score, b.score,
		       ((a.technical IS NULL) = (b.technical IS NULL)
		        AND (a.quality IS NULL) = (b.quality IS NULL)
		        AND (a.growth IS NULL) = (b.growth IS NULL)
		        AND (a.valuation IS NULL) = (b.valuation IS NULL)
		        AND (a.sentiment IS NULL) = (b.sentiment IS NULL)),
		       COALESCE(sa.inputs->'$.rankedIn' = sb.inputs->'$.rankedIn', TRUE),
		       COALESCE(sa.inputs->>'$.method' = sb.inputs->>'$.method', FALSE)
		  FROM stock_score_history a
		  JOIN stock_score_history b ON b.symbol = a.symbol AND b.trade_date = ?
		  LEFT JOIN stock_scores sa ON sa.symbol = a.symbol AND sa.trade_date = a.trade_date
		  LEFT JOIN stock_scores sb ON sb.symbol = b.symbol AND sb.trade_date = b.trade_date
		 WHERE a.trade_date = ? AND a.score IS NOT NULL AND b.score IS NOT NULL
		   AND ABS(a.score - b.score) >= ?`, prev.String, tradeDate, minDelta)
	if err != nil {
		return nil, 0, fmt.Errorf("score jumps: %w", err)
	}
	defer rows.Close()
	var out []ScoreJumpRow
	for rows.Next() {
		j := ScoreJumpRow{PrevDate: prev.String}
		if err := rows.Scan(&j.Symbol, &j.Score, &j.PrevScore, &j.SameAxes, &j.SamePeers, &j.SameMethod); err != nil {
			return nil, 0, err
		}
		out = append(out, j)
	}
	return out, compared, rows.Err()
}

// NewsEventRow is one labelled article carrying an event type.
type NewsEventRow struct {
	Symbol    string
	EventType string
	Score     float64
	Headline  string // 보여 줄 제목(번역이 있으면 번역)
	Original  string // 원문 제목 — 이 기사가 그 종목 이야기인지 가를 때 번역과 함께 본다
	Date      string
}

// NewsEventsBetween returns event-bearing labelled articles per tagged symbol.
func (r *ChangeEventRepo) NewsEventsBetween(ctx context.Context, from, to time.Time) ([]NewsEventRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT jt.sym, s.event_type, s.score, COALESCE(NULLIF(n.headline_ko, ''), n.headline),
		       n.headline, DATE_FORMAT(n.published_at, '%Y-%m-%d')
		  FROM news_articles n
		  JOIN news_article_sentiment s ON s.article_id = n.id
		  JOIN JSON_TABLE(n.symbols_json, '$[*]' COLUMNS (sym VARCHAR(32) PATH '$')) jt
		 WHERE n.published_at >= ? AND n.published_at < ? AND s.event_type IS NOT NULL`, from, to)
	if err != nil {
		return nil, fmt.Errorf("change news events: %w", err)
	}
	defer rows.Close()
	var out []NewsEventRow
	for rows.Next() {
		var e NewsEventRow
		if err := rows.Scan(&e.Symbol, &e.EventType, &e.Score, &e.Headline, &e.Original, &e.Date); err != nil {
			return nil, err
		}
		e.Symbol = strings.ToUpper(e.Symbol)
		out = append(out, e)
	}
	return out, rows.Err()
}

// NewsCountsBetween counts tagged articles per symbol in a window — the weak
// explanation for a price move that has no named event.
func (r *ChangeEventRepo) NewsCountsBetween(ctx context.Context, from, to time.Time) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT jt.sym, COUNT(*)
		  FROM news_articles n
		  JOIN JSON_TABLE(n.symbols_json, '$[*]' COLUMNS (sym VARCHAR(32) PATH '$')) jt
		 WHERE n.published_at >= ? AND n.published_at < ?
		 GROUP BY jt.sym`, from, to)
	if err != nil {
		return nil, fmt.Errorf("change news counts: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var sym string
		var n int
		if err := rows.Scan(&sym, &n); err != nil {
			return nil, err
		}
		out[strings.ToUpper(sym)] = n
	}
	return out, rows.Err()
}

// ChangeDisclosureRow is one DART filing (the feed's narrow view).
type ChangeDisclosureRow struct {
	Symbol    string
	ReceiptNo string
	Title     string
	Date      string
}

// DisclosuresBetween returns filings received in [from, to].
func (r *ChangeEventRepo) DisclosuresBetween(ctx context.Context, from, to string) ([]ChangeDisclosureRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, receipt_no, title, DATE_FORMAT(receipt_date, '%Y-%m-%d')
		  FROM stock_disclosures WHERE receipt_date BETWEEN ? AND ?`, from, to)
	if err != nil {
		return nil, fmt.Errorf("change disclosures: %w", err)
	}
	defer rows.Close()
	var out []ChangeDisclosureRow
	for rows.Next() {
		var d ChangeDisclosureRow
		if err := rows.Scan(&d.Symbol, &d.ReceiptNo, &d.Title, &d.Date); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// EarningsRow is one reported earnings event with whatever estimates exist.
type EarningsRow struct {
	Symbol         string
	Date           string
	EPSEst, EPSAct sql.NullFloat64
	OpEst, OpAct   sql.NullFloat64
	RevEst, RevAct sql.NullFloat64
}

// EarningsBetween returns events dated in [from, to] that carry an actual.
func (r *ChangeEventRepo) EarningsBetween(ctx context.Context, from, to string) ([]EarningsRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, DATE_FORMAT(event_date, '%Y-%m-%d'),
		       eps_est, eps_act, op_income_est, op_income_act, revenue_est, revenue_act
		  FROM earnings_events
		 WHERE event_date BETWEEN ? AND ?
		   AND (eps_act IS NOT NULL OR op_income_act IS NOT NULL OR revenue_act IS NOT NULL)`, from, to)
	if err != nil {
		return nil, fmt.Errorf("change earnings: %w", err)
	}
	defer rows.Close()
	var out []EarningsRow
	for rows.Next() {
		var e EarningsRow
		if err := rows.Scan(&e.Symbol, &e.Date, &e.EPSEst, &e.EPSAct, &e.OpEst, &e.OpAct, &e.RevEst, &e.RevAct); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// InsiderNetRow is one symbol's net insider shares over a short window.
type InsiderNetRow struct {
	Symbol string
	Net    float64
	Count  int
	Date   string
}

// InsiderNetBetween sums signed share changes per symbol in [from, to].
func (r *ChangeEventRepo) InsiderNetBetween(ctx context.Context, from, to string) ([]InsiderNetRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, SUM(shares_change), COUNT(*), DATE_FORMAT(MAX(txn_date), '%Y-%m-%d')
		  FROM insider_transactions
		 WHERE txn_date BETWEEN ? AND ? AND shares_change IS NOT NULL AND shares_change <> 0
		 GROUP BY symbol`, from, to)
	if err != nil {
		return nil, fmt.Errorf("change insider: %w", err)
	}
	defer rows.Close()
	var out []InsiderNetRow
	for rows.Next() {
		var i InsiderNetRow
		if err := rows.Scan(&i.Symbol, &i.Net, &i.Count, &i.Date); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// ChangeEvent is one stored feed row.
type ChangeEvent struct {
	DetectedFor string         `json:"detectedFor"`
	EventDate   string         `json:"eventDate"`
	Symbol      string         `json:"symbol"`
	Name        string         `json:"name,omitempty"`
	AssetType   string         `json:"assetType,omitempty"`
	Kind        string         `json:"kind"`
	Subtype     string         `json:"subtype,omitempty"`
	Direction   int            `json:"direction"`
	Magnitude   *float64       `json:"magnitude,omitempty"`
	Importance  float64        `json:"importance"`
	// MarketCap — 감지한 날 점수 행의 시가총액(원·달러). 목록이 이것의 내림차순이다(2026-10-09 요청 3).
	MarketCap   float64        `json:"marketCap,omitempty"`
	Title      string         `json:"title"`
	Explained   *bool          `json:"explained,omitempty"`
	Detail      map[string]any `json:"detail,omitempty"`
}

/*
SeenBefore — 이전 회차들이 이미 올린 (종목·종류·세부·사건일).

점수 배치는 휴장일에도 돈다. 금요일 미국 세션의 급등이 토·일·월 회차마다 다시 오르면
「변화」 목록이 같은 줄로 채워진다. 같은 사건일의 같은 일은 한 번만 올린다.
*/
func (r *ChangeEventRepo) SeenBefore(ctx context.Context, tradeDate string) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, kind, subtype, DATE_FORMAT(event_date, '%Y-%m-%d')
		  FROM stock_change_events
		 WHERE detected_for < ? AND detected_for >= DATE_SUB(?, INTERVAL 10 DAY)`, tradeDate, tradeDate)
	if err != nil {
		return nil, fmt.Errorf("change seen: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var s, k, st, d string
		if err := rows.Scan(&s, &k, &st, &d); err != nil {
			return nil, err
		}
		out[ChangeEventKey(s, k, st, d)] = true
	}
	return out, rows.Err()
}

// ChangeEventKey identifies "the same happening" across runs.
func ChangeEventKey(symbol, kind, subtype, eventDate string) string {
	return symbol + "|" + kind + "|" + subtype + "|" + eventDate
}

// ReplaceDay swaps one detection day's rows atomically.
func (r *ChangeEventRepo) ReplaceDay(ctx context.Context, tradeDate string, events []ChangeEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM stock_change_events WHERE detected_for = ?`, tradeDate); err != nil {
		return fmt.Errorf("change replace delete: %w", err)
	}
	const chunk = 200
	for start := 0; start < len(events); start += chunk {
		part := events[start:min(start+chunk, len(events))]
		marks := make([]string, 0, len(part))
		args := make([]any, 0, len(part)*11)
		for _, e := range part {
			marks = append(marks, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			var explained, detail any
			if e.Explained != nil {
				explained = *e.Explained
			}
			if len(e.Detail) > 0 {
				b, _ := json.Marshal(e.Detail)
				detail = string(b)
			}
			var mag any
			if e.Magnitude != nil {
				mag = *e.Magnitude
			}
			var mcap any
			if e.MarketCap > 0 {
				mcap = e.MarketCap
			}
			args = append(args, tradeDate, e.EventDate, e.Symbol, e.Kind, e.Subtype, e.Direction,
				mag, e.Importance, mcap, truncateRunes(e.Title, 250), explained, detail)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO stock_change_events
			  (detected_for, event_date, symbol, kind, subtype, direction, magnitude, importance, market_cap, title, explained, detail)
			VALUES `+strings.Join(marks, ","), args...); err != nil {
			return fmt.Errorf("change replace insert: %w", err)
		}
	}
	return tx.Commit()
}

// PruneBefore drops rows older than the retention window.
func (r *ChangeEventRepo) PruneBefore(ctx context.Context, keepDays int) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM stock_change_events WHERE detected_for < DATE_SUB(CURDATE(), INTERVAL ? DAY)`, keepDays)
	return err
}

// ChangeListQuery filters the feed.
type ChangeListQuery struct {
	Date      string   // "" → latest
	AssetType string   // "" | domestic | us
	Kinds     []string // empty → all
	Symbols   []string // non-empty → only these (내 종목)
	Limit     int
}

// Dates returns the most recent detection days, newest first.
func (r *ChangeEventRepo) Dates(ctx context.Context, n int) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(detected_for, '%Y-%m-%d') FROM stock_change_events
		 GROUP BY detected_for ORDER BY detected_for DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// List returns one day's events joined with stock names, most important first.
// scopeWhere is the date · market · 내 종목 part shared by List and KindCounts.
func (q ChangeListQuery) scopeWhere() ([]string, []any) {
	where := []string{"e.detected_for = ?"}
	args := []any{q.Date}
	if q.AssetType != "" {
		where = append(where, "st.asset_type = ?")
		args = append(args, q.AssetType)
	}
	if len(q.Symbols) > 0 {
		in, symArgs := symbolPlaceholders(q.Symbols)
		where = append(where, "e.symbol IN ("+in+")")
		args = append(args, symArgs...)
	}
	return where, args
}

/*
List — 종류를 여러 개 고르면 **그 종류를 모두 가진 종목**만 남긴다(AND), 줄은 고른 종류만.

순서는 **시가총액 내림차순**(2026-10-09 요청 3 — 화면이 순위 번호를 단다). 같은 종목의 줄은 이어서 오고,
그 안에서는 중요도 순이다. 시가총액이 없는 행(이관 전·점수 행 없음)은 맨 뒤.

2026-09-28 사용자 제보: 「공시」+「뉴스 사건」을 함께 누르면 뉴스 사건 종목이 전부 떴다
(예전엔 `kind IN (…)` — OR). 칩은 좁히는 도구로 읽힌다.
*/
func (r *ChangeEventRepo) List(ctx context.Context, q ChangeListQuery) ([]ChangeEvent, error) {
	where, args := q.scopeWhere()
	if len(q.Kinds) > 0 {
		in := strings.TrimSuffix(strings.Repeat("?,", len(q.Kinds)), ",")
		where = append(where, "e.kind IN ("+in+")")
		for _, k := range q.Kinds {
			args = append(args, k)
		}
		if len(q.Kinds) > 1 {
			where = append(where, `e.symbol IN (
				SELECT k.symbol FROM stock_change_events k
				 WHERE k.detected_for = ? AND k.kind IN (`+in+`)
				 GROUP BY k.symbol HAVING COUNT(DISTINCT k.kind) = ?)`)
			args = append(args, q.Date)
			for _, k := range q.Kinds {
				args = append(args, k)
			}
			args = append(args, len(q.Kinds))
		}
	}
	// limit은 **종목 수**다(2026-10-09). 예전엔 줄 300개에서 잘라 마지막 종목의 줄 일부만 오는 일이 있었다.
	// 하루 한 시장은 수백 줄이라 줄은 상한(5천)만 두고 다 읽는다.
	limit := q.Limit
	if limit <= 0 || limit > 1000 {
		limit = 300
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(e.detected_for, '%Y-%m-%d'), DATE_FORMAT(e.event_date, '%Y-%m-%d'),
		       e.symbol, COALESCE(st.name, ''), COALESCE(st.asset_type, ''),
		       e.kind, e.subtype, e.direction, e.magnitude, e.importance, COALESCE(e.market_cap, 0),
		       e.title, e.explained, e.detail
		  FROM stock_change_events e
		  LEFT JOIN stocks st ON st.symbol = e.symbol
		 WHERE `+strings.Join(where, " AND ")+`
		 ORDER BY e.market_cap IS NULL, e.market_cap DESC, e.symbol, e.importance DESC
		 LIMIT 5000`, args...)
	if err != nil {
		return nil, fmt.Errorf("change list: %w", err)
	}
	defer rows.Close()
	var out []ChangeEvent
	symbols := 0 // 지금까지 끝난 종목 수
	for rows.Next() {
		var (
			e         ChangeEvent
			mag       sql.NullFloat64
			explained sql.NullInt64
			detail    sql.NullString
		)
		if err := rows.Scan(&e.DetectedFor, &e.EventDate, &e.Symbol, &e.Name, &e.AssetType,
			&e.Kind, &e.Subtype, &e.Direction, &mag, &e.Importance, &e.MarketCap, &e.Title, &explained, &detail); err != nil {
			return nil, err
		}
		if len(out) > 0 && out[len(out)-1].Symbol != e.Symbol {
			if symbols++; symbols >= limit {
				break
			}
		}
		if mag.Valid {
			v := mag.Float64
			e.Magnitude = &v
		}
		if explained.Valid {
			b := explained.Int64 == 1
			e.Explained = &b
		}
		if detail.Valid && detail.String != "" {
			_ = json.Unmarshal([]byte(detail.String), &e.Detail)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// KindCounts — 종류별 종목 수(같은 날·시장·내 종목 범위). 칩이 0이면 누를 것도 없다.
func (r *ChangeEventRepo) KindCounts(ctx context.Context, q ChangeListQuery) (map[string]int, error) {
	where, args := q.scopeWhere()
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.kind, COUNT(DISTINCT e.symbol)
		  FROM stock_change_events e
		  LEFT JOIN stocks st ON st.symbol = e.symbol
		 WHERE `+strings.Join(where, " AND ")+`
		 GROUP BY e.kind`, args...)
	if err != nil {
		return nil, fmt.Errorf("change kind counts: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, rows.Err()
}

// TrackedSymbols returns one user's watchlist ∪ holdings.
func (r *ChangeEventRepo) TrackedSymbols(ctx context.Context, userID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol FROM watchlist_items WHERE user_id = ?
		UNION
		SELECT symbol FROM user_holdings WHERE user_id = ?`, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// MigrateChangeEventsIfNeeded creates the table on an existing database.
func MigrateChangeEventsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "stock_change_events")
	if err != nil {
		return err
	}
	if !exists {
		if err := applyMigrationFile(ctx, db, migrationsDir,
			"080_stock_change_events.sql", "종목 변화 이벤트"); err != nil {
			return err
		}
	}
	// 108 — magnitude DECIMAL(12,4) → (20,4). 내부자 순매매 1억 주가 그날 배치를 통째로 깼다.
	var precision int
	if err := db.QueryRowContext(ctx, `
		SELECT NUMERIC_PRECISION FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = 'stock_change_events' AND column_name = 'magnitude'`,
	).Scan(&precision); err != nil || precision >= 20 {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "108_change_magnitude_wide.sql", "변화 이벤트 magnitude 폭")
}

// ChangeSocialRow — 레딧 언급 스냅샷 + 그날 점수 행의 추세 재료(과열 판정용).
type ChangeSocialRow struct {
	Symbol         string
	Date           string
	Mentions, Prev int
	Rank           int
	RankPrev       *int
	Return5d, RSI  *float64
}

/*
SocialBetween — 종목마다 [from, to] 안의 가장 최근 레딧 스냅샷. 미국 장은 KST 새벽에 닫히고
스냅샷은 그 뒤(00:05)에 찍히므로, 거래일 D의 감지는 D 이후 며칠까지 본다.
*/
func (r *ChangeEventRepo) SocialBetween(ctx context.Context, from, to, tradeDate string) ([]ChangeSocialRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.symbol, DATE_FORMAT(m.snap_date, '%Y-%m-%d'), m.mentions, m.mentions_prev, m.rank_now, m.rank_prev,
		       CAST(JSON_EXTRACT(sc.inputs, '$.technical.return5d') AS DECIMAL(12,4)),
		       CAST(JSON_EXTRACT(sc.inputs, '$.technical.rsi14') AS DECIMAL(12,4))
		  FROM social_mentions m
		  JOIN (SELECT symbol, MAX(snap_date) d FROM social_mentions
		         WHERE source = 'apewisdom' AND snap_date BETWEEN ? AND ? GROUP BY symbol) x
		    ON x.symbol = m.symbol AND x.d = m.snap_date
		  LEFT JOIN stock_scores sc ON sc.symbol = m.symbol AND sc.trade_date = ?
		 WHERE m.source = 'apewisdom'`, from, to, tradeDate)
	if err != nil {
		return nil, fmt.Errorf("change social: %w", err)
	}
	defer rows.Close()
	var out []ChangeSocialRow
	for rows.Next() {
		var (
			s        ChangeSocialRow
			rp       sql.NullInt64
			ret, rsi sql.NullFloat64
		)
		if err := rows.Scan(&s.Symbol, &s.Date, &s.Mentions, &s.Prev, &s.Rank, &rp, &ret, &rsi); err != nil {
			return nil, err
		}
		if rp.Valid {
			v := int(rp.Int64)
			s.RankPrev = &v
		}
		if ret.Valid {
			v := ret.Float64
			s.Return5d = &v
		}
		if rsi.Valid {
			v := rsi.Float64
			s.RSI = &v
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
