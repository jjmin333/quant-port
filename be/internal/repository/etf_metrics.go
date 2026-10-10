package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/enact/quant-port/be/internal/marketdata"
)

/*
국내 ETF 점검 — 일별 지표와 점수. 표를 왜 두는지는 `migrations/086_etf_metrics.sql`.
*/

// ETFMetricsRepo owns etf_daily_metrics and etf_scores.
type ETFMetricsRepo struct {
	db *sql.DB
}

func NewETFMetricsRepo(db *sql.DB) *ETFMetricsRepo { return &ETFMetricsRepo{db: db} }

// UpsertDaily writes one day's rows (KRX gives every ETF in one call).
func (r *ETFMetricsRepo) UpsertDaily(ctx context.Context, date string, rows []marketdata.ETFDailyRow) (int, error) {
	const chunk = 400
	written := 0
	for start := 0; start < len(rows); start += chunk {
		end := min(start+chunk, len(rows))
		var b strings.Builder
		b.WriteString(`INSERT INTO etf_daily_metrics
			(symbol, trade_date, close_price, nav, net_assets, trade_value, index_name, index_value, index_change_rate) VALUES `)
		args := make([]any, 0, (end-start)*9)
		for i, row := range rows[start:end] {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString("(?,?,?,?,?,?,?,?,?)")
			name := row.IndexName
			if len([]rune(name)) > 120 {
				name = string([]rune(name)[:120])
			}
			args = append(args, row.Symbol, date, row.Close, row.NAV, row.NetAssets, row.TradeValue,
				name, row.IndexValue, row.IndexChangeRate)
		}
		b.WriteString(` ON DUPLICATE KEY UPDATE close_price = VALUES(close_price), nav = VALUES(nav),
			net_assets = VALUES(net_assets), trade_value = VALUES(trade_value), index_name = VALUES(index_name),
			index_value = VALUES(index_value), index_change_rate = VALUES(index_change_rate)`)
		if _, err := r.db.ExecContext(ctx, b.String(), args...); err != nil {
			return written, fmt.Errorf("etf metrics upsert %s: %w", date, err)
		}
		written += end - start
	}
	return written, nil
}

// LatestDate is the newest stored metrics day ("" when empty).
func (r *ETFMetricsRepo) LatestDate(ctx context.Context) (string, error) {
	var d sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT DATE_FORMAT(MAX(trade_date), '%Y-%m-%d') FROM etf_daily_metrics`).Scan(&d)
	return d.String, err
}

// ETFPoint is one stored day for one ETF.
type ETFPoint struct {
	Date            string
	Close           float64
	NAV             sql.NullFloat64
	NetAssets       sql.NullInt64
	TradeValue      sql.NullInt64
	IndexName       string
	IndexChangeRate sql.NullFloat64
}

// Window returns symbol → points (oldest first) over the last `sessions` stored days up to date.
func (r *ETFMetricsRepo) Window(ctx context.Context, date string, sessions int) (map[string][]ETFPoint, error) {
	var from sql.NullString
	if err := r.db.QueryRowContext(ctx, `
		SELECT DATE_FORMAT(MIN(d), '%Y-%m-%d') FROM (
		  SELECT DISTINCT trade_date AS d FROM etf_daily_metrics WHERE trade_date <= ?
		  ORDER BY trade_date DESC LIMIT ?) t`, date, sessions).Scan(&from); err != nil {
		return nil, fmt.Errorf("etf window: %w", err)
	}
	out := map[string][]ETFPoint{}
	if !from.Valid {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, DATE_FORMAT(trade_date, '%Y-%m-%d'), close_price, nav, net_assets, trade_value,
		       index_name, index_change_rate
		  FROM etf_daily_metrics WHERE trade_date BETWEEN ? AND ?
		 ORDER BY symbol, trade_date`, from.String, date)
	if err != nil {
		return nil, fmt.Errorf("etf window: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sym string
		var p ETFPoint
		if err := rows.Scan(&sym, &p.Date, &p.Close, &p.NAV, &p.NetAssets, &p.TradeValue, &p.IndexName, &p.IndexChangeRate); err != nil {
			return nil, err
		}
		out[sym] = append(out[sym], p)
	}
	return out, rows.Err()
}

// ETFScoreRow is one ETF's 점검 result for a date.
type ETFScoreRow struct {
	Symbol           string          `json:"symbol"`
	Name             string          `json:"name,omitempty"`
	TradeDate        string          `json:"tradeDate"`
	Score            float64         `json:"score"`
	PeerGroup        string          `json:"peerGroup"`
	PeerCount        int             `json:"peerCount"`
	PremiumPct       *float64        `json:"premiumPct,omitempty"`
	TrackingErrorPct *float64        `json:"trackingErrorPct,omitempty"`
	Beta             *float64        `json:"beta,omitempty"`
	TradeValueMedian *int64          `json:"tradeValueMedian,omitempty"`
	NetAssets        *int64          `json:"netAssets,omitempty"`
	IndexName        string          `json:"indexName,omitempty"`
	Parts            json.RawMessage `json:"parts"`
	Rank             int             `json:"rank,omitempty"`
}

// ReplaceScores rewrites one date's scores.
func (r *ETFMetricsRepo) ReplaceScores(ctx context.Context, date string, rows []ETFScoreRow) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, `DELETE FROM etf_scores WHERE trade_date = ?`, date); err != nil {
		return fmt.Errorf("etf scores clear: %w", err)
	}
	const chunk = 300
	for start := 0; start < len(rows); start += chunk {
		end := min(start+chunk, len(rows))
		var b strings.Builder
		b.WriteString(`INSERT INTO etf_scores (symbol, trade_date, score, peer_group, peer_count, premium_pct,
			tracking_error_pct, beta, trade_value_median, net_assets, parts) VALUES `)
		args := make([]any, 0, (end-start)*11)
		for i, row := range rows[start:end] {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString("(?,?,?,?,?,?,?,?,?,?,?)")
			args = append(args, row.Symbol, date, row.Score, row.PeerGroup, row.PeerCount, row.PremiumPct,
				row.TrackingErrorPct, row.Beta, row.TradeValueMedian, row.NetAssets, string(row.Parts))
		}
		if _, err := tx.ExecContext(ctx, b.String(), args...); err != nil {
			return fmt.Errorf("etf scores insert: %w", err)
		}
	}
	return tx.Commit()
}

const etfScoreSelect = `
	SELECT e.symbol, COALESCE(s.name, ''), DATE_FORMAT(e.trade_date, '%Y-%m-%d'), e.score, e.peer_group,
	       e.peer_count, e.premium_pct, e.tracking_error_pct, e.beta, e.trade_value_median, e.net_assets,
	       COALESCE(m.index_name, ''), e.parts
	  FROM etf_scores e
	  LEFT JOIN stocks s ON s.symbol = e.symbol
	  LEFT JOIN etf_daily_metrics m ON m.symbol = e.symbol AND m.trade_date = e.trade_date`

func scanETFScore(sc interface{ Scan(...any) error }) (ETFScoreRow, error) {
	var row ETFScoreRow
	var prem, te, beta sql.NullFloat64
	var tv, na sql.NullInt64
	var parts []byte
	if err := sc.Scan(&row.Symbol, &row.Name, &row.TradeDate, &row.Score, &row.PeerGroup, &row.PeerCount,
		&prem, &te, &beta, &tv, &na, &row.IndexName, &parts); err != nil {
		return row, err
	}
	if prem.Valid {
		row.PremiumPct = &prem.Float64
	}
	if te.Valid {
		row.TrackingErrorPct = &te.Float64
	}
	if beta.Valid {
		row.Beta = &beta.Float64
	}
	if tv.Valid {
		row.TradeValueMedian = &tv.Int64
	}
	if na.Valid {
		row.NetAssets = &na.Int64
	}
	row.Parts = json.RawMessage(parts)
	return row, nil
}

// LatestScore returns the newest 점검 row for one ETF (nil when none).
func (r *ETFMetricsRepo) LatestScore(ctx context.Context, symbol string) (*ETFScoreRow, error) {
	row, err := scanETFScore(r.db.QueryRowContext(ctx, etfScoreSelect+`
		 WHERE e.symbol = ? ORDER BY e.trade_date DESC LIMIT 1`, strings.ToUpper(symbol)))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("etf score: %w", err)
	}
	return &row, nil
}

// TopScores lists the newest date's ETFs by score. group "" means all.
func (r *ETFMetricsRepo) TopScores(ctx context.Context, group string, limit int) ([]ETFScoreRow, error) {
	q := etfScoreSelect + ` WHERE e.trade_date = (SELECT MAX(trade_date) FROM etf_scores)`
	args := []any{}
	if group != "" {
		q += ` AND e.peer_group = ?`
		args = append(args, group)
	}
	q += ` ORDER BY e.score DESC, e.net_assets DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("etf top: %w", err)
	}
	defer rows.Close()
	out := []ETFScoreRow{}
	for rows.Next() {
		row, err := scanETFScore(rows)
		if err != nil {
			return nil, err
		}
		row.Rank = len(out) + 1
		out = append(out, row)
	}
	return out, rows.Err()
}

// MigrateETFMetricsIfNeeded creates both tables on an existing database.
func MigrateETFMetricsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "etf_scores")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir, "086_etf_metrics.sql", "국내 ETF 점검")
}
