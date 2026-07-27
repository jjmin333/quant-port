package handler

import (
	"errors"
	"net/http"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

func writeServiceError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, authctx.ErrNoUser):
		response.Error(w, http.StatusUnauthorized, "authentication required")
	case errors.Is(err, repository.ErrBrokerNotLinked):
		response.Error(w, http.StatusForbidden, "broker account not linked")
	case errors.Is(err, service.ErrInvalidCredentials):
		response.Error(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, service.ErrWeakPassword):
		response.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrInvalidEmail):
		response.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrInvalidDisplayName):
		response.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, repository.ErrEmailTaken):
		response.Error(w, http.StatusConflict, "email already registered")
	case errors.Is(err, repository.ErrUserNotFound):
		response.Error(w, http.StatusUnauthorized, "invalid email or password")
	default:
		writeTossError(w, err)
	}
}
