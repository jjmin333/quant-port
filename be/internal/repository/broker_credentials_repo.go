package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

var ErrBrokerNotLinked = errors.New("broker not linked")

type BrokerCredentialsRepo struct {
	db *sql.DB
}

func NewBrokerCredentialsRepo(db *sql.DB) *BrokerCredentialsRepo {
	return &BrokerCredentialsRepo{db: db}
}

func (r *BrokerCredentialsRepo) Upsert(
	ctx context.Context,
	userID int64,
	brokerCode string,
	clientIDEnc, clientSecretEnc []byte,
	clientIDHint string,
) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_broker_credentials (
			user_id, broker_code, client_id_enc, client_secret_enc, client_id_hint, is_active
		) VALUES (?, ?, ?, ?, ?, 1)
		ON DUPLICATE KEY UPDATE
			client_id_enc = VALUES(client_id_enc),
			client_secret_enc = VALUES(client_secret_enc),
			client_id_hint = VALUES(client_id_hint),
			is_active = 1,
			linked_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
	`, userID, brokerCode, clientIDEnc, clientSecretEnc, clientIDHint)
	return err
}

func (r *BrokerCredentialsRepo) GetActive(ctx context.Context, userID int64, brokerCode string) (*model.BrokerCredential, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, broker_code, client_id_enc, client_secret_enc, client_id_hint, is_active, linked_at, updated_at
		FROM user_broker_credentials
		WHERE user_id = ? AND broker_code = ? AND is_active = 1
	`, userID, brokerCode)
	var c model.BrokerCredential
	if err := row.Scan(
		&c.ID, &c.UserID, &c.BrokerCode, &c.ClientIDEnc, &c.ClientSecretEnc,
		&c.ClientIDHint, &c.IsActive, &c.LinkedAt, &c.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrBrokerNotLinked
		}
		return nil, err
	}
	return &c, nil
}

func (r *BrokerCredentialsRepo) ListActiveUserIDs(ctx context.Context, brokerCode string) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT user_id FROM user_broker_credentials
		WHERE broker_code = ? AND is_active = 1
	`, brokerCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *BrokerCredentialsRepo) Deactivate(ctx context.Context, userID int64, brokerCode string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE user_broker_credentials SET is_active = 0, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND broker_code = ?
	`, userID, brokerCode)
	return err
}

func (r *BrokerCredentialsRepo) IsLinked(ctx context.Context, userID int64, brokerCode string) (bool, string, time.Time, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT client_id_hint, linked_at
		FROM user_broker_credentials
		WHERE user_id = ? AND broker_code = ? AND is_active = 1
	`, userID, brokerCode)
	var hint string
	var linkedAt time.Time
	if err := row.Scan(&hint, &linkedAt); err != nil {
		if err == sql.ErrNoRows {
			return false, "", time.Time{}, nil
		}
		return false, "", time.Time{}, err
	}
	return true, hint, linkedAt, nil
}
