package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
)

func (st *MarketIndexStore) resolveIndex(ctx context.Context, entry indexCatalogEntry, fresh model.MarketIndex) model.MarketIndex {
	if fresh.Value > 0 {
		st.cacheGoodIndexAt(ctx, fresh, time.Now().UTC())
		return fresh
	}
	if cached, ok := st.loadGoodIndex(ctx, fresh.ID); ok {
		return cached
	}
	if idx, ok := st.goldIndexFromSpotCache(ctx, entry, fresh); ok {
		return idx
	}
	if idx, ok := st.goldIndexFromLegacyCache(ctx, fresh); ok {
		return idx
	}
	if idx, at, ok := st.loadFromDBOneWithTime(ctx, fresh.ID); ok {
		st.warmGoodIndexCache(ctx, idx, at)
		return idx
	}
	return fresh
}

func (st *MarketIndexStore) cacheGoodIndexAt(ctx context.Context, idx model.MarketIndex, updatedAt time.Time) {
	if idx.Value <= 0 {
		return
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	st.cacheSetJSON(ctx, indexCacheKey(idx.ID), cachedMarketIndex{
		Index:     idx,
		UpdatedAt: updatedAt,
	}, indexGoodCacheTTL)
}

func (st *MarketIndexStore) warmGoodIndexCache(ctx context.Context, idx model.MarketIndex, updatedAt time.Time) {
	var existing cachedMarketIndex
	if st.cacheGetJSON(ctx, indexCacheKey(idx.ID), &existing) && existing.Index.Value > 0 {
		return
	}
	st.cacheGoodIndexAt(ctx, idx, updatedAt)
}

func (st *MarketIndexStore) loadGoodIndex(ctx context.Context, id string) (model.MarketIndex, bool) {
	var cached cachedMarketIndex
	if !st.cacheGetJSON(ctx, indexCacheKey(id), &cached) {
		return model.MarketIndex{}, false
	}
	if cached.Index.Value <= 0 {
		return model.MarketIndex{}, false
	}
	return cached.Index, true
}

func (st *MarketIndexStore) goldIndexFromSpotCache(ctx context.Context, entry indexCatalogEntry, fresh model.MarketIndex) (model.MarketIndex, bool) {
	if fresh.ID != "gold" || st.external == nil || st.external.GoldPrice == nil {
		return model.MarketIndex{}, false
	}
	currency := "USD"
	if fresh.Currency == model.CurrencyKRW {
		currency = "KRW"
	}
	q, err := st.external.GoldPrice.Spot(ctx, "XAU", currency)
	if err != nil || q.Value <= 0 {
		return model.MarketIndex{}, false
	}
	q = enrichGoldQuote(ctx, st.redis, q)
	at := st.goldSpotUpdatedAt(ctx, "XAU", currency)
	if at.IsZero() {
		at = time.Now().UTC()
	}
	idx := indexFromQuote(entry, q, at)
	st.warmGoodIndexCache(ctx, idx, at)
	return idx, true
}

func (st *MarketIndexStore) goldSpotUpdatedAt(ctx context.Context, symbol, currency string) time.Time {
	if st.redis == nil {
		return time.Time{}
	}
	raw, err := st.redis.Get(ctx, fmt.Sprintf("market:gold:spot:v1:%s:%s", symbol, currency)).Bytes()
	if err != nil || len(raw) == 0 {
		return time.Time{}
	}
	var cached struct {
		UpdatedAt time.Time `json:"updatedAt"`
	}
	if json.Unmarshal(raw, &cached) != nil {
		return time.Time{}
	}
	return cached.UpdatedAt
}

func (st *MarketIndexStore) goldIndexFromLegacyCache(ctx context.Context, fresh model.MarketIndex) (model.MarketIndex, bool) {
	if fresh.ID != "gold" {
		return model.MarketIndex{}, false
	}
	var price float64
	var state goldSparklineState
	if st.cacheGetJSON(ctx, goldSparklineKeyFmt, &state) && len(state.Points) > 0 {
		price = state.Points[len(state.Points)-1]
	}
	if price <= 0 {
		var baseline goldDayBaseline
		if st.cacheGetJSON(ctx, goldBaselineKeyFmt, &baseline) && baseline.Open > 0 {
			price = baseline.Open
		}
	}
	if price <= 0 {
		return model.MarketIndex{}, false
	}
	entry := indexCatalogEntry{
		id:       fresh.ID,
		name:     fresh.Name,
		currency: fresh.Currency,
		source:   indexSourceGoldPrice,
		symbol:   "XAU",
	}
	q := enrichGoldQuote(ctx, st.redis, marketdata.Quote{Value: price})
	idx := indexFromQuote(entry, q, time.Now().UTC())
	if dbIdx, at, ok := st.loadFromDBOneWithTime(ctx, fresh.ID); ok {
		st.warmGoodIndexCache(ctx, dbIdx, at)
		return dbIdx, true
	}
	return idx, true
}

func (st *MarketIndexStore) loadFromDBOneWithTime(ctx context.Context, id string) (model.MarketIndex, time.Time, bool) {
	if st.repo == nil {
		return model.MarketIndex{}, time.Time{}, false
	}
	rec, err := st.repo.Get(ctx, id)
	if err != nil || rec == nil || rec.Value <= 0 {
		return model.MarketIndex{}, time.Time{}, false
	}
	idx := model.MarketIndex{
		ID:         rec.IndexID,
		Name:       rec.Name,
		Value:      rec.Value,
		Change:     rec.Change,
		ChangeRate: rec.ChangeRate,
		Currency:   model.QuoteCurrency(rec.Currency),
		Badge:      rec.Badge,
		Sparkline:  rec.Sparkline,
		UpdatedAt:  rec.UpdatedAt.UTC().Format(time.RFC3339),
		DataKind:   dataKindForIndex(rec.IndexID),
	}
	return idx, rec.UpdatedAt, true
}

func (st *MarketIndexStore) loadFromDBOne(ctx context.Context, id string) *model.MarketIndex {
	idx, _, ok := st.loadFromDBOneWithTime(ctx, id)
	if !ok {
		return nil
	}
	out := idx
	return &out
}
