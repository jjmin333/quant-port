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
	return snap, true
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

func splitFormingCandle(candles []model.Candle, interval model.CandleInterval) ([]model.Candle, *model.Candle) {
	if len(candles) == 0 {
		return nil, nil
	}
	if interval == model.Interval1d || interval == model.Interval1w || interval == model.Interval1M {
		return candles, nil
	}

	last := candles[len(candles)-1]
	lastT := parseQuoteCandleTime(last.Time)
	now := time.Now().In(kst)

	var bucketStart time.Time
	if minutes := intervalMinutes(interval); minutes > 0 {
		bucketStart = truncateCandleTime(now, minutes)
	} else {
		bucketStart = truncateCandleTime(now, 1)
	}

	if lastT.Before(bucketStart) {
		return candles, nil
	}

	completed := append([]model.Candle(nil), candles[:len(candles)-1]...)
	return completed, &last
}

func (s *QuotesService) refreshRawCandles(ctx context.Context, symbol string) error {
	raw1m, ok1m := s.getCandlesPageUncached(ctx, symbol, model.Interval1m, "")
	raw1d, ok1d := s.getCandlesPageUncached(ctx, symbol, model.Interval1d, "")

	var rev int64
	if ok1m && len(raw1m.Candles) > 0 {
		rev = s.storeRawTail(ctx, symbol, model.Interval1m, raw1m)
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
		m1 = raw1m.Candles
	}
	if ok1d {
		d1 = raw1d.Candles
	}
	s.rebuildAggregatedSnapshots(ctx, symbol, m1, d1, raw1m.NextBefore, raw1d.NextBefore, rev)
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
		snap := s.buildCandleSnapshot(symbol, interval, agg, nextBefore)
		snap.Rev = rev
		s.cacheCandleSnapshot(ctx, snap)
	}
}

func (s *QuotesService) refreshFormingBars(ctx context.Context, symbol string) {
	if !s.TossEnabled() {
		return
	}
	currentPrice := 0.0
	if s.marketStore != nil {
		if v, ok := s.marketStore.GetPrice(ctx, symbol); ok {
			currentPrice = v
		}
	}
	if currentPrice <= 0 {
		price, err := s.toss.GetPrice(ctx, symbol)
		if err != nil || price == nil {
			return
		}
		currentPrice = parseDecimal(price.LastPrice)
	}
	if currentPrice <= 0 {
		return
	}

	for _, interval := range []model.CandleInterval{
		model.Interval1m, model.Interval5m, model.Interval10m, model.Interval30m, model.Interval60m,
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

		pub := forming
		if norm := s.normalizeCandlesKRW(ctx, symbol, []model.Candle{forming}); len(norm) > 0 {
			pub = norm[0]
		}

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
	if len(candles) == 0 {
		return ""
	}
	return candles[len(candles)-1].Time
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
			completed := s.normalizeCandlesKRW(r.Context(), symbol, snap.Completed)
			var forming *model.Candle
			if snap.Forming != nil {
				norm := s.normalizeCandlesKRW(r.Context(), symbol, []model.Candle{*snap.Forming})
				forming = &norm[0]
			}
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
