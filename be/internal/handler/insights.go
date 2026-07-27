package handler

import (
	"net/http"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type InsightsHandler struct {
	svc *service.InsightsService
}

func NewInsightsHandler(svc *service.InsightsService) *InsightsHandler {
	return &InsightsHandler{svc: svc}
}

func (h *InsightsHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetDashboard(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *InsightsHandler) Diagnostics(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]any{"data": h.svc.Diagnostics()})
}
