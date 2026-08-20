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
