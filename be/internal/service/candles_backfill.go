package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

const (
	metricsDailyBarLimit  = 400
	maxDailyBackfillPages = 3
)

// listDailyBarsFromDB returns the most recent completed 1d bars stored in MySQL.
/*
dailyBarsCacheKeyFmt / dailyBarsCacheTTL — 일봉 꼬리 읽기 캐시.

관심종목 목록은 종목마다 `WatchlistMetricsFor`를 부르고, 그 안에서 전주 기준선이
일봉 90개를, 52주 범위가 일봉 280개를 읽는다. FE는 이 목록을 **5초마다** 다시
받는다(watchlist-client.tsx의 `POLL_MS = 5_000`). 그래서 관심종목 30개를 띄워 둔
사용자 한 명이 초당 수천 행을 읽고, 그 비용이 **사용자 수만큼 곱해진다** —
같은 종목을 보는 사람이 100명이면 같은 일봉 280행을 100번 읽는다.

일봉은 하루 한 번 바뀌는 값이라 이 반복은 전부 낭비다. 캐시는 **종목 단위**라
사용자가 몇 명이든 TTL당 한 번만 읽는다.

TTL 60초는 폴링 주기(5초)의 12배이면서, 장중에 갱신되는 **오늘 일봉**의 지연을
1분으로 묶는다. 전주·전월 기준선은 지난 날짜라 오늘 봉과 무관하고, 52주 범위만
새 고저가 반영이 최대 1분 늦는데 — 1년 범위에서 1분은 화면상 차이가 없고,
애초에 그 경로는 Redis raw 캐시(`statsFromRawCache`)를 먼저 본다.
*/
const (
	dailyBarsCacheKeyFmt = "md:daily:v1:%s:%d"
	dailyBarsCacheTTL    = 60 * time.Second
)

func (s *QuotesService) listDailyBarsFromDB(ctx context.Context, symbol string, limit int) []model.Candle {
	if s.candles == nil || !s.candles.HasTable(ctx) {
		return nil
	}
	if limit <= 0 {
		limit = metricsDailyBarLimit
	}

	key := fmt.Sprintf(dailyBarsCacheKeyFmt, strings.ToUpper(strings.TrimSpace(symbol)), limit)
	var cached []model.Candle
	if s.cacheGetJSON(ctx, key, &cached) && len(cached) > 0 {
		return cached
	}

	bars, _, err := s.candles.ListBefore(ctx, symbol, model.Interval1d, "", limit)
	if err != nil || len(bars) == 0 {
		// 빈 결과는 캐싱하지 않는다 — 적재 전 종목이 TTL 동안 계속 비어 보인다.
		return nil
	}
	s.cacheSetJSON(ctx, key, bars, dailyBarsCacheTTL)
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
