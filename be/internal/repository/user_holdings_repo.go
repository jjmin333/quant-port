package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

type UserHoldingsRepo struct {
	db *sql.DB
}

func NewUserHoldingsRepo(db *sql.DB) *UserHoldingsRepo {
	return &UserHoldingsRepo{db: db}
}

func (r *UserHoldingsRepo) List(ctx context.Context, userID int64) ([]model.UserHolding, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT uh.id, uh.symbol, uh.quantity, uh.avg_price, COALESCE(uh.cost_currency, ''),
		       COALESCE(uh.broker_hint, ''), uh.source, uh.updated_at,
		       COALESCE(s.name, ''), COALESCE(s.english_name, '')
		FROM user_holdings uh
		LEFT JOIN stocks s ON s.symbol = uh.symbol
		WHERE uh.user_id = ?
		ORDER BY uh.updated_at DESC, uh.symbol ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.UserHolding
	for rows.Next() {
		var h model.UserHolding
		var qty sql.NullFloat64
		var avgPrice sql.NullFloat64
		var costCurrency, brokerHint, source, name, english string
		if err := rows.Scan(&h.ID, &h.Symbol, &qty, &avgPrice, &costCurrency, &brokerHint, &source, &h.UpdatedAt, &name, &english); err != nil {
			return nil, err
		}
		if qty.Valid {
			h.Quantity = qty.Float64
		}
		if avgPrice.Valid {
			v := avgPrice.Float64
			h.AvgPrice = &v
		}
		h.CostCurrency = costCurrency
		h.BrokerHint = brokerHint
		h.Source = source
		if name != "" {
			h.Name = name
		} else if english != "" {
			h.Name = english
		}
		out = append(out, h)
	}
	if out == nil {
		out = []model.UserHolding{}
	}
	return out, rows.Err()
}

func (r *UserHoldingsRepo) ListAllSymbols(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT symbol FROM user_holdings ORDER BY symbol`)
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

func (r *UserHoldingsRepo) Symbols(ctx context.Context, userID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol FROM user_holdings WHERE user_id = ? ORDER BY symbol
	`, userID)
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

func (r *UserHoldingsRepo) ReplaceAll(ctx context.Context, userID int64, rows []model.UserHolding) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM user_holdings WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, row := range rows {
		sym := strings.TrimSpace(strings.ToUpper(row.Symbol))
		if sym == "" || row.Quantity <= 0 {
			continue
		}
		source := strings.TrimSpace(row.Source)
		if source == "" {
			source = "manual"
		}
		brokerHint := strings.TrimSpace(row.BrokerHint)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_holdings (user_id, symbol, quantity, avg_price, cost_currency, broker_hint, source)
			VALUES (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?)
		`, userID, sym, row.Quantity, nullFloat(row.AvgPrice), strings.TrimSpace(row.CostCurrency), brokerHint, source); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *UserHoldingsRepo) UpsertMany(ctx context.Context, userID int64, rows []model.UserHolding) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, row := range rows {
		sym := strings.TrimSpace(strings.ToUpper(row.Symbol))
		if sym == "" || row.Quantity <= 0 {
			continue
		}
		source := strings.TrimSpace(row.Source)
		if source == "" {
			source = "screenshot"
		}
		brokerHint := strings.TrimSpace(row.BrokerHint)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_holdings (user_id, symbol, quantity, avg_price, cost_currency, broker_hint, source)
			VALUES (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?)
			ON DUPLICATE KEY UPDATE
				quantity = VALUES(quantity),
				avg_price = VALUES(avg_price),
				cost_currency = COALESCE(NULLIF(VALUES(cost_currency), ''), cost_currency),
				broker_hint = COALESCE(NULLIF(VALUES(broker_hint), ''), broker_hint),
				source = VALUES(source),
				updated_at = CURRENT_TIMESTAMP
		`, userID, sym, row.Quantity, nullFloat(row.AvgPrice), strings.TrimSpace(row.CostCurrency), brokerHint, source); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *UserHoldingsRepo) DeleteSymbol(ctx context.Context, userID int64, symbol string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM user_holdings WHERE user_id = ? AND symbol = ?
	`, userID, strings.TrimSpace(strings.ToUpper(symbol)))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("holding not found")
	}
	return nil
}

func nullFloat(v *float64) interface{} {
	if v == nil || *v <= 0 {
		return nil
	}
	return *v
}

func (r *UserHoldingsRepo) TouchUpdatedAt(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE user_holdings SET updated_at = ? WHERE user_id = ?
	`, time.Now(), userID)
	return err
}
