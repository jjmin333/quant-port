package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type FinancialHistoryHandler struct {
	svc *service.FinancialHistoryService
}

func NewFinancialHistoryHandler(svc *service.FinancialHistoryService) *FinancialHistoryHandler {
	return &FinancialHistoryHandler{svc: svc}
}

func (h *FinancialHistoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol is required")
		return
	}
	years, _ := strconv.Atoi(r.URL.Query().Get("years"))

	view, err := h.svc.Get(r.Context(), symbol, years)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": view})
}
