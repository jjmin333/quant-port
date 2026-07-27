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

var authRequiredTables = []string{
	"user_broker_credentials",
	"user_refresh_tokens",
}

// MigrateAuthIfNeeded applies 006_auth.sql and alters legacy tables for auth columns.
func MigrateAuthIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	missing, err := missingTables(ctx, db, authRequiredTables)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		path := filepath.Join(migrationsDir, "006_auth.sql")
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read 006_auth.sql: %w", err)
		}
		for _, stmt := range splitSQLStatements(string(body)) {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("006_auth.sql: %w", err)
			}
		}
		log.Println("mysql: applied 006_auth.sql")
	}

	alters := []string{
		`ALTER TABLE users ADD COLUMN password_hash VARCHAR(255) NULL AFTER display_name`,
		`ALTER TABLE users ADD COLUMN last_login_at TIMESTAMP NULL AFTER updated_at`,
		`ALTER TABLE broker_accounts ADD COLUMN account_seq BIGINT NULL AFTER account_name`,
		`ALTER TABLE broker_accounts ADD COLUMN account_no VARCHAR(64) NULL AFTER account_seq`,
		`ALTER TABLE broker_accounts ADD COLUMN account_type VARCHAR(32) NULL AFTER account_no`,
		`ALTER TABLE broker_accounts ADD COLUMN is_primary TINYINT(1) NOT NULL DEFAULT 0 AFTER account_type`,
		`ALTER TABLE broker_accounts ADD COLUMN synced_at TIMESTAMP NULL AFTER is_primary`,
		`ALTER TABLE portfolio_snapshots ADD COLUMN user_id BIGINT UNSIGNED NOT NULL DEFAULT 1 AFTER id`,
		`CREATE INDEX idx_portfolio_snapshots_user_recorded ON portfolio_snapshots (user_id, recorded_at)`,
	}
	for _, stmt := range alters {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if isIgnorableAlterErr(err) {
				continue
			}
			return fmt.Errorf("auth alter: %w", err)
		}
	}
	return nil
}

func isIgnorableAlterErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column") ||
		strings.Contains(msg, "duplicate key name") ||
		strings.Contains(msg, "already exists")
}
