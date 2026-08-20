package handler

import (
	"encoding/json"
	"net/http"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type UserCashBalancesHandler struct {
	svc *service.UserCashBalancesService
}

func NewUserCashBalancesHandler(svc *service.UserCashBalancesService) *UserCashBalancesHandler {
	return &UserCashBalancesHandler{svc: svc}
}

func (h *UserCashBalancesHandler) Get(w http.ResponseWriter, r *http.Request) {
	balances, err := h.svc.Get(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": balances})
}

func (h *UserCashBalancesHandler) Replace(w http.ResponseWriter, r *http.Request) {
	var body model.UserCashBalances
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.svc.Replace(r.Context(), body); err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": body})
}
