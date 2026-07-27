package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/enact/quant-port/be/pkg/response"
)

type MarketHandler struct {
	svc *service.MarketService
}

func NewMarketHandler(svc *service.MarketService) *MarketHandler {
	return &MarketHandler{svc: svc}
}

func (h *MarketHandler) ExchangeRate(w http.ResponseWriter, r *http.Request) {
	base := r.URL.Query().Get("baseCurrency")
	if base == "" {
		base = string(toss.CurrencyUSD)
	}
	quote := r.URL.Query().Get("quoteCurrency")
	if quote == "" {
		quote = string(toss.CurrencyKRW)
	}

	result, err := h.svc.ExchangeRate(r.Context(), toss.GetExchangeRateParams{
		BaseCurrency:  toss.Currency(base),
		QuoteCurrency: toss.Currency(quote),
		DateTime:      r.URL.Query().Get("dateTime"),
	})
	if err != nil {
		writeTossError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *MarketHandler) KrCalendar(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.KrCalendar(r.Context(), r.URL.Query().Get("date"))
	if err != nil {
		writeTossError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *MarketHandler) UsCalendar(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.UsCalendar(r.Context(), r.URL.Query().Get("date"))
	if err != nil {
		writeTossError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *MarketHandler) Rankings(w http.ResponseWriter, r *http.Request) {
	rankingType := r.URL.Query().Get("type")
	if rankingType == "" {
		response.Error(w, http.StatusBadRequest, "type is required")
		return
	}
	marketCountry := r.URL.Query().Get("marketCountry")
	if marketCountry == "" {
		marketCountry = string(toss.MarketKR)
	}
	duration := r.URL.Query().Get("duration")
	if duration == "" {
		duration = string(toss.RankingRealtime)
	}

	exclude, _ := strconv.ParseBool(r.URL.Query().Get("excludeInvestmentCaution"))
	count, _ := strconv.Atoi(r.URL.Query().Get("count"))

	result, err := h.svc.Rankings(r.Context(), toss.GetRankingsParams{
		Type:                     toss.RankingType(rankingType),
		MarketCountry:            toss.MarketCountry(marketCountry),
		Duration:                 toss.RankingDuration(duration),
		ExcludeInvestmentCaution: exclude,
		Count:                    count,
	})
	if err != nil {
		writeTossError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *MarketHandler) GetStock(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	result, err := h.svc.Stock(r.Context(), symbol)
	if err != nil {
		writeTossError(w, err)
		return
	}
	if result == nil {
		response.Error(w, http.StatusNotFound, "stock not found")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *MarketHandler) ListStocks(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("symbols")
	if raw == "" {
		response.Error(w, http.StatusBadRequest, "symbols is required")
		return
	}
	symbols := strings.Split(raw, ",")
	result, err := h.svc.Stocks(r.Context(), symbols)
	if err != nil {
		writeTossError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *MarketHandler) StockWarnings(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	result, err := h.svc.StockWarnings(r.Context(), symbol)
	if err != nil {
		writeTossError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *MarketHandler) MarketIndicatorPrices(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("symbols")
	if raw == "" {
		response.Error(w, http.StatusBadRequest, "symbols is required")
		return
	}
	symbols := strings.Split(raw, ",")
	result, err := h.svc.MarketIndicatorPrices(r.Context(), symbols)
	if err != nil {
		writeTossError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}
