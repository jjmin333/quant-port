package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

var ErrUserNotFound = errors.New("user not found")
var ErrEmailTaken = errors.New("email already registered")

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
			return nil, ErrEmailTaken
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
