package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// extendedMigrations — base RequiredTables 이후 추가된 테이블.
// EnsureSchema는 RequiredTables만 보므로, 기존 DB에는 별도 적용이 필요하다.
var extendedMigrations = []struct {
	table string
	file  string
}{
	{"user_portfolio_cache", "007_user_portfolio_cache.sql"},
	{"market_fx_rates", "008_market_fx_rates.sql"},
	{"market_indices", "009_market_indices.sql"},
	{"news_articles", "010_news_articles.sql"},
	{"quote_snapshots", "011_quote_terminal_persist.sql"},
}

// MigrateExtendedSchemaIfNeeded applies 007–010 when individual tables are missing.
func MigrateExtendedSchemaIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	for _, m := range extendedMigrations {
		exists, err := tableExists(ctx, db, m.table)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		path := filepath.Join(migrationsDir, m.file)
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", m.file, err)
		}
		for _, stmt := range splitSQLStatements(string(body)) {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("%s: %w", m.file, err)
			}
		}
		log.Printf("mysql: applied %s", m.file)
	}
	return nil
}
