package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type InsiderHandler struct {
	svc *service.InsiderTabService
}

func NewInsiderHandler(svc *service.InsiderTabService) *InsiderHandler {
	return &InsiderHandler{svc: svc}
}

func (h *InsiderHandler) Get(w http.ResponseWriter, r *http.Request) {
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
