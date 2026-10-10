package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

/*
보유 종목의 매수 이유(살 때 본 축). 표를 왜 따로 두는지는 `migrations/083_holding_theses.sql`.
*/

// HoldingThesisRepo owns holding_theses.
type HoldingThesisRepo struct {
	db *sql.DB
}

func NewHoldingThesisRepo(db *sql.DB) *HoldingThesisRepo { return &HoldingThesisRepo{db: db} }

// Set replaces one holding's thesis; empty axes deletes it.
func (r *HoldingThesisRepo) Set(ctx context.Context, userID int64, symbol string, axes []string) error {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if len(axes) == 0 {
		_, err := r.db.ExecContext(ctx, `DELETE FROM holding_theses WHERE user_id = ? AND symbol = ?`, userID, symbol)
		return err
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO holding_theses (user_id, symbol, axes) VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE axes = VALUES(axes)`, userID, symbol, strings.Join(axes, ","))
	if err != nil {
		return fmt.Errorf("holding thesis set: %w", err)
	}
	return nil
}

// MigrateHoldingThesesIfNeeded creates the table on an existing database.
func MigrateHoldingThesesIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "holding_theses")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir, "083_holding_theses.sql", "보유 종목 매수 이유")
}
