package service

import (
	"context"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// intradayDeltaFetchBufferBars pads the delta-fetch target so a slow tick,
// brief downtime, or clock skew doesn't leave a gap in stored 1m bars. Cheap
// insurance — the fetch still fits in a single Toss page (tossCandlePageSize
// = 200) as long as the gap since the last stored bar stays under ~3 hours.
const intradayDeltaFetchBufferBars = 15

// intraday1mFetchTarget sizes the Toss 1m request to just what could have
// changed since our last successful fetch, instead of always re-requesting
// the full maxIntradayRawBars window. Historical bars never need to be
// re-fetched — persistRawCandles already wrote them to MySQL, and
// refreshRawCandles reads them back via listIntradayBarsFromDB to rebuild
// the full-depth snapshot/aggregation source.
//
// Without this, a 30s poller tick re-requested 7,200 bars per symbol (36
// Toss pages) even though at most 1-2 minutes of bars had actually changed —
// the single largest source of MARKET_DATA_CHART rate-limit exhaustion.
func (s *QuotesService) intraday1mFetchTarget(ctx context.Context, symbol string) int {
	latest, ok := s.latestIntradayBarTime(ctx, symbol)
	if !ok {
		return maxIntradayRawBars
	}
	gapMinutes := int(time.Since(latest).Minutes())
	if gapMinutes < 0 {
		gapMinutes = 0
	}
	target := gapMinutes + intradayDeltaFetchBufferBars
	if target > maxIntradayRawBars {
		target = maxIntradayRawBars
	}
	if target < intradayDeltaFetchBufferBars {
		target = intradayDeltaFetchBufferBars
	}
	return target
}

func (s *QuotesService) latestIntradayBarTime(ctx context.Context, symbol string) (time.Time, bool) {
	if s.candles == nil || !s.candles.HasTable(ctx) {
		return time.Time{}, false
	}
	bars, _, err := s.candles.ListBefore(ctx, symbol, model.Interval1m, "", 1)
	if err != nil || len(bars) == 0 {
		return time.Time{}, false
	}
	t := parseQuoteCandleTime(bars[len(bars)-1].Time)
	if t.IsZero() {
		return time.Time{}, false
	}
	return t, true
}

// listIntradayBarsFromDB returns the most recent completed 1m bars stored in
// MySQL — the counterpart to listDailyBarsFromDB (candles_backfill.go) for
// the intraday base interval.
func (s *QuotesService) listIntradayBarsFromDB(ctx context.Context, symbol string, limit int) []model.Candle {
	if s.candles == nil || !s.candles.HasTable(ctx) {
		return nil
	}
	bars, _, err := s.candles.ListBefore(ctx, symbol, model.Interval1m, "", limit)
	if err != nil || len(bars) == 0 {
		return nil
	}
	return bars
}

// mergeIntraday1mBars unions a freshly-fetched Toss delta with DB-stored
// history so callers get the same full lookback depth as a non-delta fetch.
// On overlapping timestamps the fresh copy wins (it reflects the forming bar
// and any late corrections Toss made to just-closed bars).
func mergeIntraday1mBars(fresh, history []model.Candle) []model.Candle {
	byTime := make(map[string]model.Candle, len(fresh)+len(history))
	for _, c := range history {
		byTime[normalizeCandleBarTime(c.Time, model.Interval1m)] = c
	}
	for _, c := range fresh {
		byTime[normalizeCandleBarTime(c.Time, model.Interval1m)] = c
	}
	out := make([]model.Candle, 0, len(byTime))
	for _, c := range byTime {
		out = append(out, c)
	}
	return sortCandlesAsc(out)
}
