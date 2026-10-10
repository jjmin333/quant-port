package repository

import (
	"context"
	"database/sql"
)

// MigrateGrowthIfNeeded creates the worker-growth tables on an existing database.
// 표를 읽고 쓰는 코드는 `internal/growth`에 있다 — 그 워커 말고는 아무도 이 표를 쓰지 않는다.
func MigrateGrowthIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "growth_reports")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir, "087_growth.sql", "검색 유입 감시")
}
