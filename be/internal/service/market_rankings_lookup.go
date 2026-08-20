package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

var rankingBasePriceSlots = []toss.GetRankingsParams{
	{MarketCountry: toss.MarketKR, Type: toss.RankingMarketTradingAmount, Duration: toss.RankingRealtime, Count: rankingsPollFetchCount},
	{MarketCountry: toss.MarketKR, Type: toss.RankingMarketTradingVolume, Duration: toss.RankingRealtime, Count: rankingsPollFetchCount},
	{MarketCountry: toss.MarketUS, Type: toss.RankingMarketTradingAmount, Duration: toss.RankingRealtime, Count: rankingsPollFetchCount},
	{MarketCountry: toss.MarketUS, Type: toss.RankingMarketTradingVolume, Duration: toss.RankingRealtime, Count: rankingsPollFetchCount},
}

func loadRankingsPayloadFromRedis(ctx context.Context, rdb *redis.Client, p toss.GetRankingsParams) (rankingsStorePayload, bool) {
	if rdb == nil {
		return rankingsStorePayload{}, false
	}
	raw, err := rdb.Get(ctx, rankingsCacheKey(p)).Bytes()
	if err != nil || len(raw) == 0 {
		return rankingsStorePayload{}, false
	}
	return decodeRankingsStore(raw)
}

// lookupRankingBasePrice returns Toss ranking basePrice (전일 정규장 기준가) for a symbol.
func lookupRankingBasePrice(ctx context.Context, rdb *redis.Client, repo *repository.MarketRankingsRepo, symbol string) (float64, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return 0, false
	}
	if v, ok := lookupRankingBasePriceRedis(ctx, rdb, sym); ok {
		return v, true
	}
	return lookupRankingBasePriceDB(ctx, repo, sym)
}

func lookupRankingBasePriceRedis(ctx context.Context, rdb *redis.Client, symbol string) (float64, bool) {
	if rdb == nil {
		return 0, false
	}
	for _, slot := range rankingBasePriceSlots {
		payload, ok := loadRankingsPayloadFromRedis(ctx, rdb, slot)
		if !ok {
			continue
		}
		if v, ok := basePriceFromRankingPayload(payload, symbol); ok {
			return v, true
		}
	}
	return 0, false
}

func lookupRankingBasePriceDB(ctx context.Context, repo *repository.MarketRankingsRepo, symbol string) (float64, bool) {
	if repo == nil {
		return 0, false
	}
	for _, slot := range rankingBasePriceSlots {
		row, err := repo.Get(ctx, string(slot.MarketCountry), string(slot.Type), string(slot.Duration))
		if err != nil || row == nil || len(row.Payload) == 0 {
			continue
		}
		payload, ok := decodeRankingsStore(row.Payload)
		if !ok {
			continue
		}
		if v, ok := basePriceFromRankingPayload(payload, symbol); ok {
			return v, true
		}
	}
	return 0, false
}

func basePriceFromRankingPayload(payload rankingsStorePayload, symbol string) (float64, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	for _, item := range payload.Rankings {
		if strings.ToUpper(strings.TrimSpace(item.Symbol)) != sym {
			continue
		}
		bp := parseDecimal(item.Price.BasePrice)
		if bp > 0 {
			return bp, true
		}
	}
	return 0, false
}

func lastPriceFromRankingPayload(payload rankingsStorePayload, symbol string) (float64, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	for _, item := range payload.Rankings {
		if strings.ToUpper(strings.TrimSpace(item.Symbol)) != sym {
			continue
		}
		lp := parseDecimal(item.Price.LastPrice)
		if lp > 0 {
			return lp, true
		}
	}
	return 0, false
}

// lookupRankingLastPrice returns Toss ranking lastPrice for a symbol (realtime slots).
func lookupRankingLastPrice(ctx context.Context, rdb *redis.Client, repo *repository.MarketRankingsRepo, symbol string) (float64, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return 0, false
	}
	if rdb != nil {
		for _, slot := range rankingBasePriceSlots {
			payload, ok := loadRankingsPayloadFromRedis(ctx, rdb, slot)
			if !ok {
				continue
			}
			if v, ok := lastPriceFromRankingPayload(payload, sym); ok {
				return v, true
			}
		}
	}
	if repo != nil {
		for _, slot := range rankingBasePriceSlots {
			row, err := repo.Get(ctx, string(slot.MarketCountry), string(slot.Type), string(slot.Duration))
			if err != nil || row == nil || len(row.Payload) == 0 {
				continue
			}
			payload, ok := decodeRankingsStore(row.Payload)
			if !ok {
				continue
			}
			if v, ok := lastPriceFromRankingPayload(payload, sym); ok {
				return v, true
			}
		}
	}
	return 0, false
}
