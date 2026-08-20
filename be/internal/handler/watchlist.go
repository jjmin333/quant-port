package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type WatchlistHandler struct {
	svc *service.WatchlistService
}

func NewWatchlistHandler(svc *service.WatchlistService) *WatchlistHandler {
	return &WatchlistHandler{svc: svc}
}

func (h *WatchlistHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": items})
}

func (h *WatchlistHandler) Add(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Symbol string `json:"symbol"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.svc.Add(r.Context(), body.Symbol); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{"data": map[string]string{"symbol": body.Symbol}})
}

func (h *WatchlistHandler) Remove(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	if err := h.svc.Remove(r.Context(), symbol); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]string{"symbol": symbol}})
}

func (h *WatchlistHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Symbols []string `json:"symbols"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.svc.Reorder(r.Context(), body.Symbols); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}

func (h *WatchlistHandler) BulkAdd(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Symbols []string `json:"symbols"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	added, err := h.svc.BulkAdd(r.Context(), body.Symbols)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]int{"added": added}})
}
