package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
	return r.EnqueueAs(ctx, jobID, TriggerManual, params)
}

// 원장의 trigger_kind 값. 화면과 로그가 이 셋을 갈라 읽는다.
const (
	// TriggerSchedule — 워커가 자기 시간표대로 돌린 것.
	TriggerSchedule = "schedule"
	// TriggerManual — 사람이 관리자 화면에서 누른 것.
	TriggerManual = "manual"
	/*
		TriggerCatchUp — 다운 등으로 **놓친 슬롯**을 워치독이 뒤늦게 채운 것.

		manual과 갈라 두는 이유가 둘이다. 화면에서 「내가 누른 적 없는 manual 실행」이
		보이면 사람이 원인을 잘못 짚고, 사후에 「이 실행이 정상 스케줄이었나 복구였나」를
		구분할 수 없으면 다운의 영향 범위를 되짚을 수 없다.
	*/
	TriggerCatchUp = "catchup"
)

// EnqueueAs records a queued run with an explicit trigger kind.
func (r *BatchJobRunRepo) EnqueueAs(ctx context.Context, jobID, trigger string, params map[string]any) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	if trigger == "" {
		trigger = TriggerManual
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
		INSERT INTO batch_job_runs (job_id, status, trigger_kind, params) VALUES (?, 'queued', ?, ?)`,
		jobID, trigger, paramsJSON)
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
	// SKIP LOCKED — 남이 집은 행을 기다리지 않고 건너뛴다.
	//
	// 큐에서 기다릴 이유가 없다. 그리고 기다리면 워커끼리 교착한다 — 2026-08-23에
	// 큐 워커가 셋이 되면서 실제로 났다(Error 1213). 짝이 되는 인덱스
	// (job_id, status, id)는 migrations/058에 있다.
	err = tx.QueryRowContext(ctx, `
		SELECT id, params FROM batch_job_runs
		 WHERE job_id = ? AND status = 'queued'
		 ORDER BY id ASC LIMIT 1 FOR UPDATE SKIP LOCKED`, jobID).Scan(&id, &paramsJSON)
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

/*
BatchLedgerRow — 한 잡의 **마지막 기록 한 줄**과 마지막 성공 시각.

감시가 필요로 하는 전부다. 잡마다 `Latest`+`LastSuccess`를 따로 부르면 잡 수만큼
질의가 두 배로 나가는데, 운영 DB가 1 ECPU라 15분마다 그럴 이유가 없다.
*/
type BatchLedgerRow struct {
	JobID         string
	LastStatus    string
	LastRunAt     *time.Time
	LastFinishAt  *time.Time
	LastMessage   string
	LastSuccessAt *time.Time
}

/*
LedgerRows — 잡별 마지막 기록을 **한 번의 질의로** 모은다.

윈도 함수 두 개를 같은 스캔에 얹는다: `ROW_NUMBER`가 마지막 행을 고르고,
`MAX(CASE WHEN done ...)`가 같은 파티션에서 마지막 성공 시각을 가져온다.

30일로 자르는 이유: 그보다 오래 안 돈 잡은 「언제 마지막이었나」가 아니라 「한 번도
안 돌았다」로 다뤄야 하고, 그 판정에는 정확한 날짜가 필요 없다. 스캔 범위를 묶어
두면 원장이 몇 년치로 자라도 이 질의의 비용은 그대로다.
*/
func (r *BatchJobRunRepo) LedgerRows(ctx context.Context, since time.Duration) (map[string]BatchLedgerRow, error) {
	if r == nil || r.db == nil {
		return map[string]BatchLedgerRow{}, nil
	}
	if since <= 0 {
		since = 30 * 24 * time.Hour
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT job_id, status, started_at, finished_at, message, last_success
		  FROM (
		        SELECT job_id, status, started_at, finished_at, message,
		               ROW_NUMBER() OVER (PARTITION BY job_id ORDER BY started_at DESC) AS rn,
		               MAX(CASE WHEN status = 'done' THEN started_at END)
		                 OVER (PARTITION BY job_id) AS last_success
		          FROM batch_job_runs
		         WHERE started_at >= DATE_SUB(NOW(), INTERVAL ? SECOND)
		       ) t
		 WHERE rn = 1`, int64(since.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("ledger rows: %w", err)
	}
	defer rows.Close()

	out := map[string]BatchLedgerRow{}
	for rows.Next() {
		var (
			row      BatchLedgerRow
			finished sql.NullTime
			message  sql.NullString
			success  sql.NullTime
			started  time.Time
		)
		if err := rows.Scan(&row.JobID, &row.LastStatus, &started, &finished, &message, &success); err != nil {
			return nil, fmt.Errorf("ledger rows scan: %w", err)
		}
		row.LastRunAt = &started
		if finished.Valid {
			t := finished.Time
			row.LastFinishAt = &t
		}
		if message.Valid {
			row.LastMessage = message.String
		}
		if success.Valid {
			t := success.Time
			row.LastSuccessAt = &t
		}
		out[row.JobID] = row
	}
	return out, rows.Err()
}

/*
DeleteRunsOlderThan — 원장 보존 기간.

예약 실행이 전부 원장에 남게 되면서(2026-09-09) 하루 행 수가 20여 개에서 160여 개로
늘었다 — 뉴스 브리핑이 시장별로 슬롯마다, 워치독이 15분마다 남긴다. 그대로 두면
연 6만 행이고, 그 대부분은 「그날 정상이었다」는 사실뿐이라 오래 갖고 있을 이유가 없다.
*/
func (r *BatchJobRunRepo) DeleteRunsOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	if r == nil || r.db == nil || age <= 0 {
		return 0, nil
	}
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM batch_job_runs
		 WHERE status IN ('done','error')
		   AND started_at < DATE_SUB(NOW(), INTERVAL ? SECOND)`, int64(age.Seconds()))
	if err != nil {
		return 0, fmt.Errorf("delete old runs: %w", err)
	}
	return res.RowsAffected()
}

/*
staleRunAge — running 행을 죽은 것으로 보기까지 기다리는 시간.

가장 오래 도는 잡이 실측 2.5시간(분기 재무 국내)이고 그다음이 2시간(종목 경고)이라,
6시간이면 살아 있는 실행을 죽었다고 부를 일이 없다. 그 사이의 침묵은 워치독이
`stuck`으로 따로 잡는다(maxRuntime).
*/
const staleRunAge = 6 * time.Hour

/*
MarkStaleRunsFailed closes out runs left "running" by a crash, so the console
does not show a job as perpetually in flight.

# 왜 나이 조건이 필요한가

예전에는 조건 없이 `WHERE status = 'running'` 전부를 error로 바꿨다. 주석에는 「이
프로세스가 남긴 것뿐」이라고 적혀 있었지만 **사실이 아니다** — 잡들은 각자 다른 워커
컨테이너에서 돌고 원장은 하나다. API만 재기동해도 다른 컨테이너에서 **한창 돌고 있는**
실행이 실패로 기록된다.

2026-09-09에 실제로 그랬다: 분기 재무 국내가 도는 중에 API를 다시 올렸더니 그 실행이
「프로세스가 재시작되어…」로 죽은 것처럼 남았다. 화면만 틀리는 게 아니라 감시도 함께
속는다 — 성공으로 끝난 실행이 실패로 보이면 그 잡은 다음 판정에서 failed가 된다.
*/
func (r *BatchJobRunRepo) MarkStaleRunsFailed(ctx context.Context) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE batch_job_runs
		   SET status = 'error',
		       message = '프로세스가 재시작되어 결과를 확인할 수 없습니다',
		       finished_at = CURRENT_TIMESTAMP(3)
		 WHERE status = 'running'
		   AND started_at < DATE_SUB(NOW(), INTERVAL ? SECOND)`, int64(staleRunAge.Seconds()))
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

	// 큐 클레임 인덱스 — 워커끼리의 데드락을 막는다(migrations/058 헤더 참조).
	// params 컬럼 유무와 독립이라 그 분기보다 앞에 둔다.
	hasClaimIdx, err := indexExists(ctx, db, "batch_job_runs", "idx_batch_job_runs_claim")
	if err != nil {
		return err
	}
	if !hasClaimIdx {
		if err := applyMigrationFile(ctx, db, migrationsDir,
			"058_batch_queue_claim_index.sql", "batch queue claim index"); err != nil {
			return err
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
