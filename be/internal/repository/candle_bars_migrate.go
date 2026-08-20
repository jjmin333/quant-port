package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// legacyBarTimeExpr parses the three legacy bar_time string encodings into a
// DATETIME. Rows that match none of them are unrecoverable and get dropped.
const legacyBarTimeExpr = `COALESCE(
	STR_TO_DATE(bar_time, '%Y-%m-%dT%H:%i:%s+09:00'),
	STR_TO_DATE(bar_time, '%Y-%m-%d %H:%i:%s'),
	STR_TO_DATE(bar_time, '%Y-%m-%d')
)`

// MigrateCandleBarsV2IfNeeded upgrades bar_time from the legacy VARCHAR encoding
// to DATETIME so range scans can use the clustered index.
//
// It rebuilds the table in a single pass rather than normalizing row by row.
// The previous implementation issued one UPDATE per row behind a repeated
// unindexed `LIKE '%T%'` scan, which is quadratic — on a 2M row table it never
// finished. INSERT ... SELECT with INSERT IGNORE does the normalize and the
// dedupe (two legacy strings can normalize to the same instant) in one pass.
//
// Note: this temporarily needs room for a second copy of the table. The old
// table is kept as candle_bars_legacy_v1 so the migration is reversible; drop it
// once the new table looks right.
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

	log.Println("mysql: migrating candle_bars.bar_time to DATETIME (single-pass rebuild)")
	started := time.Now()

	var total int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM candle_bars`).Scan(&total); err == nil {
		log.Printf("mysql: candle_bars rebuild: %d source rows", total)
	}

	if err := rebuildCandleBarsAsDatetime(ctx, db); err != nil {
		return err
	}

	log.Printf("mysql: candle_bars.bar_time is DATETIME (%.1fs; previous table kept as candle_bars_legacy_v1)",
		time.Since(started).Seconds())
	return nil
}

func rebuildCandleBarsAsDatetime(ctx context.Context, db *sql.DB) error {
	// Start clean if an earlier attempt died midway.
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS candle_bars_v2`); err != nil {
		return fmt.Errorf("candle_bars: drop stale v2: %w", err)
	}

	// The secondary index the old table carried — (symbol, bar_interval,
	// bar_time DESC) — duplicated the primary key exactly, so InnoDB was
	// maintaining a second full copy of the key for no added access path.
	// The clustered PK serves those lookups, including backwards scans.
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE candle_bars_v2 (
			symbol       VARCHAR(32)  NOT NULL,
			bar_interval VARCHAR(8)   NOT NULL,
			bar_time     DATETIME     NOT NULL,
			open_price   DOUBLE       NOT NULL,
			high_price   DOUBLE       NOT NULL,
			low_price    DOUBLE       NOT NULL,
			close_price  DOUBLE       NOT NULL,
			volume       BIGINT       NOT NULL,
			created_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (symbol, bar_interval, bar_time)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
	`); err != nil {
		return fmt.Errorf("candle_bars: create v2: %w", err)
	}

	// INSERT IGNORE drops rows whose normalized bar_time collides with one
	// already inserted, which is exactly the dedupe the old code did separately.
	res, err := db.ExecContext(ctx, fmt.Sprintf(`
		INSERT IGNORE INTO candle_bars_v2
			(symbol, bar_interval, bar_time, open_price, high_price, low_price, close_price, volume, created_at)
		SELECT symbol, bar_interval, %s,
		       open_price, high_price, low_price, close_price, volume, created_at
		FROM candle_bars
		WHERE %s IS NOT NULL
	`, legacyBarTimeExpr, legacyBarTimeExpr))
	if err != nil {
		return fmt.Errorf("candle_bars: copy into v2: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil {
		log.Printf("mysql: candle_bars rebuild: %d rows copied", n)
	}

	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS candle_bars_legacy_v1`); err != nil {
		return fmt.Errorf("candle_bars: drop stale legacy: %w", err)
	}
	// Atomic swap — readers see either the old or the new table, never neither.
	if _, err := db.ExecContext(ctx, `
		RENAME TABLE candle_bars TO candle_bars_legacy_v1, candle_bars_v2 TO candle_bars
	`); err != nil {
		return fmt.Errorf("candle_bars: rename swap: %w", err)
	}
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
