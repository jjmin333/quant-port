package repository

import (
	"context"
	"database/sql"

	"github.com/enact/quant-port/be/internal/model"
)

type BrokerAccountRepo struct {
	db *sql.DB
}

func NewBrokerAccountRepo(db *sql.DB) *BrokerAccountRepo {
	return &BrokerAccountRepo{db: db}
}

func (r *BrokerAccountRepo) ReplaceForUser(ctx context.Context, userID int64, brokerCode string, accounts []model.BrokerAccount) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM broker_accounts WHERE user_id = ? AND broker_code = ?`, userID, brokerCode); err != nil {
		return err
	}

	for i, acct := range accounts {
		isPrimary := 0
		if i == 0 || acct.IsPrimary {
			isPrimary = 1
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO broker_accounts (
				user_id, broker_code, account_name, account_seq, account_no, account_type, is_primary, synced_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, userID, brokerCode, acct.AccountName, acct.AccountSeq, acct.AccountNo, acct.AccountType, isPrimary); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *BrokerAccountRepo) ListByUser(ctx context.Context, userID int64, brokerCode string) ([]model.BrokerAccount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, broker_code, account_name, account_seq, account_no, account_type, is_primary, synced_at
		FROM broker_accounts
		WHERE user_id = ? AND broker_code = ?
		ORDER BY is_primary DESC, account_name ASC
	`, userID, brokerCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.BrokerAccount
	for rows.Next() {
		var a model.BrokerAccount
		var accountNo, accountType sql.NullString
		var syncedAt sql.NullTime
		var isPrimary int
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.BrokerCode, &a.AccountName, &a.AccountSeq,
			&accountNo, &accountType, &isPrimary, &syncedAt,
		); err != nil {
			return nil, err
		}
		a.AccountNo = accountNo.String
		a.AccountType = accountType.String
		a.IsPrimary = isPrimary == 1
		if syncedAt.Valid {
			a.SyncedAt = syncedAt.Time
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *BrokerAccountRepo) DeleteForUser(ctx context.Context, userID int64, brokerCode string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM broker_accounts WHERE user_id = ? AND broker_code = ?`, userID, brokerCode)
	return err
}
