package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

/*
「잘린 실행」 표식이 **쓰이고 읽히는지** 실제 DB에서 확인한다.

# 왜 DB가 필요한가

이 규칙은 Go 코드가 아니라 SQL 두 조각이다 — `MarkStaleRunsFailed`가 남기는
`JSON_OBJECT('interrupted', TRUE)`와 `LedgerRows`가 그것을 되읽는
`JSON_EXTRACT(...) = TRUE`. 둘 중 하나만 어긋나도 **코드는 멀쩡한 채로** 재처리가
조용히 멈춘다(고치기 전의 상태가 바로 그것이었다: 한 달 69건이 그냥 버려졌다).
계산식의 결과를 Go의 bool로 스캔하는 부분도 여기서만 시험된다.

# 왜 기본적으로 건너뛰는가

`BATCH_RUNS_TEST_DSN`이 없으면 skip 한다 — 나머지 repository 단위 시험은 DB 없이
돌아야 한다(ops_incidents_db_test.go와 같은 규칙).

	BATCH_RUNS_TEST_DSN='quantport:quantport@tcp(127.0.0.1:3306)/quantport?parseTime=true' \
	  go test ./internal/repository/ -run TestMarkStaleRunsMarksInterrupted -v

# 운영 행을 건드리지 않는가

`MarkStaleRunsFailed`는 **6시간 넘게 running인 행 전부**를 닫는다. 이 시험이 그걸
부르지만, 그런 행은 프로세스가 뜰 때마다 이미 정리되므로 평시에 0건이다(실측).
남는 위험은 「진짜로 6시간 넘게 도는 실행」인데, 그건 이 함수가 평소에 하는 일과
같고 워치독이 그전에 stuck으로 먼저 잡는다.
*/
const batchRunsTestJobID = "__test__interrupted_marker"

func TestMarkStaleRunsMarksInterrupted(t *testing.T) {
	dsn := os.Getenv("BATCH_RUNS_TEST_DSN")
	if dsn == "" {
		t.Skip("BATCH_RUNS_TEST_DSN 없음 — DB 시험은 건너뛴다")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	repo := &BatchJobRunRepo{db: db}

	cleanup := func() {
		if _, err := db.ExecContext(ctx,
			`DELETE FROM batch_job_runs WHERE job_id = ?`, batchRunsTestJobID); err != nil {
			t.Errorf("시험 행 정리 실패: %v", err)
		}
	}
	cleanup()
	// t.Cleanup이 아니라 defer다 — t.Cleanup은 위의 `defer db.Close()` **뒤에**
	// 돌아 닫힌 풀에 DELETE를 쏜다(2026-09-15에 그렇게 운영 표에 시험 행을 남겼다).
	defer cleanup()

	// 일곱 시간 전에 시작해 아직 running인 행 — 프로세스가 사라진 모습이다.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO batch_job_runs (job_id, status, trigger_kind, started_at)
		VALUES (?, 'running', 'schedule', DATE_SUB(NOW(), INTERVAL 7 HOUR))`,
		batchRunsTestJobID); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := repo.MarkStaleRunsFailed(ctx); err != nil {
		t.Fatalf("MarkStaleRunsFailed: %v", err)
	}

	rows, err := repo.LedgerRows(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("LedgerRows: %v", err)
	}
	row, ok := rows[batchRunsTestJobID]
	if !ok {
		t.Fatal("원장에서 시험 행을 못 찾았다")
	}
	if row.LastStatus != "error" {
		t.Fatalf("status = %q, error 여야 한다", row.LastStatus)
	}
	if !row.LastInterrupted {
		t.Fatal("잘린 실행인데 LastInterrupted 가 false다 — 재처리가 이 행을 건너뛴다")
	}
}

// 정상적으로 실패한 실행에는 표식이 없어야 한다 — 있으면 원인 모를 실패를
// 무한히 다시 돌리게 된다.
func TestGenuineFailureHasNoInterruptedMark(t *testing.T) {
	dsn := os.Getenv("BATCH_RUNS_TEST_DSN")
	if dsn == "" {
		t.Skip("BATCH_RUNS_TEST_DSN 없음 — DB 시험은 건너뛴다")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	repo := &BatchJobRunRepo{db: db}
	id := batchRunsTestJobID + "_genuine"

	cleanup := func() {
		if _, err := db.ExecContext(ctx, `DELETE FROM batch_job_runs WHERE job_id = ?`, id); err != nil {
			t.Errorf("시험 행 정리 실패: %v", err)
		}
	}
	cleanup()
	defer cleanup()

	if _, err := db.ExecContext(ctx, `
		INSERT INTO batch_job_runs (job_id, status, trigger_kind, message, finished_at)
		VALUES (?, 'error', 'schedule', 'toss api 500 maintenance', NOW())`, id); err != nil {
		t.Fatalf("insert: %v", err)
	}

	rows, err := repo.LedgerRows(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("LedgerRows: %v", err)
	}
	if rows[id].LastInterrupted {
		t.Fatal("진짜 실패에 잘림 표식이 붙었다")
	}
}
