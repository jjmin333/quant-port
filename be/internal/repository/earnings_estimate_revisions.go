package repository

import (
	"context"
	"database/sql"
	"fmt"
)

/*
미국 EPS 컨센서스 변화 이력. 왜 「바뀐 날만」 넣는지는 `migrations/085_earnings_estimate_revisions.sql`.
*/

/*
SnapshotEstimates — 오늘 예정된(발표 전) 미국 행의 eps_est를, **직전 기록과 다를 때만** 남긴다.

한 문장이다: 예정 약 3,600행을 이력의 마지막 값과 맞대 보고 다른 것만 넣는다. 같은 날 두 번
돌면(수동 재실행) 그날 행을 덮는다.
*/
func (r *EarningsEventRepo) SnapshotEstimates(ctx context.Context, snapDate string) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO earnings_estimate_revisions (symbol, fiscal_year, fiscal_period, snap_date, eps_est)
		SELECT e.symbol, e.fiscal_year, e.fiscal_period, ?, e.eps_est
		  FROM earnings_events e
		 WHERE e.market = 'us' AND e.date_status = 'scheduled'
		   AND e.eps_est IS NOT NULL AND e.event_date >= ?
		   AND NOT EXISTS (
		     SELECT 1 FROM earnings_estimate_revisions r
		      WHERE r.symbol = e.symbol AND r.fiscal_year = e.fiscal_year AND r.fiscal_period = e.fiscal_period
		        AND r.snap_date = (
		          SELECT MAX(r2.snap_date) FROM earnings_estimate_revisions r2
		           WHERE r2.symbol = e.symbol AND r2.fiscal_year = e.fiscal_year
		             AND r2.fiscal_period = e.fiscal_period AND r2.snap_date < ?)
		        AND r.eps_est = e.eps_est)
		ON DUPLICATE KEY UPDATE eps_est = VALUES(eps_est)`, snapDate, snapDate, snapDate)
	if err != nil {
		return 0, fmt.Errorf("earnings estimate snapshot: %w", err)
	}
	return res.RowsAffected()
}

// MigrateEarningsEstimateRevisionsIfNeeded creates the table on an existing database.
func MigrateEarningsEstimateRevisionsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "earnings_estimate_revisions")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir, "085_earnings_estimate_revisions.sql", "미국 EPS 컨센서스 변화 이력")
}
