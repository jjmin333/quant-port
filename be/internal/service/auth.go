package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
가입은 **아이디 · 닉네임 · 비밀번호**다 (2026-09-27 사용자 요청 3). 이메일은 받지 않는다.

기존 회원의 이메일은 **그대로 아이디다** — 같은 `users.email` 칸을 로그인 아이디로 쓰므로
옮길 데이터가 없다. 그래서 새 형식 검사는 **가입할 때만** 하고, 로그인은 칸에 적힌 값과
그대로 맞춘다(이메일 모양의 옛 아이디도 통과).

길이 규칙(닉네임 2~10자 · 비밀번호 8~15자)도 가입·변경 때만 건다. 이미 16자 넘는
비밀번호를 쓰는 사람을 로그인에서 막으면 그 사람은 계정을 잃는다.
*/
var (
	ErrInvalidCredentials = errors.New("invalid login id or password")
	ErrWeakPassword       = errors.New("password must be 8-15 characters")
	ErrInvalidLoginID     = errors.New("login id must be 4-20 lowercase letters or digits, starting with a letter")
	ErrInvalidDisplayName = errors.New("display name must be 2-10 characters")
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

func (s *AuthService) Signup(ctx context.Context, loginID, password, displayName string) (*model.AuthSession, *TokenPair, error) {
	if err := validateLoginID(loginID); err != nil {
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

	user, err := s.users.Create(ctx, loginID, displayName, string(hash))
	if err != nil {
		return nil, nil, err
	}

	pair, err := s.issueTokens(ctx, user, "")
	if err != nil {
		return nil, nil, err
	}

	return &model.AuthSession{
		User:        user.Public(),
		AccessToken: pair.AccessToken,
		ExpiresIn:   pair.ExpiresIn,
	}, pair, nil
}

// Login matches the stored login id as-is — 기존 회원은 이메일이 곧 아이디다.
func (s *AuthService) Login(ctx context.Context, loginID, password, userAgent string) (*model.AuthSession, *TokenPair, error) {
	user, hash, err := s.users.GetByEmail(ctx, loginID)
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return nil, nil, ErrInvalidCredentials
	}
	_ = s.users.TouchLogin(ctx, user.ID)

	pair, err := s.issueTokens(ctx, user, userAgent)
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
	userID, err := s.refresh.FindValid(ctx, hash)
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	_ = s.refresh.MarkRotated(ctx, hash)

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	/*
		갱신할 때마다 만료를 **지금부터 다시** 잡는다(슬라이딩, 2026-10-07 제보 2). 예전에는 첫
		로그인의 만료를 물려줘서 매일 쓰는 사람도 정확히 7일째에 갱신이 401로 끊겼고, 열려 있던
		탭은 로그인 헤더를 단 채 「내 종목」만 비었다(nginx 10-03 07:26 · 10-05 17:27 갱신 401).
		이제 7일 동안 한 번도 안 쓴 세션만 끝난다. 쓰인 토큰은 위에서 폐기되므로 체인은 하나다.
	*/
	pair, err := s.issueTokens(ctx, user, userAgent)
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

func (s *AuthService) issueTokens(ctx context.Context, user *model.User, userAgent string) (*TokenPair, error) {
	now := time.Now()
	refreshExpiresAt := now.Add(s.refreshTTL)
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

// validateLoginID — 새 아이디: 영문 소문자로 시작하는 소문자·숫자 4~20자.
// 대문자는 받지 않는다(저장소가 소문자로 접어 저장하므로, 받으면 「Abc로 가입했는데
// abc로 보인다」가 된다).
func validateLoginID(loginID string) error {
	if len(loginID) < 4 || len(loginID) > 20 {
		return ErrInvalidLoginID
	}
	for i, r := range loginID {
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if i == 0 && !isLower {
			return ErrInvalidLoginID
		}
		if !isLower && !isDigit {
			return ErrInvalidLoginID
		}
	}
	return nil
}

// validatePassword — 8~15자 (글자 수). 가입·변경 때만 부른다.
func validatePassword(password string) error {
	n := utf8.RuneCountInString(password)
	if n < 8 || n > 15 {
		return ErrWeakPassword
	}
	return nil
}

// displayNameMaxRunes is the upper bound the API promises ("2-10 characters")
// and the signup form enforces with maxLength={10} (2026-09-27 사용자 요청 3).
// 이미 더 긴 닉네임을 쓰는 기존 회원은 그대로 두고, 바꿀 때만 이 규칙을 건다.
const displayNameMaxRunes = 10

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
