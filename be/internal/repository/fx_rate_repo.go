package repository

import (
	"context"
	"database/sql"
	"time"
)

type FxRateRecord struct {
	BaseCurrency  string
	QuoteCurrency string
	MidRate       float64
	BuyRate       float64
	UpdatedAt     time.Time
}

type FxRateRepo struct {
	db *sql.DB
}

func NewFxRateRepo(db *sql.DB) *FxRateRepo {
	return &FxRateRepo{db: db}
}

func (r *FxRateRepo) Upsert(ctx context.Context, rec FxRateRecord) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO market_fx_rates (base_currency, quote_currency, mid_rate, buy_rate, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP(3))
		ON DUPLICATE KEY UPDATE
			mid_rate = VALUES(mid_rate),
			buy_rate = VALUES(buy_rate),
			updated_at = CURRENT_TIMESTAMP(3)
	`, rec.BaseCurrency, rec.QuoteCurrency, rec.MidRate, rec.BuyRate)
	return err
}

func (r *FxRateRepo) Get(ctx context.Context, base, quote string) (*FxRateRecord, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT mid_rate, buy_rate, updated_at
		FROM market_fx_rates
		WHERE base_currency = ? AND quote_currency = ?
	`, base, quote)
	var rec FxRateRecord
	rec.BaseCurrency = base
	rec.QuoteCurrency = quote
	if err := row.Scan(&rec.MidRate, &rec.BuyRate, &rec.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}
