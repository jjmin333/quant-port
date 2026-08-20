package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

var allCandleIntervals = []model.CandleInterval{
	model.Interval1m,
	model.Interval5m,
	model.Interval10m,
	model.Interval30m,
	model.Interval60m,
	model.Interval1d,
	model.Interval1w,
	model.Interval1M,
}

func (s *QuotesService) touchSymbol(symbol string) {
	if s.poller != nil {
		s.poller.Touch(symbol)
	}
}

func (s *QuotesService) GetTerminalSnapshot(ctx context.Context, symbol string) (model.QuoteTerminalSnapshot, bool) {
	return s.EnsureTerminalSnapshot(ctx, symbol)
}

func (s *QuotesService) refreshTerminalSnapshot(ctx context.Context, symbol string) (model.QuoteTerminalSnapshot, bool) {
	detail, ok := s.getQuoteUncached(ctx, symbol)
	if !ok {
		return model.QuoteTerminalSnapshot{}, false
	}
	book, okBook := s.getOrderBookUncached(ctx, symbol)
	if !okBook {
		book = model.OrderBook{Symbol: symbol}
	}
	trades, okTrades := s.getTradesUncached(ctx, symbol)
	if !okTrades {
		trades = []model.TradeTick{}
	}

	s.rehydrateQuoteDetailWithSources(ctx, &detail, trades, &book)

	snap := model.QuoteTerminalSnapshot{
		Detail:    detail,
		OrderBook: book,
		Trades:    trades,
		Source:    s.dataSource(),
	}
	s.cacheSetJSON(ctx, fmt.Sprintf(quotesTerminalKeyFmt, symbol), snap,
		time.Duration(model.MarketDataQuoteCacheSec)*time.Second)
	s.persistQuoteDetail(ctx, symbol, detail, s.dataSource())
	s.persistOrderBook(ctx, symbol, book)
	s.persistTrades(ctx, symbol, trades)
	s.publishQuoteUpdate(detail)
	return snap, true
}

func (s *QuotesService) publishQuoteUpdate(detail model.QuoteDetail) {
	if s.quoteStreamHub == nil {
		return
	}
	s.quoteStreamHub.Publish(model.QuoteStreamEvent{
		Type:          "quote_update",
		Symbol:        detail.Symbol,
		CurrentPrice:  detail.CurrentPrice,
		Change:        detail.Change,
		ChangeRate:    detail.ChangeRate,
		PrevClose:     detail.PrevClose,
		RegularClose:  detail.RegularClose,
		MarketSession: detail.MarketSession,
		UpdatedAt:     detail.UpdatedAt,
	})
}

func (s *QuotesService) GetCandleSnapshot(ctx context.Context, symbol string, interval model.CandleInterval) (model.CandleSnapshot, bool) {
	return s.EnsureCandleSnapshot(ctx, symbol, interval)
}

func (s *QuotesService) buildCandleSnapshot(
	symbol string,
	interval model.CandleInterval,
	candles []model.Candle,
	nextBefore *string,
) model.CandleSnapshot {
	completed, forming := splitFormingCandle(candles, interval)
	return model.CandleSnapshot{
		Symbol:     symbol,
		Interval:   interval,
		Completed:  completed,
		Forming:    forming,
		NextBefore: nextBefore,
		Rev:        0,
		Source:     s.dataSource(),
	}
}

// splitFormingCandle peels off the bar covering the current bucket so it can be
// streamed as `forming` instead of being cached/persisted as final history.
func splitFormingCandle(candles []model.Candle, interval model.CandleInterval) ([]model.Candle, *model.Candle) {
	if len(candles) == 0 {
		return nil, nil
	}

	idx := newestCandleIndex(candles)
	if idx < 0 || parseQuoteCandleTime(candles[idx].Time).Before(currentBucketStart(interval)) {
		return candles, nil
	}

	forming := candles[idx]
	completed := make([]model.Candle, 0, len(candles)-1)
	completed = append(completed, candles[:idx]...)
	completed = append(completed, candles[idx+1:]...)
	return completed, &forming
}

// newestCandleIndex locates the latest bar. Pages arrive ascending from the 1m
// tail and the DB but descending from the Toss daily tail, so the newest bar
// cannot be assumed to sit at either end. Returns -1 when no time parses.
func newestCandleIndex(candles []model.Candle) int {
	newestIdx := -1
	var newest time.Time
	for i, candle := range candles {
		t := parseQuoteCandleTime(candle.Time)
		if t.IsZero() {
			continue
		}
		if newestIdx < 0 || t.After(newest) {
			newest, newestIdx = t, i
		}
	}
	return newestIdx
}

// currentBucketStart is the KST start of the bar bucket that "now" falls into.
func currentBucketStart(interval model.CandleInterval) time.Time {
	now := time.Now().In(kst)
	switch interval {
	case model.Interval1d:
		return startOfDay(now)
	case model.Interval1w:
		return weekStart(now)
	case model.Interval1M:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, kst)
	}
	if minutes := intervalMinutes(interval); minutes > 0 {
		return truncateCandleTime(now, minutes)
	}
	return truncateCandleTime(now, 1)
}

func (s *QuotesService) refreshRawCandles(ctx context.Context, symbol string) error {
	// intraday1mFetchTarget shrinks this to just the bars that could have
	// changed since our last successful fetch (usually ~1-2 minutes' worth)
	// instead of the full maxIntradayRawBars window every tick.
	target1m := s.intraday1mFetchTarget(ctx, symbol)
	raw1m, ok1m := s.getCandlesPageUncachedWithTarget(ctx, symbol, model.Interval1m, "", target1m)
	raw1d, ok1d := s.getCandlesPageUncached(ctx, symbol, model.Interval1d, "")

	// The Toss fetch above may only cover the recent delta, so merge in
	// DB-stored history — the cached snapshot, its pagination cursor, and
	// 5m-60m aggregation all need the same full lookback depth clients get
	// today; only the Toss call itself should shrink.
	m1Page := raw1m
	if ok1m {
		if history := s.listIntradayBarsFromDB(ctx, symbol, maxIntradayRawBars); len(history) > 0 {
			merged := mergeIntraday1mBars(raw1m.Candles, history)
			m1Page = CandlesPage{
				Candles:    merged,
				NextBefore: mergePaginationCursor(merged, model.Interval1m, raw1m.NextBefore),
			}
		}
	}

	var rev int64
	if ok1m && len(m1Page.Candles) > 0 {
		rev = s.storeRawTail(ctx, symbol, model.Interval1m, m1Page)
	}
	if ok1d && len(raw1d.Candles) > 0 {
		if rev == 0 {
			rev = s.storeRawTail(ctx, symbol, model.Interval1d, raw1d)
		} else {
			s.storeRawTailWithRev(ctx, symbol, model.Interval1d, raw1d, rev)
		}
	}

	if rev == 0 {
		return nil
	}

	var m1, d1 []model.Candle
	if ok1m {
		m1 = m1Page.Candles
	}
	if ok1d {
		d1 = raw1d.Candles
	}
	s.rebuildAggregatedSnapshots(ctx, symbol, m1, d1, m1Page.NextBefore, raw1d.NextBefore, rev)
	return nil
}

func (s *QuotesService) storeRawTail(ctx context.Context, symbol string, base model.CandleInterval, page CandlesPage) int64 {
	return s.storeRawTailWithRev(ctx, symbol, base, page, s.bumpRev(ctx, symbol))
}

func (s *QuotesService) storeRawTailWithRev(ctx context.Context, symbol string, base model.CandleInterval, page CandlesPage, rev int64) int64 {
	key := fmt.Sprintf(mdRaw1mKeyFmt, symbol)
	if base == model.Interval1d {
		key = fmt.Sprintf(mdRaw1dKeyFmt, symbol)
	}
	s.cacheSetJSON(ctx, key, page.Candles, time.Duration(model.MarketDataCandleTailCacheSec)*time.Second)

	if s.candles != nil && s.candles.HasTable(ctx) {
		completed, _ := splitFormingCandle(page.Candles, base)
		if len(completed) > 0 {
			_ = s.candles.UpsertBatch(ctx, symbol, base, completed)
		}
	}

	prevSnap, _ := s.loadCandleSnapshot(ctx, symbol, base)
	prevLast := lastCompletedTime(prevSnap.Completed)

	snap := s.buildCandleSnapshot(symbol, base, page.Candles, page.NextBefore)
	snap.Rev = rev
	s.cacheCandleSnapshot(ctx, snap)

	newLast := lastCompletedTime(snap.Completed)
	if prevLast != "" && newLast != "" && newLast != prevLast {
		if closed := findClosedBar(snap.Completed, newLast); closed != nil {
			s.streamHub.Publish(model.StreamEvent{
				Type:     "bar_close",
				Symbol:   symbol,
				Interval: base,
				Bar:      closed,
				Rev:      rev,
			})
		}
	}
	return rev
}

func (s *QuotesService) rebuildAggregatedSnapshots(
	ctx context.Context,
	symbol string,
	raw1m []model.Candle,
	raw1d []model.Candle,
	nextBefore1m *string,
	nextBefore1d *string,
	rev int64,
) {
	for _, interval := range allCandleIntervals {
		if interval == model.Interval1m || interval == model.Interval1d {
			continue
		}
		if !needsCandleAggregation(interval) {
			continue
		}

		var source []model.Candle
		var nextBefore *string
		if intervalMinutes(interval) > 0 {
			source = raw1m
			nextBefore = nextBefore1m
		} else {
			source = raw1d
			nextBefore = nextBefore1d
		}
		if len(source) == 0 {
			continue
		}

		agg := aggregateCandles(source, interval)
		agg = trimCandlesTail(agg, candleDisplayBarCount(interval))
		snap := s.buildCandleSnapshot(symbol, interval, agg, nextBefore)
		snap.Rev = rev
		s.cacheCandleSnapshot(ctx, snap)
	}
}

// refreshFormingBars patches the in-progress candle's close/high/low.
// knownPrice should be the price refreshTerminalSnapshot already resolved
// for this symbol this tick (model.QuoteDetail.CurrentPrice) — that value
// went through rehydrateQuoteDetailWithSources' stale-price correction
// (falls back to a direct Toss quote or the last ranking price when the
// naive resolution lands exactly on prevClose), which the plain
// resolveBestLivePrice call below does not. Reusing it keeps the header
// price (quote_update) and the daily chart's forming bar (bar_update)
// numerically identical instead of each tick computing "current price"
// twice via two different algorithms. Pass 0 to fall back to the
// independent resolution (e.g. when refreshTerminalSnapshot itself failed).
func (s *QuotesService) refreshFormingBars(ctx context.Context, symbol string, knownPrice float64) {
	if !s.TossEnabled() {
		return
	}
	currentPrice := knownPrice
	if currentPrice <= 0 {
		currentPrice = s.resolveBestLivePrice(ctx, symbol, 0, nil, nil)
	}
	if currentPrice <= 0 {
		return
	}

	for _, interval := range []model.CandleInterval{
		model.Interval1m, model.Interval5m, model.Interval10m, model.Interval30m, model.Interval60m,
		model.Interval1d,
	} {
		snap, ok := s.loadCandleSnapshot(ctx, symbol, interval)
		if !ok {
			continue
		}
		if snap.Forming == nil {
			continue
		}

		price := currentPrice
		forming := *snap.Forming
		forming.Close = price
		if price > forming.High {
			forming.High = price
		}
		if price < forming.Low || forming.Low == 0 {
			forming.Low = price
		}
		snap.Forming = &forming
		s.cacheCandleSnapshot(ctx, snap)

		if interval == model.Interval1m && s.candles != nil && s.candles.HasTable(ctx) {
			_ = s.candles.UpsertBatch(ctx, symbol, model.Interval1m, []model.Candle{forming})
		}

		pub := forming

		s.streamHub.Publish(model.StreamEvent{
			Type:     "bar_update",
			Symbol:   symbol,
			Interval: interval,
			Forming:  &pub,
			Rev:      snap.Rev,
		})
	}
}

func lastCompletedTime(candles []model.Candle) string {
	idx := newestCandleIndex(candles)
	if idx < 0 {
		return ""
	}
	return candles[idx].Time
}

func findClosedBar(candles []model.Candle, timeKey string) *model.Candle {
	for i := len(candles) - 1; i >= 0; i-- {
		if candles[i].Time == timeKey {
			c := candles[i]
			return &c
		}
	}
	return nil
}

func (s *QuotesService) StreamHub() *streamHub {
	return s.streamHub
}

func (s *QuotesService) dataSource() string {
	if s.TossEnabled() {
		return "toss"
	}
	return "mock"
}

func (s *QuotesService) ServeStream(w http.ResponseWriter, r *http.Request, symbol string, interval model.CandleInterval) {
	s.touchSymbol(symbol)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	snap, ok := s.GetCandleSnapshot(r.Context(), symbol, interval)
	if ok {
		ev := model.StreamEvent{
			Type:     "snapshot",
			Symbol:   symbol,
			Interval: interval,
			Forming:  snap.Forming,
			Rev:      snap.Rev,
		}
		writeSSE(w, flusher, ev)
		if len(snap.Completed) > 0 {
			completed := snap.Completed
			forming := snap.Forming
			payload, _ := json.Marshal(map[string]any{
				"type":      "snapshot_bars",
				"symbol":    symbol,
				"interval":  interval,
				"completed": completed,
				"forming":   forming,
				"rev":       snap.Rev,
			})
			fmt.Fprintf(w, "event: snapshot_bars\ndata: %s\n\n", payload)
			flusher.Flush()
		}
	}

	ch, cancel := s.streamHub.Subscribe(symbol, interval)
	defer cancel()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			writeSSE(w, flusher, ev)
		}
	}
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, ev model.StreamEvent) {
	payload, err := json.Marshal(ev)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", payload)
	flusher.Flush()
}

func (s *QuotesService) ServeQuoteStream(w http.ResponseWriter, r *http.Request, symbol string) {
	s.touchSymbol(symbol)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	if detail, ok := s.GetQuote(r.Context(), symbol); ok {
		ev := model.QuoteStreamEvent{
			Type:          "quote_update",
			Symbol:        detail.Symbol,
			CurrentPrice:  detail.CurrentPrice,
			Change:        detail.Change,
			ChangeRate:    detail.ChangeRate,
			PrevClose:     detail.PrevClose,
			RegularClose:  detail.RegularClose,
			MarketSession: detail.MarketSession,
			UpdatedAt:     detail.UpdatedAt,
		}
		writeQuoteSSE(w, flusher, ev)
	}

	ch, cancel := s.quoteStreamHub.Subscribe(symbol)
	defer cancel()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			writeQuoteSSE(w, flusher, ev)
		}
	}
}

func writeQuoteSSE(w http.ResponseWriter, flusher http.Flusher, ev model.QuoteStreamEvent) {
	payload, err := json.Marshal(ev)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", payload)
	flusher.Flush()
}
