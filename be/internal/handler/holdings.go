package handler

import (
	"net/http"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type HoldingsHandler struct {
	svc *service.HoldingsService
}

func NewHoldingsHandler(svc *service.HoldingsService) *HoldingsHandler {
	return &HoldingsHandler{svc: svc}
}

func (h *HoldingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	accountSeq, ok := requireAccountSeq(w, r)
	if !ok {
		return
	}

	symbol := r.URL.Query().Get("symbol")
	holdings, err := h.svc.Get(r.Context(), accountSeq, symbol)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": holdings})
}
