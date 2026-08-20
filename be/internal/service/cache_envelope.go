package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/redis/go-redis/v9"
)

const liveMicrostructureMaxAge = 5 * time.Minute

type cacheEnvelope[T any] struct {
	Data      T         `json:"data"`
	FetchedAt time.Time `json:"fetchedAt"`
	Source    string    `json:"source,omitempty"`
}

func cacheEnvelopeNow(source string) (time.Time, string) {
	return time.Now().UTC(), source
}

func (s *QuotesService) cacheSetEnvelope(ctx context.Context, key string, value any, source string, ttl time.Duration) {
	redisCacheSetEnvelope(ctx, s.redis, key, value, source, ttl)
}

func redisCacheSetEnvelope(ctx context.Context, rdb *redis.Client, key string, value any, source string, ttl time.Duration) {
	if rdb == nil {
		return
	}
	fetchedAt, src := cacheEnvelopeNow(source)
	payload, err := json.Marshal(cacheEnvelope[json.RawMessage]{
		Data:      mustRawJSON(value),
		FetchedAt: fetchedAt,
		Source:    src,
	})
	if err != nil {
		return
	}
	if ttl <= 0 {
		ttl = time.Duration(model.MarketDataLastGoodCacheSec) * time.Second
	}
	_ = rdb.Set(ctx, key, payload, ttl).Err()
}

func mustRawJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
}

func decodeCacheEnvelope[T any](raw []byte) (data T, fetchedAt time.Time, enveloped bool, ok bool) {
	var env struct {
		Data      json.RawMessage `json:"data"`
		FetchedAt time.Time       `json:"fetchedAt"`
	}
	if json.Unmarshal(raw, &env) == nil && len(env.Data) > 0 && !env.FetchedAt.IsZero() {
		if json.Unmarshal(env.Data, &data) == nil {
			return data, env.FetchedAt.UTC(), true, true
		}
	}
	var plain T
	if json.Unmarshal(raw, &plain) == nil {
		return plain, time.Time{}, false, true
	}
	return data, time.Time{}, false, false
}

func (s *QuotesService) cacheGetEnvelope(ctx context.Context, key string, dest any) (fetchedAt time.Time, enveloped bool, ok bool) {
	return redisCacheGetEnvelope(ctx, s.redis, key, dest)
}

func redisCacheGetEnvelope(ctx context.Context, rdb *redis.Client, key string, dest any) (fetchedAt time.Time, enveloped bool, ok bool) {
	if rdb == nil {
		return time.Time{}, false, false
	}
	raw, err := rdb.Get(ctx, key).Bytes()
	if err != nil || len(raw) == 0 {
		return time.Time{}, false, false
	}
	switch d := dest.(type) {
	case *[]model.TradeTick:
		v, at, env, ok := decodeCacheEnvelope[[]model.TradeTick](raw)
		if ok {
			*d = v
			return at, env, true
		}
	case *model.OrderBook:
		v, at, env, ok := decodeCacheEnvelope[model.OrderBook](raw)
		if ok {
			*d = v
			return at, env, true
		}
	case *cachedMarketPrice:
		v, at, env, ok := decodeCacheEnvelope[cachedMarketPrice](raw)
		if ok {
			*d = v
			return at, env, true
		}
	}
	return time.Time{}, false, false
}

func (s *QuotesService) terminalCacheTTL() time.Duration {
	return time.Duration(model.MarketDataLastGoodCacheSec) * time.Second
}

func envelopeFetchedAtStale(fetchedAt time.Time, maxAge time.Duration) bool {
	if fetchedAt.IsZero() {
		return true
	}
	return time.Since(fetchedAt) > maxAge
}

func tradesDataStale(trades []model.TradeTick, maxAge time.Duration) bool {
	if len(trades) == 0 {
		return true
	}
	var bestT time.Time
	foundTS := false
	for _, t := range trades {
		if t.Price <= 0 {
			continue
		}
		ts := parseQuoteCandleTime(t.Time)
		if ts.IsZero() {
			continue
		}
		if !foundTS || ts.After(bestT) {
			bestT = ts
			foundTS = true
		}
	}
	if !foundTS {
		return false
	}
	return time.Since(bestT) > maxAge
}

func (s *QuotesService) terminalDataNeedsRefresh(ctx context.Context, symbol string, fetchedAt time.Time, trades []model.TradeTick) bool {
	if !s.shouldLivePollSymbol(ctx, symbol) {
		return false
	}
	session := s.marketSessionFor(ctx, symbol)
	if !quoteSessionHasLiveMicrostructure(session) {
		return false
	}
	if len(trades) > 0 && tradesDataStale(trades, liveMicrostructureMaxAge) {
		return true
	}
	return envelopeFetchedAtStale(fetchedAt, liveMicrostructureMaxAge)
}
