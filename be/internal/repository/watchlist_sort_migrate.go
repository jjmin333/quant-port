package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func MigrateWatchlistSortOrderIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	hasCol, err := columnExists(ctx, db, "watchlist_items", "sort_order")
	if err != nil {
		return err
	}
	if hasCol {
		return nil
	}
	path := filepath.Join(migrationsDir, "024_watchlist_sort_order.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 024_watchlist_sort_order.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("024_watchlist_sort_order.sql: %w", err)
		}
	}
	log.Println("mysql: applied 024_watchlist_sort_order.sql")
	return nil
}
