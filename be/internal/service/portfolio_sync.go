package service

import (
	"context"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
)

const portfolioSyncInterval = 60 * time.Second

func (s *PortfolioService) RunSyncScheduler(ctx context.Context) {
	s.syncAllLinkedUsers(ctx)
	ticker := time.NewTicker(portfolioSyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncAllLinkedUsers(ctx)
		}
	}
}

func (s *PortfolioService) syncAllLinkedUsers(ctx context.Context) {
	if s.brokerCreds == nil {
		return
	}
	userIDs, err := s.brokerCreds.ListActiveUserIDs(ctx, "toss")
	if err != nil {
		log.Printf("portfolio sync: list users: %v", err)
		return
	}
	for _, userID := range userIDs {
		s.triggerPortfolioSync(userID)
	}
}

func (s *PortfolioService) TriggerSyncForUser(userID int64) {
	s.triggerPortfolioSync(userID)
}

func (s *PortfolioService) triggerPortfolioSync(userID int64) {
	if userID <= 0 {
		return
	}
	s.syncMu.Lock()
	if s.syncingUsers[userID] {
		s.syncMu.Unlock()
		return
	}
	s.syncingUsers[userID] = true
	s.syncMu.Unlock()

	go func() {
		defer func() {
			s.syncMu.Lock()
			delete(s.syncingUsers, userID)
			s.syncMu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		ctx = authctx.WithUserID(ctx, userID)
		if err := s.syncPortfolioForUser(ctx, userID); err != nil {
			log.Printf("portfolio sync user %d: %v", userID, err)
		}
	}()
}

func (s *PortfolioService) syncPortfolioForUser(ctx context.Context, userID int64) error {
	summary, err := s.fetchSummary(ctx)
	if err != nil {
		return err
	}
	if err := s.persistSummary(ctx, userID, summary); err != nil {
		return err
	}

	client, err := s.tossReg.ClientForUser(ctx, userID)
	if err == nil {
		cals := s.loadMarketCalendars(ctx, client)
		policy := computePollingPolicy(summary.Holdings, cals, time.Now())
		s.cacheSummaryAfterSync(ctx, userID, summary, policy)
	}

	go s.enrichHoldingsStatsAsync(userID, summary)
	return nil
}

func (s *PortfolioService) enrichHoldingsStatsAsync(userID int64, base *model.PortfolioSummary) {
	if base == nil || len(base.Holdings) == 0 {
		return
	}
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

	holdings := make([]model.UnifiedHolding, len(base.Holdings))
	copy(holdings, base.Holdings)
	s.enrichHoldingsStats(ctx, client, accounts, holdings)

	updated := *base
	updated.Holdings = holdings
	updated.Source.FetchedAt = time.Now().Format(time.RFC3339)
	_ = s.persistSummary(ctx, userID, &updated)
}
