package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

const watchlistMetricsParallelism = 8

func (s *QuotesService) CurrentPriceForSymbol(ctx context.Context, symbol string) (float64, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return 0, false
	}
	if s.marketStore != nil {
		if v, ok := s.marketStore.EnsurePrice(ctx, sym); ok && v > 0 {
			return v, true
		}
	}
	if s.toss != nil && s.toss.Enabled() {
		price, err := s.toss.GetPrice(ctx, sym)
		if err == nil && price != nil {
			v := parseDecimal(price.LastPrice)
			if v > 0 {
				return v, true
			}
		}
	}
	return 0, false
}

func (s *QuotesService) watchlistEntries(ctx context.Context) map[string]repository.WatchlistEntry {
	out := map[string]repository.WatchlistEntry{}
	if s.watchlist == nil {
		return out
	}
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return out
	}
	entries, err := s.watchlist.ListEntries(ctx, userID)
	if err != nil {
		return out
	}
	for _, entry := range entries {
		sym := strings.ToUpper(strings.TrimSpace(entry.Symbol))
		if sym == "" {
			continue
		}
		out[sym] = entry
	}
	return out
}

func (s *QuotesService) enrichWatchlistQuotes(ctx context.Context, items []model.QuoteSummary) {
	if len(items) == 0 {
		return
	}

	entries := s.watchlistEntries(ctx)
	if len(entries) == 0 {
		return
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, watchlistMetricsParallelism)
	for i := range items {
		sym := strings.ToUpper(strings.TrimSpace(items[i].Symbol))
		entry, ok := entries[sym]
		if !ok {
			continue
		}
		wg.Add(1)
		go func(idx int, e repository.WatchlistEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			metrics := s.buildWatchlistMetrics(ctx, &items[idx], e)
			items[idx].WatchlistMetrics = metrics
		}(i, entry)
	}
	wg.Wait()
}

// DailyBars exposes the DB-backed 1d series (newest first) for callers outside
// the package. Read-only: it never triggers a Toss backfill, so a demo request
// cannot spend provider quota.
func (s *QuotesService) DailyBars(ctx context.Context, symbol string, limit int) []model.Candle {
	return s.listDailyBarsFromDB(ctx, strings.ToUpper(strings.TrimSpace(symbol)), limit)
}

// DailyCloseAtOrBefore is the exported form of dailyCloseAtOrBefore.
//
// 「담은 뒤 +12%」류의 숫자는 **담은 날의 실제 종가**가 있어야 말이 된다. 예시 픽스처가
// 담은 날짜만 정하고 그날 가격은 DB에서 읽도록 하려고 연다 — 손으로 적어둔 가격은
// 시세가 움직이는 순간부터 화면과 어긋난다.
func (s *QuotesService) DailyCloseAtOrBefore(ctx context.Context, symbol string, cutoff time.Time) (float64, bool) {
	return s.dailyCloseAtOrBefore(ctx, strings.ToUpper(strings.TrimSpace(symbol)), cutoff)
}

// dailyCloseAtOrBefore looks up one daily close via indexed DB query (symbol + bar_time <= cutoff).
func (s *QuotesService) dailyCloseAtOrBefore(ctx context.Context, symbol string, cutoff time.Time) (float64, bool) {
	if s.candles == nil || !s.candles.HasTable(ctx) || cutoff.IsZero() {
		return 0, false
	}
	closePx, ok, err := s.candles.CloseAtOrBefore(ctx, symbol, model.Interval1d, cutoff)
	if err != nil || !ok {
		return 0, false
	}
	return closePx, true
}

func watchlistMetricsLocation(item *model.QuoteSummary) *time.Location {
	if item != nil && (item.AssetType == model.AssetUS || item.Currency == model.CurrencyUSD) {
		return usEastern
	}
	return kst
}

func previousWeekCutoff(now time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	now = now.In(loc)
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	thisMonday := time.Date(now.Year(), now.Month(), now.Day()-(weekday-1), 0, 0, 0, 0, loc)
	return thisMonday.Add(-time.Nanosecond)
}

func previousMonthCutoffEnd(now time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	now = now.In(loc)
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	return firstOfMonth.Add(-time.Nanosecond)
}

func (s *QuotesService) resolvePreviousMonthClose(
	ctx context.Context,
	sym string,
	item *model.QuoteSummary,
	current float64,
	now time.Time,
	loc *time.Location,
	startOfToday time.Time,
	prevMonth time.Time,
	includeKRXSlop bool,
) (float64, bool) {
	// Reject only when the baseline is today's live daily close duplicated as current.
	rejectLiveDuplicate := func(closePx float64, barDay time.Time, ok bool) (float64, bool) {
		if !ok || closePx <= 0 {
			return 0, false
		}
		if !barDay.Before(startOfToday) && isSamePrice(closePx, current) {
			return 0, false
		}
		return closePx, true
	}

	if bars := s.listDailyBarsFromDB(ctx, sym, 90); len(bars) > 0 {
		prevYear, prevMon := prevMonth.Year(), prevMonth.Month()
		if closePx, barDay, ok := calendarMonthLastCloseBar(
			bars, prevYear, prevMon, loc, includeKRXSlop, startOfToday, current,
		); ok {
			if accepted, ok := rejectLiveDuplicate(closePx, barDay, ok); ok {
				return accepted, true
			}
		}
	}

	if s.candles != nil && s.candles.HasTable(ctx) {
		if closePx, ok, err := s.candles.LastDailyCloseInCalendarMonth(
			ctx, sym, prevMonth.Year(), prevMonth.Month(), includeKRXSlop, startOfToday,
		); err == nil && ok && closePx > 0 {
			if !isSamePrice(closePx, current) {
				return closePx, true
			}
			if bars := s.listDailyBarsFromDB(ctx, sym, 90); len(bars) > 0 {
				if alt, _, ok := calendarMonthLastCloseBar(
					bars, prevMonth.Year(), prevMonth.Month(), loc, includeKRXSlop, startOfToday, current,
				); ok {
					return alt, true
				}
			}
		}
	}

	if closePx, ok := s.dailyCloseAtOrBefore(ctx, sym, previousMonthCutoffEnd(now, loc)); ok {
		if !isSamePrice(closePx, current) {
			return closePx, true
		}
	}
	return 0, false
}

func (s *QuotesService) buildWatchlistMetrics(
	ctx context.Context,
	item *model.QuoteSummary,
	entry repository.WatchlistEntry,
) *model.WatchlistMetrics {
	return s.WatchlistMetricsFor(ctx, item, entry.CreatedAt)
}

// WatchlistMetricsFor builds the 전일·전주·전월·52주 block for one quote.
//
// Split out of buildWatchlistMetrics so a caller that has no session — the
// 「예시로 둘러보기」 fixtures — gets the numbers from the **same** code path the
// signed-in list uses. A second implementation for the demo would drift, and the
// whole point of the demo is that what it teaches transfers to the real screen.
//
// Mutates item.Change / item.ChangeRate: the baseline picked here (정규장 종가 vs
// 전일 종가, by session) is the one the 전일 column must agree with.
func (s *QuotesService) WatchlistMetricsFor(
	ctx context.Context,
	item *model.QuoteSummary,
	addedAt time.Time,
) *model.WatchlistMetrics {
	if item == nil {
		return nil
	}
	current := item.CurrentPrice
	if current <= 0 {
		return nil
	}

	sym := strings.ToUpper(strings.TrimSpace(item.Symbol))
	prevClose, _ := s.resolvePrevClose(ctx, sym)
	change, changeRate := computeQuoteChange(current, prevClose)
	item.Change = change
	item.ChangeRate = changeRate

	metrics := &model.WatchlistMetrics{
		AddedAt:             addedAt.UTC().Format(time.RFC3339),
		ChangeRatePrevClose: changeRate,
		PrevClose:           prevClose,
	}

	loc := watchlistMetricsLocation(item)
	now := time.Now().In(loc)
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	prevMonth := firstOfThisMonth.AddDate(0, -1, 0)
	includeKRXSlop := item.AssetType == model.AssetDomestic

	if prevMonthClose, ok := s.resolvePreviousMonthClose(
		ctx, sym, item, current, now, loc, startOfToday, prevMonth, includeKRXSlop,
	); ok {
		metrics.PrevMonthClose = prevMonthClose
		if rate, ok := changeRateFromBaseline(current, prevMonthClose); ok {
			metrics.ChangeRatePrevMonth = &rate
		}
	}

	dailyBars := s.listDailyBarsFromDB(ctx, sym, 90)
	if len(dailyBars) > 0 {
		if prevWeek, ok := previousWeekClose(dailyBars, now, loc); ok {
			metrics.PrevWeekClose = prevWeek
			if rate, ok := changeRateFromBaseline(current, prevWeek); ok {
				metrics.ChangeRatePrevWeek = &rate
			}
		}
	} else if prevWeek, ok := s.dailyCloseAtOrBefore(ctx, sym, previousWeekCutoff(now, loc)); ok {
		metrics.PrevWeekClose = prevWeek
		if rate, ok := changeRateFromBaseline(current, prevWeek); ok {
			metrics.ChangeRatePrevWeek = &rate
		}
	}

	if high, low, ok := s.week52RangeForWatchlist(ctx, item, current); ok {
		metrics.Week52High = high
		metrics.Week52Low = low
		if rate, ok := changeRateFromBaseline(current, low); ok {
			metrics.ChangeRateWeek52Low = &rate
		}
		if rate, ok := changeRateFromBaseline(current, high); ok {
			metrics.ChangeRateWeek52High = &rate
		}
	}

	return metrics
}

func pseudoStockForWatchlist(item *model.QuoteSummary) *toss.StockInfo {
	stock := &toss.StockInfo{Currency: toss.CurrencyKRW}
	if item != nil && (item.AssetType == model.AssetUS || item.Currency == model.CurrencyUSD) {
		stock.Currency = toss.CurrencyUSD
	}
	return stock
}

// week52RangeForWatchlist uses the same Redis tail / DB daily-bar sources as quote detail
// (week52High/week52Low), not a 1-year-ago close lookup.
func (s *QuotesService) week52RangeForWatchlist(
	ctx context.Context,
	item *model.QuoteSummary,
	current float64,
) (high, low float64, ok bool) {
	if item == nil || current <= 0 {
		return 0, 0, false
	}
	sym := strings.ToUpper(strings.TrimSpace(item.Symbol))
	if sym == "" {
		return 0, 0, false
	}

	weekHigh, weekLow := 0.0, 0.0
	if stats, statsOK := s.statsFromRawCache(ctx, sym, pseudoStockForWatchlist(item), current); statsOK {
		weekHigh, weekLow = stats.Week52High, stats.Week52Low
	}
	if weekHigh <= weekLow || weekLow <= 0 {
		if s.stocks != nil {
			if dbStock, err := s.stocks.GetBySymbol(ctx, sym); err == nil && dbStock != nil {
				if dbStats, _, dbOK := s.statsFromDBCandles(ctx, sym, dbStock); dbOK {
					weekHigh, weekLow = dbStats.Week52High, dbStats.Week52Low
				}
			}
		}
	}
	if weekHigh <= weekLow || weekLow <= 0 {
		if bars := s.listDailyBarsFromDB(ctx, sym, 280); len(bars) > 0 {
			if h, l, barsOK := week52RangeFromBars(bars); barsOK {
				weekHigh, weekLow = h, l
			}
		}
	}
	if weekHigh <= weekLow || weekLow <= 0 {
		return 0, 0, false
	}

	return weekHigh, weekLow, true
}
