package service

import (
	"context"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
)

func (s *PortfolioService) ensureBaseSummary(ctx context.Context, userID int64) *model.PortfolioSummary {
	base, syncedAt, ok := s.loadPersistedSummary(ctx, userID)
	if ok && len(base.Holdings) > 0 {
		if s.shouldSyncPortfolio(syncedAt, base) {
			s.triggerPortfolioSync(userID)
		}
		return base
	}

	if _, err := s.tossReg.ClientForUser(ctx, userID); err != nil {
		if ok && base != nil {
			return base
		}
		if cached := s.loadSummaryFromRedis(ctx, userID); cached != nil && len(cached.Holdings) > 0 {
			return cached
		}
		return s.emptyPortfolioSummary()
	}

	s.triggerPortfolioSync(userID)

	if cached := s.loadSummaryFromRedis(ctx, userID); cached != nil && len(cached.Holdings) > 0 {
		return cached
	}
	if ok && base != nil {
		return base
	}
	return s.emptyPortfolioSummary()
}

func (s *PortfolioService) triggerHoldingStatsRefresh(userID int64, symbols []string) {
	if userID <= 0 || len(symbols) == 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		ctx = authctx.WithUserID(ctx, userID)

		client, err := s.tossReg.ClientForUser(ctx, userID)
		if err != nil {
			return
		}
		accounts, err := client.GetAccounts(ctx)
		if err != nil {
			return
		}

		base, _, ok := s.loadPersistedSummary(ctx, userID)
		if !ok || base == nil {
			return
		}

		held := map[string]struct{}{}
		for _, sym := range symbols {
			held[sym] = struct{}{}
		}
		var subset []model.UnifiedHolding
		for _, h := range base.Holdings {
			if _, ok := held[h.Symbol]; ok {
				subset = append(subset, h)
			}
		}
		if len(subset) == 0 {
			return
		}
		s.enrichHoldingsStats(ctx, client, accounts, subset)
	}()
}
