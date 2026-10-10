package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

/*
부동산·금융상품 점수와 AI 해설(097). 표를 왜 이렇게 두는지는 `migrations/097_asset_intel.sql`.
*/

func MigrateAssetIntelIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "fin_rate_history")
	if err != nil || exists {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "097_asset_intel.sql", "자산 점수·AI")
}

// AssetScore — 점수 한 줄.
type AssetScore struct {
	Kind  string          `json:"kind"`
	Key   string          `json:"key"`
	Name  string          `json:"name"`
	Group string          `json:"group"`
	AsOf  string          `json:"asOf"`
	Score int             `json:"score"`
	Grade string          `json:"grade"`
	Parts json.RawMessage `json:"parts"`
	Facts json.RawMessage `json:"facts"`
}

type AssetIntelRepo struct {
	db *sql.DB
}

func NewAssetIntelRepo(db *sql.DB) *AssetIntelRepo { return &AssetIntelRepo{db: db} }

// ReplaceScores swaps every score of a kind (이번 회차에 없는 키는 지운다 — 사라진 상품·단지가 남지 않게).
func (r *AssetIntelRepo) ReplaceScores(ctx context.Context, kind string, list []AssetScore) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM asset_scores WHERE kind = ?`, kind); err != nil {
		return err
	}
	for start := 0; start < len(list); start += 300 {
		end := min(start+300, len(list))
		var sb strings.Builder
		sb.WriteString(`INSERT INTO asset_scores (kind, skey, name, grp, as_of, score, grade, parts, facts) VALUES `)
		args := make([]any, 0, (end-start)*9)
		for i, s := range list[start:end] {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?,?,?,?,?,?)")
			args = append(args, kind, trimRunes(s.Key, 120), trimRunes(s.Name, 120), trimRunes(s.Group, 20), s.AsOf,
				s.Score, s.Grade, string(s.Parts), string(s.Facts))
		}
		if _, err := tx.ExecContext(ctx, sb.String(), args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const scoreCols = `kind, skey, name, grp, DATE_FORMAT(as_of, '%Y-%m-%d'), score, grade, parts, facts`

func scanScore(s interface{ Scan(...any) error }) (AssetScore, error) {
	var a AssetScore
	var parts, facts []byte
	err := s.Scan(&a.Kind, &a.Key, &a.Name, &a.Group, &a.AsOf, &a.Score, &a.Grade, &parts, &facts)
	a.Parts, a.Facts = json.RawMessage(parts), json.RawMessage(facts)
	return a, err
}

// Score returns one score (nil when none).
func (r *AssetIntelRepo) Score(ctx context.Context, kind, key string) (*AssetScore, error) {
	a, err := scanScore(r.db.QueryRowContext(ctx, `SELECT `+scoreCols+` FROM asset_scores WHERE kind = ? AND skey = ?`, kind, key))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// Scores returns many keys of a kind at once (목록에 점수 붙이기).
func (r *AssetIntelRepo) Scores(ctx context.Context, kind string, keys []string) (map[string]AssetScore, error) {
	out := map[string]AssetScore{}
	for start := 0; start < len(keys); start += 500 {
		end := min(start+500, len(keys))
		args := []any{kind}
		for _, k := range keys[start:end] {
			args = append(args, k)
		}
		rows, err := r.db.QueryContext(ctx, `SELECT `+scoreCols+` FROM asset_scores WHERE kind = ? AND skey IN (`+placeholders(end-start)+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			a, err := scanScore(rows)
			if err != nil {
				rows.Close()
				return nil, err
			}
			out[a.Key] = a
		}
		rows.Close()
	}
	return out, nil
}

// TopScores — kind의 점수 높은 순(grp가 비지 않으면 그 묶음만). prefix는 grp 앞부분(시도 코드 11 등).
func (r *AssetIntelRepo) TopScores(ctx context.Context, kind, grp, grpPrefix, order string, limit int) ([]AssetScore, error) {
	q := `SELECT ` + scoreCols + ` FROM asset_scores WHERE kind = ?`
	args := []any{kind}
	if grp != "" {
		q += ` AND grp = ?`
		args = append(args, grp)
	} else if grpPrefix != "" {
		q += ` AND grp LIKE ?`
		args = append(args, strings.NewReplacer("%", "", "_", "").Replace(grpPrefix)+"%")
	}
	// order는 facts 안의 숫자 칸 이름(화이트리스트는 핸들러가). 앞에 "-"면 작은 순(값 없는 줄은 뺀다).
	switch {
	case order == "" || order == "score":
		q += ` ORDER BY score DESC`
	case strings.HasPrefix(order, "-"):
		q += ` AND JSON_EXTRACT(facts, CONCAT('$.', ?)) IS NOT NULL ORDER BY CAST(JSON_EXTRACT(facts, CONCAT('$.', ?)) AS DOUBLE) ASC`
		args = append(args, order[1:], order[1:])
	default:
		q += ` ORDER BY CAST(JSON_EXTRACT(facts, CONCAT('$.', ?)) AS DOUBLE) DESC`
		args = append(args, order)
	}
	rows, err := r.db.QueryContext(ctx, q+` LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AssetScore{}
	for rows.Next() {
		a, err := scanScore(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AINote — AI 해설 한 줄.
type AINote struct {
	Kind      string          `json:"kind"`
	Key       string          `json:"key"`
	InputHash string          `json:"-"`
	Body      json.RawMessage `json:"body"`
	Source    string          `json:"source"`
	Model     string          `json:"model,omitempty"`
	Tokens    int             `json:"-"`
	UpdatedAt string          `json:"updatedAt"`
}

func (r *AssetIntelRepo) UpsertNote(ctx context.Context, n AINote) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO asset_ai_notes (kind, skey, input_hash, body, source, model, tokens)
		VALUES (?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE input_hash = VALUES(input_hash), body = VALUES(body),
		source = VALUES(source), model = VALUES(model), tokens = VALUES(tokens)`,
		n.Kind, trimRunes(n.Key, 120), n.InputHash, string(n.Body), n.Source, trimRunes(n.Model, 60), n.Tokens)
	return err
}

// NoteHashes — 이미 있는 해설의 입력 해시와 출처(같으면 다시 부르지 않는다).
func (r *AssetIntelRepo) NoteHashes(ctx context.Context, kind string) (map[string][2]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT skey, input_hash, source FROM asset_ai_notes WHERE kind = ?`, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][2]string{}
	for rows.Next() {
		var k, h, src string
		if err := rows.Scan(&k, &h, &src); err != nil {
			return nil, err
		}
		out[k] = [2]string{h, src}
	}
	return out, rows.Err()
}

// Notes returns notes of a kind for keys (nil keys = 전부).
func (r *AssetIntelRepo) Notes(ctx context.Context, kind string, keys []string) (map[string]AINote, error) {
	q := `SELECT kind, skey, body, source, model, DATE_FORMAT(updated_at, '%Y-%m-%d %H:%i') FROM asset_ai_notes WHERE kind = ?`
	args := []any{kind}
	if keys != nil {
		if len(keys) == 0 {
			return map[string]AINote{}, nil
		}
		q += ` AND skey IN (` + placeholders(len(keys)) + `)`
		for _, k := range keys {
			args = append(args, k)
		}
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]AINote{}
	for rows.Next() {
		var n AINote
		var body []byte
		if err := rows.Scan(&n.Kind, &n.Key, &body, &n.Source, &n.Model, &n.UpdatedAt); err != nil {
			return nil, err
		}
		n.Body = json.RawMessage(body)
		out[n.Key] = n
	}
	return out, rows.Err()
}

// UpsertRateHistory writes one month's rate per product.
func (r *AssetIntelRepo) UpsertRateHistory(ctx context.Context, productID int64, month string, base, max *float64) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO fin_rate_history (product_id, dcls_month, base_rate, max_rate) VALUES (?,?,?,?)
		ON DUPLICATE KEY UPDATE base_rate = VALUES(base_rate), max_rate = VALUES(max_rate)`, productID, month, base, max)
	return err
}

// PrevRates — 상품마다 지금 공시월 바로 전 공시월의 금리.
func (r *AssetIntelRepo) PrevRates(ctx context.Context) (map[int64][2]*float64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT h.product_id, h.base_rate, h.max_rate FROM fin_rate_history h
		JOIN fin_products p ON p.id = h.product_id
		WHERE h.dcls_month = (SELECT MAX(h2.dcls_month) FROM fin_rate_history h2 WHERE h2.product_id = h.product_id AND h2.dcls_month < p.dcls_month)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][2]*float64{}
	for rows.Next() {
		var id int64
		var b, m sql.NullFloat64
		if err := rows.Scan(&id, &b, &m); err != nil {
			return nil, err
		}
		out[id] = [2]*float64{nullF(b), nullF(m)}
	}
	return out, rows.Err()
}

// ScoreName — 이름 색인 한 줄(자산 태그 해석·자동완성, 2026-10-03). Extra = facts의 한 칸(상품은 금융회사).
type ScoreName struct {
	Key   string
	Name  string
	Group string
	Score int
	Extra string
}

// ScoreNames — kind 전체의 이름 색인. extraPath가 비지 않으면 facts의 그 칸(문자열)을 Extra로.
func (r *AssetIntelRepo) ScoreNames(ctx context.Context, kind, extraPath string) ([]ScoreName, error) {
	q := `SELECT skey, name, grp, score, '' FROM asset_scores WHERE kind = ?`
	args := []any{kind}
	if extraPath != "" {
		q = `SELECT skey, name, grp, score, COALESCE(JSON_UNQUOTE(JSON_EXTRACT(facts, CONCAT('$.', ?))), '') FROM asset_scores WHERE kind = ?`
		args = []any{extraPath, kind}
	}
	rows, err := r.db.QueryContext(ctx, q+` ORDER BY score DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScoreName
	for rows.Next() {
		var s ScoreName
		if err := rows.Scan(&s.Key, &s.Name, &s.Group, &s.Score, &s.Extra); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
