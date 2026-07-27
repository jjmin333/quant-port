package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

const (
	quotesTerminalKeyFmt   = "quotes:terminal:%s"
	quotesDetailKeyFmt     = "quotes:detail:%s"
	quotesDetailLastKeyFmt = "quotes:detail:last:%s"
	quotesBookKeyFmt       = "quotes:book:%s"
	quotesBookLastKeyFmt   = "quotes:book:last:%s"
	quotesTradesKeyFmt     = "quotes:trades:%s"
	quotesTradesLastKeyFmt = "quotes:trades:last:%s"
	quotesListKeyFmt       = "quotes:list:v2:%s:%s:%d:%d"
	mdSnapKeyFmt           = "md:snap:%s:%s"
	mdRevKeyFmt            = "md:rev:%s"
	mdRaw1mKeyFmt          = "md:raw:1m:%s"
	mdRaw1dKeyFmt          = "md:raw:1d:%s"
)

func (s *QuotesService) cacheGetJSON(ctx context.Context, key string, dest any) bool {
	if s.redis == nil {
		return false
	}
	raw, err := s.redis.Get(ctx, key).Bytes()
	if err != nil || len(raw) == 0 {
		return false
	}
	return json.Unmarshal(raw, dest) == nil
}

// cacheGetOrPopulate reads Redis first; on miss it runs fetch, stores the result, and unmarshals into dest.
func (s *QuotesService) cacheGetOrPopulate(
	ctx context.Context,
	key string,
	ttl time.Duration,
	dest any,
	fetch func() (any, bool),
) bool {
	if s.cacheGetJSON(ctx, key, dest) {
		return true
	}
	value, ok := fetch()
	if !ok {
		return false
	}
	s.cacheSetJSON(ctx, key, value, ttl)
	payload, err := json.Marshal(value)
	if err != nil {
		return false
	}
	return json.Unmarshal(payload, dest) == nil
}

func (s *QuotesService) cacheSetJSON(ctx context.Context, key string, value any, ttl time.Duration) {
	if s.redis == nil {
		return
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = s.redis.Set(ctx, key, payload, ttl).Err()
}

func (s *QuotesService) bumpRev(ctx context.Context, symbol string) int64 {
	if s.redis == nil {
		return time.Now().UnixNano()
	}
	n, err := s.redis.Incr(ctx, fmt.Sprintf(mdRevKeyFmt, symbol)).Result()
	if err != nil {
		return time.Now().UnixNano()
	}
	return n
}

func (s *QuotesService) getRev(ctx context.Context, symbol string) int64 {
	if s.redis == nil {
		return 0
	}
	n, err := s.redis.Get(ctx, fmt.Sprintf(mdRevKeyFmt, symbol)).Int64()
	if err != nil {
		return 0
	}
	return n
}

func (s *QuotesService) cacheCandleSnapshot(ctx context.Context, snap model.CandleSnapshot) {
	ttl := time.Duration(model.MarketDataCandleTailCacheSec) * time.Second
	if snap.Source == "db" {
		ttl = time.Duration(model.MarketDataCandleHistCacheSec) * time.Second
	}
	s.cacheSetJSON(ctx, fmt.Sprintf(mdSnapKeyFmt, snap.Symbol, snap.Interval), snap, ttl)
}

func (s *QuotesService) lastGoodCacheTTL() time.Duration {
	return time.Duration(model.MarketDataLastGoodCacheSec) * time.Second
}

func (s *QuotesService) cacheQuoteDetailLast(ctx context.Context, symbol string, detail model.QuoteDetail) {
	s.cacheSetJSON(ctx, fmt.Sprintf(quotesDetailLastKeyFmt, symbol), detail, s.lastGoodCacheTTL())
}

func (s *QuotesService) loadQuoteDetailLast(ctx context.Context, symbol string) (model.QuoteDetail, bool) {
	var cached model.QuoteDetail
	if s.cacheGetJSON(ctx, fmt.Sprintf(quotesDetailLastKeyFmt, symbol), &cached) {
		return cached, true
	}
	return model.QuoteDetail{}, false
}

func (s *QuotesService) cacheTradesLast(ctx context.Context, symbol string, trades []model.TradeTick) {
	if len(trades) == 0 {
		return
	}
	s.cacheSetJSON(ctx, fmt.Sprintf(quotesTradesLastKeyFmt, symbol), trades, s.lastGoodCacheTTL())
}

func (s *QuotesService) loadTradesLast(ctx context.Context, symbol string) ([]model.TradeTick, bool) {
	var cached []model.TradeTick
	if s.cacheGetJSON(ctx, fmt.Sprintf(quotesTradesLastKeyFmt, symbol), &cached) && len(cached) > 0 {
		return cached, true
	}
	return nil, false
}

func (s *QuotesService) cacheOrderBookLast(ctx context.Context, symbol string, book model.OrderBook) {
	if len(book.Asks) == 0 && len(book.Bids) == 0 {
		return
	}
	s.cacheSetJSON(ctx, fmt.Sprintf(quotesBookLastKeyFmt, symbol), book, s.lastGoodCacheTTL())
}

func (s *QuotesService) loadOrderBookLast(ctx context.Context, symbol string) (model.OrderBook, bool) {
	var cached model.OrderBook
	if s.cacheGetJSON(ctx, fmt.Sprintf(quotesBookLastKeyFmt, symbol), &cached) {
		return cached, true
	}
	return model.OrderBook{}, false
}

func (s *QuotesService) loadCandleSnapshot(ctx context.Context, symbol string, interval model.CandleInterval) (model.CandleSnapshot, bool) {
	var snap model.CandleSnapshot
	if !s.cacheGetJSON(ctx, fmt.Sprintf(mdSnapKeyFmt, symbol, interval), &snap) {
		return model.CandleSnapshot{}, false
	}
	return snap, true
}

func quotesListCacheKey(filter, query string, limit, offset int) string {
	return fmt.Sprintf(quotesListKeyFmt, filter, query, limit, offset)
}

func (s *QuotesService) redisReady() bool {
	return s.redis != nil
}
