package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type UserHoldingsHandler struct {
	svc *service.UserHoldingsService
}

func NewUserHoldingsHandler(svc *service.UserHoldingsService) *UserHoldingsHandler {
	return &UserHoldingsHandler{svc: svc}
}

func (h *UserHoldingsHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if items == nil {
		items = []model.UserHolding{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": items})
}

func (h *UserHoldingsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	if err := h.svc.DeleteSymbol(r.Context(), symbol); err != nil {
		response.Error(w, http.StatusNotFound, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]string{"symbol": symbol}})
}

func (h *UserHoldingsHandler) Replace(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Holdings []model.UserHolding `json:"holdings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.svc.ReplaceAll(r.Context(), body.Holdings); err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]int{"count": len(body.Holdings)}})
}
