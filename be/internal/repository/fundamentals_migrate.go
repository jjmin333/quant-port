package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func hasLegacyFundamentalsColumns(ctx context.Context, db *sql.DB) (bool, error) {
	var name string
	err := db.QueryRowContext(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		  AND table_name = 'stock_fundamentals'
		  AND column_name = 'per'
	`).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check legacy fundamentals columns: %w", err)
	}
	return true, nil
}

func tableExists(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var name string
	err := db.QueryRowContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = ?
	`, table).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func dropOldFundamentalsFK(ctx context.Context, exec sqlExecutor) error {
	_, err := exec.ExecContext(ctx, `
		ALTER TABLE old_stock_fundamentals DROP FOREIGN KEY fk_stock_fundamentals_symbol
	`)
	if err != nil && !isUnknownConstraintErr(err) {
		return err
	}
	return nil
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func isUnknownConstraintErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "1091") || strings.Contains(msg, "check that column/key exists")
}

func createV2FundamentalsTable(ctx context.Context, exec sqlExecutor, migrationsDir string) error {
	if err := dropOldFundamentalsFK(ctx, exec); err != nil {
		return fmt.Errorf("drop old fundamentals fk: %w", err)
	}
	path := filepath.Join(migrationsDir, "005_stock_fundamentals.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 005 migration: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := exec.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("create v2 stock_fundamentals: %w", err)
		}
	}
	return nil
}

func copyFundamentalsV2(ctx context.Context, exec sqlExecutor) (int64, error) {
	res, err := exec.ExecContext(ctx, `
		INSERT INTO stock_fundamentals (
			symbol, data_source, fiscal_year, fiscal_period, report_code,
			period_end_date, filed_at,
			revenue, operating_income, net_income, total_assets, total_equity,
			total_liabilities, current_assets, current_liabilities,
			eps_basic, eps_diluted, shares_outstanding,
			roe, operating_margin, revenue_growth_yoy, eps_growth_yoy,
			fwd_per, cy_per,
			debt_ratio, roa, net_profit_margin, current_ratio,
			calc_status, calc_error, fetched_at, created_at, updated_at
		)
		SELECT
			symbol, data_source, fiscal_year, fiscal_period, report_code,
			period_end_date, filed_at,
			revenue, operating_income, net_income, total_assets, total_equity,
			total_liabilities, current_assets, current_liabilities,
			eps_basic, eps_diluted, shares_outstanding,
			roe, operating_margin, revenue_growth_yoy, eps_growth_yoy,
			fwd_per, cy_per,
			debt_ratio, roa, net_profit_margin, current_ratio,
			calc_status, calc_error, fetched_at, created_at, updated_at
		FROM old_stock_fundamentals
	`)
	if err != nil {
		return 0, fmt.Errorf("copy fundamentals data: %w", err)
	}
	copied, _ := res.RowsAffected()
	return copied, nil
}

// MigrateFundamentalsV2IfNeeded renames legacy stock_fundamentals (with stored PER/PBR)
// to old_stock_fundamentals and creates the v2 table without price-dependent columns.
func MigrateFundamentalsV2IfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	legacy, err := hasLegacyFundamentalsColumns(ctx, db)
	if err != nil {
		return err
	}
	oldExists, err := tableExists(ctx, db, "old_stock_fundamentals")
	if err != nil {
		return err
	}
	currentExists, err := tableExists(ctx, db, "stock_fundamentals")
	if err != nil {
		return err
	}

	if !legacy {
		if oldExists && !currentExists {
			log.Println("mysql: resuming stock_fundamentals v2 migration")
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			if err := createV2FundamentalsTable(ctx, tx, migrationsDir); err != nil {
				_ = tx.Rollback()
				return err
			}
			copied, err := copyFundamentalsV2(ctx, tx)
			if err != nil {
				_ = tx.Rollback()
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}
			log.Printf("mysql: copied %d rows to v2 stock_fundamentals (resume)", copied)
			return recalcFundamentalsStatuses(ctx, db)
		}
		if oldExists && currentExists {
			var newCount, oldCount int
			_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM stock_fundamentals`).Scan(&newCount)
			_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM old_stock_fundamentals`).Scan(&oldCount)
			if newCount == 0 && oldCount > 0 {
				log.Println("mysql: completing stock_fundamentals v2 data copy")
				copied, err := copyFundamentalsV2(ctx, db)
				if err != nil {
					return err
				}
				log.Printf("mysql: copied %d rows to v2 stock_fundamentals (backfill)", copied)
				return recalcFundamentalsStatuses(ctx, db)
			}
		}
		return nil
	}

	log.Println("mysql: migrating stock_fundamentals v1 -> v2 (drop stored PER/PBR)")

	if oldExists {
		return fmt.Errorf("old_stock_fundamentals already exists alongside legacy stock_fundamentals; manual cleanup required")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `RENAME TABLE stock_fundamentals TO old_stock_fundamentals`); err != nil {
		return fmt.Errorf("rename stock_fundamentals: %w", err)
	}

	if err := createV2FundamentalsTable(ctx, tx, migrationsDir); err != nil {
		return err
	}

	copied, err := copyFundamentalsV2(ctx, tx)
	if err != nil {
		return err
	}
	log.Printf("mysql: copied %d rows to v2 stock_fundamentals", copied)

	if err := tx.Commit(); err != nil {
		return err
	}
	return recalcFundamentalsStatuses(ctx, db)
}

func recalcFundamentalsStatuses(ctx context.Context, db *sql.DB) error {
	repo := NewFundamentalsRepo(db)
	updated, err := repo.RecalcAllStatuses(ctx)
	if err != nil {
		return fmt.Errorf("recalc fundamentals status: %w", err)
	}
	log.Printf("mysql: recalculated calc_status for %d rows (PER/PBR excluded from ok criteria)", updated)
	return nil
}
