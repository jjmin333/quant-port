package handler

import (
	"net/http"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type DisclosuresHandler struct {
	insights *service.InsightsService
}

func NewDisclosuresHandler(insights *service.InsightsService) *DisclosuresHandler {
	return &DisclosuresHandler{insights: insights}
}

func (h *DisclosuresHandler) Today(w http.ResponseWriter, r *http.Request) {
	items, err := h.insights.TodayDisclosures(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": items})
}
