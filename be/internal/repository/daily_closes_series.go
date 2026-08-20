package repository

import (
	"context"
	"strings"
	"time"
)

// DailyClosePoint is a (trade_date, close) pair. Stamp performance needs the
// date alongside the price — "the close on or before D+30" is only meaningful
// if we can tell whether that close is actually after the stamp.
type DailyClosePoint struct {
	Date  string // YYYY-MM-DD
	Close float64
}

// ClosesSince returns a symbol's closes from `since` (inclusive), ascending.
// One query per symbol lets a whole stamp list resolve every horizon in memory
// instead of issuing four point lookups per stamp.
func (r *StockDailyCloseRepo) ClosesSince(ctx context.Context, symbol, since string) ([]DailyClosePoint, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT trade_date, close_price
		  FROM stock_daily_closes
		 WHERE symbol = ? AND trade_date >= ?
		 ORDER BY trade_date ASC`,
		strings.ToUpper(strings.TrimSpace(symbol)), since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClosePoints(rows)
}

// ClosesSince mirrors the stock variant for benchmark indices.
func (r *IndexDailyCloseRepo) ClosesSince(ctx context.Context, indexID, since string) ([]DailyClosePoint, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT trade_date, close_value
		  FROM index_daily_closes
		 WHERE index_id = ? AND trade_date >= ?
		 ORDER BY trade_date ASC`,
		strings.ToLower(strings.TrimSpace(indexID)), since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClosePoints(rows)
}

func scanClosePoints(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]DailyClosePoint, error) {
	out := make([]DailyClosePoint, 0)
	for rows.Next() {
		var (
			date  time.Time
			close float64
		)
		if err := rows.Scan(&date, &close); err != nil {
			return nil, err
		}
		out = append(out, DailyClosePoint{Date: date.Format(dailyCloseDateLayout), Close: close})
	}
	return out, rows.Err()
}
