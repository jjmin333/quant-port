package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

/*
운영 사건 원장 (migrations/068).

설계의 이유는 `model.Incident` 머리글에 있다. 이 파일이 지키는 것은 그 규칙 하나다 —
**(kind, subject)당 열린 사건은 최대 하나.**

그 보장을 애플리케이션이 아니라 **DB가** 한다(`uniq_open`). 워치독은 15분마다 돌고
프로세스가 여럿이라, 「먼저 읽고 없으면 INSERT」는 두 회차가 겹치는 순간 두 행을
만든다. INSERT ... ON DUPLICATE KEY UPDATE 한 문장이면 그 창이 없다.
*/
type OpsIncidentRepo struct {
	db *sql.DB
}

func NewOpsIncidentRepo(db *sql.DB) *OpsIncidentRepo {
	return &OpsIncidentRepo{db: db}
}

// resolvedKeyOpen is the sentinel that keeps open rows in the unique index and
// closed rows out of each other's way. See migration 068's header.
const resolvedKeyOpen = "open"

/*
Observe — 「이 사건이 지금 보인다」. `opened`는 **이번에 처음 열렸다**는 뜻이다.

처음 보이면 행을 열고, 이미 열려 있으면 `occurrences`와 `last_seen_at`만 올린다.
**메시지와 detail은 최신으로 덮는다** — 사람이 보는 것은 「지금 어떤 상태인가」이고,
처음 본 순간의 문구는 first_seen_at 이 이미 말해 준다.

`severity`는 **올라가기만 한다.** warn으로 열린 사건이 critical이 되면 critical로
남는다 — 한 번 심각해진 사건이 다음 회차에 조용해 보이면 안 된다.
*/
func (r *OpsIncidentRepo) Observe(
	ctx context.Context,
	kind model.IncidentKind,
	subject, severity, message string,
	detail map[string]any,
) (opened bool, err error) {
	if r == nil || r.db == nil {
		return false, nil
	}
	subject = strings.TrimSpace(subject)
	if kind == "" || subject == "" {
		return false, nil
	}
	if severity == "" {
		severity = model.IncidentSeverityWarn
	}
	var payload any
	if len(detail) > 0 {
		raw, mErr := json.Marshal(detail)
		if mErr != nil {
			return false, fmt.Errorf("marshal incident detail: %w", mErr)
		}
		payload = string(raw)
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO ops_incidents
			(kind, subject, severity, message, detail, occurrences, resolved_key)
		VALUES (?, ?, ?, ?, ?, 1, ?)
		ON DUPLICATE KEY UPDATE
		  occurrences  = occurrences + 1,
		  last_seen_at = CURRENT_TIMESTAMP(3),
		  message      = VALUES(message),
		  detail       = VALUES(detail),
		  -- critical 로 한 번 올라간 사건은 내려오지 않는다
		  severity     = IF(severity = ? OR VALUES(severity) = ?, ?, VALUES(severity))`,
		kind, subject, severity, message, payload, resolvedKeyOpen,
		model.IncidentSeverityCritical, model.IncidentSeverityCritical, model.IncidentSeverityCritical,
	)
	if err != nil {
		return false, fmt.Errorf("observe incident %s/%s: %w", kind, subject, err)
	}
	/*
		**RowsAffected 로 「처음 열렸나」를 읽는다.**

		MySQL 의 INSERT ... ON DUPLICATE KEY UPDATE 는 새로 넣으면 1, 기존 행을
		바꾸면 2를 준다(값이 그대로면 0). 호출부가 이 값으로 로그를 정한다 — 새로
		열린 사건만 찍고, 15분마다 다시 보이는 사건은 조용히 카운터만 올린다.
	*/
	n, _ := res.RowsAffected()
	return n == 1, nil
}

/*
Resolve — 조건이 풀렸다.

닫으면서 `resolved_key`를 그 행의 id로 바꿔 유니크 인덱스에서 비켜 준다. 그래서 같은
(kind, subject)가 다시 깨지면 **새 행**이 열리고, 재발 횟수가 행 수로 세어진다.

이미 닫힌 사건에는 아무 일도 하지 않는다(영향 0행) — 성공이 이어지는 동안 워치독이
매 회차 부르기 때문이다.
*/
func (r *OpsIncidentRepo) Resolve(ctx context.Context, kind model.IncidentKind, subject string) (bool, error) {
	if r == nil || r.db == nil {
		return false, nil
	}
	subject = strings.TrimSpace(subject)
	if kind == "" || subject == "" {
		return false, nil
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE ops_incidents
		   SET resolved_at  = CURRENT_TIMESTAMP(3),
		       resolved_key = CAST(id AS CHAR)
		 WHERE kind = ? AND subject = ? AND resolved_key = ?`,
		kind, subject, resolvedKeyOpen,
	)
	if err != nil {
		return false, fmt.Errorf("resolve incident %s/%s: %w", kind, subject, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

/*
ListOpen — 지금 열려 있는 사건 전부. 오래 열려 있는 것이 위에 온다.

정렬을 last_seen_at 이 아니라 first_seen_at 으로 두는 이유: 방금 다시 보인 사건보다
**오래 방치된 사건**이 먼저 보여야 한다. 15분마다 다시 보이는 사건은 last_seen_at 으로
정렬하면 언제나 맨 위에 서서, 3일째 열려 있는 사건을 밀어낸다.
*/
func (r *OpsIncidentRepo) ListOpen(ctx context.Context, limit int) ([]model.Incident, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return r.query(ctx, `
		SELECT id, kind, subject, severity, message, detail, occurrences,
		       first_seen_at, last_seen_at, resolved_at
		  FROM ops_incidents
		 WHERE resolved_at IS NULL
		 ORDER BY severity = 'critical' DESC, first_seen_at ASC
		 LIMIT `+strconv.Itoa(limit))
}

// ListRecent returns the newest incidents regardless of state, for the
// "지난 며칠 무엇이 깨졌나" view.
func (r *OpsIncidentRepo) ListRecent(ctx context.Context, since time.Time, limit int) ([]model.Incident, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return r.query(ctx, `
		SELECT id, kind, subject, severity, message, detail, occurrences,
		       first_seen_at, last_seen_at, resolved_at
		  FROM ops_incidents
		 WHERE last_seen_at >= ?
		 ORDER BY resolved_at IS NULL DESC, last_seen_at DESC
		 LIMIT `+strconv.Itoa(limit), since)
}

func (r *OpsIncidentRepo) query(ctx context.Context, q string, args ...any) ([]model.Incident, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	var out []model.Incident
	for rows.Next() {
		var (
			it     model.Incident
			detail sql.NullString
			done   sql.NullTime
		)
		if err := rows.Scan(
			&it.ID, &it.Kind, &it.Subject, &it.Severity, &it.Message, &detail,
			&it.Occurrences, &it.FirstSeenAt, &it.LastSeenAt, &done,
		); err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		if detail.Valid && detail.String != "" {
			_ = json.Unmarshal([]byte(detail.String), &it.Detail)
		}
		if done.Valid {
			t := done.Time
			it.ResolvedAt = &t
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

/*
DeleteResolvedOlderThan — 닫힌 사건 정리.

열린 사건은 **지우지 않는다.** 아무리 오래됐어도 안 고쳐진 것이고, 오래됐다는 사실이
그 행의 값이다.
*/
func (r *OpsIncidentRepo) DeleteResolvedOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	if r == nil || r.db == nil || age <= 0 {
		return 0, nil
	}
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM ops_incidents WHERE resolved_at IS NOT NULL AND resolved_at < ?`,
		time.Now().Add(-age),
	)
	if err != nil {
		return 0, fmt.Errorf("purge incidents: %w", err)
	}
	return res.RowsAffected()
}

// MigrateOpsIncidentsIfNeeded creates the table on an existing database.
func MigrateOpsIncidentsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "ops_incidents")
	if err != nil || exists {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "068_ops_incidents.sql", "운영 사건 원장")
}
