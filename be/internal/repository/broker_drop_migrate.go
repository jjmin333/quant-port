package repository

import (
	"context"
	"database/sql"
	"log"
)

var brokerLinkingTables = []string{
	"user_portfolio_cache",
	"user_broker_credentials",
	"broker_accounts",
	"portfolio_snapshots",
}

// MigrateDropBrokerLinkingIfNeeded drops per-user Toss account-linking tables.
// Market-data Toss credentials (env TOSS_PRIMARY_* / TOSS_SECONDARY_*) are not stored here.
func MigrateDropBrokerLinkingIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	var any bool
	for _, table := range brokerLinkingTables {
		exists, err := tableExists(ctx, db, table)
		if err != nil {
			return err
		}
		if exists {
			any = true
			break
		}
	}
	if !any {
		return nil
	}
	if err := applyMigrationFile(ctx, db, migrationsDir, "054_drop_broker_linking.sql", "토스 계좌 연동 테이블 제거"); err != nil {
		return err
	}
	log.Println("mysql: dropped Toss account-linking tables")
	return nil
}
