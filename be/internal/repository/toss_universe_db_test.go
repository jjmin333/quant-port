package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

/*
2026-09-12 실사고의 회귀선 — **「전부 숨김」은 절대 커밋되지 않는다.**

무슨 일이 있었나: `ApplyUniverse`의 두 문장(「받은 것을 1로」·「나머지를 0으로」)이
트랜잭션 밖에 있었고, 01:00 예약 실행 중에 배포로 워커가 재시작됐다. 두 번째 문장만
남은 상태 — 17,918종목 전부 `toss_tradable=0` — 가 되어 **검색이 통째로 비었다**
(AAPL 조회도 0건).

지금은 두 가지가 막는다: 한 트랜잭션이라 커밋 전에 죽으면 예전 상태가 남고,
커밋 직전에 「보이는 종목 수」를 세어 문턱 아래면 롤백한다.

DB가 필요하므로 기본은 skip 한다:

	OPS_INCIDENT_TEST_DSN='quantport:quantport@tcp(127.0.0.1:3306)/quantport?parseTime=true' \
	  go test ./internal/repository/ -run TestApplyUniverseNeverHidesEverything -v
*/
func TestApplyUniverseNeverHidesEverything(t *testing.T) {
	dsn := os.Getenv("OPS_INCIDENT_TEST_DSN")
	if dsn == "" {
		t.Skip("OPS_INCIDENT_TEST_DSN 미설정 — DB 시험을 건너뛴다")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	repo := NewTossUniverseRepo(db)
	ctx := context.Background()

	before := visibleCount(t, db)
	if before < minUniverse {
		t.Skipf("현재 보이는 종목이 %d건 — 이 시험의 전제(정상 상태)가 아니다", before)
	}

	/*
		존재하지 않는 심볼 1,500개로 반영을 시도한다.

		입력 검증(1,000건 이상)은 통과하지만 **실제로 1이 되는 행이 하나도 없으므로**
		두 번째 문장이 전부를 0으로 만들려 한다. 커밋 직전 검사가 그것을 잡아야 한다.
	*/
	ghosts := make([]string, 1500)
	for i := range ghosts {
		ghosts[i] = "__NOSUCH__" + string(rune('A'+i%26)) + string(rune('A'+(i/26)%26)) + string(rune('A'+(i/676)%26))
	}
	_, _, err = repo.ApplyUniverse(ctx, ghosts, true)
	if err == nil {
		t.Fatal("전 종목을 숨기는 반영이 커밋됐다")
	}

	after := visibleCount(t, db)
	if after != before {
		t.Fatalf("롤백되지 않았다: 보이는 종목 %d → %d", before, after)
	}
}

func visibleCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM stocks
		 WHERE status = 'NORMAL' AND (toss_tradable IS NULL OR toss_tradable = 1)`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}
