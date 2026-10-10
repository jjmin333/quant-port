package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// EarningsDigestRepo owns earnings_digests — 실적 화면 맨 위 AI 요약, 시장별 최신 1행 (2026-10-09 요청 1).
type EarningsDigestRepo struct {
	db *sql.DB
}

func NewEarningsDigestRepo(db *sql.DB) *EarningsDigestRepo { return &EarningsDigestRepo{db: db} }

// EarningsDigestRow is one market's stored digest.
type EarningsDigestRow struct {
	Market      string          `json:"market"`
	Body        json.RawMessage `json:"body"`
	Facts       json.RawMessage `json:"facts"`
	Source      string          `json:"source"`
	Model       string          `json:"model,omitempty"`
	InputHash   string          `json:"-"`
	Tokens      int64           `json:"-"`
	GeneratedAt time.Time       `json:"generatedAt"`
}

// Get returns the stored digest, or nil when the batch has never run for that market.
func (r *EarningsDigestRepo) Get(ctx context.Context, market string) (*EarningsDigestRow, error) {
	var row EarningsDigestRow
	var body, facts string
	err := r.db.QueryRowContext(ctx, `
		SELECT market, body, facts, source, model, input_hash, tokens, generated_at
		  FROM earnings_digests WHERE market = ?`, market).
		Scan(&row.Market, &body, &facts, &row.Source, &row.Model, &row.InputHash, &row.Tokens, &row.GeneratedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	row.Body, row.Facts = json.RawMessage(body), json.RawMessage(facts)
	return &row, nil
}

// Upsert replaces one market's digest.
func (r *EarningsDigestRepo) Upsert(ctx context.Context, row EarningsDigestRow) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO earnings_digests (market, body, facts, source, model, input_hash, tokens, generated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP(3))
		ON DUPLICATE KEY UPDATE body = VALUES(body), facts = VALUES(facts), source = VALUES(source),
		  model = VALUES(model), input_hash = VALUES(input_hash), tokens = VALUES(tokens),
		  generated_at = CURRENT_TIMESTAMP(3)`,
		row.Market, string(row.Body), string(row.Facts), row.Source, row.Model, row.InputHash, row.Tokens)
	return err
}
