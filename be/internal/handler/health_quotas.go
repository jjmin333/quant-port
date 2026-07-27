package handler

import (
	"net/http"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/pkg/response"
)

func (h *HealthHandler) APIQuotas(w http.ResponseWriter, r *http.Request) {
	if h.apiBudget == nil {
		response.JSON(w, http.StatusOK, []apibudget.Usage{})
		return
	}
	response.JSON(w, http.StatusOK, h.apiBudget.AllUsage(r.Context()))
}
