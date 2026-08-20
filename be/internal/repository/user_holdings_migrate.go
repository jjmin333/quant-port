package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// MigrateUserHoldingsIfNeeded applies 014_user_holdings.sql when the table is missing.
func MigrateUserHoldingsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "user_holdings")
	if err != nil {
		return err
	}
	if !exists {
		path := filepath.Join(migrationsDir, "014_user_holdings.sql")
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read 014_user_holdings.sql: %w", err)
		}
		for _, stmt := range splitSQLStatements(string(body)) {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("014_user_holdings.sql: %w", err)
			}
		}
		log.Println("mysql: applied 014_user_holdings.sql")
	}
	return MigrateUserHoldingsAvgPriceIfNeeded(ctx, db, migrationsDir)
}

// MigrateUserHoldingsAvgPriceIfNeeded applies 015_user_holdings_avg_price.sql column additions.
func MigrateUserHoldingsAvgPriceIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	hasCol, err := columnExists(ctx, db, "user_holdings", "avg_price")
	if err != nil {
		return err
	}
	if hasCol {
		return nil
	}
	path := filepath.Join(migrationsDir, "015_user_holdings_avg_price.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 015_user_holdings_avg_price.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("015_user_holdings_avg_price.sql: %w", err)
		}
	}
	log.Println("mysql: applied 015_user_holdings_avg_price.sql")
	return nil
}