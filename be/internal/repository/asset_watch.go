package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

/*
관심 부동산 · 글 안의 자산 태그 (098). 표를 왜 이렇게 두는지는 `migrations/098_asset_watch_mentions.sql`.
*/

func MigrateAssetWatchIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "content_asset_mentions")
	if err != nil || exists {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "098_asset_watch_mentions.sql", "관심 부동산·자산 태그")
}

// ReWatchMax — 한 사람이 담을 수 있는 관심 단지·지역 수.
const ReWatchMax = 50

// ReWatch — 관심 단지·지역 한 줄.
type ReWatch struct {
	Kind      string    `json:"kind"`
	Key       string    `json:"key"`
	LawdCd    string    `json:"lawdCd"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type AssetWatchRepo struct{ db *sql.DB }

func NewAssetWatchRepo(db *sql.DB) *AssetWatchRepo { return &AssetWatchRepo{db: db} }

func (r *AssetWatchRepo) List(ctx context.Context, userID int64) ([]ReWatch, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT kind, skey, lawd_cd, name, created_at FROM re_watchlist
		WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReWatch{}
	for rows.Next() {
		var w ReWatch
		if err := rows.Scan(&w.Kind, &w.Key, &w.LawdCd, &w.Name, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// Add stores one watch. false = 이미 상한(ReWatchMax)이라 넣지 않았다(이미 담긴 것은 true).
func (r *AssetWatchRepo) Add(ctx context.Context, userID int64, w ReWatch) (bool, error) {
	var n int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM re_watchlist WHERE user_id = ?`, userID).Scan(&n); err != nil {
		return false, err
	}
	if n >= ReWatchMax {
		var one int
		err := r.db.QueryRowContext(ctx, `SELECT 1 FROM re_watchlist WHERE user_id = ? AND kind = ? AND skey = ?`, userID, w.Kind, w.Key).Scan(&one)
		return err == nil, nil
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO re_watchlist (user_id, kind, skey, lawd_cd, name) VALUES (?,?,?,?,?)
		ON DUPLICATE KEY UPDATE name = VALUES(name)`, userID, w.Kind, w.Key, w.LawdCd, w.Name)
	return err == nil, err
}

func (r *AssetWatchRepo) Remove(ctx context.Context, userID int64, kind, key string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM re_watchlist WHERE user_id = ? AND kind = ? AND skey = ?`, userID, kind, key)
	return err
}

// ── 글 안의 자산 태그 ────────────────────────────────────────────────────────

// AssetMentionRow — 저장된 자산 태그 하나.
type AssetMentionRow struct {
	Kind   string
	Key    string
	Name   string
	Names  []string
	Snap   map[string]float64
	SnapAt time.Time
}

func (r *AssetWatchRepo) LoadMentions(ctx context.Context, targetType string, targetID int64) ([]AssetMentionRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT kind, skey, name, names, snap, snap_at FROM content_asset_mentions
		WHERE target_type = ? AND target_id = ?`, targetType, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AssetMentionRow
	for rows.Next() {
		var (
			m           AssetMentionRow
			names, snap []byte
		)
		if err := rows.Scan(&m.Kind, &m.Key, &m.Name, &names, &snap, &m.SnapAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(names, &m.Names)
		_ = json.Unmarshal(snap, &m.Snap)
		out = append(out, m)
	}
	return out, rows.Err()
}

// MentionID — 자산 태그의 표 안 이름(kind|key).
func MentionID(kind, key string) string { return kind + "|" + key }

/*
ReplaceMentions — 본문의 자산 태그 집합으로 맞춘다. 이미 있던 태그는 스냅샷을 지키고(이름만 갱신),
새 태그만 fresh에서 넣고, 빠진 태그는 지운다 — 종목 태그(MentionRepo.Replace)와 같은 규칙.
*/
func (r *AssetWatchRepo) ReplaceMentions(ctx context.Context, targetType string, targetID int64,
	all map[string]AssetMentionRow, fresh map[string]bool) error {
	if r == nil || r.db == nil {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	keep := []string{}
	for id, m := range all {
		keep = append(keep, id)
		nb, _ := json.Marshal(m.Names)
		if !fresh[id] {
			if _, err := tx.ExecContext(ctx, `UPDATE content_asset_mentions SET names = ?, name = ?
				WHERE target_type = ? AND target_id = ? AND kind = ? AND skey = ?`, nb, m.Name, targetType, targetID, m.Kind, m.Key); err != nil {
				return err
			}
			continue
		}
		snap := m.Snap
		if snap == nil {
			snap = map[string]float64{}
		}
		sb, _ := json.Marshal(snap)
		if _, err := tx.ExecContext(ctx, `INSERT INTO content_asset_mentions (target_type, target_id, kind, skey, name, names, snap)
			VALUES (?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE names = VALUES(names), name = VALUES(name)`,
			targetType, targetID, m.Kind, m.Key, m.Name, nb, sb); err != nil {
			return err
		}
	}
	del := `DELETE FROM content_asset_mentions WHERE target_type = ? AND target_id = ?`
	args := []any{targetType, targetID}
	if len(keep) > 0 {
		del += ` AND CONCAT(kind, '|', skey) NOT IN (` + strings.TrimSuffix(strings.Repeat("?,", len(keep)), ",") + `)`
		for _, k := range keep {
			args = append(args, k)
		}
	}
	if _, err := tx.ExecContext(ctx, del, args...); err != nil {
		return err
	}
	return tx.Commit()
}

// MentionIDs — 이미 저장된 자산 태그(kind|key) — 새로 얼릴 것을 고르려고.
func (r *AssetWatchRepo) MentionIDs(ctx context.Context, targetType string, targetID int64) (map[string]bool, error) {
	out := map[string]bool{}
	if r == nil || r.db == nil || targetID == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT kind, skey FROM content_asset_mentions WHERE target_type = ? AND target_id = ?`, targetType, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, s string
		if err := rows.Scan(&k, &s); err != nil {
			return nil, err
		}
		out[MentionID(k, s)] = true
	}
	return out, rows.Err()
}
