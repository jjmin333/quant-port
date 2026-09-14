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
	/*
		Name은 표시 이름이다 — **로그를 사람이 읽게 하려고** 넣었다.

		FE의 프록시(fe/src/proxy.ts)가 요청마다 「누가 눌렀는가」를 찍는데, 그 자리에서
		할 수 있는 일은 쿠키에 실려 온 이 토큰을 읽는 것뿐이다. 이름이 없으면 이메일
		주소가 로그에 그대로 남고(개인정보를 로그에 흘리는 셈이다), 숫자 ID만 찍으면
		사람이 읽을 수 없다.

		**옛 토큰에는 이 칸이 없다.** 빈 문자열로 파싱되고, 다음 갱신(액세스 15분)에
		자연히 채워진다 — 마이그레이션이 필요 없다.
	*/
	Name string `json:"name,omitempty"`
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
		Name:  user.DisplayName,
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

// displayNameMaxRunes is the upper bound the API promises ("2-100 characters")
// and the signup form enforces with maxLength={100}.
const displayNameMaxRunes = 100

// validateDisplayName measures length in *characters*, both ends.
//
// It used to mix units: the lower bound counted runes but the upper bound
// counted bytes (`len(name) > 100`). A Korean name is 3 bytes per character, so
// the real ceiling was 33 characters while the error message and the signup
// form both said 100 — the form accepted an input the server then rejected with
// a reason the user could not act on. `users.display_name` is VARCHAR(100) and
// MySQL counts characters there too, so runes are also what the column stores.
func validateDisplayName(displayName string) error {
	name := strings.TrimSpace(displayName)
	runes := len([]rune(name))
	if name == "" || runes < 2 || runes > displayNameMaxRunes {
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

// ---------------------------------------------------------------- 계정 관리

/*
계정 화면이 없어서 생기던 구멍을 메운다.

이 앱에는 **비밀번호를 바꿀 방법도, 탈퇴할 방법도 없었다.** 비밀번호를 잊으면 계정으로
돌아올 길이 없고, 그만두고 싶은 사람은 데이터를 남긴 채 떠나야 했다.

세 가지 모두 **현재 비밀번호를 다시 받는다.** 로그인 세션은 오래 유지되고 남의 자리에
열린 채 남아 있을 수 있으므로, 계정 자체를 바꾸는 행동은 「지금 이 사람이 주인인가」를
한 번 더 묻는 것이 맞다.
*/

// ChangePassword swaps the password after re-authenticating with the current one.
func (s *AuthService) ChangePassword(ctx context.Context, userID int64, current, next string) error {
	hash, err := s.users.PasswordHash(ctx, userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(current)) != nil {
		return ErrInvalidCredentials
	}
	if err := validatePassword(next); err != nil {
		return err
	}
	if current == next {
		return errors.New("새 비밀번호가 현재 비밀번호와 같습니다")
	}
	nextHash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.users.UpdatePassword(ctx, userID, string(nextHash)); err != nil {
		return err
	}
	/*
		비밀번호를 바꾸면 **다른 기기의 세션을 전부 끊는다.**

		비밀번호를 바꾸는 이유의 대부분은 「누가 내 계정을 보고 있는 것 같다」이다.
		그런데 기존 refresh 토큰을 살려 두면 정확히 그 사람이 계속 들어와 있다.
		본인은 지금 쓰는 브라우저에서 다시 로그인하면 된다.
	*/
	return s.refresh.RevokeAllForUser(ctx, userID)
}

// ChangeDisplayName renames the account after the same re-authentication.
func (s *AuthService) ChangeDisplayName(ctx context.Context, userID int64, displayName string) (*model.AuthSession, error) {
	if err := validateDisplayName(displayName); err != nil {
		return nil, err
	}
	if err := s.users.UpdateDisplayName(ctx, userID, displayName); err != nil {
		return nil, err
	}
	return s.Me(ctx, userID)
}

// DeleteAccount erases the account after re-authenticating.
func (s *AuthService) DeleteAccount(ctx context.Context, userID int64, password string) error {
	hash, err := s.users.PasswordHash(ctx, userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return ErrInvalidCredentials
	}
	return s.users.Delete(ctx, userID)
}
