package handler

import (
	"encoding/json"
	"net/http"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}

	session, pair, err := h.auth.Signup(r.Context(), body.Email, body.Password, body.DisplayName)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	setAuthCookies(w, h.auth, pair)
	response.JSON(w, http.StatusCreated, map[string]any{"data": session})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}

	session, pair, err := h.auth.Login(r.Context(), body.Email, body.Password, r.UserAgent())
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
