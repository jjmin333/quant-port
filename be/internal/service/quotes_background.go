package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

const quoteRefreshTimeout = 45 * time.Second

type quoteRefreshCoordinator struct {
	mu sync.Mutex
	in map[string]bool
}

func newQuoteRefreshCoordinator() *quoteRefreshCoordinator {
	return &quoteRefreshCoordinator{in: make(map[string]bool)}
}

func (c *quoteRefreshCoordinator) runOnce(key string, fn func(context.Context)) {
	c.mu.Lock()
	if c.in[key] {
		c.mu.Unlock()
		return
	}
	c.in[key] = true
	c.mu.Unlock()

	go func() {
		defer func() {
			c.mu.Lock()
			delete(c.in, key)
			c.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), quoteRefreshTimeout)
		defer cancel()
		fn(ctx)
	}()
}

func (c *quoteRefreshCoordinator) doSync(ctx context.Context, key string, fn func(context.Context)) {
	c.mu.Lock()
	if c.in[key] {
		c.mu.Unlock()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
			c.mu.Lock()
			busy := c.in[key]
			c.mu.Unlock()
			if !busy {
				return
			}
		}
		return
	}
	c.in[key] = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.in, key)
		c.mu.Unlock()
	}()
	fn(ctx)
}

func (s *QuotesService) initQuoteRefresh() {
	if s.quoteRefresh == nil {
		s.quoteRefresh = newQuoteRefreshCoordinator()
	}
}

func (s *QuotesService) EnsureTerminalSnapshot(ctx context.Context, symbol string) (model.QuoteTerminalSnapshot, bool) {
	s.initQuoteRefresh()
	s.touchSymbol(symbol)

	sym := strings.ToUpper(strings.TrimSpace(symbol))
	key := fmt.Sprintf(quotesTerminalKeyFmt, sym)
	var snap model.QuoteTerminalSnapshot
	if s.cacheGetJSON(ctx, key, &snap) {
		return snap, true
	}

	if s.shouldLivePollSymbol(ctx, sym) {
		s.quoteRefresh.doSync(ctx, "terminal:"+sym, func(refreshCtx context.Context) {
			s.refreshTerminalSnapshot(refreshCtx, sym)
		})
		if s.cacheGetJSON(ctx, key, &snap) {
			return snap, true
		}
	}
	return s.loadPartialTerminalSnapshot(ctx, sym)
}

func (s *QuotesService) loadPartialTerminalSnapshot(ctx context.Context, symbol string) (model.QuoteTerminalSnapshot, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	var snap model.QuoteTerminalSnapshot
	var ok bool

	if detail, found := s.ensureQuoteDetailCached(ctx, sym); found {
		snap.Detail = detail
		ok = true
	}
	if book, found := s.ensureOrderBookCached(ctx, sym); found {
		snap.OrderBook = book
		ok = true
	}
	if trades, found := s.ensureTradesCached(ctx, sym); found {
		snap.Trades = trades
		ok = true
	}
	if ok {
		snap.Source = s.dataSource()
	}
	return snap, ok
}

func (s *QuotesService) EnsureQuoteDetail(ctx context.Context, symbol string) (model.QuoteDetail, bool) {
	s.initQuoteRefresh()
	s.touchSymbol(symbol)
	return s.ensureQuoteDetailCached(ctx, symbol)
}

func (s *QuotesService) ensureQuoteDetailCached(ctx context.Context, symbol string) (model.QuoteDetail, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	key := fmt.Sprintf(quotesDetailKeyFmt, sym)
	var cached model.QuoteDetail
	if s.cacheGetJSON(ctx, key, &cached) {
		return cached, true
	}

	if !s.shouldLivePollSymbol(ctx, sym) {
		return s.loadClosedMarketQuoteDetail(ctx, sym)
	}

	s.quoteRefresh.doSync(ctx, "detail:"+sym, func(refreshCtx context.Context) {
		if detail, ok := s.getQuoteUncached(refreshCtx, sym); ok {
			s.cacheSetJSON(refreshCtx, key, detail, time.Duration(model.MarketDataQuoteCacheSec)*time.Second)
			s.persistQuoteDetail(refreshCtx, sym, detail, "toss")
		}
	})

	if s.cacheGetJSON(ctx, key, &cached) {
		return cached, true
	}
	if detail, ok := s.loadQuoteDetailPersisted(ctx, sym); ok {
		return detail, true
	}
	if detail, ok := s.buildQuoteDetailFromDB(ctx, sym); ok {
		s.persistQuoteDetail(ctx, sym, detail, "db")
		return detail, true
	}
	return model.QuoteDetail{}, false
}

func (s *QuotesService) EnsureOrderBook(ctx context.Context, symbol string) (model.OrderBook, bool) {
	s.initQuoteRefresh()
	s.touchSymbol(symbol)
	return s.ensureOrderBookCached(ctx, symbol)
}

func (s *QuotesService) ensureOrderBookCached(ctx context.Context, symbol string) (model.OrderBook, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	key := fmt.Sprintf(quotesBookKeyFmt, sym)
	var cached model.OrderBook
	if s.cacheGetJSON(ctx, key, &cached) {
		return cached, true
	}

	if !s.shouldLivePollSymbol(ctx, sym) {
		return s.loadClosedMarketOrderBook(ctx, sym)
	}

	s.quoteRefresh.doSync(ctx, "book:"+sym, func(refreshCtx context.Context) {
		if book, ok := s.getOrderBookUncached(refreshCtx, sym); ok {
			s.cacheSetJSON(refreshCtx, key, book, time.Duration(model.MarketDataOrderBookCacheSec)*time.Second)
			s.persistOrderBook(refreshCtx, sym, book)
		}
	})

	if s.cacheGetJSON(ctx, key, &cached) {
		return cached, true
	}
	if book, ok := s.loadOrderBookPersisted(ctx, sym); ok {
		return book, true
	}
	return model.OrderBook{}, false
}

func (s *QuotesService) EnsureTrades(ctx context.Context, symbol string) ([]model.TradeTick, bool) {
	s.initQuoteRefresh()
	s.touchSymbol(symbol)
	return s.ensureTradesCached(ctx, symbol)
}

func (s *QuotesService) ensureTradesCached(ctx context.Context, symbol string) ([]model.TradeTick, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	key := fmt.Sprintf(quotesTradesKeyFmt, sym)
	var cached []model.TradeTick
	if s.cacheGetJSON(ctx, key, &cached) {
		return cached, true
	}

	if !s.shouldLivePollSymbol(ctx, sym) {
		return s.loadClosedMarketTrades(ctx, sym)
	}

	s.quoteRefresh.doSync(ctx, "trades:"+sym, func(refreshCtx context.Context) {
		if trades, ok := s.getTradesUncached(refreshCtx, sym); ok {
			s.cacheSetJSON(refreshCtx, key, trades, time.Duration(model.MarketDataTradesCacheSec)*time.Second)
			s.persistTrades(refreshCtx, sym, trades)
		}
	})

	if s.cacheGetJSON(ctx, key, &cached) {
		return cached, true
	}
	if trades, ok := s.loadTradesPersisted(ctx, sym); ok {
		return trades, true
	}
	return nil, false
}

func (s *QuotesService) EnsureCandleSnapshot(ctx context.Context, symbol string, interval model.CandleInterval) (model.CandleSnapshot, bool) {
	s.initQuoteRefresh()
	s.touchSymbol(symbol)

	sym := strings.ToUpper(strings.TrimSpace(symbol))

	if !s.shouldLivePollSymbol(ctx, sym) {
		if snap, ok := s.loadLatestCandlesFromDB(ctx, sym, interval); ok && len(snap.Completed) > 0 {
			snap.Completed = s.normalizeCandlesKRW(ctx, sym, snap.Completed)
			s.cacheCandleSnapshot(ctx, snap)
			return snap, true
		}
		if snap, ok := s.loadCandleSnapshot(ctx, sym, interval); ok && len(snap.Completed) > 0 {
			snap.Completed = s.normalizeCandlesKRW(ctx, sym, snap.Completed)
			return snap, true
		}
		s.backfillClosedMarketFromToss(ctx, sym)
		if snap, ok := s.loadLatestCandlesFromDB(ctx, sym, interval); ok && len(snap.Completed) > 0 {
			snap.Completed = s.normalizeCandlesKRW(ctx, sym, snap.Completed)
			s.cacheCandleSnapshot(ctx, snap)
			return snap, true
		}
		if snap, ok := s.loadCandleSnapshot(ctx, sym, interval); ok && len(snap.Completed) > 0 {
			snap.Completed = s.normalizeCandlesKRW(ctx, sym, snap.Completed)
			return snap, true
		}
		return model.CandleSnapshot{}, false
	}

	if snap, ok := s.loadCandleSnapshot(ctx, sym, interval); ok && len(snap.Completed) > 0 {
		if snap.Rev == 0 {
			snap.Rev = s.getRev(ctx, sym)
		}
		snap.Completed = s.normalizeCandlesKRW(ctx, sym, snap.Completed)
		if snap.Forming != nil {
			norm := s.normalizeCandlesKRW(ctx, sym, []model.Candle{*snap.Forming})
			snap.Forming = &norm[0]
		}
		return snap, true
	}

	s.quoteRefresh.doSync(ctx, fmt.Sprintf("candles:%s:%s", sym, interval), func(refreshCtx context.Context) {
		_ = s.refreshRawCandles(refreshCtx, sym)
	})

	if snap, ok := s.loadCandleSnapshot(ctx, sym, interval); ok && len(snap.Completed) > 0 {
		if snap.Rev == 0 {
			snap.Rev = s.getRev(ctx, sym)
		}
		snap.Completed = s.normalizeCandlesKRW(ctx, sym, snap.Completed)
		if snap.Forming != nil {
			norm := s.normalizeCandlesKRW(ctx, sym, []model.Candle{*snap.Forming})
			snap.Forming = &norm[0]
		}
		return snap, true
	}

	if snap, ok := s.loadLatestCandlesFromDB(ctx, sym, interval); ok && len(snap.Completed) > 0 {
		s.warmRawCacheFromDB(ctx, sym)
		snap.Completed = s.normalizeCandlesKRW(ctx, sym, snap.Completed)
		s.cacheCandleSnapshot(ctx, snap)
		return snap, true
	}
	return model.CandleSnapshot{}, false
}

func (s *QuotesService) triggerQuoteStatsRefresh(symbol string) {
	s.initQuoteRefresh()
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	s.quoteRefresh.runOnce("stats:"+sym, func(ctx context.Context) {
		s.refreshQuoteStatsCached(ctx, sym)
	})
}

func (s *QuotesService) refreshQuoteStatsCached(ctx context.Context, symbol string) {
	if !s.TossEnabled() {
		return
	}
	stock, err := s.toss.GetStock(ctx, symbol)
	if err != nil || stock == nil {
		return
	}
	currentPrice := 0.0
	if s.marketStore != nil {
		if v, ok := s.marketStore.EnsurePrice(ctx, symbol); ok {
			currentPrice = v
		}
	}
	if currentPrice <= 0 {
		return
	}
	stats := s.fetchQuoteStats(ctx, symbol, stock, currentPrice)
	if stats.PrevClose <= 0 {
		return
	}
	key := quoteStatsCachePrefix + strings.ToUpper(symbol)
	s.cacheSetJSON(ctx, key, stats, 60*time.Second)
}

func (s *QuotesService) triggerCatalogBackfill(symbol string) {
	s.initQuoteRefresh()
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	s.quoteRefresh.runOnce("catalog:"+sym, func(ctx context.Context) {
		if !s.TossEnabled() || s.stocks == nil {
			return
		}
		fromToss, err := s.toss.GetStocks(ctx, []string{sym})
		if err != nil || len(fromToss) == 0 {
			return
		}
		_ = s.stocks.Upsert(ctx, stockFromTossInfo(fromToss[0]))
	})
}

func (s *QuotesService) triggerTerminalRefreshAsync(symbol string) {
	s.initQuoteRefresh()
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	s.touchSymbol(sym)
	s.quoteRefresh.runOnce("terminal:"+sym, func(ctx context.Context) {
		s.refreshTerminalSnapshot(ctx, sym)
	})
}

func (s *QuotesService) triggerRawCandlesRefresh(symbol string) {
	s.initQuoteRefresh()
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	s.touchSymbol(sym)
	s.quoteRefresh.runOnce("rawcandles:"+sym, func(ctx context.Context) {
		_ = s.refreshRawCandles(ctx, sym)
	})
}
