package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// MigrateNewsPreviewIfNeeded applies 013_news_preview.sql column additions.
func MigrateNewsPreviewIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	hasCol, err := columnExists(ctx, db, "news_articles", "original_url")
	if err != nil {
		return err
	}
	if hasCol {
		return MigrateNewsTranslationIfNeeded(ctx, db, migrationsDir)
	}

	path := filepath.Join(migrationsDir, "013_news_preview.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 013_news_preview.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("013_news_preview.sql: %w", err)
		}
	}
	log.Println("mysql: applied 013_news_preview.sql")
	return MigrateNewsTranslationIfNeeded(ctx, db, migrationsDir)
}

// MigrateNewsTranslationIfNeeded applies 018_news_translation.sql column additions.
func MigrateNewsTranslationIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	hasCol, err := columnExists(ctx, db, "news_articles", "headline_ko")
	if err != nil {
		return err
	}
	if hasCol {
		return MigrateNewsReaderTranslationIfNeeded(ctx, db, migrationsDir)
	}

	path := filepath.Join(migrationsDir, "018_news_translation.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 018_news_translation.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("018_news_translation.sql: %w", err)
		}
	}
	log.Println("mysql: applied 018_news_translation.sql")
	return MigrateNewsReaderTranslationIfNeeded(ctx, db, migrationsDir)
}

// MigrateNewsReaderTranslationIfNeeded applies 019_news_reader_translation.sql.
func MigrateNewsReaderTranslationIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "news_reader_translations")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	path := filepath.Join(migrationsDir, "019_news_reader_translation.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 019_news_reader_translation.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("019_news_reader_translation.sql: %w", err)
		}
	}
	log.Println("mysql: applied 019_news_reader_translation.sql")
	return nil
}

func columnExists(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	var name string
	err := db.QueryRowContext(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?
	`, table, column).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

/*
MigrateNewsTranslateAttemptsIfNeeded adds the retry counter on an existing database.

컬럼이 이미 있으면 아무것도 안 한다 — 새 DB는 마이그레이션 파일 전체를 순서대로
돌리므로 이 함수는 **운영 중인 DB**만을 위한 길이다(다른 Migrate*IfNeeded 와 같다).
*/
func MigrateNewsTranslateAttemptsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	has, err := columnExists(ctx, db, "news_articles", "translate_attempts")
	if err != nil || has {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "071_news_translate_attempts.sql", "번역 재시도 횟수")
}
