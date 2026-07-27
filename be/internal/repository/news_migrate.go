package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// MigrateNewsIfNeeded applies 010_news_articles.sql when the table is missing.
func MigrateNewsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "news_articles")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	path := filepath.Join(migrationsDir, "010_news_articles.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 010_news_articles.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("010_news_articles.sql: %w", err)
		}
	}
	log.Println("mysql: applied 010_news_articles.sql")
	return nil
}
