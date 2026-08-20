package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/enact/quant-port/be/internal/model"
)

// BatchJobRunRepo persists one row per batch execution so the admin console can
// answer "did it run, and what did it produce" after the process restarts.
type BatchJobRunRepo struct {
	db *sql.DB
}

func NewBatchJobRunRepo(db *sql.DB) *BatchJobRunRepo {
	return &BatchJobRunRepo{db: db}
}

// Begin records a run as started and returns its id. Used by a worker that
// runs a job on its own internal schedule (trigger_kind="schedule"), where
// there is no separate enqueue step.
func (r *BatchJobRunRepo) Begin(ctx context.Context, jobID, trigger string) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	if trigger == "" {
		trigger = "manual"
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO batch_job_runs (job_id, status, trigger_kind) VALUES (?, 'running', ?)`,
		jobID, trigger)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Enqueue records a manually requested run as queued and returns its id.
//
// This is the only thing the API process does for a manual trigger now — the
// job itself runs in a separate worker process, which polls ClaimNext for its
// own job_id. Decoupling this way means a batch that hangs or leaks memory
// can never take the request-serving API down with it.
func (r *BatchJobRunRepo) Enqueue(ctx context.Context, jobID string, params map[string]any) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	var paramsJSON any
	if len(params) > 0 {
		b, err := json.Marshal(params)
		if err != nil {
			return 0, err
		}
		paramsJSON = string(b)
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO batch_job_runs (job_id, status, trigger_kind, params) VALUES (?, 'queued', 'manual', ?)`,
		jobID, paramsJSON)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// HasPending reports whether a job already has a queued or running row, so the
// API does not pile up duplicate enqueues while a worker is still catching up.
func (r *BatchJobRunRepo) HasPending(ctx context.Context, jobID string) (bool, error) {
	if r == nil || r.db == nil {
		return false, nil
	}
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM batch_job_runs WHERE job_id = ? AND status IN ('queued', 'running')`,
		jobID).Scan(&n)
	return n > 0, err
}

// ClaimNext atomically takes the oldest queued run for one job and marks it
// running, returning its id and params. ok=false means nothing was waiting.
//
// The UPDATE ... ORDER BY ... LIMIT form is MySQL-specific but is exactly what
// makes this safe without a separate locking round trip: only one row can ever
// match "status='queued' ORDER BY id LIMIT 1" at the instant the statement
// runs, so two workers polling the same job_id (which should not happen, but
// isn't assumed away) cannot both claim it.
func (r *BatchJobRunRepo) ClaimNext(ctx context.Context, jobID string) (id int64, params map[string]any, ok bool, err error) {
	if r == nil || r.db == nil {
		return 0, nil, false, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, nil, false, err
	}
	defer func() { _ = tx.Rollback() }()

	var paramsJSON sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT id, params FROM batch_job_runs
		 WHERE job_id = ? AND status = 'queued'
		 ORDER BY id ASC LIMIT 1 FOR UPDATE`, jobID).Scan(&id, &paramsJSON)
	if err == sql.ErrNoRows {
		return 0, nil, false, nil
	}
	if err != nil {
		return 0, nil, false, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE batch_job_runs SET status = 'running' WHERE id = ?`, id); err != nil {
		return 0, nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return 0, nil, false, err
	}
	if paramsJSON.Valid && paramsJSON.String != "" {
		_ = json.Unmarshal([]byte(paramsJSON.String), &params)
	}
	return id, params, true, nil
}

// Finish closes out a run. Called from a detached goroutine, so it takes its own
// context rather than the request's.
func (r *BatchJobRunRepo) Finish(ctx context.Context, id int64, status, message, result string) error {
	if r == nil || r.db == nil || id == 0 {
		return nil
	}
	var payload any
	if result != "" && result != "null" {
		payload = result
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE batch_job_runs
		   SET status = ?, message = ?, result = ?, finished_at = CURRENT_TIMESTAMP(3)
		 WHERE id = ?`, status, message, payload, id)
	return err
}

const batchRunCols = `id, job_id, status, trigger_kind, started_at, finished_at, message, result`

func scanBatchRun(rows *sql.Rows) (model.BatchJobRun, error) {
	var (
		run        model.BatchJobRun
		jobID      string
		finished   sql.NullTime
		message    sql.NullString
		resultJSON sql.NullString
	)
	if err := rows.Scan(&run.ID, &jobID, &run.Status, &run.TriggerKind,
		&run.StartedAt, &finished, &message, &resultJSON); err != nil {
		return run, err
	}
	run.JobID = model.BatchJobID(jobID)
	if finished.Valid {
		t := finished.Time
		run.FinishedAt = &t
	}
	run.Message = message.String
	if resultJSON.Valid && resultJSON.String != "" {
		_ = json.Unmarshal([]byte(resultJSON.String), &run.Result)
	}
	return run, nil
}

// Latest returns one job's most recent run.
func (r *BatchJobRunRepo) Latest(ctx context.Context, jobID string) (*model.BatchJobRun, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+batchRunCols+` FROM batch_job_runs
		 WHERE job_id = ? ORDER BY started_at DESC LIMIT 1`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	run, err := scanBatchRun(rows)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// LastSuccess returns the most recent run that actually finished as done.
//
// Latest는 실패·큐 대기까지 포함해서 "마지막으로 건드린 시각"을 보여 주지만,
// catch-up은 "마지막으로 성공한 적재"가 지난 슬롯보다 오래됐는지만 본다.
// 실패한 실행을 성공으로 치면 구멍을 메우지 않은 채 다음 슬롯까지 기다린다.
func (r *BatchJobRunRepo) LastSuccess(ctx context.Context, jobID string) (*model.BatchJobRun, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+batchRunCols+` FROM batch_job_runs
		 WHERE job_id = ? AND status = 'done'
		 ORDER BY COALESCE(finished_at, started_at) DESC LIMIT 1`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	run, err := scanBatchRun(rows)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// Recent returns runs across all jobs, newest first.
func (r *BatchJobRunRepo) Recent(ctx context.Context, limit int) ([]model.BatchJobRun, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+batchRunCols+` FROM batch_job_runs
		 ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.BatchJobRun, 0, limit)
	for rows.Next() {
		run, err := scanBatchRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// MarkStaleRunsFailed closes out runs left "running" by a crash or restart, so
// the console does not show a job as perpetually in flight.
func (r *BatchJobRunRepo) MarkStaleRunsFailed(ctx context.Context) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE batch_job_runs
		   SET status = 'error',
		       message = '프로세스가 재시작되어 결과를 확인할 수 없습니다',
		       finished_at = CURRENT_TIMESTAMP(3)
		 WHERE status = 'running'`)
	return err
}

// MigrateBatchJobRunsIfNeeded applies 032_batch_job_runs.sql and
// 033_batch_job_runs_queue.sql.
func MigrateBatchJobRunsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "batch_job_runs")
	if err != nil {
		return err
	}
	if !exists {
		path := filepath.Join(migrationsDir, "032_batch_job_runs.sql")
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read 032_batch_job_runs.sql: %w", err)
		}
		for _, stmt := range splitSQLStatements(string(body)) {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("032_batch_job_runs.sql: %w", err)
			}
		}
	}

	hasParams, err := columnExists(ctx, db, "batch_job_runs", "params")
	if err != nil {
		return err
	}
	if hasParams {
		return nil
	}
	path := filepath.Join(migrationsDir, "033_batch_job_runs_queue.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 033_batch_job_runs_queue.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("033_batch_job_runs_queue.sql: %w", err)
		}
	}
	return nil
}
