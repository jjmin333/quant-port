package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/redis/go-redis/v9"
)

const insightsCacheKeyPrefix = "insights:dashboard:v2:user:"
const insightsStaleAfter = 1 * time.Hour

func insightsCacheKey(userID int64) string {
	return fmt.Sprintf("%s%d", insightsCacheKeyPrefix, userID)
}

type InsightsService struct {
	portfolio   *PortfolioService
	market      *MarketService
	llm         *llm.Client
	dart        *DartClient
	earnings    *EarningsClient
	marketStore *MarketDataStore
	redis       *redis.Client
	cacheTTL    time.Duration
}

func NewInsightsService(
	portfolio *PortfolioService,
	market *MarketService,
	llmClient *llm.Client,
	dart *DartClient,
	earnings *EarningsClient,
	redisClient *redis.Client,
) *InsightsService {
	return &InsightsService{
		portfolio: portfolio,
		market:    market,
		llm:       llmClient,
		dart:      dart,
		earnings:  earnings,
		redis:     redisClient,
		cacheTTL:  6 * time.Hour,
	}
}

func (s *InsightsService) SetMarketStore(store *MarketDataStore) {
	s.marketStore = store
}

func (s *InsightsService) GetDashboard(ctx context.Context) (*model.DashboardInsights, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	cacheKey := insightsCacheKey(userID)

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil && len(cached) > 0 {
			var out model.DashboardInsights
			if json.Unmarshal(cached, &out) == nil {
				if s.isDashboardFresh(&out) {
					return &out, nil
				}
				go s.rebuildDashboardAsync(userID)
				return &out, nil
			}
		}
	}

	out, err := s.buildDashboard(ctx)
	if err != nil {
		return nil, err
	}

	if s.redis != nil && out != nil {
		if payload, err := json.Marshal(out); err == nil {
			_ = s.redis.Set(ctx, cacheKey, payload, s.cacheTTL).Err()
		}
	}
	return out, nil
}

func (s *InsightsService) isDashboardFresh(out *model.DashboardInsights) bool {
	if out == nil || out.GeneratedAt == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, out.GeneratedAt)
	if err != nil {
		return false
	}
	return time.Since(t) < insightsStaleAfter
}

func (s *InsightsService) rebuildDashboardAsync(userID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ctx = authctx.WithUserID(ctx, userID)

	out, err := s.buildDashboard(ctx)
	if err != nil {
		log.Printf("insights async rebuild user %d: %v", userID, err)
		return
	}
	if s.redis != nil {
		if payload, err := json.Marshal(out); err == nil {
			_ = s.redis.Set(ctx, insightsCacheKey(userID), payload, s.cacheTTL).Err()
		}
	}
}

func (s *InsightsService) buildDashboard(ctx context.Context) (*model.DashboardInsights, error) {
	summary, _, err := s.portfolio.GetBaseSummary(ctx)
	if err != nil {
		return nil, err
	}

	warnings := s.fetchStockWarnings(ctx, summary)
	risks := EvaluateRiskAlerts(summary, warnings)
	volatility := ComputeVolatility(summary)

	earnings := s.cachedEarnings(ctx, summary.Holdings)
	disclosures := s.cachedDisclosures(ctx, summary.Holdings)

	briefing := s.buildBriefing(ctx, summary, risks, volatility, earnings, disclosures)
	now := time.Now().Format(time.RFC3339)
	briefing.GeneratedAt = now

	return &model.DashboardInsights{
		Briefing:    briefing,
		Risks:       risks,
		GeneratedAt: now,
	}, nil
}

func (s *InsightsService) buildBriefing(
	ctx context.Context,
	summary *model.PortfolioSummary,
	risks []model.RiskAlert,
	volatility string,
	earnings, disclosures []model.BriefingEvent,
) model.MorningBriefing {
	fallback := ruleBasedBriefing(summary, risks, volatility)
	fallback.Earnings = earnings
	fallback.Disclosures = disclosures
	if len(fallback.Earnings) == 0 {
		fallback.Earnings = []model.BriefingEvent{}
	}
	if len(fallback.Disclosures) == 0 {
		fallback.Disclosures = []model.BriefingEvent{}
	}

	if s.llm == nil || !s.llm.Enabled() {
		return fallback
	}

	facts := map[string]any{
		"totalAssetsKRW":  summary.TotalAssetsKRW,
		"todayChangeRate": summary.TodayChangeRate,
		"totalReturnRate": summary.TotalReturnRate,
		"holdingsCount":   len(summary.Holdings),
		"topMovers":       topMovers(summary, 5),
		"volatility":      volatility,
		"earnings":        earnings,
		"disclosures":     disclosures,
		"riskAlerts":      risks,
	}
	factsJSON, _ := json.Marshal(facts)

	llmOut, err := s.llm.GenerateBriefing(ctx, string(factsJSON))
	if err != nil {
		log.Printf("insights llm fallback: %v", err)
		return fallback
	}

	out := fallback
	out.Source = "llm"
	if llmOut.Summary != "" {
		out.Summary = llmOut.Summary
	}
	if len(llmOut.Highlights) > 0 {
		out.Highlights = llmOut.Highlights
	}
	if llmOut.Volatility != "" {
		out.Volatility = llmOut.Volatility
	}
	return out
}

func (s *InsightsService) cachedEarnings(ctx context.Context, holdings []model.UnifiedHolding) []model.BriefingEvent {
	if s.marketStore == nil {
		return nil
	}
	return s.marketStore.EnsureEarningsForSymbols(ctx, usSymbols(holdings))
}

func (s *InsightsService) cachedDisclosures(ctx context.Context, holdings []model.UnifiedHolding) []model.BriefingEvent {
	if s.marketStore == nil {
		return nil
	}
	return s.marketStore.EnsureTodayDisclosures(ctx, domesticSymbols(holdings))
}

func (s *InsightsService) fetchStockWarnings(ctx context.Context, summary *model.PortfolioSummary) map[string][]string {
	out := map[string][]string{}
	if s.market == nil {
		return out
	}
	for _, h := range summary.Holdings {
		warnings, err := s.market.StockWarnings(ctx, h.Symbol)
		if err != nil || len(warnings) == 0 {
			continue
		}
		var msgs []string
		for _, w := range warnings {
			msg := strings.TrimSpace(w.WarningType)
			if msg != "" {
				msgs = append(msgs, msg)
			}
		}
		if len(msgs) > 0 {
			out[h.Symbol] = msgs
		}
	}
	return out
}

func (s *InsightsService) TodayDisclosures(ctx context.Context) ([]model.BriefingEvent, error) {
	summary, err := s.portfolio.GetSummary(ctx)
	if err != nil {
		return nil, err
	}
	events := s.cachedDisclosures(ctx, summary.Holdings)
	if len(events) == 0 {
		return []model.BriefingEvent{}, nil
	}
	return events, nil
}

func (s *InsightsService) Diagnostics() *model.InsightsDiagnostics {
	d := &model.InsightsDiagnostics{
		LLMEnabled:      s.llm != nil && s.llm.Enabled(),
		DartEnabled:     s.dart != nil && s.dart.cfg.Enabled(),
		EarningsEnabled: s.earnings != nil && s.earnings.cfg.Enabled(),
		CheckedAt:       time.Now().Format(time.RFC3339),
	}
	if !d.LLMEnabled {
		d.Hints = append(d.Hints, "OPENAI_API_KEY 설정 시 AI 브리핑 요약 활성화")
	}
	if !d.DartEnabled {
		d.Hints = append(d.Hints, "DART_API_KEY 설정 시 국내 공시 조회 활성화")
	}
	if !d.EarningsEnabled {
		d.Hints = append(d.Hints, "FINNHUB_API_KEY 설정 시 실적 캘린더 활성화")
	}
	return d
}

func (s *InsightsService) RunScheduler(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	s.refreshCache(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshCache(ctx)
		}
	}
}

func (s *InsightsService) refreshCache(ctx context.Context) {
	if s.portfolio == nil || s.portfolio.brokerCreds == nil {
		return
	}
	userIDs, err := s.portfolio.brokerCreds.ListActiveUserIDs(ctx, "toss")
	if err != nil {
		log.Printf("insights scheduler: list users: %v", err)
		return
	}
	for _, userID := range userIDs {
		s.rebuildDashboardAsync(userID)
	}
}

func topMovers(summary *model.PortfolioSummary, n int) []map[string]any {
	type mover struct {
		symbol string
		rate   float64
	}
	var list []mover
	for _, h := range summary.Holdings {
		list = append(list, mover{symbol: h.Symbol, rate: h.SessionChangeRate})
	}
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if abs(list[j].rate) > abs(list[i].rate) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	if len(list) > n {
		list = list[:n]
	}
	out := make([]map[string]any, 0, len(list))
	for _, m := range list {
		out = append(out, map[string]any{"symbol": m.symbol, "changeRate": m.rate})
	}
	return out
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
