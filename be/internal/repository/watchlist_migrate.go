package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func MigrateWatchlistAddedPriceIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	hasCol, err := columnExists(ctx, db, "watchlist_items", "added_price")
	if err != nil {
		return err
	}
	if hasCol {
		return nil
	}
	path := filepath.Join(migrationsDir, "023_watchlist_added_price.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 023_watchlist_added_price.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("023_watchlist_added_price.sql: %w", err)
		}
	}
	log.Println("mysql: applied 023_watchlist_added_price.sql")
	return nil
}
