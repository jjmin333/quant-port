package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

type CandleBarRepo struct {
	db *sql.DB

	barTimeTypeOnce sync.Once
	barTimeDatetime bool
}

func NewCandleBarRepo(db *sql.DB) *CandleBarRepo {
	return &CandleBarRepo{db: db}
}

func (r *CandleBarRepo) barTimeIsDatetime(ctx context.Context) bool {
	r.barTimeTypeOnce.Do(func() {
		dt, err := columnDataType(ctx, r.db, "candle_bars", "bar_time")
		if err == nil && strings.EqualFold(dt, "datetime") {
			r.barTimeDatetime = true
		}
	})
	return r.barTimeDatetime
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
		c, err := r.scanCandle(rows, interval)
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

type candleScanner interface {
	Scan(dest ...any) error
}

func (r *CandleBarRepo) scanCandle(row candleScanner, interval model.CandleInterval) (model.Candle, error) {
	var c model.Candle
	if r.barTimeDatetime {
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

func (r *CandleBarRepo) UpsertBatch(ctx context.Context, symbol string, interval model.CandleInterval, candles []model.Candle) error {
	if len(candles) == 0 {
		return nil
	}

	const stmt = `
		INSERT INTO candle_bars (symbol, bar_interval, bar_time, open_price, high_price, low_price, close_price, volume)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			open_price = VALUES(open_price),
			high_price = VALUES(high_price),
			low_price = VALUES(low_price),
			close_price = VALUES(close_price),
			volume = VALUES(volume)
	`

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	useDatetime := r.barTimeIsDatetime(ctx)
	for _, c := range candles {
		barTime := model.NormalizeBarTime(c.Time, interval)
		var barTimeArg any = barTime
		if useDatetime {
			t := model.BeforeCursorTime(barTime)
			if t.IsZero() {
				return fmt.Errorf("invalid bar_time %q", barTime)
			}
			barTimeArg = t.In(model.KST)
		}
		if _, err := tx.ExecContext(ctx, stmt,
			symbol, string(interval), barTimeArg, c.Open, c.High, c.Low, c.Close, c.Volume,
		); err != nil {
			return fmt.Errorf("upsert %s %s %v: %w", symbol, interval, barTimeArg, err)
		}
	}
	return tx.Commit()
}

func (r *CandleBarRepo) HasTable(ctx context.Context) bool {
	var name string
	err := r.db.QueryRowContext(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = 'candle_bars'
	`).Scan(&name)
	return err == nil && strings.TrimSpace(name) != ""
}
