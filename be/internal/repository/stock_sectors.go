package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

/*
종목별 산업 코드와 공통 섹터. 표를 왜 따로 두는지는 `migrations/078_stock_sectors.sql`.
*/

// StockSectorRow is one symbol's classification.
type StockSectorRow struct {
	Symbol       string
	Source       string // dart | sec
	IndustryCode string
	IndustryName string
	Sector       string // "" when the code does not map
}

// StockSectorRepo owns stock_sectors.
type StockSectorRepo struct {
	db *sql.DB
}

func NewStockSectorRepo(db *sql.DB) *StockSectorRepo {
	return &StockSectorRepo{db: db}
}

// Upsert writes one classification. 빈 섹터는 NULL로 — 「모른다」를 빈 문자열
// 칸으로 세면 GROUP BY가 그것을 하나의 섹터로 센다.
func (r *StockSectorRepo) Upsert(ctx context.Context, row StockSectorRow) error {
	if r == nil || r.db == nil {
		return nil
	}
	var sector any
	if row.Sector != "" {
		sector = row.Sector
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO stock_sectors (symbol, source, industry_code, industry_name, sector, fetched_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP(3))
		ON DUPLICATE KEY UPDATE
		  source = VALUES(source),
		  industry_code = VALUES(industry_code),
		  industry_name = VALUES(industry_name),
		  sector = VALUES(sector),
		  fetched_at = CURRENT_TIMESTAMP(3)`,
		row.Symbol, row.Source, row.IndustryCode, truncateRunes(row.IndustryName, 160), sector)
	if err != nil {
		return fmt.Errorf("stock sector upsert %s: %w", row.Symbol, err)
	}
	return nil
}

/*
FreshSymbols — `symbols` 중 `maxAge` 안에 받은 것.

섹터는 거의 안 바뀐다. 로테이션(`SelectTargets`)은 한 바퀴를 다 돈 뒤에도 가장
오래된 것부터 계속 내주므로, 여기서 아직 신선한 것을 걸러야 매일 같은 종목을
다시 받지 않는다.
*/
func (r *StockSectorRepo) FreshSymbols(ctx context.Context, symbols []string, maxAge time.Duration) (map[string]bool, error) {
	out := map[string]bool{}
	if r == nil || r.db == nil || len(symbols) == 0 {
		return out, nil
	}
	in, args := symbolPlaceholders(symbols)
	args = append(args, time.Now().Add(-maxAge))
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol FROM stock_sectors
		 WHERE symbol IN (`+in+`) AND fetched_at >= ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("stock sector fresh: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out[s] = true
	}
	return out, rows.Err()
}

// SectorsFor returns symbol → sector for the given symbols (mapped ones only).
func (r *StockSectorRepo) SectorsFor(ctx context.Context, symbols []string) (map[string]string, error) {
	out := map[string]string{}
	if r == nil || r.db == nil || len(symbols) == 0 {
		return out, nil
	}
	in, args := symbolPlaceholders(symbols)
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, sector FROM stock_sectors
		 WHERE symbol IN (`+in+`) AND sector IS NOT NULL`, args...)
	if err != nil {
		return nil, fmt.Errorf("stock sectors for: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sym, sector string
		if err := rows.Scan(&sym, &sector); err != nil {
			return nil, err
		}
		out[sym] = sector
	}
	return out, rows.Err()
}

// MigrateStockSectorsIfNeeded creates the table on an existing database.
func MigrateStockSectorsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "stock_sectors")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir,
		"078_stock_sectors.sql", "종목별 산업 코드 · 공통 섹터")
}
