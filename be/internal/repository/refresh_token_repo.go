package repository

import (
	"context"
	"database/sql"
	"time"
)

type RefreshTokenRepo struct {
	db *sql.DB
}

func NewRefreshTokenRepo(db *sql.DB) *RefreshTokenRepo {
	return &RefreshTokenRepo{db: db}
}

func (r *RefreshTokenRepo) Create(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, userAgent string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_refresh_tokens (user_id, token_hash, expires_at, user_agent)
		VALUES (?, ?, ?, ?)
	`, userID, tokenHash, expiresAt, userAgent)
	return err
}

func (r *RefreshTokenRepo) FindValid(ctx context.Context, tokenHash string) (int64, time.Time, error) {
	var userID int64
	var expiresAt time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT user_id, expires_at FROM user_refresh_tokens
		WHERE token_hash = ? AND revoked_at IS NULL AND expires_at > CURRENT_TIMESTAMP
	`, tokenHash).Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		return 0, time.Time{}, sql.ErrNoRows
	}
	return userID, expiresAt, err
}

func (r *RefreshTokenRepo) Revoke(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE user_refresh_tokens SET revoked_at = CURRENT_TIMESTAMP WHERE token_hash = ?
	`, tokenHash)
	return err
}

func (r *RefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE user_refresh_tokens SET revoked_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND revoked_at IS NULL
	`, userID)
	return err
}
