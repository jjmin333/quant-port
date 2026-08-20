package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

const dailyCloseDateLayout = "2006-01-02"

// StockDailyCloseRow is one persisted row of stock_daily_closes.
type StockDailyCloseRow struct {
	Symbol         string
	TradeDate      time.Time
	Close          float64
	PrevClose      sql.NullFloat64
	ChangeAmount   sql.NullFloat64
	ChangeRate     sql.NullFloat64
	Volume         int64
	Currency       string
	Score          sql.NullInt64
	AISummary      string
	AnalysisSource string
	Source         string
}

type StockDailyCloseRepo struct {
	db    *sql.DB
	table *tableExistsCache
}

func NewStockDailyCloseRepo(db *sql.DB) *StockDailyCloseRepo {
	return &StockDailyCloseRepo{db: db, table: newTableExistsCache("stock_daily_closes")}
}

func (r *StockDailyCloseRepo) HasTable(ctx context.Context) bool {
	if r == nil || r.db == nil {
		return false
	}
	return r.table.Has(ctx, r.db)
}

// UpsertBatch writes closes for one symbol. Price columns are always restated;
// score/ai_summary are NOT touched here so an analysis written later is never
// clobbered by a re-run of the price ingestion.
func (r *StockDailyCloseRepo) UpsertBatch(ctx context.Context, rows []StockDailyCloseRow) error {
	if r == nil || r.db == nil || len(rows) == 0 {
		return nil
	}

	const chunk = 400
	for start := 0; start < len(rows); start += chunk {
		end := start + chunk
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[start:end]

		var sb strings.Builder
		sb.WriteString(`INSERT INTO stock_daily_closes
			(symbol, trade_date, close_price, prev_close, change_amount, change_rate, volume, currency, ai_summary, source)
			VALUES `)
		args := make([]any, 0, len(batch)*10)
		for i, row := range batch {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?,?,?,?,?,'',?)")
			args = append(args,
				row.Symbol,
				row.TradeDate.Format(dailyCloseDateLayout),
				row.Close,
				nullFloatArg(row.PrevClose),
				nullFloatArg(row.ChangeAmount),
				nullFloatArg(row.ChangeRate),
				row.Volume,
				dailyCloseDefault(row.Currency, "KRW"),
				dailyCloseDefault(row.Source, "candle_bars"),
			)
		}
		sb.WriteString(` ON DUPLICATE KEY UPDATE
			close_price = VALUES(close_price),
			prev_close = VALUES(prev_close),
			change_amount = VALUES(change_amount),
			change_rate = VALUES(change_rate),
			volume = VALUES(volume),
			currency = VALUES(currency),
			source = VALUES(source)`)

		if _, err := r.db.ExecContext(ctx, sb.String(), args...); err != nil {
			return fmt.Errorf("stock_daily_closes upsert: %w", err)
		}
	}
	return nil
}

// UpsertAnalysis attaches a score and AI summary to an existing (or new) date
// row. Inserting a placeholder close of 0 would be wrong, so the analysis is
// only written when the date row exists — the ingestion always runs first.
func (r *StockDailyCloseRepo) UpsertAnalysis(ctx context.Context, a model.DailyCloseAnalysis) (bool, error) {
	if r == nil || r.db == nil {
		return false, nil
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE stock_daily_closes
		   SET score = ?, ai_summary = ?, analysis_source = ?, analyzed_at = CURRENT_TIMESTAMP(3)
		 WHERE symbol = ? AND trade_date = ?`,
		nullIntArg(a.Score), a.AISummary, a.Source, strings.ToUpper(strings.TrimSpace(a.Symbol)), a.TradeDate,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// List returns the most recent `limit` rows for a symbol, newest first.
func (r *StockDailyCloseRepo) List(ctx context.Context, symbol string, limit int) ([]StockDailyCloseRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 60
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, trade_date, close_price, prev_close, change_amount, change_rate,
		       volume, currency, score, ai_summary, analysis_source, source
		  FROM stock_daily_closes
		 WHERE symbol = ?
		 ORDER BY trade_date DESC
		 LIMIT ?`, strings.ToUpper(strings.TrimSpace(symbol)), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StockDailyCloseRow
	for rows.Next() {
		var row StockDailyCloseRow
		if err := rows.Scan(
			&row.Symbol, &row.TradeDate, &row.Close, &row.PrevClose, &row.ChangeAmount,
			&row.ChangeRate, &row.Volume, &row.Currency, &row.Score, &row.AISummary,
			&row.AnalysisSource, &row.Source,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// LastCloseBefore returns the most recent stored close strictly before
// `date` — the previous-close reference for an incremental EOD write.
func (r *StockDailyCloseRepo) LastCloseBefore(ctx context.Context, symbol, date string) (float64, bool, error) {
	if r == nil || r.db == nil {
		return 0, false, nil
	}
	var close float64
	err := r.db.QueryRowContext(ctx, `
		SELECT close_price FROM stock_daily_closes
		 WHERE symbol = ? AND trade_date < ?
		 ORDER BY trade_date DESC LIMIT 1`,
		strings.ToUpper(strings.TrimSpace(symbol)), date,
	).Scan(&close)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return close, true, nil
}

// Stats reports coverage for operational reporting after a backfill.
func (r *StockDailyCloseRepo) Stats(ctx context.Context, since string) (rowCount, symbolCount int, minDate, maxDate string, err error) {
	if r == nil || r.db == nil {
		return 0, 0, "", "", nil
	}
	var mn, mx sql.NullString
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COUNT(DISTINCT symbol), MIN(trade_date), MAX(trade_date)
		  FROM stock_daily_closes WHERE trade_date >= ?`, since,
	).Scan(&rowCount, &symbolCount, &mn, &mx)
	if err != nil {
		return 0, 0, "", "", err
	}
	return rowCount, symbolCount, mn.String, mx.String, nil
}

// IndexDailyCloseRow is one persisted row of index_daily_closes.
type IndexDailyCloseRow struct {
	IndexID      string
	TradeDate    time.Time
	Close        float64
	PrevClose    sql.NullFloat64
	ChangeAmount sql.NullFloat64
	ChangeRate   sql.NullFloat64
	Currency     string
	Source       string
	// Score/AISummary are written by the index score batch, not by the close
	// upsert — the same one-way split the stock side uses.
	Score     sql.NullInt64
	AISummary string
}

type IndexDailyCloseRepo struct {
	db    *sql.DB
	table *tableExistsCache
}

func NewIndexDailyCloseRepo(db *sql.DB) *IndexDailyCloseRepo {
	return &IndexDailyCloseRepo{db: db, table: newTableExistsCache("index_daily_closes")}
}

func (r *IndexDailyCloseRepo) HasTable(ctx context.Context) bool {
	if r == nil || r.db == nil {
		return false
	}
	return r.table.Has(ctx, r.db)
}

func (r *IndexDailyCloseRepo) UpsertBatch(ctx context.Context, rows []IndexDailyCloseRow) error {
	if r == nil || r.db == nil || len(rows) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString(`INSERT INTO index_daily_closes
		(index_id, trade_date, close_value, prev_close, change_amount, change_rate, currency, source)
		VALUES `)
	args := make([]any, 0, len(rows)*8)
	for i, row := range rows {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("(?,?,?,?,?,?,?,?)")
		args = append(args,
			row.IndexID,
			row.TradeDate.Format(dailyCloseDateLayout),
			row.Close,
			nullFloatArg(row.PrevClose),
			nullFloatArg(row.ChangeAmount),
			nullFloatArg(row.ChangeRate),
			dailyCloseDefault(row.Currency, "KRW"),
			row.Source,
		)
	}
	sb.WriteString(` ON DUPLICATE KEY UPDATE
		close_value = VALUES(close_value),
		prev_close = VALUES(prev_close),
		change_amount = VALUES(change_amount),
		change_rate = VALUES(change_rate),
		currency = VALUES(currency),
		source = VALUES(source)`)

	if _, err := r.db.ExecContext(ctx, sb.String(), args...); err != nil {
		return fmt.Errorf("index_daily_closes upsert: %w", err)
	}
	return nil
}

func (r *IndexDailyCloseRepo) List(ctx context.Context, indexID string, limit int) ([]IndexDailyCloseRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 60
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT index_id, trade_date, close_value, prev_close, change_amount, change_rate, currency, source, score, ai_summary
		  FROM index_daily_closes
		 WHERE index_id = ?
		 ORDER BY trade_date DESC
		 LIMIT ?`, strings.ToLower(strings.TrimSpace(indexID)), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []IndexDailyCloseRow
	for rows.Next() {
		var row IndexDailyCloseRow
		if err := rows.Scan(
			&row.IndexID, &row.TradeDate, &row.Close, &row.PrevClose,
			&row.ChangeAmount, &row.ChangeRate, &row.Currency, &row.Source,
			&row.Score, &row.AISummary,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *IndexDailyCloseRepo) LastCloseBefore(ctx context.Context, indexID, date string) (float64, bool, error) {
	if r == nil || r.db == nil {
		return 0, false, nil
	}
	var close float64
	err := r.db.QueryRowContext(ctx, `
		SELECT close_value FROM index_daily_closes
		 WHERE index_id = ? AND trade_date < ?
		 ORDER BY trade_date DESC LIMIT 1`,
		strings.ToLower(strings.TrimSpace(indexID)), date,
	).Scan(&close)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return close, true, nil
}

func nullFloatArg(v sql.NullFloat64) any {
	if !v.Valid {
		return nil
	}
	return v.Float64
}

func nullIntArg(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func dailyCloseDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

// MigrateDailyClosesIfNeeded applies 029_daily_closes.sql.
func MigrateDailyClosesIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "stock_daily_closes")
	if err != nil {
		return err
	}
	indexExists, err := tableExists(ctx, db, "index_daily_closes")
	if err != nil {
		return err
	}
	if exists && indexExists {
		return nil
	}
	path := filepath.Join(migrationsDir, "029_daily_closes.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 029_daily_closes.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("029_daily_closes.sql: %w", err)
		}
	}
	return nil
}
