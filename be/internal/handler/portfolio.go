package handler

import (
	"net/http"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type PortfolioHandler struct {
	svc *service.PortfolioService
}

func NewPortfolioHandler(svc *service.PortfolioService) *PortfolioHandler {
	return &PortfolioHandler{svc: svc}
}

func (h *PortfolioHandler) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.svc.GetSummary(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": summary})
}

func (h *PortfolioHandler) Live(w http.ResponseWriter, r *http.Request) {
	summary, err := h.svc.GetLive(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": summary})
}

func (h *PortfolioHandler) History(w http.ResponseWriter, r *http.Request) {
	rangeKey, _ := service.ParseHistoryRange(r.URL.Query().Get("range"))

	var currentTotal float64
	if base, _, err := h.svc.GetBaseSummary(r.Context()); err == nil && base != nil {
		currentTotal = base.TotalAssetsKRW
	}

	history, err := h.svc.GetHistory(r.Context(), rangeKey, currentTotal)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": history})
}

func (h *PortfolioHandler) Diagnostics(w http.ResponseWriter, r *http.Request) {
	d := h.svc.Diagnostics(r.Context())
	status := http.StatusOK
	if !d.TossEnabled || d.LastError != "" {
		status = http.StatusServiceUnavailable
	}
	response.JSON(w, status, map[string]any{"data": d})
}
