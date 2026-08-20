package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type HoldingTradesHandler struct {
	svc *service.HoldingTradesService
}

func NewHoldingTradesHandler(svc *service.HoldingTradesService) *HoldingTradesHandler {
	return &HoldingTradesHandler{svc: svc}
}

// List serves GET /user/holdings/trades — the whole log, or one symbol's when
// ?symbol= is given.
func (h *HoldingTradesHandler) List(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
	var (
		rows []model.HoldingTrade
		err  error
	)
	if symbol != "" {
		rows, err = h.svc.ListSymbol(r.Context(), symbol)
	} else {
		rows, err = h.svc.List(r.Context())
	}
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if rows == nil {
		rows = []model.HoldingTrade{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": rows})
}

func (h *HoldingTradesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body model.HoldingTrade
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	pos, err := h.svc.Create(r.Context(), body)
	if err != nil {
		if strings.Contains(err.Error(), "invalid trade") {
			response.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": pos})
}

func (h *HoldingTradesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "id")), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "invalid trade id")
		return
	}
	// The symbol comes along so the derived holding can be recomputed without a
	// second lookup of a row we just deleted.
	symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
	pos, err := h.svc.Delete(r.Context(), id, symbol)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.Error(w, http.StatusNotFound, err.Error())
			return
		}
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": pos})
}

// Valuation serves GET /user/holdings/valuation?days=90.
func (h *HoldingTradesHandler) Valuation(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("days")))
	series, err := h.svc.Valuation(r.Context(), days)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": series})
}

func (h *HoldingTradesHandler) BulkRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Items []model.BulkRegisterTradeItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.svc.BulkRegister(r.Context(), body.Items); err != nil {
		if strings.Contains(err.Error(), "invalid trade") {
			response.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}
