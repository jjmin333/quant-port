package service

import (
	"context"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/metrics"
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
	s.touchTerminal(symbol)
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

		/*
			캐시·DB에 아무것도 없다 — 여기가 「차트가 안 뜨는」 유일한 경로다.

			예전에는 배경 보강만 걸어 두고 **빈 응답**을 돌려줬다. 그러면 화면은
			사용자가 새로 고치거나 다음 폴링이 올 때까지 비어 있다.

			상류 예산이 초 단위로 리셋된다는 점을 쓴다 — `market_data_store.go`의
			실측 주석에 「observed resets are as short as 1s」가 있다. 즉 지금
			막혔더라도 **잠깐 기다리면 풀린다.** 그래서 배경 보강을 걸고 짧게
			기다렸다가 한 번 더 읽는다. 이 대기는 **화면 최초 진입 시점**이라
			FE가 어차피 로딩 중이고, 성공하면 빈 차트를 아예 안 보여준다.

			끝내 못 채우면 그때는 정직하게 실패를 알린다(호출자가 재시도 UI를
			띄운다) — 인터벌을 말없이 바꿔 **다른 시간축의 데이터**를 그리는 것은
			「늦다」가 아니라 「엉뚱한 것을 보여준다」이기 때문이다.
		*/
		s.triggerRawCandlesRefresh(symbol)
		if snap, ok := s.waitForCandleSnapshot(ctx, symbol, interval); ok {
			out := snap.Completed
			if snap.Forming != nil {
				out = append(out, *snap.Forming)
			}
			page := CandlesPage{Candles: out, NextBefore: snap.NextBefore}
			if baseOK {
				page = normalizeCandlesPageCursor(page, base)
			}
			metrics.ChartFallback("retry")
			return page, true
		}
		metrics.ChartFallback("nodata")
		return CandlesPage{}, false
	}

	if page, ok := s.loadHistoricalFromDB(ctx, symbol, interval, before); ok {
		if baseOK {
			page = normalizeCandlesPageCursor(page, base)
		}
		return dropCandlesAtOrAfter(page, before), true
	}
	// DB miss: one-time Toss backfill for unseen history, then persist (07-18 design).
	if page, ok := s.getCandlesPageUncached(ctx, symbol, interval, before); ok {
		if baseOK {
			page = normalizeCandlesPageCursor(page, base)
		}
		return dropCandlesAtOrAfter(page, before), true
	}
	return CandlesPage{}, false
}

/*
dropCandlesAtOrAfter — 「before X」로 요청한 페이지에서 X 이후 봉을 버린다.

집계 간격에서는 한 버킷이 여러 원본 봉에 걸쳐 있어, 커서가 버킷 **한가운데를**
가리키면 같은 버킷이 두 페이지에 나뉘어 들어간다. 실측(005930 60분봉): DB에 남은
2023-03-27 조각이 19:31부터라 앞 페이지가 19:00 버킷을 19:31~19:59만으로 만들고,
다음 페이지에서 Toss가 그 버킷의 앞부분(19:00~19:30)을 채워 **같은 시각의 봉이
두 번** 나왔다.

원본 1분봉 조회는 SQL이 `bar_time < cursor`로 이미 거르므로 여기서는 아무 것도
바뀌지 않는다. 걸리는 것은 집계 경로뿐이다.
*/
func dropCandlesAtOrAfter(page CandlesPage, before string) CandlesPage {
	before = strings.TrimSpace(before)
	if before == "" || len(page.Candles) == 0 {
		return page
	}
	cutoff := parseQuoteCandleTime(before)
	if cutoff.IsZero() {
		return page
	}
	// 제자리로 자르지 않는다 — 호출자가 넘긴 배열을 공유한 다른 참조(캐시된
	// 스냅샷 등)까지 함께 잘려 나간다.
	out := make([]model.Candle, 0, len(page.Candles))
	for _, c := range page.Candles {
		t := parseQuoteCandleTime(c.Time)
		if t.IsZero() || t.Before(cutoff) {
			out = append(out, c)
		}
	}
	page.Candles = out
	return page
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

/*
waitForCandleSnapshot — 배경 보강이 캔들을 채울 때까지 짧게 기다린다.

`/candles` 예산은 초 단위로 리셋되므로(market_data_store.go 실측 주석), 지금
막혀 있어도 잠깐 뒤에는 통과한다. 그 짧은 창을 쓰면 빈 차트를 보여주지 않아도 된다.

상한을 2초로 둔 이유: 이 대기는 **화면 최초 진입**에서만 일어나고 그 동안 FE는
이미 로딩 상태다. 더 길게 잡으면 요청 슬롯을 오래 잡아 다른 요청을 밀어내고,
더 짧으면 리셋 창을 못 넘긴다.

폴링 간격을 150ms로 둔 것은 Redis 조회 한 번의 비용이 매우 작기 때문이다 —
채워지는 즉시 나가는 쪽이 고정 대기보다 체감이 좋다.
*/
func (s *QuotesService) waitForCandleSnapshot(
	ctx context.Context, symbol string, interval model.CandleInterval,
) (model.CandleSnapshot, bool) {
	const (
		budget = 2 * time.Second
		step   = 150 * time.Millisecond
	)
	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return model.CandleSnapshot{}, false
		case <-time.After(step):
		}
		if snap, ok := s.loadCandleSnapshot(ctx, symbol, interval); ok && len(snap.Completed) > 0 {
			return s.prepareCandleSnapshotResponse(ctx, symbol, snap), true
		}
	}
	return model.CandleSnapshot{}, false
}
