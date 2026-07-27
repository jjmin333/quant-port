package repository

import (
	"context"
	"database/sql"
	"time"
)

type PortfolioSnapshot struct {
	UserID         int64
	RecordedAt     time.Time
	TotalAssetsKRW float64
	TotalCashKRW   float64
	TotalProfitKRW float64
	ExchangeRate   float64
}

type SnapshotRepo struct {
	db *sql.DB
}

func NewSnapshotRepo(db *sql.DB) *SnapshotRepo {
	return &SnapshotRepo{db: db}
}

func (r *SnapshotRepo) LastRecordedAt(ctx context.Context, userID int64) (time.Time, error) {
	var t sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT recorded_at FROM portfolio_snapshots
		WHERE user_id = ?
		ORDER BY recorded_at DESC LIMIT 1
	`, userID).Scan(&t)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	if !t.Valid {
		return time.Time{}, nil
	}
	return t.Time, nil
}

func (r *SnapshotRepo) InsertIfDue(ctx context.Context, snap PortfolioSnapshot, minInterval time.Duration) error {
	last, err := r.LastRecordedAt(ctx, snap.UserID)
	if err != nil {
		return err
	}
	if !last.IsZero() && time.Since(last) < minInterval {
		return nil
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO portfolio_snapshots
		  (user_id, recorded_at, total_assets_krw, total_cash_krw, total_profit_krw, exchange_rate)
		VALUES (?, ?, ?, ?, ?, ?)
	`, snap.UserID, snap.RecordedAt, snap.TotalAssetsKRW, snap.TotalCashKRW, snap.TotalProfitKRW, snap.ExchangeRate)
	return err
}

func (r *SnapshotRepo) ListSince(ctx context.Context, userID int64, since time.Time) ([]PortfolioSnapshot, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT recorded_at, total_assets_krw, total_cash_krw, total_profit_krw, exchange_rate
		FROM portfolio_snapshots
		WHERE user_id = ? AND recorded_at >= ?
		ORDER BY recorded_at ASC
	`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PortfolioSnapshot
	for rows.Next() {
		var s PortfolioSnapshot
		s.UserID = userID
		if err := rows.Scan(&s.RecordedAt, &s.TotalAssetsKRW, &s.TotalCashKRW, &s.TotalProfitKRW, &s.ExchangeRate); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *SnapshotRepo) Earliest(ctx context.Context, userID int64) (time.Time, error) {
	var t sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT recorded_at FROM portfolio_snapshots
		WHERE user_id = ?
		ORDER BY recorded_at ASC LIMIT 1
	`, userID).Scan(&t)
	if err == sql.ErrNoRows || !t.Valid {
		return time.Time{}, nil
	}
	return t.Time, err
}
