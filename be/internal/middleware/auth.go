package middleware

import (
	"net/http"
	"strings"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

func RequireAuth(auth *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := authenticateRequest(r, auth)
			if !ok {
				response.Error(w, http.StatusUnauthorized, "authentication required")
				return
			}
			ctx := authctx.WithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OptionalAuth(auth *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userID, ok := authenticateRequest(r, auth); ok {
				r = r.WithContext(authctx.WithUserID(r.Context(), userID))
			}
			next.ServeHTTP(w, r)
		})
	}
}

func authenticateRequest(r *http.Request, auth *service.AuthService) (int64, bool) {
	token := accessTokenFromRequest(r)
	if token == "" {
		return 0, false
	}
	userID, _, err := auth.ParseAccessToken(token)
	if err != nil || userID <= 0 {
		return 0, false
	}
	return userID, true
}

func accessTokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie("qp_access"); err == nil && c.Value != "" {
		return c.Value
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return ""
}
