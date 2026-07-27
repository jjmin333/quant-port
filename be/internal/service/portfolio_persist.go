package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
)

func portfolioSymbolsCacheKey(userID int64) string {
	return fmt.Sprintf("portfolio:symbols:v1:user:%d", userID)
}

func (s *PortfolioService) emptyPortfolioSummary() *model.PortfolioSummary {
	empty := &model.PortfolioSummary{
		Source: model.PortfolioSource{
			Provider:  "toss",
			FetchedAt: time.Now().Format(time.RFC3339),
		},
		Holdings: []model.UnifiedHolding{},
		Accounts: []model.BrokerAccountSummary{},
	}
	finalizeSummary(empty, false)
	return empty
}

func (s *PortfolioService) loadSummaryFromRedis(ctx context.Context, userID int64) *model.PortfolioSummary {
	if s.redis == nil {
		return nil
	}
	cached, err := s.redis.Get(ctx, portfolioSummaryCacheKey(userID)).Bytes()
	if err != nil || len(cached) == 0 {
		return nil
	}
	var summary model.PortfolioSummary
	if json.Unmarshal(cached, &summary) != nil {
		return nil
	}
	return &summary
}

func (s *PortfolioService) loadPersistedSummary(ctx context.Context, userID int64) (*model.PortfolioSummary, time.Time, bool) {
	if s.portfolioCache != nil {
		rec, err := s.portfolioCache.Get(ctx, userID)
		if err == nil && len(rec.SummaryJSON) > 0 {
			var summary model.PortfolioSummary
			if json.Unmarshal(rec.SummaryJSON, &summary) == nil {
				return &summary, rec.SyncedAt, true
			}
		}
	}
	if cached := s.loadSummaryFromRedis(ctx, userID); cached != nil {
		return cached, time.Time{}, true
	}
	return nil, time.Time{}, false
}

func (s *PortfolioService) GetBaseSummary(ctx context.Context) (*model.PortfolioSummary, time.Time, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}
	base := s.ensureBaseSummary(ctx, userID)
	_, syncedAt, _ := s.loadPersistedSummary(ctx, userID)
	return base, syncedAt, nil
}

func (s *PortfolioService) persistSummary(ctx context.Context, userID int64, summary *model.PortfolioSummary) error {
	if summary == nil {
		return nil
	}

	payload, err := json.Marshal(summary)
	if err != nil {
		return err
	}

	symbols := make([]string, 0, len(summary.Holdings))
	for _, h := range summary.Holdings {
		symbols = append(symbols, h.Symbol)
	}
	symbolsJSON, err := json.Marshal(symbols)
	if err != nil {
		return err
	}

	if s.portfolioCache != nil {
		if err := s.portfolioCache.Upsert(ctx, userID, payload, symbolsJSON); err != nil {
			log.Printf("portfolio cache upsert user %d: %v", userID, err)
		}
	}

	if s.redis != nil {
		_ = s.redis.Set(ctx, portfolioSummaryCacheKey(userID), payload, time.Duration(model.PortfolioIdleFullRefreshIntervalSec)*time.Second).Err()
		_ = s.redis.Set(ctx, portfolioSymbolsCacheKey(userID), symbolsJSON, time.Duration(model.PortfolioIdleFullRefreshIntervalSec)*time.Second).Err()
	}
	return nil
}

func (s *PortfolioService) HeldSymbols(ctx context.Context) []string {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return nil
	}
	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, portfolioSymbolsCacheKey(userID)).Bytes(); err == nil && len(cached) > 0 {
			var symbols []string
			if json.Unmarshal(cached, &symbols) == nil {
				return symbols
			}
		}
	}
	if base, _, ok := s.loadPersistedSummary(ctx, userID); ok {
		out := make([]string, 0, len(base.Holdings))
		seen := map[string]struct{}{}
		for _, h := range base.Holdings {
			sym := strings.ToUpper(strings.TrimSpace(h.Symbol))
			if sym == "" {
				continue
			}
			if _, dup := seen[sym]; dup {
				continue
			}
			seen[sym] = struct{}{}
			out = append(out, sym)
		}
		return out
	}
	return nil
}

func (s *PortfolioService) AllTrackedSymbols(ctx context.Context) []string {
	if s.brokerCreds == nil {
		return nil
	}
	userIDs, err := s.brokerCreds.ListActiveUserIDs(ctx, "toss")
	if err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, userID := range userIDs {
		uctx := authctx.WithUserID(ctx, userID)
		for _, sym := range s.HeldSymbols(uctx) {
			sym = strings.ToUpper(strings.TrimSpace(sym))
			if sym == "" {
				continue
			}
			if _, ok := seen[sym]; ok {
				continue
			}
			seen[sym] = struct{}{}
			out = append(out, sym)
		}
	}
	return out
}

func (s *PortfolioService) InvalidateForUser(ctx context.Context, userID int64) {
	if s.portfolioCache != nil {
		_ = s.portfolioCache.Delete(ctx, userID)
	}
	if s.redis != nil {
		_ = s.redis.Del(ctx, portfolioSummaryCacheKey(userID), portfolioSymbolsCacheKey(userID), portfolioLiveCacheKey(userID)).Err()
	}
}

func (s *PortfolioService) shouldSyncPortfolio(syncedAt time.Time, base *model.PortfolioSummary) bool {
	if base == nil || len(base.Holdings) == 0 {
		return false
	}
	if syncedAt.IsZero() {
		return true
	}
	interval := time.Duration(model.PortfolioFullRefreshIntervalSec) * time.Second
	return time.Since(syncedAt) > interval
}

func (s *PortfolioService) cacheSummaryAfterSync(ctx context.Context, userID int64, summary *model.PortfolioSummary, policy pollingPolicy) {
	if s.redis == nil || summary == nil {
		return
	}
	payload, err := json.Marshal(summary)
	if err != nil {
		return
	}
	ttl := time.Duration(policy.FullRefreshIntervalSec) * time.Second
	if ttl <= 0 {
		ttl = time.Duration(model.PortfolioFullRefreshIntervalSec) * time.Second
	}
	_ = s.redis.Set(ctx, portfolioSummaryCacheKey(userID), payload, ttl).Err()
	s.recordSnapshot(ctx, userID, summary)
}
