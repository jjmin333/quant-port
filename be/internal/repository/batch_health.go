package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

/*
배치 감시 보고서 한 행 (migrations/067).

`computed_at`이 이 표의 핵심이다 — 판정 내용이 아니라 **워치독이 살아 있는지**를
말한다. 죽은 프로세스는 자기가 죽었다고 쓰지 못하므로, 침묵을 보는 방법은 마지막
기록의 나이를 재는 것뿐이다.
*/
type BatchHealthRepo struct {
	db *sql.DB
}

func NewBatchHealthRepo(db *sql.DB) *BatchHealthRepo {
	return &BatchHealthRepo{db: db}
}

// Save replaces the single stored report.
func (r *BatchHealthRepo) Save(ctx context.Context, report model.BatchHealthReport) error {
	if r == nil || r.db == nil {
		return nil
	}
	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal batch health: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO batch_health (id, unhealthy, payload, computed_at)
		VALUES (1, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  unhealthy = VALUES(unhealthy),
		  payload = VALUES(payload),
		  computed_at = VALUES(computed_at)`,
		report.Unhealthy, string(payload), report.ComputedAt)
	if err != nil {
		return fmt.Errorf("save batch health: %w", err)
	}
	return nil
}

// LastComputedAt reports when the watchdog last finished a pass. ok=false means
// it has never run (a fresh deployment, or a watchdog that has never started).
func (r *BatchHealthRepo) LastComputedAt(ctx context.Context) (time.Time, bool, error) {
	if r == nil || r.db == nil {
		return time.Time{}, false, nil
	}
	var at time.Time
	err := r.db.QueryRowContext(ctx, `SELECT computed_at FROM batch_health WHERE id = 1`).Scan(&at)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("read batch health: %w", err)
	}
	return at, true, nil
}

// MigrateBatchHealthIfNeeded creates the table on an existing database.
func MigrateBatchHealthIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "batch_health")
	if err != nil || exists {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "067_batch_health.sql", "배치 감시 보고서")
}
