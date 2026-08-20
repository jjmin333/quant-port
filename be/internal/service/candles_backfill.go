package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

const (
	metricsDailyBarLimit   = 400
	maxDailyBackfillPages  = 3
)

// listDailyBarsFromDB returns the most recent completed 1d bars stored in MySQL.
func (s *QuotesService) listDailyBarsFromDB(ctx context.Context, symbol string, limit int) []model.Candle {
	if s.candles == nil || !s.candles.HasTable(ctx) {
		return nil
	}
	if limit <= 0 {
		limit = metricsDailyBarLimit
	}
	bars, _, err := s.candles.ListBefore(ctx, symbol, model.Interval1d, "", limit)
	if err != nil || len(bars) == 0 {
		return nil
	}
	return bars
}

// ensureDailyBarsHistory reads 1d bars from DB and, when history is short, backfills
// older pages through the same Toss→persist path used by chart pagination.
func (s *QuotesService) ensureDailyBarsHistory(ctx context.Context, symbol string, limit int) []model.Candle {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil
	}
	if limit <= 0 {
		limit = metricsDailyBarLimit
	}

	bars := s.listDailyBarsFromDB(ctx, sym, limit)
	if barsCoverYearHistory(bars) || !s.TossEnabled() {
		return bars
	}

	cursor := ""
	if len(bars) == 0 {
		if page, ok := s.getCandlesPageUncached(ctx, sym, model.Interval1d, ""); ok && len(page.Candles) > 0 {
			bars = s.listDailyBarsFromDB(ctx, sym, limit)
			if barsCoverYearHistory(bars) {
				return bars
			}
		}
	}

	if c := oldestBarCursor(bars, model.Interval1d); c != nil {
		cursor = *c
	}

	for page := 0; page < maxDailyBackfillPages && !barsCoverYearHistory(bars); page++ {
		if cursor == "" {
			break
		}
		prevCursor := cursor
		if _, ok := s.getCandlesPageUncached(ctx, sym, model.Interval1d, cursor); !ok {
			break
		}
		bars = s.listDailyBarsFromDB(ctx, sym, limit)
		next := oldestBarCursor(bars, model.Interval1d)
		if next == nil {
			break
		}
		cursor = *next
		if cursor == prevCursor {
			break
		}
	}

	return bars
}

// dailyBarsForMetrics returns DB bars immediately and schedules async backfill when
// history is too short, so watchlist list responses are not blocked on Toss pagination.
func (s *QuotesService) dailyBarsForMetrics(ctx context.Context, symbol string, limit int) []model.Candle {
	bars := s.listDailyBarsFromDB(ctx, symbol, limit)
	if !barsCoverYearHistory(bars) && s.TossEnabled() {
		s.triggerDailyBarsBackfill(symbol)
	}
	return bars
}

func (s *QuotesService) triggerDailyBarsBackfill(symbol string) {
	s.initQuoteRefresh()
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return
	}
	s.quoteRefresh.runOnce("daily-backfill:"+sym, func(ctx context.Context) {
		_ = s.ensureDailyBarsHistory(ctx, sym, metricsDailyBarLimit)
	})
}

func week52RangeFromBars(bars []model.Candle) (high, low float64, ok bool) {
	for _, c := range bars {
		if c.High > high {
			high = c.High
		}
		if c.Low > 0 && (low == 0 || c.Low < low) {
			low = c.Low
		}
	}
	return high, low, high > 0 && low > 0
}
