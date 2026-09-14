package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/textutil"
)

// syncNotFoundCooldown is how long a symbol the upstream simply does not carry
// (no EDGAR CIK, no DART corp_code) is left alone. Without it the rotation
// would spend a slot on the same absent ticker every cycle — US preferred/unit
// tickers alone are hundreds of rows in our catalog.
const syncNotFoundCooldown = 30 * 24 * time.Hour

// SyncStateRepo is the shared "who do we fetch next" table for every
// per-symbol ingest batch (내부자 거래 · 분기 재무 · 종목 경고).
//
// 배치마다 이 로직을 복제하지 않는 이유: 아래 SelectTargets의 ORDER BY가
// **등급제의 전부**다. 복제하면 우선순위 규칙이 잡마다 조용히 갈라진다.
type SyncStateRepo struct {
	db *sql.DB
}

func NewSyncStateRepo(db *sql.DB) *SyncStateRepo {
	return &SyncStateRepo{db: db}
}

// SyncOutcome is what a batch reports back after handling one symbol.
type SyncOutcome struct {
	Kind       string
	Symbol     string
	Status     string
	Message    string
	RowsStored int
	ItemsSeen  int
}

// Mark records an attempt. Called for failures too — last_synced_at is the
// rotation cursor, and a symbol that errors must still move to the back of the
// queue or one broken ticker blocks the batch forever.
func (r *SyncStateRepo) Mark(ctx context.Context, out SyncOutcome) error {
	if r == nil || r.db == nil {
		return nil
	}
	// message 컬럼은 VARCHAR(255) — MySQL도 여기도 **문자**로 센다. 바이트로 자르면
	// 상한에 걸린 한글이 반쪽만 저장돼 관리자 화면과 로그에서 깨진다(textutil 참조).
	message := textutil.TruncateRunes(out.Message, 255)
	success := out.Status == model.SyncOK || out.Status == model.SyncEmpty
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO symbol_sync_state
			(kind, symbol, last_synced_at, last_success_at, status, message, rows_stored, items_seen)
		VALUES (?, ?, NOW(), IF(?, NOW(), NULL), ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  last_synced_at = NOW(),
		  last_success_at = IF(VALUES(last_success_at) IS NULL, last_success_at, NOW()),
		  status = VALUES(status),
		  message = VALUES(message),
		  rows_stored = VALUES(rows_stored),
		  items_seen = VALUES(items_seen)`,
		out.Kind, strings.ToUpper(strings.TrimSpace(out.Symbol)),
		success, out.Status, message, out.RowsStored, out.ItemsSeen)
	return err
}

// WasSynced reports whether this kind of batch ever successfully visited symbol.
//
// 화면이 "내역 없음"과 "아직 수집 전"을 구분하기 위한 한 줄이다. 이 구분이
// 없으면 배치가 아직 못 온 종목이 "해당 내역이 없는 종목"처럼 보인다.
func (r *SyncStateRepo) WasSynced(ctx context.Context, kind, symbol string) (bool, error) {
	if r == nil || r.db == nil {
		return false, nil
	}
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM symbol_sync_state
		WHERE kind = ? AND symbol = ? AND last_success_at IS NOT NULL`,
		kind, strings.ToUpper(strings.TrimSpace(symbol)),
	).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// SelectTargets picks the next `limit` symbols for one batch.
//
// 등급제의 구현이 이 한 개의 ORDER BY다.
//
//	0순위 — 관심·보유 종목 중 staleAfter보다 오래된 것 (사용자가 실제로 보는 것)
//	1순위 — 한 번도 받지 않은 종목
//	2순위 — 나머지를 오래된 것부터 (요일 로테이션이 아니라 자연 순환)
//
// 로테이션을 요일로 쪼개지 않은 이유: 한도가 바뀌거나 종목 수가 늘면 요일
// 배분표를 다시 짜야 하는데, "가장 오래 안 본 것부터"는 limit만 바꾸면 알아서
// 주기가 조절된다.
//
// staleAfter를 0으로 주면 **전 종목 매일** 모드다 — 관심·보유 우선순위가 사라지고
// 마지막 수집이 오래된 순서만 남는다. 갱신이 잦은 데이터(종목 경고)가 이 모드를
// 쓴다.
func (r *SyncStateRepo) SelectTargets(
	ctx context.Context,
	kind string,
	assetTypes []string,
	staleAfter time.Duration,
	limit int,
) ([]string, error) {
	if r == nil || r.db == nil || limit <= 0 || len(assetTypes) == 0 {
		return nil, nil
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(assetTypes)), ",")
	args := make([]any, 0, len(assetTypes)+4)
	args = append(args, kind)
	for _, t := range assetTypes {
		args = append(args, t)
	}
	args = append(args, time.Now().Add(-syncNotFoundCooldown))

	priority := `
		  CASE
		    WHEN EXISTS (SELECT 1 FROM watchlist_items w WHERE w.symbol = s.symbol)
		      OR EXISTS (SELECT 1 FROM user_holdings h WHERE h.symbol = s.symbol)
		    THEN IF(st.last_synced_at IS NULL OR st.last_synced_at < ?, 0, 2)
		    ELSE IF(st.last_synced_at IS NULL, 1, 2)
		  END,`
	if staleAfter <= 0 {
		// 전 종목 매일 모드: 우선순위 없이 오래된 순서 하나로 돈다.
		priority = ""
	} else {
		args = append(args, time.Now().Add(-staleAfter))
	}
	args = append(args, limit)

	/*
		**토스가 취급하지 않는 종목은 수집하지 않는다** (2026-09-12 사용자 결정).

		그 종목은 검색에서도 빠지므로(`TossTradableFilter`) 화면에 설 자리가 없는데,
		예전에는 배치가 계속 그것을 받으려 했다 — 실측으로 경고 스윕 호출의 19%
		(17,918 중 3,388)가 그 종목들의 `404 stock-not-found`였다.

		이 한 줄이 종목 단위 적재 **전부**에 걸린다(경고·내부자·분기 재무) — 이 함수가
		「누구를 다음에 받을까」의 유일한 자리이기 때문이다(이 파일 머리글).

		조건이 `= 1`이 아니라 `IS NULL OR = 1`인 이유는 NULL이 「아직 확인 안 했다」라서다.
		유니버스 배치가 처음 돌기 전에 전부 건너뛰면 그날 수집이 통째로 멈춘다.
	*/
	query := `
		SELECT s.symbol
		FROM stocks s
		LEFT JOIN symbol_sync_state st
		       ON st.symbol = s.symbol AND st.kind = ?
		WHERE s.status = 'NORMAL'
		  AND s.asset_type IN (` + placeholders + `)
		  AND ` + TossTradableFilter("s") + `
		  AND (st.status IS NULL OR st.status <> 'not_found' OR st.last_synced_at < ?)
		ORDER BY` + priority + `
		  st.last_synced_at ASC,
		  s.symbol ASC
		LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0, limit)
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, err
		}
		out = append(out, symbol)
	}
	return out, rows.Err()
}

// Coverage summarises adoption for the admin console: how many symbols of a
// market this batch has visited, and how fresh the oldest visit is.
func (r *SyncStateRepo) Coverage(ctx context.Context, kind string, assetTypes []string) (map[string]any, error) {
	if r == nil || r.db == nil || len(assetTypes) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(assetTypes)), ",")
	args := make([]any, 0, len(assetTypes)+1)
	args = append(args, kind)
	for _, t := range assetTypes {
		args = append(args, t)
	}

	var (
		total, synced, withRows, errored int
		oldest                           sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT
		  COUNT(*),
		  COUNT(st.symbol),
		  COALESCE(SUM(IF(st.status = 'ok', 1, 0)), 0),
		  COALESCE(SUM(IF(st.status = 'error', 1, 0)), 0),
		  MIN(st.last_synced_at)
		FROM stocks s
		LEFT JOIN symbol_sync_state st ON st.symbol = s.symbol AND st.kind = ?
		WHERE s.status = 'NORMAL' AND s.asset_type IN (`+placeholders+`)`,
		args...,
	).Scan(&total, &synced, &withRows, &errored, &oldest)
	if err != nil {
		return nil, err
	}

	out := map[string]any{
		"symbols":  total,
		"synced":   synced,
		"withData": withRows,
		"errors":   errored,
	}
	if oldest.Valid {
		out["oldestSyncedAt"] = oldest.Time.Format(time.RFC3339)
	}
	return out, nil
}
