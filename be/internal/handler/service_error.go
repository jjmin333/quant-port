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
	// 입력이 잘못됐다는 표식이 붙어 있으면 400. 이게 없으면 아래 default가
	// writeTossError로 넘기고, 거기 기본 분기가 502라 사용자 오타가 게이트웨이
	// 장애로 보고된다 (service/invalid_input.go).
	var invalid *service.InvalidInputError
	if errors.As(err, &invalid) {
		response.Error(w, http.StatusBadRequest, invalid.Reason)
		return
	}

	switch {
	case errors.Is(err, authctx.ErrNoUser):
		response.Error(w, http.StatusUnauthorized, "authentication required")
	case errors.Is(err, service.ErrInvalidCredentials):
		response.Error(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, service.ErrWeakPassword):
		response.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrInvalidLoginID):
		response.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrInvalidDisplayName):
		response.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, repository.ErrEmailTaken):
		response.Error(w, http.StatusConflict, "login id already registered")
	case errors.Is(err, repository.ErrDisplayNameTaken):
		response.Error(w, http.StatusConflict, "display name already taken")
	case errors.Is(err, repository.ErrUserNotFound):
		response.Error(w, http.StatusUnauthorized, "invalid login id or password")
	// 카탈로그에 없는 종목을 사용자 데이터에 넣으려 한 것 — 입력 오류가 아니라
	// 「그런 종목이 없다」이므로 404다 (service/symbol_catalog.go).
	case errors.Is(err, service.ErrUnknownSymbol):
		response.Error(w, http.StatusNotFound, err.Error())
	default:
		writeTossError(w, err)
	}
}
