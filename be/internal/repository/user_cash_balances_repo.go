package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

type UserCashBalancesRepo struct {
	db *sql.DB
}

func NewUserCashBalancesRepo(db *sql.DB) *UserCashBalancesRepo {
	return &UserCashBalancesRepo{db: db}
}

func (r *UserCashBalancesRepo) Get(ctx context.Context, userID int64) (model.UserCashBalances, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT currency, amount FROM user_cash_balances WHERE user_id = ?
	`, userID)
	if err != nil {
		return model.UserCashBalances{}, err
	}
	defer rows.Close()

	out := model.UserCashBalances{}
	for rows.Next() {
		var currency string
		var amount sql.NullFloat64
		if err := rows.Scan(&currency, &amount); err != nil {
			return model.UserCashBalances{}, err
		}
		if !amount.Valid {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(currency)) {
		case "KRW":
			out.KRW = amount.Float64
		case "USD":
			out.USD = amount.Float64
		}
	}
	return out, rows.Err()
}

func (r *UserCashBalancesRepo) Upsert(ctx context.Context, userID int64, balances model.UserCashBalances) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, row := range []struct {
		currency string
		amount   float64
	}{
		{"KRW", balances.KRW},
		{"USD", balances.USD},
	} {
		if row.amount < 0 {
			return fmt.Errorf("amount for %s must be >= 0", row.currency)
		}
		if row.amount == 0 {
			if _, err := tx.ExecContext(ctx, `
				DELETE FROM user_cash_balances WHERE user_id = ? AND currency = ?
			`, userID, row.currency); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_cash_balances (user_id, currency, amount)
			VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE amount = VALUES(amount), updated_at = CURRENT_TIMESTAMP
		`, userID, row.currency, row.amount); err != nil {
			return err
		}
	}
	return tx.Commit()
}
