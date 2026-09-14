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
		// 모든 실패를 500으로 덮으면 「없는 종목」이 장애로 보고된다.
		// writeServiceError가 ErrUnknownSymbol을 404로 옮긴다.
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": view})
}
