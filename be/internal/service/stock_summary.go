package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

const (
	stockSummaryCacheKeyPrefix = "stock:summary:v3:"
	stockSummaryStaleAfter     = 6 * time.Hour
	stockSummaryCacheTTL       = 24 * time.Hour
	stockSummaryNewsLimit      = 12

	// llmRequestLogKey/Max — capped, newest-first log of every
	// GenerateStockSummary call (input facts + success/failure), for the
	// LLM 모델 관리 admin page's "요청 INPUT 기록" section.
	llmRequestLogKey = "llm:requests:v1:log"
	llmRequestLogMax = 300
	llmRequestLogTTL = 7 * 24 * time.Hour
)

func stockSummaryCacheKey(symbol string) string {
	return stockSummaryCacheKeyPrefix + strings.ToUpper(strings.TrimSpace(symbol))
}

// StockSummaryService assembles the per-symbol AI 요약 shown atop the
// 재무제표/실적/전망/내부자거래/차트/뉴스 tabs. One LLM call produces all four
// text sections at once (see llm.Client.GenerateStockSummary) — this service
// never calls the LLM per tab. Results are cached per symbol (not per user)
// with a stale-while-revalidate cache.
//
// LLM spend has exactly two entry points: the scheduled per-market batch
// (stock_summary_batch.go) and an explicit admin refresh. Ordinary reads never
// call the LLM, however stale their cache is — that is what makes a day's cost
// equal to the number of symbols the batch planned, and therefore predictable
// against a free-tier quota. A read that finds stale data still rebuilds the
// numbers; it just carries the previous run's narrative forward instead of
// paying for a new one.
type StockSummaryService struct {
	fundamentals     *FundamentalsService
	financialHistory *FinancialHistoryService
	earnings         *EarningsTabService
	insider          *InsiderTabService
	news             *NewsStore
	candles          *repository.CandleBarRepo
	stocks           *repository.StockRepo
	llm              *llm.Client
	redis            *redis.Client

	// rankings supplies 마켓 레이더's stored 1일 기준 top 100, which the batch
	// draws its targets from. Optional — without it the batch falls back to
	// the pinned symbol list only.
	rankings *repository.MarketRankingsRepo

	// settings holds the admin-editable batch knobs and always-include
	// symbol list (see /admin/llm — LLM 모델 관리). Re-read on every check,
	// so admin edits apply immediately.
	settings *LLMSettingsStore

	// dailyCloses persists each generated summary onto that trading day's
	// 일별 종가 row (score + AI 요약 text). Optional — nil simply skips it.
	dailyCloses *DailyCloseService

	// scores supplies the server-computed 종합 점수 and its axis breakdown to
	// the prompt. Without it the prose and the number shown beside it would be
	// reasoning from different inputs.
	scores *ScoreService

	// mu guards bgCtx, manualJob and lastBatchDate — written by the admin
	// manual refresh path (stock_summary_manual.go) and the batch runner,
	// read by HTTP handlers.
	mu        sync.RWMutex
	bgCtx     context.Context
	manualJob *model.LLMManualRefreshJob
	// lastBatchDate is the Redis-less fallback for "has this market already
	// run today", keyed by market.
	lastBatchDate map[toss.MarketCountry]string
	// runningMarkets guards against the scheduled batch and a 배치 관리 manual
	// trigger (stock_summary_batch.go RunBatchNow) executing for the same
	// market at once — both would write the same Redis run/history keys and
	// check the same LLM budget independently, so overlap could double-spend
	// the daily quota rather than just race on the record.
	runningMarkets map[toss.MarketCountry]bool
}

// SetRankings wires 마켓 레이더's stored rankings so the scheduled batch can
// pick its targets from the 1일 기준 top 100.
func (s *StockSummaryService) SetRankings(repo *repository.MarketRankingsRepo) {
	s.rankings = repo
}

// SetDailyCloses wires the 일별 종가 store so generated summaries are recorded
// against the trading day they analysed.
func (s *StockSummaryService) SetDailyCloses(svc *DailyCloseService) {
	s.dailyCloses = svc
}

// SetScores wires the 종합 점수 service so the AI 요약 can explain the score
// rather than restate raw figures alongside it.
func (s *StockSummaryService) SetScores(svc *ScoreService) {
	s.scores = svc
}

func NewStockSummaryService(
	fundamentals *FundamentalsService,
	financialHistory *FinancialHistoryService,
	earnings *EarningsTabService,
	insider *InsiderTabService,
	news *NewsStore,
	candles *repository.CandleBarRepo,
	stocks *repository.StockRepo,
	llmClient *llm.Client,
	redisClient *redis.Client,
	settings *LLMSettingsStore,
) *StockSummaryService {
	return &StockSummaryService{
		fundamentals:     fundamentals,
		financialHistory: financialHistory,
		earnings:         earnings,
		insider:          insider,
		news:             news,
		candles:          candles,
		stocks:           stocks,
		llm:              llmClient,
		redis:            redisClient,
		settings:         settings,
	}
}

// Provider reports the configured LLM backend ("openai"/"gemini"/"") and
// whether it's enabled, for admin monitoring.
func (s *StockSummaryService) Provider() (provider string, enabled bool) {
	if s.llm == nil {
		return "", false
	}
	return s.llm.Provider(), s.llm.Enabled()
}

// PilotSymbols returns the always-include symbols, in admin-defined order.
func (s *StockSummaryService) PilotSymbols(ctx context.Context) []string {
	if s.settings == nil {
		return nil
	}
	return s.settings.PilotSymbols(ctx)
}

// PilotStatus reads the cached summary for each pilot symbol without
// triggering a build — for admin monitoring only, never forces generation.
func (s *StockSummaryService) PilotStatus(ctx context.Context) []model.StockSummaryPilotStatus {
	symbols := s.PilotSymbols(ctx)
	out := make([]model.StockSummaryPilotStatus, 0, len(symbols))
	for _, symbol := range symbols {
		status := model.StockSummaryPilotStatus{Symbol: symbol}
		if s.redis != nil {
			if cached, err := s.redis.Get(ctx, stockSummaryCacheKey(symbol)).Bytes(); err == nil && len(cached) > 0 {
				var view model.StockSummaryView
				if json.Unmarshal(cached, &view) == nil {
					status.Cached = true
					status.Source = view.Source
					status.GeneratedAt = view.GeneratedAt
				}
			}
		}
		out = append(out, status)
	}
	return out
}

func (s *StockSummaryService) Get(ctx context.Context, symbol string) (*model.StockSummaryView, error) {
	cacheKey := stockSummaryCacheKey(symbol)
	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil && len(cached) > 0 {
			var out model.StockSummaryView
			if json.Unmarshal(cached, &out) == nil {
				if s.isFresh(&out) {
					return &out, nil
				}
				go s.refreshOne(symbol)
				return &out, nil
			}
		}
	}

	out, _, err := s.build(ctx, symbol, false)
	if err != nil {
		return nil, err
	}
	s.store(ctx, symbol, out)
	return out, nil
}

func (s *StockSummaryService) isFresh(out *model.StockSummaryView) bool {
	if out == nil || out.GeneratedAt == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, out.GeneratedAt)
	if err != nil {
		return false
	}
	return time.Since(t) < stockSummaryStaleAfter
}

// refreshOne rebuilds and re-caches one symbol with its own bounded context,
// without spending an LLM call — the async stale-cache path. Any narrative a
// previous run generated is carried forward by build().
func (s *StockSummaryService) refreshOne(symbol string) {
	if _, _, err := s.refreshSymbol(context.Background(), symbol, false); err != nil {
		log.Printf("stock summary refresh %s: %v", symbol, err)
	}
}

// refreshSymbol rebuilds one symbol under its own bounded context and returns
// the stored view plus what the build cost. useLLM is set only by the
// scheduled batch and by an explicit admin refresh.
func (s *StockSummaryService) refreshSymbol(parent context.Context, symbol string, useLLM bool) (*model.StockSummaryView, buildStats, error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	out, stats, err := s.build(ctx, symbol, useLLM)
	if err != nil {
		return nil, stats, err
	}
	s.store(ctx, symbol, out)
	return out, stats, nil
}

func (s *StockSummaryService) store(ctx context.Context, symbol string, out *model.StockSummaryView) {
	if s.redis == nil || out == nil {
		return
	}
	if payload, err := json.Marshal(out); err == nil {
		_ = s.redis.Set(ctx, stockSummaryCacheKey(symbol), payload, stockSummaryCacheTTL).Err()
	}
}

// buildStats reports what one build spent, so the batch runner can track its
// token consumption and tell a failed LLM call apart from a build that never
// attempted one.
type buildStats struct {
	llmAttempted bool
	llmTokens    int64
	llmErr       error
}

func (s *StockSummaryService) build(ctx context.Context, symbol string, useLLM bool) (*model.StockSummaryView, buildStats, error) {
	var stats buildStats
	stock, err := s.stocks.GetBySymbol(ctx, symbol)
	if err != nil {
		return nil, stats, err
	}
	if stock == nil {
		return nil, stats, fmt.Errorf("stock summary: unknown symbol %s", symbol)
	}
	assetType := ResolveFundamentalsAssetType(stock)

	var fundamentals *model.FundamentalsView
	if s.fundamentals != nil {
		fundamentals, _ = s.fundamentals.GetWithValuation(ctx, stock.Symbol)
	}
	history := s.recentHistory(ctx, stock.Symbol)
	insiderTxns := s.recentInsider(ctx, stock.Symbol)
	technical := s.technicalIndicators(ctx, stock.Symbol)
	articles := s.recentArticles(ctx, stock, assetType)

	out := ruleBasedStockSummary(fundamentals, history, insiderTxns, technical, len(articles))
	out.Symbol = stock.Symbol
	out.AssetType = assetType
	out.Keywords = ruleBasedKeywords(stock, fundamentals, technical)

	// Fetched once and shared by the briefing and the LLM facts: both must
	// reason from the same score the UI displays.
	var scoreView *model.StockScoreView
	if s.scores != nil {
		scoreView, _ = s.scores.Get(ctx, stock.Symbol)
	}
	out.Briefing = buildMarketBriefing(stock, fundamentals, history, insiderTxns, technical, scoreView)

	if useLLM && s.llm != nil && s.llm.Enabled() {
		stats.llmAttempted = true
		facts := buildStockSummaryFacts(stock, fundamentals, history, insiderTxns, technical, articles, scoreView)
		factsJSON, err := json.Marshal(facts)
		if err != nil {
			stats.llmErr = err
		} else {
			llmOut, llmErr := s.llm.GenerateStockSummary(ctx, string(factsJSON), describeScoreForPrompt(facts))
			entry := model.LLMRequestLogEntry{
				Symbol:    stock.Symbol,
				Name:      stock.Name,
				Timestamp: time.Now().Format(time.RFC3339),
				Success:   llmErr == nil,
				FactsJSON: string(factsJSON),
				Manual:    true,
			}
			if llmErr != nil {
				stats.llmErr = llmErr
				entry.Error = llmErr.Error()
				log.Printf("stock summary llm fallback %s: %v", stock.Symbol, llmErr)
			} else {
				stats.llmTokens = llmOut.TotalTokens
				applyStockSummaryLLMOutput(out, llmOut, articles)
				out.Source = "llm"
			}
			s.logLLMRequest(ctx, entry)
		}
	}

	// A rebuild that did not produce fresh narrative keeps the last one the
	// batch paid for. Without this, every stale-cache read would silently
	// downgrade an AI 요약 to rule-based text a few hours after it was
	// generated — the numbers would stay current and the prose would vanish.
	if out.Source != "llm" {
		s.carryOverNarrative(ctx, stock.Symbol, out)
	}

	out.GeneratedAt = time.Now().Format(time.RFC3339)
	s.recordDailyAnalysis(ctx, stock, assetType, out, technical)
	return out, stats, nil
}

// carryOverNarrative copies the previous cached run's LLM prose onto a
// freshly rebuilt view, leaving every recomputed number alone.
func (s *StockSummaryService) carryOverNarrative(ctx context.Context, symbol string, out *model.StockSummaryView) {
	if s.redis == nil || out == nil {
		return
	}
	raw, err := s.redis.Get(ctx, stockSummaryCacheKey(symbol)).Bytes()
	if err != nil || len(raw) == 0 {
		return
	}
	var prev model.StockSummaryView
	if json.Unmarshal(raw, &prev) != nil || prev.Source != "llm" {
		return
	}
	out.Valuation = prev.Valuation
	out.Technical = prev.Technical
	out.Insider = prev.Insider
	out.News = prev.News
	if len(prev.Keywords) > 0 {
		out.Keywords = prev.Keywords
	}
	// The sentiment tally is only ever produced by the LLM, from the same
	// article set the news sentence describes. Carrying the prose without it
	// would blank the sentiment badge and drop the news axis out of the
	// daily close score between two batches.
	if prev.NewsSentiment != nil {
		out.NewsSentiment = prev.NewsSentiment
	}
	out.Source = prev.Source
	// GeneratedAt is overwritten by the caller with "now", which would claim
	// the carried-over prose is fresh. AnalyzedAt keeps the honest timestamp
	// of the run that actually produced it.
	if prev.AnalyzedAt != "" {
		out.AnalyzedAt = prev.AnalyzedAt
	} else {
		out.AnalyzedAt = prev.GeneratedAt
	}
}

// recordDailyAnalysis attaches this summary to the trading day it actually
// analysed. The trade date is the market's last *confirmed* close, computed per
// market: at 00:00 KST the domestic session closed yesterday, while the US
// session is still mid-day, so a naive "today − 1" would file US rows against a
// date whose close does not exist yet.
func (s *StockSummaryService) recordDailyAnalysis(
	ctx context.Context,
	stock *model.Stock,
	assetType model.QuoteAssetType,
	view *model.StockSummaryView,
	technical *TechnicalIndicators,
) {
	if s.dailyCloses == nil || view == nil {
		return
	}

	schedule := calendar.ScheduleKRX
	if assetType == model.AssetUS {
		schedule = calendar.ScheduleUS
	}
	lastClose := calendar.LastSessionCloseAt(schedule, time.Now())
	tradeDate := lastClose.In(calendar.LocationFor(schedule)).Format("2006-01-02")

	analysis := model.DailyCloseAnalysis{
		Symbol:    stock.Symbol,
		TradeDate: tradeDate,
		Score:     stockSummaryScore(view, technical),
		AISummary: dailyCloseSummaryText(view),
		Source:    view.Source,
	}
	if ok, err := s.dailyCloses.AttachAnalysis(ctx, analysis); err != nil {
		log.Printf("daily close analysis %s %s: %v", stock.Symbol, tradeDate, err)
	} else if !ok {
		// No close row yet for that date — the price ingestion has not run.
		// Not an error: the next ingestion plus the next summary will fill it.
		log.Printf("daily close analysis %s: no close row for %s yet", stock.Symbol, tradeDate)
	}
}

// dailyCloseSummaryText is the one-paragraph 요약 shown under a dated row. The
// valuation and technical sentences carry the day's actual reasoning; empty
// when nothing was generated, so the column stays a plain empty string.
func dailyCloseSummaryText(view *model.StockSummaryView) string {
	parts := make([]string, 0, 2)
	for _, section := range []string{view.Valuation, view.Technical} {
		if trimmed := strings.TrimSpace(section); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, " ")
}

func (s *StockSummaryService) logLLMRequest(ctx context.Context, entry model.LLMRequestLogEntry) {
	if s.redis == nil {
		return
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return
	}
	pipe := s.redis.Pipeline()
	pipe.LPush(ctx, llmRequestLogKey, payload)
	pipe.LTrim(ctx, llmRequestLogKey, 0, llmRequestLogMax-1)
	pipe.Expire(ctx, llmRequestLogKey, llmRequestLogTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("stock summary: log llm request: %v", err)
	}
}

// RecentRequests returns the most recent GenerateStockSummary calls
// (newest-first), for admin auditing of what input was actually sent.
func (s *StockSummaryService) RecentRequests(ctx context.Context) []model.LLMRequestLogEntry {
	if s.redis == nil {
		return nil
	}
	raw, err := s.redis.LRange(ctx, llmRequestLogKey, 0, -1).Result()
	if err != nil {
		return nil
	}
	out := make([]model.LLMRequestLogEntry, 0, len(raw))
	for _, item := range raw {
		var entry model.LLMRequestLogEntry
		if json.Unmarshal([]byte(item), &entry) == nil {
			out = append(out, entry)
		}
	}
	return out
}

// RequestLogPage filters the full request log (onlyFailed narrows to
// success == false) and slices out one page (1-indexed), newest-first. total
// is the count after filtering, before pagination, so callers can render a
// page count.
func (s *StockSummaryService) RequestLogPage(ctx context.Context, onlyFailed bool, page, pageSize int) (entries []model.LLMRequestLogEntry, total int) {
	all := s.RecentRequests(ctx)
	if onlyFailed {
		filtered := make([]model.LLMRequestLogEntry, 0, len(all))
		for _, entry := range all {
			if !entry.Success {
				filtered = append(filtered, entry)
			}
		}
		all = filtered
	}
	total = len(all)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 30
	}
	start := (page - 1) * pageSize
	if start >= total {
		return []model.LLMRequestLogEntry{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return all[start:end], total
}

func (s *StockSummaryService) recentHistory(ctx context.Context, symbol string) []model.FinancialHistoryPeriod {
	if s.financialHistory == nil {
		return nil
	}
	view, err := s.financialHistory.Get(ctx, symbol, 1)
	if err != nil || view == nil {
		return nil
	}
	return view.Periods
}

func (s *StockSummaryService) recentInsider(ctx context.Context, symbol string) []model.InsiderTransaction {
	if s.insider == nil {
		return nil
	}
	view, err := s.insider.Get(ctx, symbol)
	if err != nil || view == nil {
		return nil
	}
	return view.Transactions
}

func (s *StockSummaryService) technicalIndicators(ctx context.Context, symbol string) *TechnicalIndicators {
	if s.candles == nil {
		return nil
	}
	candles, _, err := s.candles.ListBefore(ctx, symbol, model.Interval1d, "", 200)
	if err != nil || len(candles) == 0 {
		return nil
	}
	return ComputeTechnicalIndicators(candles)
}

func (s *StockSummaryService) recentArticles(ctx context.Context, stock *model.Stock, assetType model.QuoteAssetType) []model.NewsArticle {
	if s.news == nil {
		return nil
	}
	market := "domestic"
	if assetType == model.AssetUS {
		market = "us"
	}
	articles, _ := s.news.List(ctx, model.NewsListFilter{
		Market: market,
		Sort:   "latest",
		Symbol: stock.Symbol,
		Limit:  stockSummaryNewsLimit,
	})
	return articles
}

// Start records the process-wide background context and launches the
// scheduler. Manual admin refreshes (see stock_summary_manual.go) derive
// their context from the same root, so a shutdown cancels them too.
func (s *StockSummaryService) Start(ctx context.Context) {
	s.mu.Lock()
	s.bgCtx = ctx
	s.mu.Unlock()
	go s.RunScheduler(ctx)
}

func (s *StockSummaryService) backgroundContext() context.Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.bgCtx == nil {
		return context.Background()
	}
	return s.bgCtx
}

// kstZone matches the calendar package's fallback handling — the container
// image may not ship tzdata, in which case KST is a fixed +09:00 offset
// anyway (Korea observes no DST).
var kstZone = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.FixedZone("KST", 9*3600)
	}
	return loc
}()

// llmRefreshSpacing is the floor on the gap between LLM calls in a sequential
// run. It must stay above apibudget's ProviderOpenAI MinInterval (4s) —
// calling back-to-back would otherwise trip our own rate limit and silently
// fall back to the rule-based summary for every symbol after the first. The
// batch runner stretches it further when token pacing demands it (see
// batchSpacing).
const llmRefreshSpacing = 6 * time.Second
