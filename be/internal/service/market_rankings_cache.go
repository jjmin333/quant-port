package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

const (
	rankingsCacheKeyPrefix = "market:rankings:v2:"
)

func rankingsCacheTTLFor(d toss.RankingDuration) time.Duration {
	switch d {
	case toss.RankingRealtime:
		return 2 * time.Minute
	case toss.Ranking1d, toss.Ranking1w:
		return 30 * time.Minute
	default:
		return 2 * time.Hour
	}
}

func rankingsCacheKey(p toss.GetRankingsParams) string {
	return fmt.Sprintf("%s%s:%s:%s:%t",
		rankingsCacheKeyPrefix,
		p.MarketCountry,
		p.Type,
		p.Duration,
		p.ExcludeInvestmentCaution,
	)
}

func rankingsCacheKeyV1(p toss.GetRankingsParams) string {
	return fmt.Sprintf("market:rankings:v1:%s:%s:%s:%t",
		p.MarketCountry, p.Type, p.Duration, p.ExcludeInvestmentCaution)
}

func (s *MarketService) loadRankingsPayload(ctx context.Context, key string) (rankingsStorePayload, bool) {
	if s.redis == nil {
		return rankingsStorePayload{}, false
	}
	raw, err := s.redis.Get(ctx, key).Bytes()
	if err != nil || len(raw) == 0 {
		return rankingsStorePayload{}, false
	}
	return decodeRankingsStore(raw)
}

func (s *MarketService) storeRankingsPayload(ctx context.Context, key string, payload rankingsStorePayload, ttl time.Duration) {
	if s.redis == nil {
		return
	}
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = s.redis.Set(ctx, key, raw, ttl).Err()
}

func (s *MarketService) fetchRankingsFromToss(ctx context.Context, p toss.GetRankingsParams) (*toss.RankingResponse, error) {
	client, err := s.requireToss()
	if err != nil {
		return nil, err
	}
	return client.GetRankings(ctx, p)
}

// Deprecated helpers kept for tests — rankings API no longer reads Toss on demand.
func (s *MarketService) loadRankingsCache(ctx context.Context, key string) (*toss.RankingResponse, bool) {
	wrapped, ok := s.loadRankingsPayload(ctx, key)
	if !ok {
		return nil, false
	}
	return &toss.RankingResponse{
		RankedAt: wrapped.TossRankedAt,
		Rankings: wrapped.Rankings,
	}, true
}

func (s *MarketService) storeRankingsCache(ctx context.Context, key string, resp *toss.RankingResponse) {
	s.storeRankingsPayload(ctx, key, wrapRankingsStore(resp, time.Now().UTC()), 2*time.Minute)
}
