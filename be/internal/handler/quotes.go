package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type QuotesHandler struct {
	svc *service.QuotesService
}

func NewQuotesHandler(svc *service.QuotesService) *QuotesHandler {
	return &QuotesHandler{svc: svc}
}

func (h *QuotesHandler) ListIndices(w http.ResponseWriter, r *http.Request) {
	indices, updatedAt := h.svc.ListIndices(r.Context())
	payload := map[string]any{
		"data":   indices,
		"source": h.source(),
	}
	if !updatedAt.IsZero() {
		payload["updatedAt"] = updatedAt.Format(time.RFC3339)
	}
	if groups := h.svc.ListIndexGroups(r.Context()); len(groups) > 0 {
		payload["groups"] = groups
	}
	response.JSON(w, http.StatusOK, payload)
}

func (h *QuotesHandler) ListQuotes(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("filter")
	query := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	result := h.svc.ListQuotes(r.Context(), filter, query, limit, offset)

	response.JSON(w, http.StatusOK, map[string]any{
		"data":   result.Items,
		"total":  result.Total,
		"source": h.source(),
	})
}

func (h *QuotesHandler) SearchStocksCatalog(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, total, err := h.svc.SearchStocksCatalog(r.Context(), model.StockSearchParams{
		Query:  r.URL.Query().Get("q"),
		Market: r.URL.Query().Get("market"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "stock search failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"data":  items,
		"total": total,
	})
}

func (h *QuotesHandler) BatchQuotes(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.URL.Query().Get("symbols"))
	if raw == "" {
		response.JSON(w, http.StatusOK, map[string]any{
			"data":   []model.QuoteSummary{},
			"source": h.source(),
		})
		return
	}
	parts := parseSymbolList(raw, service.BatchQuoteSummariesMax)
	summaries := h.svc.BatchQuoteSummaries(r.Context(), parts)
	response.JSON(w, http.StatusOK, map[string]any{
		"data":   summaries,
		"source": h.source(),
		// 서버가 한 번에 풀어 주는 심볼 수. 보유·메모·목표가처럼 사용자 데이터에서
		// 심볼을 만드는 화면은 이 수를 넘길 수 있는데, 지금까지는 넘겼다는 사실이
		// 응답 어디에도 없어 화면이 조용히 일부만 그렸다.
		"limit": service.BatchQuoteSummariesMax,
	})
}

func (h *QuotesHandler) GetQuote(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	quote, ok := h.svc.GetQuote(r.Context(), symbol)
	if !ok {
		response.Error(w, http.StatusNotFound, "quote not found")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"data":   quote,
		"source": h.source(),
	})
}

func (h *QuotesHandler) GetOrderBook(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	book, ok := h.svc.GetOrderBook(r.Context(), symbol)
	if !ok {
		response.Error(w, http.StatusNotFound, "orderbook not found")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"data":   book,
		"source": h.source(),
	})
}

func (h *QuotesHandler) GetTrades(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	trades, ok := h.svc.GetTrades(r.Context(), symbol)
	if !ok {
		response.Error(w, http.StatusNotFound, "trades not found")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"data":   trades,
		"source": h.source(),
	})
}

func (h *QuotesHandler) GetCandles(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "5m"
	}

	candles, ok := h.svc.GetCandlesPage(r.Context(), symbol, model.CandleInterval(interval), r.URL.Query().Get("before"))
	if !ok {
		response.Error(w, http.StatusNotFound, "candles not found")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"data":       candles.Candles,
		"nextBefore": candles.NextBefore,
		"interval":   interval,
		"source":     h.source(),
	})
}

func (h *QuotesHandler) GetTerminalSnapshot(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	snap, ok := h.svc.GetTerminalSnapshot(r.Context(), symbol)
	if !ok {
		response.Error(w, http.StatusNotFound, "quote not found")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"data":   snap,
		"source": h.source(),
	})
}

func (h *QuotesHandler) GetCandleSnapshot(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "5m"
	}

	snap, ok := h.svc.GetCandleSnapshot(r.Context(), symbol, model.CandleInterval(interval))
	if !ok {
		response.Error(w, http.StatusNotFound, "candles not found")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"data":   snap,
		"source": h.source(),
	})
}

func (h *QuotesHandler) Stream(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "5m"
	}
	h.svc.ServeStream(w, r, symbol, model.CandleInterval(interval))
}

func (h *QuotesHandler) QuoteStream(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	h.svc.ServeQuoteStream(w, r, symbol)
}

func (h *QuotesHandler) source() string {
	if h.svc.TossEnabled() {
		return "toss"
	}
	return "mock"
}
