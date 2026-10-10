package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

/*
실적 「관전 포인트」 저장소 (2026-09-27 사용자 결정 C ② — migrations/089_earnings_previews.sql).

사실표(facts)는 **숫자와 기사 제목만** 담는다 — 모델이 지어낼 자리를 줄이고, 입력 토큰을
콜당 1~1.5K로 묶는다(기사 본문을 넣으면 한 종목이 1만 토큰이 된다).
*/

// EarningsPreviewTarget is one upcoming report in the window.
type EarningsPreviewTarget struct {
	Symbol       string
	Name         string
	Market       string
	EventDate    string
	DateStatus   string
	Session      string
	FiscalYear   int
	FiscalPeriod string
	Currency     string
	EPSEst       *float64
	RevenueEst   *float64
	OpIncomeEst  *float64
	Held         bool
	Watched      bool
	MarketCap    float64
}

// FiscalLabel is 「2026 3Q」 style.
func (t EarningsPreviewTarget) FiscalLabel() string {
	p := strings.TrimSpace(t.FiscalPeriod)
	if strings.HasPrefix(p, "Q") && len(p) == 2 {
		p = p[1:] + "Q"
	}
	return fmt.Sprintf("%d %s", t.FiscalYear, p)
}

type EarningsPreviewRepo struct{ db *sql.DB }

func NewEarningsPreviewRepo(db *sql.DB) *EarningsPreviewRepo { return &EarningsPreviewRepo{db: db} }

// Targets lists reports between from and to (inclusive, YYYY-MM-DD) for one market.
func (r *EarningsPreviewRepo) Targets(ctx context.Context, market, from, to string) ([]EarningsPreviewTarget, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.symbol, COALESCE(s.name, e.symbol), e.market, DATE_FORMAT(e.event_date, '%Y-%m-%d'),
		       e.date_status, COALESCE(e.session, ''), e.fiscal_year, e.fiscal_period, e.currency,
		       e.eps_est, e.revenue_est, e.op_income_est,
		       EXISTS(SELECT 1 FROM user_holdings h WHERE h.symbol = e.symbol AND h.quantity > 0),
		       EXISTS(SELECT 1 FROM watchlist_items w WHERE w.symbol = e.symbol),
		       COALESCE((SELECT CAST(JSON_EXTRACT(sc.inputs, '$.marketCap') AS DOUBLE) FROM stock_scores sc
		                  WHERE sc.symbol = e.symbol ORDER BY sc.trade_date DESC LIMIT 1), 0)
		  FROM earnings_events e
		  LEFT JOIN stocks s ON s.symbol = e.symbol
		 WHERE e.event_date BETWEEN ? AND ? AND e.market = ?
		 ORDER BY e.event_date, e.symbol`, from, to, market)
	if err != nil {
		return nil, fmt.Errorf("earnings preview targets: %w", err)
	}
	defer rows.Close()
	seen := map[string]bool{}
	var out []EarningsPreviewTarget
	for rows.Next() {
		var t EarningsPreviewTarget
		var eps, rev, op sql.NullFloat64
		var held, watched int
		if err := rows.Scan(&t.Symbol, &t.Name, &t.Market, &t.EventDate, &t.DateStatus, &t.Session,
			&t.FiscalYear, &t.FiscalPeriod, &t.Currency, &eps, &rev, &op, &held, &watched, &t.MarketCap); err != nil {
			return nil, err
		}
		// 같은 종목의 두 행(기준이 다른 연결·별도)은 첫 행만 — 발표는 하나다.
		if seen[t.Symbol] {
			continue
		}
		seen[t.Symbol] = true
		t.EPSEst, t.RevenueEst, t.OpIncomeEst = nullF(eps), nullF(rev), nullF(op)
		t.Held, t.Watched = held == 1, watched == 1
		out = append(out, t)
	}
	return out, rows.Err()
}

func nullF(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	x := v.Float64
	return &x
}

// EarningsPreviewFacts is exactly what the model sees (and what the rule text uses).
type EarningsPreviewFacts struct {
	Name        string             `json:"name"`
	Symbol      string             `json:"symbol"`
	Market      string             `json:"market"`
	EventDate   string             `json:"eventDate"`
	DaysToEvent int                `json:"daysToEvent"`
	Fiscal      string             `json:"fiscal"`
	DateStatus  string             `json:"dateStatus"`
	Session     string             `json:"session,omitempty"`
	Currency    string             `json:"currency"`
	Estimates   map[string]float64 `json:"estimates,omitempty"`
	Score       *int               `json:"aiScore,omitempty"`
	ScoreDelta  *int               `json:"aiScoreChange,omitempty"`
	Axes        map[string]float64 `json:"axes,omitempty"`
	Surprises   []PreviewSurprise  `json:"pastSurprises,omitempty"`
	EPSRevision *PreviewRevision   `json:"epsEstimateRevision30d,omitempty"`
	Return20d   *float64           `json:"return20dPct,omitempty"`
	Pos52w      *float64           `json:"position52wPct,omitempty"`
	Headlines   []string           `json:"recentHeadlines,omitempty"`
}

type PreviewSurprise struct {
	Fiscal string  `json:"fiscal"`
	Metric string  `json:"metric"`
	Pct    float64 `json:"surprisePct"`
}

type PreviewRevision struct {
	From float64 `json:"from"`
	To   float64 `json:"to"`
	Pct  float64 `json:"pct"`
}

// Facts builds the fact sheet. Every section is optional — a missing table row
// leaves the field empty instead of failing the symbol.
func (r *EarningsPreviewRepo) Facts(ctx context.Context, t EarningsPreviewTarget, today time.Time) (*EarningsPreviewFacts, error) {
	f := &EarningsPreviewFacts{
		Name: t.Name, Symbol: t.Symbol, Market: t.Market, EventDate: t.EventDate, Fiscal: t.FiscalLabel(),
		DateStatus: t.DateStatus, Session: t.Session, Currency: t.Currency,
	}
	if d, err := time.ParseInLocation("2006-01-02", t.EventDate, today.Location()); err == nil {
		y, m, dd := today.Date()
		f.DaysToEvent = int(d.Sub(time.Date(y, m, dd, 0, 0, 0, 0, today.Location())).Hours() / 24)
	}
	est := map[string]float64{}
	if t.EPSEst != nil {
		est["eps"] = *t.EPSEst
	}
	if t.RevenueEst != nil {
		est["revenue"] = *t.RevenueEst
	}
	if t.OpIncomeEst != nil {
		est["operatingIncome"] = *t.OpIncomeEst
	}
	if len(est) > 0 {
		f.Estimates = est
	}

	// 점수 — 최근 두 점수일.
	srows, err := r.db.QueryContext(ctx, `
		SELECT score, breakdown FROM stock_scores WHERE symbol = ? ORDER BY trade_date DESC LIMIT 2`, t.Symbol)
	if err != nil {
		return nil, fmt.Errorf("preview score: %w", err)
	}
	var scores []sql.NullInt64
	var firstBreakdown []byte
	for srows.Next() {
		var sc sql.NullInt64
		var bd []byte
		if err := srows.Scan(&sc, &bd); err != nil {
			srows.Close()
			return nil, err
		}
		if firstBreakdown == nil {
			firstBreakdown = bd
		}
		scores = append(scores, sc)
	}
	srows.Close()
	if len(scores) > 0 && scores[0].Valid {
		v := int(scores[0].Int64)
		f.Score = &v
		if len(scores) > 1 && scores[1].Valid {
			d := v - int(scores[1].Int64)
			f.ScoreDelta = &d
		}
		f.Axes = numericTopLevel(firstBreakdown)
	}

	// 지난 발표의 서프라이즈 — 추정·실적이 둘 다 있는 최근 네 번.
	prow, err := r.db.QueryContext(ctx, `
		SELECT fiscal_year, fiscal_period, eps_est, eps_act, op_income_est, op_income_act
		  FROM earnings_events
		 WHERE symbol = ? AND event_date < ? AND
		       ((eps_est IS NOT NULL AND eps_act IS NOT NULL) OR (op_income_est IS NOT NULL AND op_income_act IS NOT NULL))
		 ORDER BY event_date DESC LIMIT 4`, t.Symbol, t.EventDate)
	if err != nil {
		return nil, fmt.Errorf("preview surprises: %w", err)
	}
	for prow.Next() {
		var fy int
		var fp string
		var epsE, epsA, opE, opA sql.NullFloat64
		if err := prow.Scan(&fy, &fp, &epsE, &epsA, &opE, &opA); err != nil {
			prow.Close()
			return nil, err
		}
		label := EarningsPreviewTarget{FiscalYear: fy, FiscalPeriod: fp}.FiscalLabel()
		if t.Market == "domestic" && opE.Valid && opA.Valid && opE.Float64 != 0 {
			f.Surprises = append(f.Surprises, PreviewSurprise{Fiscal: label, Metric: "operatingIncome", Pct: round1((opA.Float64 - opE.Float64) / math.Abs(opE.Float64) * 100)})
		} else if epsE.Valid && epsA.Valid && epsE.Float64 != 0 {
			f.Surprises = append(f.Surprises, PreviewSurprise{Fiscal: label, Metric: "eps", Pct: round1((epsA.Float64 - epsE.Float64) / math.Abs(epsE.Float64) * 100)})
		}
	}
	prow.Close()

	// EPS 추정치 30일 변화(해외).
	var nowEst, beforeEst float64
	var nowDate string
	err = r.db.QueryRowContext(ctx, `
		SELECT DATE_FORMAT(snap_date, '%Y-%m-%d'), eps_est FROM earnings_estimate_revisions
		 WHERE symbol = ? AND fiscal_year = ? AND fiscal_period = ? ORDER BY snap_date DESC LIMIT 1`,
		t.Symbol, t.FiscalYear, t.FiscalPeriod).Scan(&nowDate, &nowEst)
	if err == nil {
		if err := r.db.QueryRowContext(ctx, `
			SELECT eps_est FROM earnings_estimate_revisions
			 WHERE symbol = ? AND fiscal_year = ? AND fiscal_period = ? AND snap_date <= DATE_SUB(?, INTERVAL 25 DAY)
			 ORDER BY snap_date DESC LIMIT 1`, t.Symbol, t.FiscalYear, t.FiscalPeriod, nowDate).Scan(&beforeEst); err == nil && beforeEst != 0 {
			f.EPSRevision = &PreviewRevision{From: beforeEst, To: nowEst, Pct: round1((nowEst - beforeEst) / math.Abs(beforeEst) * 100)}
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("preview revision: %w", err)
	}

	// 가격 맥락 — 최근 260봉.
	crow, err := r.db.QueryContext(ctx, `
		SELECT close_price FROM stock_daily_closes WHERE symbol = ? ORDER BY trade_date DESC LIMIT 260`, t.Symbol)
	if err != nil {
		return nil, fmt.Errorf("preview closes: %w", err)
	}
	var closes []float64
	for crow.Next() {
		var c float64
		if err := crow.Scan(&c); err != nil {
			crow.Close()
			return nil, err
		}
		closes = append(closes, c)
	}
	crow.Close()
	if ret, pos, ok := PriceContext(closes); ok {
		f.Return20d, f.Pos52w = &ret, &pos
	}

	// 최근 14일 기사 제목 여섯.
	newsMarket := "domestic"
	if t.Market != "domestic" {
		newsMarket = "us"
	}
	nrow, err := r.db.QueryContext(ctx, `
		SELECT COALESCE(NULLIF(headline_ko, ''), headline)
		  FROM news_articles FORCE INDEX (idx_news_market_published)
		 WHERE market = ? AND published_at >= ? AND JSON_CONTAINS(symbols_json, JSON_QUOTE(?))
		 ORDER BY published_at DESC LIMIT 6`, newsMarket, today.AddDate(0, 0, -14), t.Symbol)
	if err != nil {
		return nil, fmt.Errorf("preview news: %w", err)
	}
	for nrow.Next() {
		var h string
		if err := nrow.Scan(&h); err != nil {
			nrow.Close()
			return nil, err
		}
		f.Headlines = append(f.Headlines, clipRunes(h, 90))
	}
	nrow.Close()
	return f, nil
}

// PriceContext returns the 20-session return and the 52-week position (0~100)
// from closes ordered newest first.
func PriceContext(closes []float64) (ret20, pos52 float64, ok bool) {
	if len(closes) < 21 || closes[20] == 0 {
		return 0, 0, false
	}
	ret20 = round1((closes[0] - closes[20]) / closes[20] * 100)
	n := len(closes)
	if n > 252 {
		n = 252
	}
	lo, hi := closes[0], closes[0]
	for _, c := range closes[:n] {
		lo = math.Min(lo, c)
		hi = math.Max(hi, c)
	}
	if hi > lo {
		pos52 = round1((closes[0] - lo) / (hi - lo) * 100)
	} else {
		pos52 = 50
	}
	return ret20, pos52, true
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func numericTopLevel(raw []byte) map[string]float64 {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	out := map[string]float64{}
	for k, v := range m {
		if f, ok := v.(float64); ok {
			out[k] = round1(f)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// EarningsPreviewRow is one stored preview.
type EarningsPreviewRow struct {
	Symbol      string          `json:"symbol"`
	Name        string          `json:"name"`
	Market      string          `json:"market"`
	EventDate   string          `json:"eventDate"`
	FiscalLabel string          `json:"fiscalLabel"`
	Body        json.RawMessage `json:"body"`
	Source      string          `json:"source"`
	GeneratedAt time.Time       `json:"generatedAt"`
	Score       *int            `json:"score,omitempty"`
	InputHash   string          `json:"-"`
}

// Existing returns the stored hash and time for (symbol, eventDate).
func (r *EarningsPreviewRepo) Existing(ctx context.Context, symbol, eventDate string) (hash, source string, at time.Time, ok bool, err error) {
	err = r.db.QueryRowContext(ctx, `
		SELECT input_hash, source, generated_at FROM earnings_previews WHERE symbol = ? AND event_date = ?`,
		symbol, eventDate).Scan(&hash, &source, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", time.Time{}, false, nil
	}
	return hash, source, at, err == nil, err
}

type EarningsPreviewUpsert struct {
	Symbol, EventDate, Market, FiscalLabel string
	Body, Facts                            []byte
	Source, Model, InputHash               string
	Tokens                                 int64
}

func (r *EarningsPreviewRepo) Upsert(ctx context.Context, u EarningsPreviewUpsert) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO earnings_previews (symbol, event_date, market, fiscal_label, body, facts, source, model, input_hash, tokens, generated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP(3))
		ON DUPLICATE KEY UPDATE market = VALUES(market), fiscal_label = VALUES(fiscal_label), body = VALUES(body),
		  facts = VALUES(facts), source = VALUES(source), model = VALUES(model), input_hash = VALUES(input_hash),
		  tokens = VALUES(tokens), generated_at = CURRENT_TIMESTAMP(3)`,
		u.Symbol, u.EventDate, u.Market, u.FiscalLabel, string(u.Body), string(u.Facts), u.Source, u.Model, u.InputHash, u.Tokens)
	return err
}

// List returns stored previews for reports between from and to (inclusive).
func (r *EarningsPreviewRepo) List(ctx context.Context, market, from, to string) ([]EarningsPreviewRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.symbol, COALESCE(s.name, p.symbol), p.market, DATE_FORMAT(p.event_date, '%Y-%m-%d'),
		       p.fiscal_label, p.body, p.source, p.generated_at,
		       (SELECT sc.score FROM stock_scores sc WHERE sc.symbol = p.symbol ORDER BY sc.trade_date DESC LIMIT 1)
		  FROM earnings_previews p
		  LEFT JOIN stocks s ON s.symbol = p.symbol
		 WHERE p.market = ? AND p.event_date BETWEEN ? AND ?
		 ORDER BY p.event_date, p.symbol`, market, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EarningsPreviewRow{}
	for rows.Next() {
		var row EarningsPreviewRow
		var body []byte
		var score sql.NullInt64
		if err := rows.Scan(&row.Symbol, &row.Name, &row.Market, &row.EventDate, &row.FiscalLabel, &body, &row.Source, &row.GeneratedAt, &score); err != nil {
			return nil, err
		}
		row.Body = json.RawMessage(body)
		if score.Valid {
			v := int(score.Int64)
			row.Score = &v
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// MigrateEarningsPreviewsIfNeeded creates earnings_previews on an existing database.
func MigrateEarningsPreviewsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "earnings_previews")
	if err != nil || exists {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "089_earnings_previews.sql", "실적 관전 포인트")
}

// TargetMeta fills name · 보유/관심 여부 · 시가총액 for symbols that came from the
// domestic *estimate* (작년 발표일 + 1년) rather than a stored event row.
func (r *EarningsPreviewRepo) TargetMeta(ctx context.Context, symbols []string) (map[string]EarningsPreviewTarget, error) {
	out := map[string]EarningsPreviewTarget{}
	if len(symbols) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(symbols))
	for _, s := range symbols {
		args = append(args, s)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.symbol, s.name,
		       EXISTS(SELECT 1 FROM user_holdings h WHERE h.symbol = s.symbol AND h.quantity > 0),
		       EXISTS(SELECT 1 FROM watchlist_items w WHERE w.symbol = s.symbol),
		       COALESCE((SELECT CAST(JSON_EXTRACT(sc.inputs, '$.marketCap') AS DOUBLE) FROM stock_scores sc
		                  WHERE sc.symbol = s.symbol ORDER BY sc.trade_date DESC LIMIT 1), 0)
		  FROM stocks s WHERE s.symbol IN (`+placeholders(len(symbols))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t EarningsPreviewTarget
		var held, watched int
		if err := rows.Scan(&t.Symbol, &t.Name, &held, &watched, &t.MarketCap); err != nil {
			return nil, err
		}
		t.Held, t.Watched = held == 1, watched == 1
		out[t.Symbol] = t
	}
	return out, rows.Err()
}
