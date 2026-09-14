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

/*
intradayDeltaMaxBars — 델타 페치가 한 번에 받아 올 수 있는 상한.

`gapMinutes`는 **벽시계**인데 Toss가 돌려주는 것은 **거래시간 봉**이다. 장이 닫혀
있으면 이 둘이 크게 벌어진다 — 실측(2026-08-23 일요일 01:09): 005930의 마지막
1분봉이 금요일 20:00이라 gap 1,751분, 그래서 1,766봉을 요청했다. Toss는 그것을
2.4 거래일치로 채워 돌려주는데, 그 구간은 **이미 5분봉으로 접힌 뒤**다. 결과는
Toss 9페이지 + 1,766 upsert가 30초 틱마다 낭비되고, 다음 롤업이 그것을 다시
지운다(실측: 폴러가 2분 동안 3일 밖 1분봉 246개를 되넣었다).

720은 저장된 하루치 커버리지다(KRX 08:00~20:00 실측 720봉). 30초 틱이 정상으로
도는 동안 한 번에 그보다 더 벌어질 수 없고, 벌어졌다면 그것은 시세 공백이 아니라
장 마감이다. 이 상한이면 델타가 1분봉 계층(3일) 밖으로 나가지 않는다.

저장된 봉이 하나도 없는 종목은 이 상한을 쓰지 않는다 — 최초 적재는
maxIntradayRawBars 그대로다.
*/
const intradayDeltaMaxBars = 720

/*
dailyRefetchInterval — 같은 심볼의 Toss **일봉**을 다시 받기까지 기다리는 시간.

`refreshRawCandles`는 1분봉에는 델타 최적화(`intraday1mFetchTarget`)를 적용하면서
**일봉은 30초마다 200봉을 통째로 다시 받았다.** 일봉은 하루에 한 칸 늘어나는
값인데 하루 2,880번을 받은 셈이고, 그 호출은 인트라데이 차트와 **같은
`MARKET_DATA_CHART` 버킷**을 쓴다 — 폴러가 스스로 사용자의 차트 요청을
밀어내고 있었다.

5분으로 두는 이유: 장중에는 「오늘 봉」의 고가·저가·거래량이 계속 움직이므로
아주 길게 잡으면 일봉 차트의 마지막 봉이 눈에 띄게 낡는다. 5분이면 그 지연이
일봉 화면에서 식별되지 않으면서 호출은 **1/10**이 된다.

**현재가는 이 주기와 무관하다** — 헤더와 형성봉은 `refreshFormingBars`가
가격으로 따로 갱신하므로 여기서 늦춰도 화면의 숫자는 계속 움직인다.
*/
const dailyRefetchInterval = 5 * time.Minute

/*
shouldFetchDailyCandles — 이 심볼의 일봉을 지금 다시 받아야 하는지.

판단과 기록을 한 잠금 안에서 한다. 갈라 두면 두 틱이 겹칠 때 **둘 다 「방금
받았다」고 보고 아무도 안 받는** 경우가 생긴다 — 가격 쪽 `lastPriceChanged`가
같은 이유로 같은 모양을 하고 있다.
*/
func (s *QuotesService) shouldFetchDailyCandles(symbol string) bool {
	s.dailyFetchMu.Lock()
	defer s.dailyFetchMu.Unlock()
	if s.dailyFetchAt == nil {
		s.dailyFetchAt = map[string]time.Time{}
	}
	if at, ok := s.dailyFetchAt[symbol]; ok && time.Since(at) < dailyRefetchInterval {
		return false
	}
	s.dailyFetchAt[symbol] = time.Now()
	return true
}

// forgetDailyFetch drops the record when the fetch failed, so the next tick
// retries instead of waiting out dailyRefetchInterval on nothing.
func (s *QuotesService) forgetDailyFetch(symbol string) {
	s.dailyFetchMu.Lock()
	defer s.dailyFetchMu.Unlock()
	delete(s.dailyFetchAt, symbol)
}

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
	if target > intradayDeltaMaxBars {
		target = intradayDeltaMaxBars
	}
	if target < intradayDeltaFetchBufferBars {
		target = intradayDeltaFetchBufferBars
	}
	return target
}

/*
latestIntradayBarTime — 저장된 인트라데이 봉 중 **가장 최근** 것의 시각.

1분봉만 보면 안 된다. 롤업이 3일 지난 1분봉을 접어 지우므로, 최근 3일 거래가
없는 종목은 1분봉 계층이 **완전히 빈다**. 그때 「저장된 봉이 없다」로 판단하면
`intraday1mFetchTarget`이 maxIntradayRawBars(7,200)를 통째로 받아 오고, 그것을
저장하면 다음 롤업이 다시 접어 지운다 — 30초 틱마다 7,200봉을 오가는 무한 반복이
된다(실측 2026-08-23: 66종목이 10일 전 구간을 계속 되받고 있었다).

접힌 봉도 「우리가 이미 가진 것」이다. 계층을 함께 보면 그 종목의 실제 공백이
나오고, 델타는 정상 크기로 줄어든다.

흔한 경우(1분봉 계층에 최신 봉이 있음)는 조회 한 번으로 끝난다. 나머지 계층까지
훑는 것은 1분봉이 비었을 때뿐이다.
*/
func (s *QuotesService) latestIntradayBarTime(ctx context.Context, symbol string) (time.Time, bool) {
	if s.candles == nil || !s.candles.HasTable(ctx) {
		return time.Time{}, false
	}

	newestIn := func(interval model.CandleInterval) (time.Time, bool) {
		bars, _, err := s.candles.ListBefore(ctx, symbol, interval, "", 1)
		if err != nil || len(bars) == 0 {
			return time.Time{}, false
		}
		t := parseQuoteCandleTime(bars[len(bars)-1].Time)
		return t, !t.IsZero()
	}

	// 1분봉 계층이 살아 있으면 그것이 가장 최근이다 — 조회 한 번.
	if t, ok := newestIn(model.Interval1m); ok &&
		time.Since(t) < time.Duration(intradayTiers[0].ageDays)*24*time.Hour {
		return t, true
	}

	// 비었거나 낡았다면 접힌 계층에서 찾는다.
	var newest time.Time
	for _, tier := range intradayTiers {
		if t, ok := newestIn(tier.interval); ok && t.After(newest) {
			newest = t
		}
	}
	if newest.IsZero() {
		return time.Time{}, false
	}
	return newest, true
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
