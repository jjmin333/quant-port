package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type StockSummaryHandler struct {
	svc *service.StockSummaryService
}

func NewStockSummaryHandler(svc *service.StockSummaryService) *StockSummaryHandler {
	return &StockSummaryHandler{svc: svc}
}

func (h *StockSummaryHandler) Get(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol is required")
		return
	}

	view, err := h.svc.Get(r.Context(), symbol)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": view})
}
