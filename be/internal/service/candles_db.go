package service

import (
	"context"

	"github.com/enact/quant-port/be/internal/model"
)

// storageBaseInterval returns the only intervals persisted in candle_bars.
// Intraday aggregates (5m–60m) roll up from 1m; week/month from 1d.
func storageBaseInterval(interval model.CandleInterval) (model.CandleInterval, bool) {
	if minutes := intervalMinutes(interval); minutes >= 1 {
		return model.Interval1m, true
	}
	switch interval {
	case model.Interval1d, model.Interval1w, model.Interval1M:
		return model.Interval1d, true
	default:
		return "", false
	}
}

func rawFetchLimitForInterval(interval model.CandleInterval, pageSize int) int {
	if minutes := intervalMinutes(interval); minutes > 1 {
		// Extra 1m bars so the leading aggregate bucket is complete after trim.
		return pageSize*minutes + minutes
	}
	switch interval {
	case model.Interval1w:
		return pageSize*7 + 7
	case model.Interval1M:
		return pageSize*31 + 31
	default:
		return pageSize
	}
}

// oldestBarCursor returns the pagination cursor for the earliest bar in a page.
func oldestBarCursor(candles []model.Candle, base model.CandleInterval) *string {
	if len(candles) == 0 {
		return nil
	}
	oldest := candles[0]
	for _, c := range candles[1:] {
		if parseQuoteCandleTime(c.Time).Before(parseQuoteCandleTime(oldest.Time)) {
			oldest = c
		}
	}
	nb := normalizeCandleBarTime(oldest.Time, base)
	return model.CanonicalPaginationCursorPtr(&nb, base)
}

// mergePaginationCursor keeps DB continuation token, or falls back to oldest bar
// so clients can reach Toss backfill when the DB tail is partial.
func mergePaginationCursor(stored []model.Candle, base model.CandleInterval, dbNext *string) *string {
	if dbNext != nil {
		return model.CanonicalPaginationCursorPtr(dbNext, base)
	}
	return oldestBarCursor(stored, base)
}

func (s *QuotesService) persistRawCandles(ctx context.Context, symbol string, base model.CandleInterval, raw []model.Candle) {
	if s.candles == nil || !s.candles.HasTable(ctx) || len(raw) == 0 {
		return
	}
	completed, _ := splitFormingCandle(raw, base)
	if len(completed) == 0 {
		return
	}
	for i := range completed {
		completed[i].Time = normalizeCandleBarTime(completed[i].Time, base)
	}
	_ = s.candles.UpsertBatch(ctx, symbol, base, completed)
}

func (s *QuotesService) loadHistoricalFromDB(
	ctx context.Context,
	symbol string,
	interval model.CandleInterval,
	before string,
) (CandlesPage, bool) {
	if s.candles == nil || !s.candles.HasTable(ctx) {
		return CandlesPage{}, false
	}

	base, ok := storageBaseInterval(interval)
	if !ok {
		return CandlesPage{}, false
	}

	pageSize := candlePageSize(interval)
	if interval == base {
		stored, nextBefore, err := s.candles.ListBefore(ctx, symbol, base, before, pageSize)
		if err != nil || len(stored) == 0 {
			return CandlesPage{}, false
		}
		cursor := mergePaginationCursor(stored, base, nextBefore)
		page := CandlesPage{Candles: stored, NextBefore: cursor}
		return normalizeCandlesPageCursor(page, base), true
	}

	rawLimit := rawFetchLimitForInterval(interval, pageSize)
	stored, rawNext, err := s.candles.ListBefore(ctx, symbol, base, before, rawLimit)
	if err != nil || len(stored) == 0 {
		return CandlesPage{}, false
	}

	page := paginateAggregatedFromRaw(stored, rawNext, interval, base, pageSize)
	if len(page.Candles) == 0 {
		return CandlesPage{}, false
	}
	if page.NextBefore == nil {
		page.NextBefore = oldestBarCursor(page.Candles, base)
	}
	return normalizeCandlesPageCursor(page, base), true
}

func ensureSnapshotPaginationCursor(snap model.CandleSnapshot) model.CandleSnapshot {
	base, ok := storageBaseInterval(snap.Interval)
	if !ok {
		return snap
	}
	if snap.NextBefore == nil && len(snap.Completed) > 0 {
		snap.NextBefore = oldestBarCursor(snap.Completed, base)
	}
	return normalizeSnapshotCursor(snap)
}

// loadLatestCandlesFromDB returns the most recent candle tail from MySQL when live
// feeds are unavailable (weekend, cache expiry, Toss outage).
func (s *QuotesService) loadLatestCandlesFromDB(
	ctx context.Context,
	symbol string,
	interval model.CandleInterval,
) (model.CandleSnapshot, bool) {
	if s.candles == nil || !s.candles.HasTable(ctx) {
		return model.CandleSnapshot{}, false
	}

	base, ok := storageBaseInterval(interval)
	if !ok {
		return model.CandleSnapshot{}, false
	}

	limit := candleDisplayBarCount(interval)
	if interval == base {
		stored, nextBefore, err := s.candles.ListBefore(ctx, symbol, base, "", limit)
		if err != nil || len(stored) == 0 {
			return model.CandleSnapshot{}, false
		}
		return ensureSnapshotPaginationCursor(model.CandleSnapshot{
			Symbol:     symbol,
			Interval:   interval,
			Completed:  stored,
			NextBefore: mergePaginationCursor(stored, base, nextBefore),
			Source:     "db",
		}), true
	}

	rawLimit := rawFetchLimitForInterval(interval, limit)
	stored, rawNext, err := s.candles.ListBefore(ctx, symbol, base, "", rawLimit)
	if err != nil || len(stored) == 0 {
		return model.CandleSnapshot{}, false
	}

	page := paginateAggregatedFromRaw(stored, rawNext, interval, base, limit)
	if len(page.Candles) == 0 {
		return model.CandleSnapshot{}, false
	}

	return ensureSnapshotPaginationCursor(model.CandleSnapshot{
		Symbol:     symbol,
		Interval:   interval,
		Completed:  page.Candles,
		NextBefore: page.NextBefore,
		Source:     "db",
	}), true
}
