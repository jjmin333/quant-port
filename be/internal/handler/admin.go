package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/enact/quant-port/be/pkg/response"
)

type AdminHandler struct {
	keywords     *repository.NewsKeywordRepo
	newsStore    *service.NewsStore
	apiBudget    *apibudget.Manager
	dataSettings *service.MarketDataSettingsStore
	cacheAdmin   *service.CacheAdminService
	dbAdmin      *service.DBAdminService
	stockSummary *service.StockSummaryService
	llmSettings  *service.LLMSettingsStore

	scores        *service.ScoreService
	scoreSettings *service.ScoreSettingsStore
	batch         *service.BatchAdminService
}

func NewAdminHandler(
	keywords *repository.NewsKeywordRepo,
	newsStore *service.NewsStore,
	apiBudget *apibudget.Manager,
	dataSettings *service.MarketDataSettingsStore,
	cacheAdmin *service.CacheAdminService,
	dbAdmin *service.DBAdminService,
	stockSummary *service.StockSummaryService,
	llmSettings *service.LLMSettingsStore,
	scores *service.ScoreService,
	scoreSettings *service.ScoreSettingsStore,
	batch *service.BatchAdminService,
) *AdminHandler {
	return &AdminHandler{
		keywords:      keywords,
		newsStore:     newsStore,
		apiBudget:     apiBudget,
		dataSettings:  dataSettings,
		cacheAdmin:    cacheAdmin,
		dbAdmin:       dbAdmin,
		stockSummary:  stockSummary,
		llmSettings:   llmSettings,
		scores:        scores,
		batch:         batch,
		scoreSettings: scoreSettings,
	}
}

// GetLLMSettings returns the admin-editable AI 요약 batch settings.
func (h *AdminHandler) GetLLMSettings(w http.ResponseWriter, r *http.Request) {
	if h.llmSettings == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": model.DefaultLLMSettings()})
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": h.llmSettings.Get(r.Context())})
}

// UpdateLLMSettings replaces the batch settings. Takes effect immediately (no
// restart) — StockSummaryService re-reads this store on every call. Omitted
// numeric fields keep their stored value rather than resetting to zero, so a
// caller that only edits the symbol list cannot accidentally disable the
// daily cap.
func (h *AdminHandler) UpdateLLMSettings(w http.ResponseWriter, r *http.Request) {
	if h.llmSettings == nil {
		response.Error(w, http.StatusServiceUnavailable, "llm settings unavailable")
		return
	}
	var body struct {
		PilotSymbols []string `json:"pilotSymbols"`
		TargetMode   *string  `json:"targetMode"`
		DailyCap     *int     `json:"dailyCap"`
		QuotaReserve *int     `json:"quotaReserve"`
		TokenReserve *int64   `json:"tokenReserve"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	next := h.llmSettings.Get(r.Context())
	next.PilotSymbols = body.PilotSymbols
	if body.TargetMode != nil {
		next.TargetMode = *body.TargetMode
	}
	if body.DailyCap != nil {
		next.DailyCap = *body.DailyCap
	}
	if body.QuotaReserve != nil {
		next.QuotaReserve = *body.QuotaReserve
	}
	if body.TokenReserve != nil {
		next.TokenReserve = *body.TokenReserve
	}

	updated, err := h.llmSettings.Update(r.Context(), next)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": updated})
}

// LLMBatchStatus reports each market's scheduled AI 요약 run: next window,
// last result, recent history, and today's remaining provider allowance.
// Read-only — never triggers generation.
func (h *AdminHandler) LLMBatchStatus(w http.ResponseWriter, r *http.Request) {
	if h.stockSummary == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": model.LLMBatchStatus{}})
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": h.stockSummary.BatchStatus(r.Context())})
}

// LLMBatchTargets previews the symbols a run would pick for one market right
// now, with the exclusion tally behind them. Read-only.
func (h *AdminHandler) LLMBatchTargets(w http.ResponseWriter, r *http.Request) {
	if h.stockSummary == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": model.LLMBatchSelection{}})
		return
	}
	market := toss.MarketCountry(strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("market"))))
	if market != toss.MarketKR && market != toss.MarketUS {
		response.Error(w, http.StatusBadRequest, "market must be KR or US")
		return
	}
	selection, err := h.stockSummary.PreviewBatchTargets(r.Context(), market)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": selection})
}

// TriggerLLMRefresh regenerates the AI 요약 for arbitrary symbols right away,
// outside the fixed 00:00:00 KST schedule. Pilot-list membership is not
// required — the admin picked these symbols explicitly — but the apibudget
// quota guard still applies. Returns immediately; poll LLMRefreshStatus.
func (h *AdminHandler) TriggerLLMRefresh(w http.ResponseWriter, r *http.Request) {
	if h.stockSummary == nil {
		response.Error(w, http.StatusServiceUnavailable, "stock summary unavailable")
		return
	}
	var body struct {
		Symbols []string `json:"symbols"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	job, err := h.stockSummary.TriggerManualRefresh(body.Symbols)
	if err != nil {
		if errors.Is(err, service.ErrManualRefreshBusy) {
			response.Error(w, http.StatusConflict, err.Error())
			return
		}
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusAccepted, map[string]any{"data": job})
}

// LLMRefreshStatus reports the most recent manual regeneration's per-symbol
// progress and result.
func (h *AdminHandler) LLMRefreshStatus(w http.ResponseWriter, r *http.Request) {
	if h.stockSummary == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": model.LLMManualRefreshJob{}})
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": h.stockSummary.ManualRefreshStatus()})
}

// GetLLMRequestLog returns a page of GenerateStockSummary calls (input facts
// + success/failure), newest-first — read-only auditing. Supports
// ?status=failed to narrow to failures only, and ?page=&pageSize= to page
// through history beyond the most recent entries.
func (h *AdminHandler) GetLLMRequestLog(w http.ResponseWriter, r *http.Request) {
	if h.stockSummary == nil {
		response.JSON(w, http.StatusOK, map[string]any{
			"data": map[string]any{"entries": []any{}, "total": 0, "page": 1, "pageSize": 30},
		})
		return
	}
	q := r.URL.Query()
	onlyFailed := strings.EqualFold(strings.TrimSpace(q.Get("status")), "failed")
	page, err := strconv.Atoi(q.Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(q.Get("pageSize"))
	if err != nil || pageSize < 1 {
		pageSize = 30
	}
	if pageSize > 200 {
		pageSize = 200
	}
	entries, total := h.stockSummary.RequestLogPage(r.Context(), onlyFailed, page, pageSize)
	response.JSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"entries":  entries,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

// StockSummaryStatus reports the AI 요약 feature's active LLM provider and
// each pilot symbol's cache status — read-only, never triggers generation.
func (h *AdminHandler) StockSummaryStatus(w http.ResponseWriter, r *http.Request) {
	if h.stockSummary == nil {
		response.JSON(w, http.StatusOK, map[string]any{
			"data": map[string]any{"provider": "", "enabled": false, "symbols": []any{}},
		})
		return
	}
	provider, enabled := h.stockSummary.Provider()
	response.JSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"provider": provider,
			"enabled":  enabled,
			"symbols":  h.stockSummary.PilotStatus(r.Context()),
		},
	})
}

func (h *AdminHandler) ListNewsKeywords(w http.ResponseWriter, r *http.Request) {
	market := strings.TrimSpace(r.URL.Query().Get("market"))
	if market == "" {
		response.Error(w, http.StatusBadRequest, "market query required (domestic or us)")
		return
	}
	items, err := h.keywords.List(r.Context(), market, true)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": items})
}

func (h *AdminHandler) CreateNewsKeyword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Market    string `json:"market"`
		Keyword   string `json:"keyword"`
		Enabled   *bool  `json:"enabled"`
		SortOrder int    `json:"sortOrder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	item, err := h.keywords.Create(r.Context(), body.Market, body.Keyword, enabled, body.SortOrder)
	if err != nil {
		if err == repository.ErrNewsKeywordDuplicate {
			response.Error(w, http.StatusConflict, "keyword already exists")
			return
		}
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	h.newsStore.InvalidateAdminKeywords()
	h.newsStore.ResetKeywordCursors(r.Context())
	response.JSON(w, http.StatusCreated, map[string]any{"data": item})
}

func (h *AdminHandler) UpdateNewsKeyword(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Keyword   string `json:"keyword"`
		Enabled   bool   `json:"enabled"`
		SortOrder int    `json:"sortOrder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	item, err := h.keywords.Update(r.Context(), id, body.Keyword, body.Enabled, body.SortOrder)
	if err != nil {
		if err == repository.ErrNewsKeywordNotFound {
			response.Error(w, http.StatusNotFound, "keyword not found")
			return
		}
		if err == repository.ErrNewsKeywordDuplicate {
			response.Error(w, http.StatusConflict, "keyword already exists")
			return
		}
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	h.newsStore.InvalidateAdminKeywords()
	h.newsStore.ResetKeywordCursors(r.Context())
	response.JSON(w, http.StatusOK, map[string]any{"data": item})
}

func (h *AdminHandler) DeleteNewsKeyword(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.keywords.Delete(r.Context(), id); err != nil {
		if err == repository.ErrNewsKeywordNotFound {
			response.Error(w, http.StatusNotFound, "keyword not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.newsStore.InvalidateAdminKeywords()
	h.newsStore.ResetKeywordCursors(r.Context())
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}

func (h *AdminHandler) NewsStats(w http.ResponseWriter, r *http.Request) {
	domesticCount, usCount, domesticUpdated, usUpdated := h.newsStore.NewsStats(r.Context())
	response.JSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"domesticCount":   domesticCount,
			"usCount":         usCount,
			"domesticUpdated": formatTimeOrNil(domesticUpdated),
			"usUpdated":       formatTimeOrNil(usUpdated),
		},
	})
}

func (h *AdminHandler) ResetNewsCursors(w http.ResponseWriter, r *http.Request) {
	h.newsStore.ResetKeywordCursors(r.Context())
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}

func (h *AdminHandler) GetNewsPollSettings(w http.ResponseWriter, r *http.Request) {
	settings := h.newsStore.GetPollSettings(r.Context())
	response.JSON(w, http.StatusOK, map[string]any{"data": settings})
}

func (h *AdminHandler) UpdateNewsPollSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DomesticNaverKeywordsPerTick int `json:"domesticNaverKeywordsPerTick"`
		DomesticKakaoKeywordsPerTick int `json:"domesticKakaoKeywordsPerTick"`
		DomesticKakaoPollMinutes     int `json:"domesticKakaoPollMinutes"`
		USGoogleKeywordsPerTick      int `json:"usGoogleKeywordsPerTick"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	updated, err := h.newsStore.UpdatePollSettings(r.Context(), model.NewsPollSettings{
		DomesticNaverKeywordsPerTick: body.DomesticNaverKeywordsPerTick,
		DomesticKakaoKeywordsPerTick: body.DomesticKakaoKeywordsPerTick,
		DomesticKakaoPollMinutes:     body.DomesticKakaoPollMinutes,
		USGoogleKeywordsPerTick:      body.USGoogleKeywordsPerTick,
	})
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": updated})
}

type apiQuotaEntry struct {
	apibudget.Usage
	Label    string             `json:"label"`
	Purpose  string             `json:"purpose"`
	Category apibudget.Category `json:"category"`
}

func (h *AdminHandler) APIQuotas(w http.ResponseWriter, r *http.Request) {
	var usages []apibudget.Usage
	if h.apiBudget != nil {
		usages = h.apiBudget.AllUsage(r.Context())
	}
	entries := make([]apiQuotaEntry, 0, len(usages))
	for _, u := range usages {
		cat := apibudget.CatalogFor(u.Provider)
		entries = append(entries, apiQuotaEntry{
			Usage:    u,
			Label:    cat.Label,
			Purpose:  cat.Purpose,
			Category: cat.Category,
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"data":       entries,
		"categories": apibudget.CategoryLabels,
	})
}

func (h *AdminHandler) GetMarketDataSettings(w http.ResponseWriter, r *http.Request) {
	if h.dataSettings == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": model.DefaultMarketDataSettings()})
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": h.dataSettings.Get(r.Context())})
}

func (h *AdminHandler) UpdateMarketDataSettings(w http.ResponseWriter, r *http.Request) {
	if h.dataSettings == nil {
		response.Error(w, http.StatusServiceUnavailable, "market data settings not configured")
		return
	}
	var body model.MarketDataSettings
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	updated, err := h.dataSettings.Update(r.Context(), body)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": updated})
}

func (h *AdminHandler) ListCacheNamespaces(w http.ResponseWriter, r *http.Request) {
	if h.cacheAdmin == nil {
		response.Error(w, http.StatusServiceUnavailable, "cache admin unavailable")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": h.cacheAdmin.Namespaces()})
}

func (h *AdminHandler) ListCacheEntries(w http.ResponseWriter, r *http.Request) {
	if h.cacheAdmin == nil {
		response.Error(w, http.StatusServiceUnavailable, "cache admin unavailable")
		return
	}
	pattern := strings.TrimSpace(r.URL.Query().Get("pattern"))
	cursor, _ := strconv.ParseUint(r.URL.Query().Get("cursor"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	result, err := h.cacheAdmin.List(r.Context(), pattern, cursor, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *AdminHandler) GetCacheEntry(w http.ResponseWriter, r *http.Request) {
	if h.cacheAdmin == nil {
		response.Error(w, http.StatusServiceUnavailable, "cache admin unavailable")
		return
	}
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	entry, err := h.cacheAdmin.Get(r.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrCacheKeyForbidden) {
			response.Error(w, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, service.ErrCacheKeyNotFound) {
			response.Error(w, http.StatusNotFound, err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": entry})
}

func (h *AdminHandler) UpdateCacheEntry(w http.ResponseWriter, r *http.Request) {
	if h.cacheAdmin == nil {
		response.Error(w, http.StatusServiceUnavailable, "cache admin unavailable")
		return
	}
	var body struct {
		Key        string `json:"key"`
		Value      string `json:"value"`
		TTLSeconds *int64 `json:"ttlSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.cacheAdmin.Set(r.Context(), body.Key, body.Value, body.TTLSeconds); err != nil {
		if errors.Is(err, service.ErrCacheKeyForbidden) {
			response.Error(w, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, service.ErrCacheKeyReadOnly) {
			response.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	entry, err := h.cacheAdmin.Get(r.Context(), body.Key)
	if err != nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": entry})
}

func (h *AdminHandler) DeleteCacheEntry(w http.ResponseWriter, r *http.Request) {
	if h.cacheAdmin == nil {
		response.Error(w, http.StatusServiceUnavailable, "cache admin unavailable")
		return
	}
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	if err := h.cacheAdmin.Delete(r.Context(), key); err != nil {
		if errors.Is(err, service.ErrCacheKeyForbidden) {
			response.Error(w, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, service.ErrCacheKeyNotFound) {
			response.Error(w, http.StatusNotFound, err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}

func formatTimeOrNil(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}
