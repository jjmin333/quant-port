package middleware

import (
	"net/http"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/pkg/response"
)

func RequireAdmin(users *repository.UserRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, err := authctx.RequireUserID(r.Context())
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "authentication required")
				return
			}
			user, err := users.GetByID(r.Context(), userID)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "authentication required")
				return
			}
			if !model.IsAdminUser(user.Email, user.Role) {
				response.Error(w, http.StatusForbidden, "admin access required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MarkAdmin lets the request through either way and only records whether the
// caller is an admin — for routes an author owns but an admin may also moderate
// (edit·delete someone else's post). One user lookup, only on those routes.
func MarkAdmin(users *repository.UserRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userID, ok := authctx.UserID(r.Context()); ok {
				if user, err := users.GetByID(r.Context(), userID); err == nil && model.IsAdminUser(user.Email, user.Role) {
					r = r.WithContext(authctx.WithAdmin(r.Context()))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
