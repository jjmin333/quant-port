package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

const candleBarsBatchSize = 2000

// MigrateCandleBarsV2IfNeeded normalizes legacy bar_time strings, deduplicates rows,
// and upgrades bar_time to DATETIME so pagination can use the composite index.
func MigrateCandleBarsV2IfNeeded(ctx context.Context, db *sql.DB) error {
	exists, err := tableExists(ctx, db, "candle_bars")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	dt, err := columnDataType(ctx, db, "candle_bars", "bar_time")
	if err != nil {
		return err
	}
	if strings.EqualFold(dt, "datetime") {
		return nil
	}

	log.Println("mysql: migrating candle_bars.bar_time to DATETIME (normalize + dedupe)")
	if err := normalizeCandleBarTimes(ctx, db); err != nil {
		return err
	}
	if err := dedupeCandleBars(ctx, db); err != nil {
		return err
	}
	if err := alterCandleBarTimeDatetime(ctx, db); err != nil {
		return err
	}
	log.Println("mysql: candle_bars.bar_time is DATETIME")
	return nil
}

func columnDataType(ctx context.Context, db *sql.DB, table, column string) (string, error) {
	var dataType string
	err := db.QueryRowContext(ctx, `
		SELECT DATA_TYPE
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		  AND table_name = ?
		  AND column_name = ?
	`, table, column).Scan(&dataType)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("column type %s.%s: %w", table, column, err)
	}
	return dataType, nil
}

func normalizeCandleBarTimes(ctx context.Context, db *sql.DB) error {
	const q = `
		SELECT symbol, bar_interval, bar_time
		FROM candle_bars
		WHERE bar_time LIKE '%T%' OR bar_time LIKE '% %:%'
		ORDER BY symbol, bar_interval, bar_time
		LIMIT ?
	`
	for {
		rows, err := db.QueryContext(ctx, q, candleBarsBatchSize)
		if err != nil {
			return err
		}

		type row struct {
			symbol   string
			interval model.CandleInterval
			raw      string
		}
		var batch []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.symbol, &r.interval, &r.raw); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, r)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()

		if len(batch) == 0 {
			return nil
		}

		for _, r := range batch {
			norm := model.NormalizeBarTime(r.raw, r.interval)
			if norm == r.raw {
				continue
			}
			if err := rewriteCandleBarTime(ctx, db, r.symbol, r.interval, r.raw, norm); err != nil {
				return err
			}
		}

		if len(batch) < candleBarsBatchSize {
			return nil
		}
	}
}

func rewriteCandleBarTime(
	ctx context.Context,
	db *sql.DB,
	symbol string,
	interval model.CandleInterval,
	raw string,
	norm string,
) error {
	res, err := db.ExecContext(ctx, `
		UPDATE candle_bars
		SET bar_time = ?
		WHERE symbol = ? AND bar_interval = ? AND bar_time = ?
	`, norm, symbol, string(interval), raw)
	if err != nil {
		if isDuplicateKeyErr(err) {
			_, delErr := db.ExecContext(ctx, `
				DELETE FROM candle_bars
				WHERE symbol = ? AND bar_interval = ? AND bar_time = ?
			`, symbol, string(interval), raw)
			return delErr
		}
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	return nil
}

func dedupeCandleBars(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		DELETE cb1 FROM candle_bars cb1
		INNER JOIN candle_bars cb2
		  ON cb1.symbol = cb2.symbol
		 AND cb1.bar_interval = cb2.bar_interval
		 AND cb1.bar_time = cb2.bar_time
		 AND cb1.created_at > cb2.created_at
	`)
	return err
}

func alterCandleBarTimeDatetime(ctx context.Context, db *sql.DB) error {
	const expr = `COALESCE(
		STR_TO_DATE(bar_time, '%Y-%m-%dT%H:%i:%s+09:00'),
		STR_TO_DATE(bar_time, '%Y-%m-%d %H:%i:%s'),
		STR_TO_DATE(bar_time, '%Y-%m-%d')
	)`

	if _, err := db.ExecContext(ctx, fmt.Sprintf(`
		UPDATE candle_bars
		SET bar_time = DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:%%i:%%s')
		WHERE bar_time LIKE '%%T%%' OR bar_time LIKE '%% %%'
	`, expr)); err != nil {
		return fmt.Errorf("candle_bars pre-datetime format: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		ALTER TABLE candle_bars
		MODIFY bar_time DATETIME NOT NULL
	`); err != nil {
		return fmt.Errorf("candle_bars bar_time datetime: %w", err)
	}
	return nil
}

func isDuplicateKeyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate entry") || strings.Contains(msg, "duplicate key")
}
