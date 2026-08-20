package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
	ErrInvalidEmail       = errors.New("invalid email address")
	ErrInvalidDisplayName = errors.New("display name must be 2-100 characters")
)

type AuthService struct {
	users        *repository.UserRepo
	refresh      *repository.RefreshTokenRepo
	jwtSecret    []byte
	accessTTL    time.Duration
	refreshTTL   time.Duration
	cookieSecure bool
}

func NewAuthService(
	cfg config.AuthConfig,
	users *repository.UserRepo,
	refresh *repository.RefreshTokenRepo,
) (*AuthService, error) {
	if strings.TrimSpace(cfg.JWTSecret) == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	return &AuthService{
		users:        users,
		refresh:      refresh,
		jwtSecret:    []byte(cfg.JWTSecret),
		accessTTL:    time.Duration(cfg.AccessTokenMinutes) * time.Minute,
		refreshTTL:   time.Duration(cfg.RefreshTokenDays) * 24 * time.Hour,
		cookieSecure: cfg.CookieSecure,
	}, nil
}

type accessClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	ExpiresIn        int
	RefreshExpiresAt time.Time
}

func (s *AuthService) Signup(ctx context.Context, email, password, displayName string) (*model.AuthSession, *TokenPair, error) {
	if err := validateEmail(email); err != nil {
		return nil, nil, err
	}
	if err := validatePassword(password); err != nil {
		return nil, nil, err
	}
	if err := validateDisplayName(displayName); err != nil {
		return nil, nil, err
	}
	displayName = strings.TrimSpace(displayName)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, err
	}

	user, err := s.users.Create(ctx, email, displayName, string(hash))
	if err != nil {
		return nil, nil, err
	}

	pair, err := s.issueTokens(ctx, user, "", time.Time{})
	if err != nil {
		return nil, nil, err
	}

	return &model.AuthSession{
		User:        user.Public(),
		AccessToken: pair.AccessToken,
		ExpiresIn:   pair.ExpiresIn,
	}, pair, nil
}

func (s *AuthService) Login(ctx context.Context, email, password, userAgent string) (*model.AuthSession, *TokenPair, error) {
	user, hash, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return nil, nil, ErrInvalidCredentials
	}
	_ = s.users.TouchLogin(ctx, user.ID)

	pair, err := s.issueTokens(ctx, user, userAgent, time.Time{})
	if err != nil {
		return nil, nil, err
	}

	return &model.AuthSession{
		User:        user.Public(),
		AccessToken: pair.AccessToken,
		ExpiresIn:   pair.ExpiresIn,
	}, pair, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken, userAgent string) (*model.AuthSession, *TokenPair, error) {
	hash := hashToken(refreshToken)
	userID, sessionExpiresAt, err := s.refresh.FindValid(ctx, hash)
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	_ = s.refresh.Revoke(ctx, hash)

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	pair, err := s.issueTokens(ctx, user, userAgent, sessionExpiresAt)
	if err != nil {
		return nil, nil, err
	}

	return &model.AuthSession{
		User:        user.Public(),
		AccessToken: pair.AccessToken,
		ExpiresIn:   pair.ExpiresIn,
	}, pair, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string, userID int64) error {
	if refreshToken != "" {
		_ = s.refresh.Revoke(ctx, hashToken(refreshToken))
	}
	if userID > 0 {
		return s.refresh.RevokeAllForUser(ctx, userID)
	}
	return nil
}

func (s *AuthService) Me(ctx context.Context, userID int64) (*model.AuthSession, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &model.AuthSession{
		User:      user.Public(),
		ExpiresIn: int(s.accessTTL.Seconds()),
	}, nil
}

func (s *AuthService) ParseAccessToken(token string) (int64, string, error) {
	parsed, err := jwt.ParseWithClaims(token, &accessClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil || !parsed.Valid {
		return 0, "", ErrInvalidCredentials
	}
	claims, ok := parsed.Claims.(*accessClaims)
	if !ok || claims.Subject == "" {
		return 0, "", ErrInvalidCredentials
	}
	var userID int64
	if _, err := fmt.Sscan(claims.Subject, &userID); err != nil || userID <= 0 {
		return 0, "", ErrInvalidCredentials
	}
	return userID, claims.Email, nil
}

func (s *AuthService) AccessCookieName() string  { return "qp_access" }
func (s *AuthService) RefreshCookieName() string { return "qp_refresh" }
func (s *AuthService) CookieSecure() bool        { return s.cookieSecure }

func (s *AuthService) RefreshMaxAge() int {
	return int(s.refreshTTL.Seconds())
}

func (s *AuthService) RefreshMaxAgeUntil(expiresAt time.Time) int {
	if expiresAt.IsZero() {
		return s.RefreshMaxAge()
	}
	secs := int(time.Until(expiresAt).Seconds())
	if secs < 0 {
		return 0
	}
	return secs
}

func (s *AuthService) issueTokens(ctx context.Context, user *model.User, userAgent string, refreshExpiresAt time.Time) (*TokenPair, error) {
	now := time.Now()
	if refreshExpiresAt.IsZero() {
		refreshExpiresAt = now.Add(s.refreshTTL)
	}
	expiresAt := now.Add(s.accessTTL)
	claims := accessClaims{
		Email: user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	access, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return nil, err
	}

	refreshRaw, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	refreshHash := hashToken(refreshRaw)
	if err := s.refresh.Create(ctx, user.ID, refreshHash, refreshExpiresAt, userAgent); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:      access,
		RefreshToken:     refreshRaw,
		ExpiresIn:        int(s.accessTTL.Seconds()),
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

func validateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrInvalidEmail
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return ErrInvalidEmail
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return ErrWeakPassword
	}
	return nil
}

func validateDisplayName(displayName string) error {
	name := strings.TrimSpace(displayName)
	if name == "" || len([]rune(name)) < 2 || len(name) > 100 {
		return ErrInvalidDisplayName
	}
	return nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
