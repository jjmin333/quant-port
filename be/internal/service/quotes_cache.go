package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

const (
	quotesTerminalKeyFmt = "quotes:terminal:%s"
	quotesBookKeyFmt     = "quotes:book:%s"
	quotesTradesKeyFmt   = "quotes:trades:%s"
	quotesListKeyFmt       = "quotes:list:v2:%s:%s:%d:%d" // catalog lists only; watchlist/held bypass cache
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

func tradesNewestTime(trades []model.TradeTick) time.Time {
	var best time.Time
	for _, t := range trades {
		ts := parseQuoteCandleTime(t.Time)
		if !ts.IsZero() && ts.After(best) {
			best = ts
		}
	}
	return best
}

// loadQuoteDetailFromTerminalCache reads the 1s hot terminal snapshot (live detail path).
func (s *QuotesService) loadQuoteDetailFromTerminalCache(ctx context.Context, symbol string) (model.QuoteDetail, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	var snap model.QuoteTerminalSnapshot
	if !s.cacheGetJSON(ctx, fmt.Sprintf(quotesTerminalKeyFmt, sym), &snap) {
		return model.QuoteDetail{}, false
	}
	if strings.TrimSpace(snap.Detail.Symbol) == "" {
		return model.QuoteDetail{}, false
	}
	return snap.Detail, true
}

func (s *QuotesService) cacheOrderBook(ctx context.Context, symbol string, book model.OrderBook, source string) {
	if len(book.Asks) == 0 && len(book.Bids) == 0 {
		return
	}
	s.cacheSetEnvelope(ctx, fmt.Sprintf(quotesBookKeyFmt, symbol), book, source, s.terminalCacheTTL())
}

func (s *QuotesService) loadOrderBookFromCache(ctx context.Context, symbol string, requireFresh bool) (model.OrderBook, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	var book model.OrderBook
	fetchedAt, _, ok := s.cacheGetEnvelope(ctx, fmt.Sprintf(quotesBookKeyFmt, sym), &book)
	if !ok {
		return model.OrderBook{}, false
	}
	if requireFresh && s.terminalDataNeedsRefresh(ctx, sym, fetchedAt, nil) {
		return model.OrderBook{}, false
	}
	return book, true
}

func (s *QuotesService) cacheTrades(ctx context.Context, symbol string, trades []model.TradeTick, source string) {
	if len(trades) == 0 {
		return
	}
	s.cacheSetEnvelope(ctx, fmt.Sprintf(quotesTradesKeyFmt, symbol), trades, source, s.terminalCacheTTL())
}

func (s *QuotesService) loadTradesFromCache(ctx context.Context, symbol string, requireFresh bool) ([]model.TradeTick, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	var trades []model.TradeTick
	fetchedAt, _, ok := s.cacheGetEnvelope(ctx, fmt.Sprintf(quotesTradesKeyFmt, sym), &trades)
	if !ok || len(trades) == 0 {
		return nil, false
	}
	if requireFresh && s.terminalDataNeedsRefresh(ctx, sym, fetchedAt, trades) {
		return nil, false
	}
	return trades, true
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

func (s *QuotesService) InvalidateQuotesListCache(ctx context.Context, filter string) {
	if s.redis == nil {
		return
	}
	for _, limit := range []int{100, 200} {
		key := quotesListCacheKey(filter, "", limit, 0)
		_ = s.redis.Del(ctx, key).Err()
	}
}

func (s *QuotesService) redisReady() bool {
	return s.redis != nil
}
