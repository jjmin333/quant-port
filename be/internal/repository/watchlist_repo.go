package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type WatchlistRepo struct {
	db *sql.DB
}

func NewWatchlistRepo(db *sql.DB) *WatchlistRepo {
	return &WatchlistRepo{db: db}
}

func (r *WatchlistRepo) ListAllSymbols(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT symbol FROM watchlist_items ORDER BY symbol`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var sym string
		if err := rows.Scan(&sym); err != nil {
			return nil, err
		}
		out = append(out, sym)
	}
	return out, rows.Err()
}

func (r *WatchlistRepo) List(ctx context.Context, userID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol FROM watchlist_items
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, err
		}
		out = append(out, symbol)
	}
	return out, rows.Err()
}

func (r *WatchlistRepo) Add(ctx context.Context, userID int64, symbol string) error {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return sql.ErrNoRows
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT IGNORE INTO watchlist_items (user_id, symbol)
		VALUES (?, ?)
	`, userID, symbol)
	return err
}

func (r *WatchlistRepo) Remove(ctx context.Context, userID int64, symbol string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM watchlist_items WHERE user_id = ? AND symbol = ?
	`, userID, strings.TrimSpace(symbol))
	return err
}

func (r *WatchlistRepo) Set(ctx context.Context, userID int64, symbol string) (time.Time, error) {
	symbol = strings.TrimSpace(symbol)
	var createdAt time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT created_at FROM watchlist_items
		WHERE user_id = ? AND symbol = ?
	`, userID, symbol).Scan(&createdAt)
	return createdAt, err
}
