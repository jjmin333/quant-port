package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type FundamentalsHandler struct {
	svc *service.FundamentalsService
}

func NewFundamentalsHandler(svc *service.FundamentalsService) *FundamentalsHandler {
	return &FundamentalsHandler{svc: svc}
}

func (h *FundamentalsHandler) Get(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol is required")
		return
	}

	view, err := h.svc.GetWithValuation(r.Context(), symbol)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if view == nil {
		response.Error(w, http.StatusNotFound, "stock not found")
		return
	}

	stock, _ := h.svc.LookupStock(r.Context(), symbol)
	resp := service.BuildFundamentalsResponse(view, stock)
	response.JSON(w, http.StatusOK, map[string]any{"data": resp})
}
