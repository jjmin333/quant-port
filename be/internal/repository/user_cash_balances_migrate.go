package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func MigrateUserCashBalancesIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "user_cash_balances")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	path := filepath.Join(migrationsDir, "041_user_cash_balances.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 041_user_cash_balances.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("041_user_cash_balances.sql: %w", err)
		}
	}
	log.Println("mysql: applied 041_user_cash_balances.sql")
	return nil
}
