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

/*
refreshTokenGrace — 만료 뒤에도 남겨 두는 기간.

만료 즉시 지우지 않는 이유는 두 가지다. ① 재발급이 만료 직후에 몰릴 수 있는데
행이 사라져 있으면 「없는 토큰」과 「만료된 토큰」을 구분할 수 없다 —
사용자에게 줄 메시지가 달라진다. ② 이상 로그인을 되짚을 때 최근 며칠의 흔적은
남아 있어야 한다.
*/
const refreshTokenGrace = 7 * 24 * time.Hour

/*
PurgeExpired — 만료된 지 오래된 리프레시 토큰을 지운다.

이 표에는 삭제 코드가 없었다(2026-08-23 실측: 2,734행 중 **2,145행이 이미
만료**). 토큰은 로그인·갱신마다 한 행씩 생기고 `JWT_REFRESH_TOKEN_DAYS=7`로
회전하므로 **활성 사용자당 연 365행**씩 쌓인다. 사용자가 늘면 그대로 곱해진다.

만료 토큰을 남겨 둘 이유는 위 유예 기간이 지나면 없다 — 인증에 쓸 수 없고,
보안 위생상으로도 해시가 오래 남을 이유가 없다.

`idx_refresh_expires`(expires_at)가 이미 있다. 다만 **지금은 옵티마이저가 그것을
안 쓴다** — 대상이 전체의 절반이 넘어(실측 2,737행 중 1,432행) 풀스캔이 더 싸기
때문이고, 그 판단은 옳다. 정리가 매일 돌아 잔여가 적어지면 인덱스 범위 스캔으로
바뀐다.

한 번에 지우는 행 수를 제한하는 이유는 커진 표에서 단일 DELETE가 긴 트랜잭션이
되는 것을 막기 위해서다 — 남으면 다음 날 이어서 지운다.
*/
func (r *RefreshTokenRepo) PurgeExpired(ctx context.Context, limit int) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	if limit <= 0 {
		limit = 50_000
	}
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM user_refresh_tokens
		 WHERE expires_at < ?
		 LIMIT ?
	`, time.Now().Add(-refreshTokenGrace), limit)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
