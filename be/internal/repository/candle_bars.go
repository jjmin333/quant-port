package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// barTimeTypeRecheck throttles information_schema lookups while bar_time is
// still the legacy VARCHAR type. MigrateCandleBarsV2IfNeeded runs in the
// background (see migrateCandleBarsAsync), so a false result must not be cached
// forever — otherwise every query keeps using the non-sargable STR_TO_DATE path
// long after the column became DATETIME.
const barTimeTypeRecheck = 30 * time.Second

type CandleBarRepo struct {
	db    *sql.DB
	table *tableExistsCache

	// barTimeDatetime latches to true once the column is DATETIME; the
	// migration is one-way so it never needs to go back.
	barTimeDatetime atomic.Bool
	barTimeCheckMu  sync.Mutex
	barTimeCheckAt  time.Time
}

func NewCandleBarRepo(db *sql.DB) *CandleBarRepo {
	return &CandleBarRepo{db: db, table: newTableExistsCache("candle_bars")}
}

func (r *CandleBarRepo) barTimeIsDatetime(ctx context.Context) bool {
	if r.barTimeDatetime.Load() {
		return true
	}

	r.barTimeCheckMu.Lock()
	defer r.barTimeCheckMu.Unlock()
	// Another goroutine may have latched it while we waited for the lock.
	if r.barTimeDatetime.Load() {
		return true
	}
	if !r.barTimeCheckAt.IsZero() && time.Since(r.barTimeCheckAt) < barTimeTypeRecheck {
		return false
	}
	r.barTimeCheckAt = time.Now()

	dt, err := columnDataType(ctx, r.db, "candle_bars", "bar_time")
	if err == nil && strings.EqualFold(dt, "datetime") {
		r.barTimeDatetime.Store(true)
		return true
	}
	return false
}

// barTimeOrderExpr compares mixed legacy bar_time strings (RFC3339 vs space-separated).
const barTimeOrderExpr = `COALESCE(
	STR_TO_DATE(bar_time, '%Y-%m-%dT%H:%i:%s+09:00'),
	STR_TO_DATE(bar_time, '%Y-%m-%d %H:%i:%s'),
	STR_TO_DATE(bar_time, '%Y-%m-%d')
)`

func (r *CandleBarRepo) ListBefore(
	ctx context.Context,
	symbol string,
	interval model.CandleInterval,
	before string,
	limit int,
) ([]model.Candle, *string, error) {
	if limit <= 0 {
		limit = 200
	}

	useDatetime := r.barTimeIsDatetime(ctx)

	query := `
		SELECT bar_time, open_price, high_price, low_price, close_price, volume
		FROM candle_bars
		WHERE symbol = ? AND bar_interval = ?
	`
	args := []any{symbol, string(interval)}
	if before != "" {
		if useDatetime {
			cursor := model.BeforeCursorTime(before)
			if cursor.IsZero() {
				return nil, nil, fmt.Errorf("invalid before cursor %q", before)
			}
			query += ` AND bar_time < ?`
			args = append(args, cursor.In(model.KST))
		} else {
			query += ` AND ` + barTimeOrderExpr + ` < STR_TO_DATE(?, '%Y-%m-%dT%H:%i:%s+09:00')`
			args = append(args, model.BeforeCursorRFC3339(before))
		}
	}
	if useDatetime {
		query += ` ORDER BY bar_time DESC LIMIT ?`
	} else {
		query += ` ORDER BY ` + barTimeOrderExpr + ` DESC LIMIT ?`
	}
	args = append(args, limit+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var out []model.Candle
	for rows.Next() {
		c, err := r.scanCandle(rows, interval, useDatetime)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	var nextBefore *string
	if len(out) > limit {
		nb := out[limit].Time
		nextBefore = &nb
		out = out[:limit]
	}

	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nextBefore, nil
}

// CloseAtOrBefore returns the close of the latest completed bar at or before cutoff.
// Uses idx_candle_bars_symbol_interval_time (symbol, bar_interval, bar_time DESC).
// DailyCloseSeries returns the most recent `limit` daily closes ascending.
// Research target odds need the longest run of history we hold, and candle_bars
// keeps more of it than the stock_daily_closes projection does, so the odds are
// measured here rather than off the projection.
func (r *CandleBarRepo) DailyCloseSeries(ctx context.Context, symbol string, limit int) ([]DailyClosePoint, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 5000 {
		limit = 2000
	}
	useDatetime := r.barTimeIsDatetime(ctx)
	order := barTimeOrderExpr
	if useDatetime {
		order = "bar_time"
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+order+` AS bar_at, close_price
		  FROM candle_bars
		 WHERE symbol = ? AND bar_interval = ?
		 ORDER BY bar_at DESC LIMIT ?`,
		strings.ToUpper(strings.TrimSpace(symbol)), string(model.Interval1d), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DailyClosePoint, 0, limit)
	for rows.Next() {
		var (
			at    sql.NullTime
			close float64
		)
		if err := rows.Scan(&at, &close); err != nil {
			return nil, err
		}
		if !at.Valid || close <= 0 {
			continue
		}
		out = append(out, DailyClosePoint{Date: at.Time.Format(dailyCloseDateLayout), Close: close})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Query was DESC so the caller gets ascending order here.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// DailyExtreme is one trading day's high/low — enough to answer "did the price
// ever touch this level" without pulling whole candles.
type DailyExtreme struct {
	Date string
	High float64
	Low  float64
}

// DailyExtremesBetween lists daily highs and lows in [from, to] (YYYY-MM-DD,
// inclusive), oldest first.
//
// Stamps use this for two things: finding the first day a target was reached
// intraday, and reporting the peak/trough a stamp lived through. Both need the
// bar's range, not its close — a target that was touched and given back is
// still a target that was reached.
func (r *CandleBarRepo) DailyExtremesBetween(
	ctx context.Context, symbol, from, to string,
) ([]DailyExtreme, error) {
	if r == nil || r.db == nil || from == "" {
		return nil, nil
	}
	dateExpr := barTimeOrderExpr
	if r.barTimeIsDatetime(ctx) {
		dateExpr = "bar_time"
	}
	// DATE(...) on both sides so a bar stored with a time component still
	// matches a plain YYYY-MM-DD bound.
	query := `
		SELECT DATE(` + dateExpr + `) AS bar_day, MAX(high_price), MIN(low_price)
		  FROM candle_bars
		 WHERE symbol = ? AND bar_interval = ?
		   AND DATE(` + dateExpr + `) >= ?`
	args := []any{strings.ToUpper(strings.TrimSpace(symbol)), string(model.Interval1d), from}
	if to != "" {
		query += ` AND DATE(` + dateExpr + `) <= ?`
		args = append(args, to)
	}
	query += ` GROUP BY bar_day ORDER BY bar_day ASC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DailyExtreme, 0, 64)
	for rows.Next() {
		var (
			day       sql.NullTime
			high, low sql.NullFloat64
		)
		if err := rows.Scan(&day, &high, &low); err != nil {
			return nil, err
		}
		// A bar with no high/low is a data gap, not a zero-priced day.
		if !day.Valid || !high.Valid || !low.Valid || high.Float64 <= 0 || low.Float64 <= 0 {
			continue
		}
		out = append(out, DailyExtreme{
			Date: day.Time.Format(dailyCloseDateLayout),
			High: high.Float64,
			Low:  low.Float64,
		})
	}
	return out, rows.Err()
}

func (r *CandleBarRepo) CloseAtOrBefore(
	ctx context.Context,
	symbol string,
	interval model.CandleInterval,
	cutoff time.Time,
) (float64, bool, error) {
	if cutoff.IsZero() {
		return 0, false, nil
	}

	useDatetime := r.barTimeIsDatetime(ctx)
	query := `
		SELECT close_price
		FROM candle_bars
		WHERE symbol = ? AND bar_interval = ?
	`
	args := []any{symbol, string(interval)}
	cutoffKST := cutoff.In(model.KST)
	if useDatetime {
		query += ` AND bar_time <= ?`
		args = append(args, cutoffKST)
		query += ` ORDER BY bar_time DESC LIMIT 1`
	} else {
		cutoffDate := cutoffKST.Format("2006-01-02")
		if interval == model.Interval1d || interval == model.Interval1w || interval == model.Interval1M {
			query += ` AND ` + barTimeOrderExpr + ` <= STR_TO_DATE(?, '%Y-%m-%d')`
			args = append(args, cutoffDate)
		} else {
			cutoffStr := model.FormatCandleTime(cutoffKST, interval)
			query += ` AND ` + barTimeOrderExpr + ` <= STR_TO_DATE(?, '%Y-%m-%dT%H:%i:%s+09:00')`
			args = append(args, cutoffStr)
		}
		query += ` ORDER BY ` + barTimeOrderExpr + ` DESC LIMIT 1`
	}

	var closePx float64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&closePx)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if closePx <= 0 {
		return 0, false, nil
	}
	return closePx, true, nil
}

// LastDailyCloseInCalendarMonth returns the last 1d close for a calendar month.
// When includeNextDayMidnight is true, includes bars stamped through 00:00 on the 1st
// of the following month (KRX month-end convention). beforeDay excludes bars on/after that day.
func (r *CandleBarRepo) LastDailyCloseInCalendarMonth(
	ctx context.Context,
	symbol string,
	year int,
	month time.Month,
	includeNextDayMidnight bool,
	beforeDay time.Time,
) (float64, bool, error) {
	if r == nil || !r.HasTable(ctx) {
		return 0, false, nil
	}
	beforeDay = beforeDay.In(model.KST)
	monthStart := time.Date(year, month, 1, 0, 0, 0, 0, model.KST)
	rangeEnd := monthStart.AddDate(0, 1, 0)
	if includeNextDayMidnight {
		rangeEnd = rangeEnd.AddDate(0, 0, 1)
	}

	useDatetime := r.barTimeIsDatetime(ctx)
	var query string
	args := []any{symbol, string(model.Interval1d)}
	if useDatetime {
		query = `
			SELECT close_price
			FROM candle_bars
			WHERE symbol = ? AND bar_interval = ?
			  AND bar_time >= ? AND bar_time < ?
			  AND bar_time < ?
			ORDER BY bar_time DESC LIMIT 1
		`
		args = append(args, monthStart, rangeEnd, beforeDay)
	} else {
		query = `
			SELECT close_price
			FROM candle_bars
			WHERE symbol = ? AND bar_interval = ?
			  AND ` + barTimeOrderExpr + ` >= ?
			  AND ` + barTimeOrderExpr + ` < ?
			  AND ` + barTimeOrderExpr + ` < ?
			ORDER BY ` + barTimeOrderExpr + ` DESC LIMIT 1
		`
		args = append(args,
			monthStart.Format("2006-01-02"),
			rangeEnd.Format("2006-01-02"),
			beforeDay.Format("2006-01-02"),
		)
	}

	var closePx float64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&closePx)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if closePx <= 0 {
		return 0, false, nil
	}
	return closePx, true, nil
}

type candleScanner interface {
	Scan(dest ...any) error
}

func (r *CandleBarRepo) scanCandle(row candleScanner, interval model.CandleInterval, useDatetime bool) (model.Candle, error) {
	var c model.Candle
	if useDatetime {
		var bt time.Time
		if err := row.Scan(&bt, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume); err != nil {
			return c, err
		}
		c.Time = model.FormatCandleTime(bt.In(model.KST), interval)
		return c, nil
	}
	if err := row.Scan(&c.Time, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume); err != nil {
		return c, err
	}
	c.Time = model.NormalizeBarTime(c.Time, interval)
	return c, nil
}

// candleUpsertChunk bounds rows per INSERT so a large backfill stays well under
// max_allowed_packet and the 65535 placeholder limit (8 placeholders per row).
const candleUpsertChunk = 500

const candleUpsertPrefix = `
	INSERT INTO candle_bars (symbol, bar_interval, bar_time, open_price, high_price, low_price, close_price, volume)
	VALUES `

const candleUpsertSuffix = `
	ON DUPLICATE KEY UPDATE
		open_price = VALUES(open_price),
		high_price = VALUES(high_price),
		low_price = VALUES(low_price),
		close_price = VALUES(close_price),
		volume = VALUES(volume)
`

func (r *CandleBarRepo) UpsertBatch(ctx context.Context, symbol string, interval model.CandleInterval, candles []model.Candle) error {
	if len(candles) == 0 {
		return nil
	}

	useDatetime := r.barTimeIsDatetime(ctx)

	// Resolve every bar_time up front so an invalid row fails before we open a
	// transaction, matching the previous all-or-nothing behaviour.
	barTimes := make([]any, len(candles))
	for i, c := range candles {
		barTime := model.NormalizeBarTime(c.Time, interval)
		if !useDatetime {
			barTimes[i] = barTime
			continue
		}
		t := model.BeforeCursorTime(barTime)
		if t.IsZero() {
			return fmt.Errorf("invalid bar_time %q", barTime)
		}
		if model.IsDailyOrCoarser(interval) {
			// Daily bars are stored as a KST wall-clock literal rather than a
			// time.Time, because the driver would otherwise convert to the
			// DSN's loc (Local). With the container on UTC that turned KST
			// midnight into the PREVIOUS day at 15:00, and any consumer doing
			// DATE(bar_time) in SQL — the 일별 종가 projection — then read the
			// trade date one day early. Handing MySQL a formatted string keeps
			// the stored date identical no matter where the process runs.
			barTimes[i] = t.In(model.KST).Format("2006-01-02 15:04:05")
			continue
		}
		barTimes[i] = t.In(model.KST)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for start := 0; start < len(candles); start += candleUpsertChunk {
		end := min(start+candleUpsertChunk, len(candles))
		chunk := candles[start:end]

		var b strings.Builder
		b.Grow(len(candleUpsertPrefix) + len(chunk)*26 + len(candleUpsertSuffix))
		b.WriteString(candleUpsertPrefix)
		args := make([]any, 0, len(chunk)*8)
		for i, c := range chunk {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString("(?,?,?,?,?,?,?,?)")
			args = append(args,
				symbol, string(interval), barTimes[start+i],
				c.Open, c.High, c.Low, c.Close, c.Volume,
			)
		}
		b.WriteString(candleUpsertSuffix)

		if _, err := tx.ExecContext(ctx, b.String(), args...); err != nil {
			return fmt.Errorf("upsert %s %s (%d rows from %v): %w",
				symbol, interval, len(chunk), barTimes[start], err)
		}
	}
	return tx.Commit()
}

func (r *CandleBarRepo) HasTable(ctx context.Context) bool {
	return r.table.Has(ctx, r.db)
}
