package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

/*
사건 원장의 **수명주기**를 실제 DB에서 확인한다.

# 왜 DB가 필요한가

이 표가 지키는 규칙은 애플리케이션이 아니라 **DB가** 보장한다 — (kind, subject)당
열린 사건 하나는 `uniq_open` 인덱스와 `INSERT ... ON DUPLICATE KEY UPDATE` 한 문장의
성질이고, 그 둘 중 하나만 틀려도 코드는 멀쩡한 채로 원장이 거짓을 말한다. 목 객체로는
그 규칙을 시험할 수 없다.

# 왜 기본적으로 건너뛰는가

`OPS_INCIDENT_TEST_DSN`이 없으면 skip 한다. 단위 시험은 DB 없이 돌아야 하고(나머지
repository 시험이 전부 순수 로직이다), 이 시험은 **사람이 의심할 때** 돌리는 것이다:

	OPS_INCIDENT_TEST_DSN='quantport:quantport@tcp(127.0.0.1:3306)/quantport?parseTime=true' \
	  go test ./internal/repository/ -run TestOpsIncidentLifecycle -v

# 남기지 않는다

subject 에 시험 전용 접두사를 붙이고 끝에서 지운다 — 운영 원장에 시험 행이 남으면
그 표를 보는 사람이 제일 먼저 속는다.
*/

const opsIncidentTestSubject = "__test__/ops-incident-lifecycle"

func openIncidentTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("OPS_INCIDENT_TEST_DSN")
	if dsn == "" {
		t.Skip("OPS_INCIDENT_TEST_DSN 미설정 — DB 시험을 건너뛴다 (머리글 참조)")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM ops_incidents WHERE subject = ?`, opsIncidentTestSubject)
		_ = db.Close()
	})
	_, _ = db.Exec(`DELETE FROM ops_incidents WHERE subject = ?`, opsIncidentTestSubject)
	return db
}

func TestOpsIncidentLifecycle(t *testing.T) {
	db := openIncidentTestDB(t)
	repo := NewOpsIncidentRepo(db)
	ctx := context.Background()
	const kind = model.IncidentBatchMissed

	// ── 1. 처음 보이면 열린다
	opened, err := repo.Observe(ctx, kind, opsIncidentTestSubject, model.IncidentSeverityWarn, "첫 관측", nil)
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if !opened {
		t.Fatal("처음 관측인데 opened=false — 호출부가 새 사건을 못 알아본다")
	}

	// ── 2. 다시 보여도 **행은 하나**고 occurrences 만 오른다
	for i := 0; i < 3; i++ {
		opened, err = repo.Observe(ctx, kind, opsIncidentTestSubject, model.IncidentSeverityWarn, "다시 관측", nil)
		if err != nil {
			t.Fatalf("observe again: %v", err)
		}
		if opened {
			t.Fatal("이어지는 관측을 새 사건으로 봤다 — 로그가 15분마다 시끄러워진다")
		}
	}
	open, err := repo.ListOpen(ctx, 50)
	if err != nil {
		t.Fatalf("list open: %v", err)
	}
	mine := filterTestIncidents(open)
	if len(mine) != 1 {
		t.Fatalf("열린 사건이 %d개 — (kind,subject)당 하나여야 한다", len(mine))
	}
	if mine[0].Occurrences != 4 {
		t.Fatalf("occurrences=%d, want 4", mine[0].Occurrences)
	}
	if mine[0].Message != "다시 관측" {
		t.Fatalf("메시지는 최신으로 덮여야 한다: %q", mine[0].Message)
	}

	// ── 3. severity 는 올라가기만 한다
	if _, err := repo.Observe(ctx, kind, opsIncidentTestSubject, model.IncidentSeverityCritical, "심각", nil); err != nil {
		t.Fatalf("escalate: %v", err)
	}
	if _, err := repo.Observe(ctx, kind, opsIncidentTestSubject, model.IncidentSeverityWarn, "다시 경고", nil); err != nil {
		t.Fatalf("de-escalate attempt: %v", err)
	}
	open, _ = repo.ListOpen(ctx, 50)
	mine = filterTestIncidents(open)
	if len(mine) != 1 || mine[0].Severity != model.IncidentSeverityCritical {
		t.Fatalf("critical 이 warn 으로 내려갔다: %+v", mine)
	}

	// ── 4. 닫으면 열린 목록에서 빠진다. 두 번 닫아도 아무 일 없다
	resolved, err := repo.Resolve(ctx, kind, opsIncidentTestSubject)
	if err != nil || !resolved {
		t.Fatalf("resolve: %v (resolved=%v)", err, resolved)
	}
	again, err := repo.Resolve(ctx, kind, opsIncidentTestSubject)
	if err != nil {
		t.Fatalf("resolve twice: %v", err)
	}
	if again {
		t.Fatal("이미 닫힌 사건을 다시 닫았다 — 워치독이 매 회차 부르므로 0행이어야 한다")
	}
	open, _ = repo.ListOpen(ctx, 50)
	if n := len(filterTestIncidents(open)); n != 0 {
		t.Fatalf("닫았는데 열린 목록에 %d개 남았다", n)
	}

	// ── 5. **재발은 새 행이다** — 그래야 횟수가 세어진다
	opened, err = repo.Observe(ctx, kind, opsIncidentTestSubject, model.IncidentSeverityWarn, "재발", nil)
	if err != nil {
		t.Fatalf("recur: %v", err)
	}
	if !opened {
		t.Fatal("재발이 새 사건으로 열리지 않았다 — 닫힌 행을 되살리면 지속 시간이 거짓이 된다")
	}
	recent, err := repo.ListRecent(ctx, time.Now().Add(-time.Hour), 50)
	if err != nil {
		t.Fatalf("list recent: %v", err)
	}
	if n := len(filterTestIncidents(recent)); n != 2 {
		t.Fatalf("이력에 %d행 — 닫힌 것 1 + 열린 것 1 이어야 한다", n)
	}
}

func filterTestIncidents(in []model.Incident) []model.Incident {
	var out []model.Incident
	for _, it := range in {
		if it.Subject == opsIncidentTestSubject {
			out = append(out, it)
		}
	}
	return out
}
