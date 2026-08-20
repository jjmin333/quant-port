package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// dailyCloseMaxLimit bounds one page of 일별 종가 rows. The UI is a dated list,
// not an infinite feed, so a few months of sessions is plenty.
const dailyCloseMaxLimit = 120

type DailyCloseHandler struct {
	svc *service.DailyCloseService
}

func NewDailyCloseHandler(svc *service.DailyCloseService) *DailyCloseHandler {
	return &DailyCloseHandler{svc: svc}
}

// StockDailyCloses serves GET /stocks/{symbol}/daily-closes.
func (h *DailyCloseHandler) StockDailyCloses(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimSpace(chi.URLParam(r, "symbol"))
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol required")
		return
	}
	if h.svc == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": emptyDailyCloseSeries()})
		return
	}

	series, err := h.svc.StockSeries(r.Context(), symbol, dailyCloseLimit(r))
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": series})
}

// IndexDailyCloses serves GET /indices/{indexId}/daily-closes.
func (h *DailyCloseHandler) IndexDailyCloses(w http.ResponseWriter, r *http.Request) {
	indexID := strings.TrimSpace(chi.URLParam(r, "indexId"))
	if indexID == "" {
		response.Error(w, http.StatusBadRequest, "index id required")
		return
	}
	if h.svc == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": emptyDailyCloseSeries()})
		return
	}

	series, err := h.svc.IndexSeries(r.Context(), indexID, dailyCloseLimit(r))
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": series})
}

func dailyCloseLimit(r *http.Request) int {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		return 60
	}
	if limit > dailyCloseMaxLimit {
		return dailyCloseMaxLimit
	}
	return limit
}

func emptyDailyCloseSeries() model.DailyCloseSeries {
	return model.DailyCloseSeries{Currency: "KRW", Rows: []model.DailyCloseRow{}}
}
