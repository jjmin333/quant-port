package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

var ErrUserNotFound = errors.New("user not found")
// ErrEmailTaken — 아이디 칸(`users.email`)이 이미 있다. 이름은 칸 이름을 따른다.
var ErrEmailTaken = errors.New("login id already registered")

// ErrDisplayNameTaken — 닉네임 중복 (2026-09-27 요청 3 · 유니크 인덱스).
var ErrDisplayNameTaken = errors.New("display name already taken")

// duplicateUserErr tells which unique key a duplicate-key error hit — 아이디와
// 닉네임 둘 다 유니크라 「이미 있다」가 어느 쪽인지 사용자에게 말해야 한다.
func duplicateUserErr(err error) error {
	if strings.Contains(err.Error(), displayNameUniqueIndex) {
		return ErrDisplayNameTaken
	}
	return ErrEmailTaken
}

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, email, displayName, passwordHash string) (*model.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	role := model.RoleUser
	if model.IsAdminUser(email, "") {
		role = model.RoleAdmin
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO users (email, display_name, password_hash, role)
		VALUES (?, ?, ?, ?)
	`, email, strings.TrimSpace(displayName), passwordHash, role)
	if err != nil {
		if isDuplicateKey(err) {
			return nil, duplicateUserErr(err)
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return r.GetByID(ctx, id)
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (*model.User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, email, display_name, role, last_login_at, created_at, updated_at
		FROM users WHERE id = ?
	`, id)
	return scanUser(row)
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*model.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	row := r.db.QueryRowContext(ctx, `
		SELECT id, email, display_name, role, last_login_at, created_at, updated_at, password_hash
		FROM users WHERE email = ?
	`, email)
	var u model.User
	var lastLogin sql.NullTime
	var passwordHash sql.NullString
	if err := row.Scan(
		&u.ID, &u.Email, &u.DisplayName, &u.Role, &lastLogin, &u.CreatedAt, &u.UpdatedAt, &passwordHash,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, "", ErrUserNotFound
		}
		return nil, "", err
	}
	if lastLogin.Valid {
		t := lastLogin.Time
		u.LastLoginAt = &t
	}
	if !passwordHash.Valid || passwordHash.String == "" {
		return nil, "", ErrUserNotFound
	}
	return &u, passwordHash.String, nil
}

func (r *UserRepo) TouchLogin(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET last_login_at = CURRENT_TIMESTAMP WHERE id = ?`, userID)
	return err
}

type userRowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row userRowScanner) (*model.User, error) {
	var u model.User
	var lastLogin sql.NullTime
	var role sql.NullString
	if err := row.Scan(&u.ID, &u.Email, &u.DisplayName, &role, &lastLogin, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if role.Valid && role.String != "" {
		u.Role = role.String
	} else {
		u.Role = model.RoleUser
	}
	if lastLogin.Valid {
		t := lastLogin.Time
		u.LastLoginAt = &t
	}
	return &u, nil
}

func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate")
}

/*
UpdatePassword replaces the stored bcrypt hash.

호출부(`AuthService.ChangePassword`)가 현재 비밀번호를 먼저 확인한다 — 저장소는
「누가 요청했는지」를 알 수 없으므로 여기서 검사하지 않는다.
*/
func (r *UserRepo) UpdatePassword(ctx context.Context, userID int64, passwordHash string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, passwordHash, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdateDisplayName renames the account.
func (r *UserRepo) UpdateDisplayName(ctx context.Context, userID int64, displayName string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE users SET display_name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, strings.TrimSpace(displayName), userID)
	if err != nil {
		if isDuplicateKey(err) {
			return ErrDisplayNameTaken
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// PasswordHash returns the stored hash for re-authentication (탈퇴·비밀번호 변경).
func (r *UserRepo) PasswordHash(ctx context.Context, userID int64) (string, error) {
	var hash sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", ErrUserNotFound
	}
	if err != nil {
		return "", err
	}
	if !hash.Valid || hash.String == "" {
		return "", ErrUserNotFound
	}
	return hash.String, nil
}

/*
Delete removes the account and everything it owns.

**한 트랜잭션 안에서 지운다.** 중간에 실패하면 「관심종목은 지워졌는데 계정은 남은」
상태가 되고, 사용자가 다시 로그인했을 때 데이터가 반쯤 사라진 것으로 보인다.
FK가 걸린 표(holding_trades·user_holdings·user_cash_balances·user_refresh_tokens)가
있으므로 소유 데이터를 먼저 지우고 users를 마지막에 지운다.

**발행된 리포트와 댓글은 지우지 않는다.** 그것들은 공개 기록이고 다른 사람의 댓글·평가가
붙어 있다 — 계정 하나가 사라졌다고 그 대화까지 사라지면 안 된다. 대신 작성자 이름은
`users` 행과 함께 사라지므로(조회가 `LEFT JOIN users`다) 화면에는 「탈퇴한 사용자」로
나온다. 개인정보는 지우면서 남의 글은 지키는 경계가 여기다.
*/
func (r *UserRepo) Delete(ctx context.Context, userID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// 「내 투자」 화면 순서대로. 전부 이 사용자만 소유하는 행이다.
	owned := []string{
		`DELETE FROM watchlist_items WHERE user_id = ?`,
		`DELETE FROM fin_watchlist WHERE user_id = ?`,
		// 순자산(096): 평가 이력은 user_assets FK CASCADE로 같이 지워진다.
		`DELETE FROM user_liabilities WHERE user_id = ?`,
		`DELETE FROM user_assets WHERE user_id = ?`,
		`DELETE FROM holding_trades WHERE user_id = ?`,
		`DELETE FROM user_holdings WHERE user_id = ?`,
		`DELETE FROM user_cash_balances WHERE user_id = ?`,
		`DELETE FROM user_price_targets WHERE user_id = ?`,
		`DELETE FROM journal_notes WHERE user_id = ?`,
		`DELETE FROM stamps WHERE user_id = ?`,
		`DELETE FROM user_refresh_tokens WHERE user_id = ?`,
	}
	for _, q := range owned {
		if _, err := tx.ExecContext(ctx, q, userID); err != nil {
			// 이 배포에 없는 표라면 넘어간다 — 스키마 편차 때문에 탈퇴가 영영
			// 막히는 쪽이 더 나쁘다.
			if isUnknownTableErr(err) {
				continue
			}
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func isUnknownTableErr(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "doesn't exist")
}
