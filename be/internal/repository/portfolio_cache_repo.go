package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var ErrPortfolioCacheMiss = errors.New("portfolio cache miss")

type PortfolioCacheRecord struct {
	UserID      int64
	SummaryJSON []byte
	SymbolsJSON []byte
	SyncedAt    time.Time
}

type PortfolioCacheRepo struct {
	db *sql.DB
}

func NewPortfolioCacheRepo(db *sql.DB) *PortfolioCacheRepo {
	return &PortfolioCacheRepo{db: db}
}

func (r *PortfolioCacheRepo) Upsert(ctx context.Context, userID int64, summaryJSON, symbolsJSON []byte) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_portfolio_cache (user_id, summary_json, symbols_json, synced_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP(3))
		ON DUPLICATE KEY UPDATE
			summary_json = VALUES(summary_json),
			symbols_json = VALUES(symbols_json),
			synced_at = CURRENT_TIMESTAMP(3)
	`, userID, summaryJSON, symbolsJSON)
	return err
}

func (r *PortfolioCacheRepo) Get(ctx context.Context, userID int64) (*PortfolioCacheRecord, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT summary_json, symbols_json, synced_at
		FROM user_portfolio_cache
		WHERE user_id = ?
	`, userID)
	var rec PortfolioCacheRecord
	rec.UserID = userID
	if err := row.Scan(&rec.SummaryJSON, &rec.SymbolsJSON, &rec.SyncedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPortfolioCacheMiss
		}
		return nil, err
	}
	return &rec, nil
}

func (r *PortfolioCacheRepo) Delete(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_portfolio_cache WHERE user_id = ?`, userID)
	return err
}

func EncodeJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}
