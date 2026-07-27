package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type MarketIndexRecord struct {
	IndexID    string
	Name       string
	Currency   string
	Value      float64
	Change     float64
	ChangeRate float64
	Badge      string
	Sparkline  []float64
	Source     string
	UpdatedAt  time.Time
}

type MarketIndexRepo struct {
	db *sql.DB
}

func NewMarketIndexRepo(db *sql.DB) *MarketIndexRepo {
	return &MarketIndexRepo{db: db}
}

func (r *MarketIndexRepo) Upsert(ctx context.Context, rec MarketIndexRecord) error {
	sparkline, err := json.Marshal(rec.Sparkline)
	if err != nil {
		sparkline = []byte("null")
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO market_indices (
			index_id, name, currency, value_amount, change_amount, change_rate,
			badge, sparkline, source, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP(3))
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			currency = VALUES(currency),
			value_amount = VALUES(value_amount),
			change_amount = VALUES(change_amount),
			change_rate = VALUES(change_rate),
			badge = VALUES(badge),
			sparkline = VALUES(sparkline),
			source = VALUES(source),
			updated_at = CURRENT_TIMESTAMP(3)
	`, rec.IndexID, rec.Name, rec.Currency, rec.Value, rec.Change, rec.ChangeRate,
		nullIfEmpty(rec.Badge), sparkline, rec.Source)
	return err
}

func (r *MarketIndexRepo) Get(ctx context.Context, indexID string) (*MarketIndexRecord, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT index_id, name, currency, value_amount, change_amount, change_rate,
			COALESCE(badge, ''), sparkline, source, updated_at
		FROM market_indices
		WHERE index_id = ?
	`, indexID)
	return scanMarketIndexRow(row)
}

func (r *MarketIndexRepo) ListAll(ctx context.Context) ([]MarketIndexRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT index_id, name, currency, value_amount, change_amount, change_rate,
			COALESCE(badge, ''), sparkline, source, updated_at
		FROM market_indices
		ORDER BY index_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MarketIndexRecord
	for rows.Next() {
		rec, err := scanMarketIndexRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rec)
	}
	return out, rows.Err()
}

func scanMarketIndexRow(scanner interface {
	Scan(dest ...any) error
}) (*MarketIndexRecord, error) {
	var rec MarketIndexRecord
	var sparklineRaw []byte
	if err := scanner.Scan(
		&rec.IndexID, &rec.Name, &rec.Currency, &rec.Value, &rec.Change, &rec.ChangeRate,
		&rec.Badge, &sparklineRaw, &rec.Source, &rec.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if len(sparklineRaw) > 0 && string(sparklineRaw) != "null" {
		_ = json.Unmarshal(sparklineRaw, &rec.Sparkline)
	}
	return &rec, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
