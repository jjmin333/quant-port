package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// MentionRepo stores 글 안의 데이터 태그 (092_content_mentions.sql).
type MentionRepo struct{ db *sql.DB }

func NewMentionRepo(db *sql.DB) *MentionRepo { return &MentionRepo{db: db} }

// MentionStock is one resolvable stock.
type MentionStock struct {
	Symbol    string
	Name      string
	AssetType string
}

// MentionLookup — 이름(그대로)·티커(대문자)로 찾는 두 표.
type MentionLookup struct {
	ByName   map[string]MentionStock
	BySymbol map[string]MentionStock
}

// Find — 티커가 먼저, 없으면 한글명.
func (l MentionLookup) Find(c string) (MentionStock, bool) {
	if s, ok := l.BySymbol[strings.ToUpper(c)]; ok {
		return s, true
	}
	s, ok := l.ByName[c]
	return s, ok
}

/*
ResolveNames — 이름 후보를 종목으로. 같은 이름이 둘이면 국내 → 짧은 티커 순(「삼성전자」가
해외 DR에 가지 않게).
*/
func (r *MentionRepo) ResolveNames(ctx context.Context, candidates []string) (MentionLookup, error) {
	out := MentionLookup{ByName: map[string]MentionStock{}, BySymbol: map[string]MentionStock{}}
	if r == nil || r.db == nil || len(candidates) == 0 {
		return out, nil
	}
	seen := map[string]bool{}
	var names, syms []any
	for _, c := range candidates {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		names = append(names, c)
		syms = append(syms, strings.ToUpper(c))
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(names)), ",")
	args := append(append([]any{}, names...), syms...)
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, name, asset_type FROM stocks
		 WHERE status = 'NORMAL' AND (name IN (`+marks+`) OR symbol IN (`+marks+`))
		 ORDER BY asset_type = 'domestic' DESC, CHAR_LENGTH(symbol), symbol`, args...)
	if err != nil {
		return out, fmt.Errorf("mention resolve: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var s MentionStock
		if err := rows.Scan(&s.Symbol, &s.Name, &s.AssetType); err != nil {
			return out, err
		}
		out.BySymbol[s.Symbol] = s
		if _, ok := out.ByName[s.Name]; !ok {
			out.ByName[s.Name] = s
		}
	}
	return out, rows.Err()
}

// MentionRow is one stored mention with the stock's current name.
type MentionRow struct {
	Symbol    string
	Name      string
	AssetType string
	Names     []string
	Snap      model.MentionSnap
	SnapAt    time.Time
}

// Load returns a target's mentions in insertion-agnostic symbol order.
func (r *MentionRepo) Load(ctx context.Context, targetType string, targetID int64) ([]MentionRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.symbol, COALESCE(s.name, m.symbol), COALESCE(s.asset_type, ''), m.names, m.snap, m.snap_at
		  FROM content_mentions m
		  LEFT JOIN stocks s ON s.symbol = m.symbol
		 WHERE m.target_type = ? AND m.target_id = ?`, targetType, targetID)
	if err != nil {
		return nil, fmt.Errorf("mention load: %w", err)
	}
	defer rows.Close()
	var out []MentionRow
	for rows.Next() {
		var (
			m           MentionRow
			names, snap []byte
		)
		if err := rows.Scan(&m.Symbol, &m.Name, &m.AssetType, &names, &snap, &m.SnapAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(names, &m.Names)
		_ = json.Unmarshal(snap, &m.Snap)
		out = append(out, m)
	}
	return out, rows.Err()
}

/*
Replace — 본문이 가리키는 종목 집합으로 맞춘다. 이미 있는 종목은 **스냅샷을 지킨다**(이름만 갱신) —
글을 고쳐도 「그때」 값은 처음 태그한 순간이다. 새 종목만 snaps에서 넣고, 빠진 종목은 지운다.
*/
func (r *MentionRepo) Replace(
	ctx context.Context, targetType string, targetID int64,
	names map[string][]string, snaps map[string]model.MentionSnap,
) error {
	if r == nil || r.db == nil {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	keep := make([]any, 0, len(names))
	for sym, ns := range names {
		keep = append(keep, sym)
		nb, _ := json.Marshal(ns)
		snap, isNew := snaps[sym]
		if !isNew {
			if _, err := tx.ExecContext(ctx, `UPDATE content_mentions SET names = ?
				 WHERE target_type = ? AND target_id = ? AND symbol = ?`, nb, targetType, targetID, sym); err != nil {
				return err
			}
			continue
		}
		sb, _ := json.Marshal(snap)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO content_mentions (target_type, target_id, symbol, names, snap)
			VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE names = VALUES(names)`, targetType, targetID, sym, nb, sb); err != nil {
			return err
		}
	}
	del := `DELETE FROM content_mentions WHERE target_type = ? AND target_id = ?`
	args := []any{targetType, targetID}
	if len(keep) > 0 {
		del += ` AND symbol NOT IN (` + strings.TrimSuffix(strings.Repeat("?,", len(keep)), ",") + `)`
		args = append(args, keep...)
	}
	if _, err := tx.ExecContext(ctx, del, args...); err != nil {
		return err
	}
	return tx.Commit()
}

// Symbols returns which of the given symbols a target already has (to decide what to snapshot).
func (r *MentionRepo) Symbols(ctx context.Context, targetType string, targetID int64) (map[string]bool, error) {
	out := map[string]bool{}
	if r == nil || r.db == nil || targetID == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT symbol FROM content_mentions WHERE target_type = ? AND target_id = ?`, targetType, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out[s] = true
	}
	return out, rows.Err()
}
