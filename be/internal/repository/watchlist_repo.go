package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type WatchlistRepo struct {
	db *sql.DB
}

type WatchlistEntry struct {
	ID         int64
	Symbol     string
	SortOrder  int
	CreatedAt  time.Time
	AddedPrice sql.NullFloat64
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
	entries, err := r.ListEntries(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Symbol)
	}
	return out, nil
}

func (r *WatchlistRepo) ListEntries(ctx context.Context, userID int64) ([]WatchlistEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, symbol, sort_order, created_at, added_price
		FROM watchlist_items
		WHERE user_id = ?
		ORDER BY sort_order ASC, id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WatchlistEntry
	for rows.Next() {
		var entry WatchlistEntry
		if err := rows.Scan(&entry.ID, &entry.Symbol, &entry.SortOrder, &entry.CreatedAt, &entry.AddedPrice); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func (r *WatchlistRepo) nextSortOrder(ctx context.Context, userID int64) (int, error) {
	var max sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT MAX(sort_order) FROM watchlist_items WHERE user_id = ?
	`, userID).Scan(&max)
	if err != nil {
		return 0, err
	}
	if max.Valid {
		return int(max.Int64) + 1, nil
	}
	return 0, nil
}

func (r *WatchlistRepo) Add(ctx context.Context, userID int64, symbol string) error {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return sql.ErrNoRows
	}
	sortOrder, err := r.nextSortOrder(ctx, userID)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT IGNORE INTO watchlist_items (user_id, symbol, sort_order)
		VALUES (?, ?, ?)
	`, userID, symbol, sortOrder)
	return err
}

func (r *WatchlistRepo) Reorder(ctx context.Context, userID int64, symbols []string) error {
	if len(symbols) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for i, sym := range symbols {
		sym = strings.TrimSpace(strings.ToUpper(sym))
		if sym == "" {
			continue
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE watchlist_items
			SET sort_order = ?
			WHERE user_id = ? AND UPPER(symbol) = ?
		`, i, userID, sym)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("watchlist symbol not found: %s", sym)
		}
	}
	return tx.Commit()
}

func (r *WatchlistRepo) SetAddedPrice(ctx context.Context, userID int64, symbol string, price float64) error {
	if price <= 0 {
		return nil
	}
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	_, err := r.db.ExecContext(ctx, `
		UPDATE watchlist_items
		SET added_price = ?
		WHERE user_id = ? AND symbol = ? AND (added_price IS NULL OR added_price = 0)
	`, price, userID, symbol)
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
