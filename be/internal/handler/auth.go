package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

/*
loginBody — 가입·로그인 공용 입력. 아이디는 `loginId`로 받고, 배포 직후 옛 화면이
보내는 `email`도 같은 뜻으로 받는다 (2026-09-27 요청 3 — 기존 회원은 이메일이 곧 아이디).
*/
type loginBody struct {
	LoginID     string `json:"loginId"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

func (b loginBody) loginID() string {
	if id := strings.TrimSpace(b.LoginID); id != "" {
		return id
	}
	return strings.TrimSpace(b.Email)
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}

	session, pair, err := h.auth.Signup(r.Context(), body.loginID(), body.Password, body.DisplayName)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	setAuthCookies(w, h.auth, pair)
	response.JSON(w, http.StatusCreated, map[string]any{"data": session})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}

	session, pair, err := h.auth.Login(r.Context(), body.loginID(), body.Password, r.UserAgent())
	if err != nil {
		writeServiceError(w, err)
		return
	}

	setAuthCookies(w, h.auth, pair)
	response.JSON(w, http.StatusOK, map[string]any{"data": session})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	refreshToken := refreshTokenFromRequest(r)
	if refreshToken == "" {
		response.Error(w, http.StatusUnauthorized, "refresh token required")
		return
	}

	session, pair, err := h.auth.Refresh(r.Context(), refreshToken, r.UserAgent())
	if err != nil {
		clearAuthCookies(w, h.auth)
		writeServiceError(w, err)
		return
	}

	setAuthCookies(w, h.auth, pair)
	response.JSON(w, http.StatusOK, map[string]any{"data": session})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	refreshToken := refreshTokenFromRequest(r)
	userID, _ := authctx.UserID(r.Context())
	_ = h.auth.Logout(r.Context(), refreshToken, userID)
	clearAuthCookies(w, h.auth)
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}

	session, err := h.auth.Me(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": session})
}

/*
Session serves GET /auth/session — 앱이 뜰 때 한 번 묻는 「지금 누구인가」.

`/auth/me`는 로그인이 전제라 비로그인 방문자에게 401을 주고, 그러면 FE가 갱신을
시도해 `/auth/refresh`도 401이 된다. 처음 온 사람·검색엔진 렌더러가 **모든 화면에서
콘솔 오류 두 줄**을 봤다(2026-09-26 외부 방문자 점검). 비로그인은 오류가 아니라
정상 상태이므로 이 경로는 언제나 200이고, 대신 갱신 쿠키가 있는지(`refreshable`)를
알려 준다 — 액세스 토큰만 만료된 사용자는 FE가 그걸 보고 갱신한다. 갱신 쿠키는
경로가 `/api/v1/auth`라 여기까지 실려 온다.

DB 오류는 삼키지 않는다 — 5xx는 FE가 「답을 못 받았다」로 보고 세션을 비우지 않는다.
*/
func (h *AuthHandler) Session(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	if userID, err := authctx.RequireUserID(r.Context()); err == nil {
		session, err := h.auth.Me(r.Context(), userID)
		switch {
		case err == nil:
			response.JSON(w, http.StatusOK, map[string]any{"data": session, "refreshable": true})
			return
		case errors.Is(err, repository.ErrUserNotFound):
			// 토큰은 멀쩡한데 계정이 없다(탈퇴) — 비로그인과 같다.
		default:
			writeServiceError(w, err)
			return
		}
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"data":        nil,
		"refreshable": refreshTokenFromRequest(r) != "",
	})
}

func setAuthCookies(w http.ResponseWriter, auth *service.AuthService, pair *service.TokenPair) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.AccessCookieName(),
		Value:    pair.AccessToken,
		Path:     "/",
		MaxAge:   pair.ExpiresIn,
		HttpOnly: true,
		Secure:   auth.CookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     auth.RefreshCookieName(),
		Value:    pair.RefreshToken,
		Path:     "/api/v1/auth",
		MaxAge:   auth.RefreshMaxAgeUntil(pair.RefreshExpiresAt),
		HttpOnly: true,
		Secure:   auth.CookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearAuthCookies(w http.ResponseWriter, auth *service.AuthService) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.AccessCookieName(),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   auth.CookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     auth.RefreshCookieName(),
		Value:    "",
		Path:     "/api/v1/auth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   auth.CookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func refreshTokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie("qp_refresh"); err == nil && c.Value != "" {
		return c.Value
	}
	return r.URL.Query().Get("refresh")
}

// ---------------------------------------------------------------- 계정 관리

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.auth.ChangePassword(r.Context(), userID, body.CurrentPassword, body.NewPassword); err != nil {
		writeServiceError(w, err)
		return
	}
	// 모든 세션을 끊었으므로 이 브라우저의 쿠키도 함께 지운다 — 남겨 두면
	// 화면은 로그인 상태인데 요청은 전부 401이 되는 어정쩡한 상태가 된다.
	clearAuthCookies(w, h.auth)
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}

func (h *AuthHandler) ChangeDisplayName(w http.ResponseWriter, r *http.Request) {
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	var body struct {
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	session, err := h.auth.ChangeDisplayName(r.Context(), userID, body.DisplayName)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": session})
}

func (h *AuthHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.auth.DeleteAccount(r.Context(), userID, body.Password); err != nil {
		writeServiceError(w, err)
		return
	}
	clearAuthCookies(w, h.auth)
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}
