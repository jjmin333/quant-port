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
	// 값이 바뀐 시각(updatedAt)과 별개로 「마지막으로 확인한 시각」. 화면의 「낡은 값」
	// 배지가 이것을 본다 — 주말에 값이 안 변한 것은 낡은 것이 아니다.
	if checked := h.svc.IndicesCheckedAt(); !checked.IsZero() {
		payload["checkedAt"] = checked.Format(time.RFC3339)
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
	res := h.svc.BatchQuoteSummariesResult(r.Context(), parts)
	payload := map[string]any{
		"data":   res.Summaries,
		"source": h.source(),
		// 서버가 한 번에 풀어 주는 심볼 수. 보유·메모·목표가처럼 사용자 데이터에서
		// 심볼을 만드는 화면은 이 수를 넘길 수 있는데, 지금까지는 넘겼다는 사실이
		// 응답 어디에도 없어 화면이 조용히 일부만 그렸다.
		"limit": service.BatchQuoteSummariesMax,
	}
	// **못 푼 심볼을 조용히 버리지 않는다.**
	//
	// 예전에는 `data`가 짧아지는 것이 전부였다. 호출부는 「없는 종목」과 「서버가
	// 시간 안에 못 푼 종목」을 구분할 수 없어, 뉴스 칩이 이름 대신 종목코드를
	// 그리는 것으로 나타났다. deadline=true면 재요청이 값어치가 있고,
	// false면 그 심볼은 다시 물어도 같은 답이다.
	if len(res.Missing) > 0 {
		payload["missing"] = res.Missing
	}
	if res.Truncated > 0 {
		payload["truncated"] = res.Truncated
	}
	if res.Deadline {
		payload["deadline"] = true
	}
	response.JSON(w, http.StatusOK, payload)
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
	// 화면이 호가를 **실제로 그릴 때만** book=1 을 붙인다. 그 종목만 웹소켓에서
	// 호가를 구독한다(planTopics 참조) — 상세 화면 진입만으로는 붙지 않는다.
	if wantsBook, _ := strconv.ParseBool(r.URL.Query().Get("book")); wantsBook {
		h.svc.TouchOrderBookInterest(symbol)
	}
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

/*
ListPriceStream serves GET /quotes/prices/stream?symbols=A,B,C&assetType=us —
목록 화면 여러 줄의 가격을 **연결 하나로** 밀어 준다.

폴링을 대신하는 자리라 실패는 조용해야 한다: 화면은 스트림이 안 붙으면 폴링으로
버티도록 만들어져 있으므로, 여기서 오류를 내도 목록이 비지 않는다.
*/
func (h *QuotesHandler) ListPriceStream(w http.ResponseWriter, r *http.Request) {
	symbols := service.ParseListStreamSymbols(r.URL.Query().Get("symbols"))
	h.svc.ServeListPriceStream(w, r, service.ListPriceStreamParams{
		Symbols:   symbols,
		AssetType: r.URL.Query().Get("assetType"),
	})
}

func (h *QuotesHandler) source() string {
	if h.svc.TossEnabled() {
		return "toss"
	}
	return "mock"
}
