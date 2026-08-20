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

// quoteRefreshWait bounds how long a request will piggyback on an in-flight
// refresh before falling through to whatever data is already cached.
const quoteRefreshWait = 5 * time.Second

// quoteRefreshCoordinator collapses concurrent refreshes of the same key into a
// single upstream call. Waiters block on the in-flight run's done channel, so
// they wake the moment it finishes.
type quoteRefreshCoordinator struct {
	mu sync.Mutex
	in map[string]chan struct{}
}

func newQuoteRefreshCoordinator() *quoteRefreshCoordinator {
	return &quoteRefreshCoordinator{in: make(map[string]chan struct{})}
}

// begin claims key for the caller. It returns (done, true) when the caller owns
// the run and must close done, or (existing, false) when a run is already in
// flight.
func (c *quoteRefreshCoordinator) begin(key string) (chan struct{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if done, busy := c.in[key]; busy {
		return done, false
	}
	done := make(chan struct{})
	c.in[key] = done
	return done, true
}

func (c *quoteRefreshCoordinator) finish(key string, done chan struct{}) {
	c.mu.Lock()
	// Only clear the entry if it is still ours.
	if cur, ok := c.in[key]; ok && cur == done {
		delete(c.in, key)
	}
	c.mu.Unlock()
	close(done)
}

func (c *quoteRefreshCoordinator) runOnce(key string, fn func(context.Context)) {
	done, owner := c.begin(key)
	if !owner {
		return
	}

	go func() {
		defer c.finish(key, done)
		ctx, cancel := context.WithTimeout(context.Background(), quoteRefreshTimeout)
		defer cancel()
		fn(ctx)
	}()
}

func (c *quoteRefreshCoordinator) doSync(ctx context.Context, key string, fn func(context.Context)) {
	done, owner := c.begin(key)
	if !owner {
		timer := time.NewTimer(quoteRefreshWait)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
		case <-ctx.Done():
		}
		return
	}

	defer c.finish(key, done)
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
		s.finalizeTerminalSnapshot(ctx, &snap)
		return snap, true
	}

	if s.shouldLivePollSymbol(ctx, sym) {
		s.quoteRefresh.doSync(ctx, "terminal:"+sym, func(refreshCtx context.Context) {
			s.refreshTerminalSnapshot(refreshCtx, sym)
		})
		if s.cacheGetJSON(ctx, key, &snap) {
			s.finalizeTerminalSnapshot(ctx, &snap)
			return snap, true
		}
	}
	snap, ok := s.loadPartialTerminalSnapshot(ctx, sym)
	if ok {
		s.finalizeTerminalSnapshot(ctx, &snap)
	}
	return snap, ok
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

	if detail, ok := s.loadQuoteDetailFromTerminalCache(ctx, sym); ok {
		s.finalizeQuoteDetail(ctx, &detail)
		return detail, true
	}

	if !s.shouldLivePollSymbol(ctx, sym) {
		return s.loadClosedMarketQuoteDetail(ctx, sym)
	}

	s.quoteRefresh.doSync(ctx, "terminal:"+sym, func(refreshCtx context.Context) {
		s.refreshTerminalSnapshot(refreshCtx, sym)
	})

	if detail, ok := s.loadQuoteDetailFromTerminalCache(ctx, sym); ok {
		s.finalizeQuoteDetail(ctx, &detail)
		return detail, true
	}
	if detail, ok := s.loadQuoteDetailPersisted(ctx, sym); ok {
		return detail, true
	}
	if detail, ok := s.buildQuoteDetailFromDB(ctx, sym); ok {
		s.finalizeQuoteDetail(ctx, &detail)
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
	requireFresh := s.shouldLivePollSymbol(ctx, sym)
	if book, ok := s.loadOrderBookFromCache(ctx, sym, requireFresh); ok {
		return book, true
	}

	if !requireFresh {
		return s.loadClosedMarketOrderBook(ctx, sym)
	}

	s.quoteRefresh.doSync(ctx, "book:"+sym, func(refreshCtx context.Context) {
		if book, ok := s.getOrderBookUncached(refreshCtx, sym); ok {
			s.persistOrderBook(refreshCtx, sym, book)
		}
	})

	if book, ok := s.loadOrderBookFromCache(ctx, sym, false); ok {
		return book, true
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
	requireFresh := s.shouldLivePollSymbol(ctx, sym)
	if trades, ok := s.loadTradesFromCache(ctx, sym, requireFresh); ok {
		return trades, true
	}

	if !requireFresh {
		return s.loadClosedMarketTrades(ctx, sym)
	}

	s.quoteRefresh.doSync(ctx, "trades:"+sym, func(refreshCtx context.Context) {
		if trades, ok := s.getTradesUncached(refreshCtx, sym); ok {
			s.persistTrades(refreshCtx, sym, trades)
		}
	})

	if trades, ok := s.loadTradesFromCache(ctx, sym, false); ok {
		return trades, true
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
			snap = s.prepareCandleSnapshotResponse(ctx, sym, snap)
			s.cacheCandleSnapshot(ctx, snap)
			if needsIntradayDepthRefresh(interval, snap) {
				s.backfillClosedMarketFromToss(ctx, sym)
				if refreshed, ok := s.loadCandleSnapshot(ctx, sym, interval); ok && len(refreshed.Completed) > 0 {
					return s.prepareCandleSnapshotResponse(ctx, sym, refreshed), true
				}
			}
			return snap, true
		}
		if snap, ok := s.loadCandleSnapshot(ctx, sym, interval); ok && len(snap.Completed) > 0 {
			snap = s.prepareCandleSnapshotResponse(ctx, sym, snap)
			if needsIntradayDepthRefresh(interval, snap) {
				s.backfillClosedMarketFromToss(ctx, sym)
				if refreshed, ok := s.loadCandleSnapshot(ctx, sym, interval); ok && len(refreshed.Completed) > 0 {
					return s.prepareCandleSnapshotResponse(ctx, sym, refreshed), true
				}
			}
			return snap, true
		}
		s.backfillClosedMarketFromToss(ctx, sym)
		if snap, ok := s.loadLatestCandlesFromDB(ctx, sym, interval); ok && len(snap.Completed) > 0 {
			snap = s.prepareCandleSnapshotResponse(ctx, sym, snap)
			s.cacheCandleSnapshot(ctx, snap)
			return snap, true
		}
		if snap, ok := s.loadCandleSnapshot(ctx, sym, interval); ok && len(snap.Completed) > 0 {
			return s.prepareCandleSnapshotResponse(ctx, sym, snap), true
		}
		return model.CandleSnapshot{}, false
	}

	if snap, ok := s.loadCandleSnapshot(ctx, sym, interval); ok && len(snap.Completed) > 0 {
		snap = s.prepareCandleSnapshotResponse(ctx, sym, snap)
		if needsIntradayDepthRefresh(interval, snap) {
			s.triggerRawCandlesRefresh(sym)
		}
		return snap, true
	}

	// Stale-while-revalidate: return DB snapshot immediately, refresh Toss in background.
	if snap, ok := s.loadLatestCandlesFromDB(ctx, sym, interval); ok && len(snap.Completed) > 0 {
		s.warmRawCacheFromDB(ctx, sym)
		snap = s.prepareCandleSnapshotResponse(ctx, sym, snap)
		s.cacheCandleSnapshot(ctx, snap)
		s.triggerRawCandlesRefresh(sym)
		return snap, true
	}

	// Cold start: no Redis or DB data — block once on Toss sync.
	s.quoteRefresh.doSync(ctx, fmt.Sprintf("candles:%s:%s", sym, interval), func(refreshCtx context.Context) {
		_ = s.refreshRawCandles(refreshCtx, sym)
	})

	if snap, ok := s.loadCandleSnapshot(ctx, sym, interval); ok && len(snap.Completed) > 0 {
		return s.prepareCandleSnapshotResponse(ctx, sym, snap), true
	}

	if snap, ok := s.loadLatestCandlesFromDB(ctx, sym, interval); ok && len(snap.Completed) > 0 {
		s.warmRawCacheFromDB(ctx, sym)
		snap = s.prepareCandleSnapshotResponse(ctx, sym, snap)
		s.cacheCandleSnapshot(ctx, snap)
		return snap, true
	}
	return model.CandleSnapshot{}, false
}

func (s *QuotesService) prepareCandleSnapshotResponse(ctx context.Context, sym string, snap model.CandleSnapshot) model.CandleSnapshot {
	if snap.Rev == 0 {
		snap.Rev = s.getRev(ctx, sym)
	}
	snap = s.applyLivePriceToCandleSnapshot(ctx, sym, snap)
	return ensureSnapshotPaginationCursor(snap)
}

func (s *QuotesService) applyLivePriceToCandleSnapshot(ctx context.Context, sym string, snap model.CandleSnapshot) model.CandleSnapshot {
	session := s.marketSessionFor(ctx, sym)
	if !quoteSessionHasLiveMicrostructure(session) {
		return snap
	}
	live := s.ResolveLiveQuotePrice(ctx, sym, 0)
	if live <= 0 {
		return snap
	}
	patch := func(c model.Candle) model.Candle {
		c.Close = live
		if live > c.High {
			c.High = live
		}
		if c.Low <= 0 || live < c.Low {
			c.Low = live
		}
		return c
	}
	if snap.Forming != nil {
		f := patch(*snap.Forming)
		snap.Forming = &f
		return snap
	}
	if len(snap.Completed) == 0 {
		return snap
	}
	// Daily pages arrive newest-first from Toss, so find the latest bar rather
	// than assuming it sits at the end.
	newestIdx := newestCandleIndex(snap.Completed)
	if newestIdx < 0 {
		return snap
	}
	newestT := parseQuoteCandleTime(snap.Completed[newestIdx].Time).In(kst)
	today := startOfDay(time.Now().In(kst))
	if snap.Interval == model.Interval1d && !newestT.Before(today) {
		snap.Completed[newestIdx] = patch(snap.Completed[newestIdx])
	}
	return snap
}

// quoteStatsRefreshFreshWindow skips re-triggering a stats refresh when the
// cached entry still has at least this much TTL left. quotes:stats:v3 is
// cached for 60s, but tickSlow calls this every 30s for every tracked
// symbol regardless of staleness — without this check, roughly half of
// those calls re-fetched (GetStock + prevClose + intraday stats) data that
// hadn't gone stale yet.
const quoteStatsRefreshFreshWindow = 20 * time.Second

func (s *QuotesService) triggerQuoteStatsRefresh(symbol string) {
	s.initQuoteRefresh()
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if s.redis != nil {
		if ttl, err := s.redis.TTL(context.Background(), quoteStatsCachePrefix+sym).Result(); err == nil && ttl > quoteStatsRefreshFreshWindow {
			return
		}
	}
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
