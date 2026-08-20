package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

const tossCandlePageSize = 200

type CandlesPage struct {
	Candles    []model.Candle
	NextBefore *string
}

func (s *QuotesService) GetCandlesPage(
	ctx context.Context,
	symbol string,
	interval model.CandleInterval,
	before string,
) (CandlesPage, bool) {
	s.touchSymbol(symbol)
	base, baseOK := paginationBaseInterval(interval)
	before = strings.TrimSpace(before)
	if before != "" && baseOK {
		before = model.CanonicalPaginationCursor(before, base)
	}

	if before == "" {
		if snap, ok := s.EnsureCandleSnapshot(ctx, symbol, interval); ok {
			out := snap.Completed
			if snap.Forming != nil {
				out = append(out, *snap.Forming)
			}
			page := CandlesPage{Candles: out, NextBefore: snap.NextBefore}
			if baseOK {
				page = normalizeCandlesPageCursor(page, base)
			}
			return page, true
		}
		s.triggerRawCandlesRefresh(symbol)
		return CandlesPage{}, false
	}

	if page, ok := s.loadHistoricalFromDB(ctx, symbol, interval, before); ok {
		if baseOK {
			page = normalizeCandlesPageCursor(page, base)
		}
		return page, true
	}
	// DB miss: one-time Toss backfill for unseen history, then persist (07-18 design).
	if page, ok := s.getCandlesPageUncached(ctx, symbol, interval, before); ok {
		if baseOK {
			page = normalizeCandlesPageCursor(page, base)
		}
		return page, true
	}
	return CandlesPage{}, false
}

func (s *QuotesService) getCandlesPageUncached(
	ctx context.Context,
	symbol string,
	interval model.CandleInterval,
	before string,
) (CandlesPage, bool) {
	return s.getCandlesPageUncachedWithTarget(ctx, symbol, interval, before, 0)
}

func (s *QuotesService) getCandlesPageUncachedWithTarget(
	ctx context.Context,
	symbol string,
	interval model.CandleInterval,
	before string,
	targetOverride int,
) (CandlesPage, bool) {
	baseInterval, ok := mapCandleInterval(interval)
	if !ok {
		return CandlesPage{}, false
	}

	targetRaw := candleFetchCount(interval)
	if targetOverride > 0 {
		targetRaw = targetOverride
	}
	if before != "" {
		if minutes := intervalMinutes(interval); minutes > 1 {
			targetRaw = rawFetchLimitForInterval(interval, candlePageSize(interval))
		} else {
			targetRaw = candlePageSize(interval)
		}
	}

	if s.TossEnabled() {
		raw, nextBefore, err := s.fetchTossRawCandles(ctx, symbol, baseInterval, before, targetRaw)
		if err != nil {
			return CandlesPage{}, false
		}
		if len(raw) == 0 {
			return CandlesPage{NextBefore: nextBefore}, true
		}
		s.persistRawCandles(ctx, symbol, model.CandleInterval(baseInterval), raw)
		out := raw
		if needsCandleAggregation(interval) {
			out = aggregateCandles(raw, interval)
			out = trimCandlesTail(out, candleDisplayBarCount(interval))
		}
		return CandlesPage{Candles: out, NextBefore: nextBefore}, true
	}
	return CandlesPage{}, false
}

func (s *QuotesService) fetchTossRawCandles(
	ctx context.Context,
	symbol string,
	baseInterval toss.CandleInterval,
	before string,
	targetCount int,
) ([]model.Candle, *string, error) {
	if targetCount <= 0 {
		targetCount = tossCandlePageSize
	}

	var collected []toss.Candle
	var nextBefore *string
	cursor := strings.TrimSpace(before)
	maxPages := (targetCount + tossCandlePageSize - 1) / tossCandlePageSize
	if maxPages > 40 {
		maxPages = 40
	}

	for pageNum := 0; pageNum < maxPages && len(collected) < targetCount; pageNum++ {
		count := min(tossCandlePageSize, targetCount-len(collected))
		params := toss.GetCandlesParams{
			Symbol:   symbol,
			Interval: baseInterval,
			Count:    count,
		}
		if cursor != "" {
			params.Before = cursor
		}

		page, err := s.fetchTossCandlesWithRetry(ctx, params)
		if err != nil {
			if len(collected) > 0 {
				return mapCandles(collected, tossBaseModelInterval(baseInterval)), nextBefore, nil
			}
			return nil, nil, err
		}
		if page == nil || len(page.Candles) == 0 {
			break
		}

		collected = append(collected, page.Candles...)
		nextBefore = page.NextBefore
		if page.NextBefore == nil {
			break
		}
		cursor = *page.NextBefore
	}

	if len(collected) == 0 {
		return nil, nextBefore, nil
	}
	return mapCandles(collected, tossBaseModelInterval(baseInterval)), nextBefore, nil
}

func tossBaseModelInterval(base toss.CandleInterval) model.CandleInterval {
	if base == toss.CandleInterval1d {
		return model.Interval1d
	}
	return model.Interval1m
}

func candlePageSize(interval model.CandleInterval) int {
	if minutes := intervalMinutes(interval); minutes >= 1 {
		return candleDisplayBarCount(interval)
	}
	switch interval {
	case model.Interval1w:
		return 400
	case model.Interval1M:
		return 240
	default:
		return tossCandlePageSize
	}
}
