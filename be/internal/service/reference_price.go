package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

const (
	referencePriceKeyFmt = "market:refprice:v3:%s"
	referencePriceTTL    = 26 * time.Hour
)

type cachedReferencePrice struct {
	BasePrice float64   `json:"basePrice"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func cacheReferencePrices(ctx context.Context, rdb *redis.Client, items []toss.RankingItem) {
	if rdb == nil || len(items) == 0 {
		return
	}
	for _, item := range items {
		bp := parseDecimal(item.Price.BasePrice)
		if bp > 0 {
			cacheReferencePrice(ctx, rdb, item.Symbol, bp)
		}
	}
}

func cacheReferencePrice(ctx context.Context, rdb *redis.Client, symbol string, basePrice float64) {
	if rdb == nil || basePrice <= 0 {
		return
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return
	}
	payload, err := json.Marshal(cachedReferencePrice{
		BasePrice: basePrice,
		UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		return
	}
	key := fmt.Sprintf(referencePriceKeyFmt, sym)
	_ = rdb.Set(ctx, key, payload, referencePriceTTL).Err()
}

func loadReferencePrice(ctx context.Context, rdb *redis.Client, symbol string) (float64, bool) {
	if rdb == nil {
		return 0, false
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return 0, false
	}
	raw, err := rdb.Get(ctx, fmt.Sprintf(referencePriceKeyFmt, sym)).Bytes()
	if err != nil || len(raw) == 0 {
		return 0, false
	}
	var cached cachedReferencePrice
	if json.Unmarshal(raw, &cached) != nil || cached.BasePrice <= 0 {
		return 0, false
	}
	return cached.BasePrice, true
}

func (s *QuotesService) resolvePrevClose(ctx context.Context, symbol string) (float64, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return 0, false
	}

	// Rankings basePrice is authoritative (전일 정규장 기준가). Always prefer over stale caches.
	if v, ok := s.lookupReferencePrice(ctx, sym); ok {
		cacheReferencePrice(ctx, s.redis, sym, v)
		return v, true
	}

	if s.toss != nil && s.toss.Enabled() {
		assetType := string(s.quoteAssetType(ctx, sym))
		var cals marketCalendars
		if s.marketStore != nil {
			cals, _ = s.marketStore.EnsureCalendars(ctx)
		}
		if v, err := fetchPrevClose(ctx, s.toss, sym, assetType, cals); err == nil && v > 0 {
			cacheReferencePrice(ctx, s.redis, sym, v)
			return v, true
		}
	}
	return 0, false
}

func (s *QuotesService) lookupReferencePrice(ctx context.Context, symbol string) (float64, bool) {
	if v, ok := lookupRankingBasePrice(ctx, s.redis, s.rankingsRepo, symbol); ok {
		return v, true
	}
	if s.redis != nil {
		if v, ok := loadReferencePrice(ctx, s.redis, symbol); ok {
			return v, true
		}
	}
	return 0, false
}
