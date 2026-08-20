package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type PriceTargetHandler struct {
	svc *service.PriceTargetService
}

func NewPriceTargetHandler(svc *service.PriceTargetService) *PriceTargetHandler {
	return &PriceTargetHandler{svc: svc}
}

func (h *PriceTargetHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.List(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": rows})
}

// priceTargetBody carries no base price — registration price is measured by the
// server (service.PriceTargetService.captureBasePrice).
type priceTargetBody struct {
	Symbol       string  `json:"symbol"`
	TargetPrice  float64 `json:"targetPrice"`
	TargetPrice2 float64 `json:"targetPrice2"`
	StopPrice    float64 `json:"stopPrice"`
	Currency     string  `json:"currency"`
	Note         string  `json:"note"`
}

func (b priceTargetBody) input() service.PriceTargetInput {
	return service.PriceTargetInput{
		Symbol:       b.Symbol,
		TargetPrice:  b.TargetPrice,
		TargetPrice2: b.TargetPrice2,
		StopPrice:    b.StopPrice,
		Currency:     b.Currency,
		Note:         b.Note,
	}
}

func (h *PriceTargetHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body priceTargetBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	row, err := h.svc.Create(r.Context(), body.input())
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{"data": row})
}

func (h *PriceTargetHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body priceTargetBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	row, err := h.svc.Update(r.Context(), id, body.input())
	if err != nil {
		if err == repository.ErrPriceTargetNotFound {
			response.Error(w, http.StatusNotFound, "target not found")
			return
		}
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": row})
}

func (h *PriceTargetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if err == repository.ErrPriceTargetNotFound {
			response.Error(w, http.StatusNotFound, "target not found")
			return
		}
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]int64{"id": id}})
}
