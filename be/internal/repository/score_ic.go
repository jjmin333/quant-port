package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

/*
AI 점수 성적표(IC). 표를 왜 두는지는 `migrations/081_score_ic_daily.sql`.
*/

// ScoreICRepo owns score_ic_daily and its source query.
type ScoreICRepo struct {
	db *sql.DB
}

func NewScoreICRepo(db *sql.DB) *ScoreICRepo { return &ScoreICRepo{db: db} }

// ScoreForwardRow is one symbol's stored axes on a date plus its forward return.
type ScoreForwardRow struct {
	Symbol string
	Score  sql.NullFloat64
	Axes   map[string]sql.NullFloat64
	Return float64 // %, from the latest close ≤ date to the horizon-th session after date
}

/*
ForwardRows — 점수일 하나 · 시장 하나의 (축 점수, 앞으로 h거래일 수익률).

랭킹 자격(축 3개 이상 · 지수 추종 상품 제외)을 그대로 쓴다 — 화면의 랭킹과 다른 우주로
재면 「랭킹이 맞았나」에 답하지 못한다. 기준가는 D 이하의 마지막 종가, 끝값은 D **다음**
세션부터 h번째 종가다. 끝값이 아직 없는 종목(최근 날짜)은 빠진다.
*/
func (r *ScoreICRepo) ForwardRows(ctx context.Context, tradeDate, assetType string, horizon int) ([]ScoreForwardRow, error) {
	productClause, productArgs := scoreRankableProductClause("st")
	args := []any{tradeDate, tradeDate, horizon - 1, tradeDate, assetType}
	args = append(args, productArgs...)
	rows, err := r.db.QueryContext(ctx, `
		SELECT x.symbol, x.score, x.technical, x.quality, x.growth, x.valuation, x.sentiment, x.base, x.fwd
		  FROM (
		    SELECT h.symbol, h.score, h.technical, h.quality, h.growth, h.valuation, h.sentiment,
		           (SELECT c.close_price FROM stock_daily_closes c
		             WHERE c.symbol = h.symbol AND c.trade_date <= ?
		             ORDER BY c.trade_date DESC LIMIT 1) AS base,
		           (SELECT c.close_price FROM stock_daily_closes c
		             WHERE c.symbol = h.symbol AND c.trade_date > ?
		             ORDER BY c.trade_date ASC LIMIT 1 OFFSET ?) AS fwd
		      FROM stock_score_history h
		      JOIN stocks st ON st.symbol = h.symbol
		     WHERE h.trade_date = ? AND st.asset_type = ?`+productClause+`
		       AND ((h.technical IS NOT NULL) + (h.quality IS NOT NULL) + (h.growth IS NOT NULL)
		            + (h.valuation IS NOT NULL) + (h.sentiment IS NOT NULL)) >= 3
		  ) x
		 WHERE x.base > 0 AND x.fwd > 0`, args...)
	if err != nil {
		return nil, fmt.Errorf("score ic rows %s %s: %w", tradeDate, assetType, err)
	}
	defer rows.Close()
	var out []ScoreForwardRow
	for rows.Next() {
		var (
			f                        ScoreForwardRow
			tech, qual, grow, val, s sql.NullFloat64
			base, fwd                float64
		)
		if err := rows.Scan(&f.Symbol, &f.Score, &tech, &qual, &grow, &val, &s, &base, &fwd); err != nil {
			return nil, err
		}
		f.Axes = map[string]sql.NullFloat64{
			"technical": tech, "quality": qual, "growth": grow, "valuation": val, "sentiment": s,
		}
		f.Return = (fwd/base - 1) * 100
		out = append(out, f)
	}
	return out, rows.Err()
}

// ScoreICRow is one stored (date, market, axis, horizon) measurement.
type ScoreICRow struct {
	TradeDate string
	Market    string
	Axis      string
	Horizon   int
	IC        sql.NullFloat64
	TopExcess sql.NullFloat64
	TopHit    sql.NullFloat64
	N         int
}

// Upsert writes one date's rows.
func (r *ScoreICRepo) Upsert(ctx context.Context, rows []ScoreICRow) error {
	if len(rows) == 0 {
		return nil
	}
	marks := make([]string, 0, len(rows))
	args := make([]any, 0, len(rows)*8)
	for _, x := range rows {
		marks = append(marks, "(?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args, x.TradeDate, x.Market, x.Axis, x.Horizon, x.IC, x.TopExcess, x.TopHit, x.N)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO score_ic_daily (trade_date, market, axis, horizon, ic, top_excess, top_hit, n)
		VALUES `+strings.Join(marks, ",")+`
		ON DUPLICATE KEY UPDATE ic = VALUES(ic), top_excess = VALUES(top_excess),
		  top_hit = VALUES(top_hit), n = VALUES(n)`, args...)
	if err != nil {
		return fmt.Errorf("score ic upsert: %w", err)
	}
	return nil
}

// MeasuredDates returns dates already stored for a horizon.
func (r *ScoreICRepo) MeasuredDates(ctx context.Context, horizon int) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT DATE_FORMAT(trade_date, '%Y-%m-%d') FROM score_ic_daily WHERE horizon = ?`, horizon)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out[d] = true
	}
	return out, rows.Err()
}

// HistoryDates returns every date in stock_score_history, oldest first.
func (r *ScoreICRepo) HistoryDates(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(trade_date, '%Y-%m-%d') FROM stock_score_history
		 GROUP BY trade_date ORDER BY trade_date`)
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

// List returns one market/horizon's rows, oldest first.
func (r *ScoreICRepo) List(ctx context.Context, market string, horizon int) ([]ScoreICRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(trade_date, '%Y-%m-%d'), market, axis, horizon, ic, top_excess, top_hit, n
		  FROM score_ic_daily WHERE market = ? AND horizon = ?
		 ORDER BY trade_date, axis`, market, horizon)
	if err != nil {
		return nil, fmt.Errorf("score ic list: %w", err)
	}
	defer rows.Close()
	var out []ScoreICRow
	for rows.Next() {
		var x ScoreICRow
		if err := rows.Scan(&x.TradeDate, &x.Market, &x.Axis, &x.Horizon, &x.IC, &x.TopExcess, &x.TopHit, &x.N); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// MigrateScoreICIfNeeded creates the table on an existing database.
func MigrateScoreICIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "score_ic_daily")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir, "081_score_ic_daily.sql", "AI 점수 성적표(IC)")
}
