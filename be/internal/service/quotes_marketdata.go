package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/metrics"
	"github.com/enact/quant-port/be/internal/model"
)

var allCandleIntervals = []model.CandleInterval{
	model.Interval1m,
	model.Interval5m,
	model.Interval10m,
	model.Interval30m,
	model.Interval1d,
	model.Interval1w,
	model.Interval1M,
}

// touchSymbol — 이 종목의 **가격**이 따뜻해야 한다(목록 화면 수준).
// 호가·체결·차트를 실제로 읽는 경로는 touchTerminal을 써야 한다.
func (s *QuotesService) touchSymbol(symbol string) {
	if s.poller != nil {
		s.poller.Touch(symbol)
	}
}

// touchTerminal — 이 종목을 **상세 화면에서 보고 있다**(호가창·체결·차트).
// 종목당 Toss 2호출이 드는 tickFast의 대상이 되므로, 목록을 만들려고 스치는
// 경로에서는 부르면 안 된다 (marketPoller.Touch 헤더 참조).
func (s *QuotesService) touchTerminal(symbol string) {
	if s.poller != nil {
		s.poller.TouchTerminal(symbol)
	}
}

// TouchOrderBookInterest — 이 종목의 **호가창이 지금 열려 있다**. 핸들러가 부른다.
// 이 표시가 붙은 종목만 웹소켓 호가를 구독한다(marketPoller.book 주석 참조).
func (s *QuotesService) TouchOrderBookInterest(symbol string) {
	if s.poller != nil {
		s.poller.TouchOrderBook(symbol)
	}
}

func (s *QuotesService) GetTerminalSnapshot(ctx context.Context, symbol string) (model.QuoteTerminalSnapshot, bool) {
	return s.EnsureTerminalSnapshot(ctx, symbol)
}

/*
refreshTerminalSnapshot — 차트·호가 화면이 읽는 스냅샷을 다시 조립한다.

# 구독이 있으면 REST를 타지 않는다

이 함수는 화면이 2초마다 부르는 경로의 끝이다(스냅샷 캐시 TTL이 1초라 사실상
요청마다 들어온다). 전에는 구독 여부와 무관하게 **매번 `/orderbook` + `/trades`를
새로 받았다** — 같은 데이터가 이미 웹소켓으로 와 있는데도.

실측(2026-08-25 미국 정규장, 시청자 1명·30초·요청 15건, 구독 중인 종목):

	/api/v1/orderbook  +19    /api/v1/trades  +19    /api/v1/stocks  +19

체결·호가는 구독이 주므로 그쪽을 먼저 본다. **구독이 못 주는 것만** REST로 간다:

	구독이 살아 있고 그 채널의 버퍼가 차 있다      → 구독
	구독은 살아 있으나 그 채널이 구독 대상이 아니다 → 신선한 캐시 → REST
	구독이 없거나 프레임이 끊겼다                  → 예전 경로 그대로(REST)

마지막 줄이 안전장치다. `IsLive`는 「선언했다」가 아니라 「최근 프레임이 왔다」를
보므로, LOSSY 드롭·구독 거부·연결 장애 어느 쪽이든 자동으로 REST로 되돌아간다.

# 구독에서 온 데이터는 여기서 저장하지 않는다

플러시 goroutine이 이미 소유하고 있다(quotes_stream_terminal.go). 여기서 또 쓰면
같은 값을 Redis·MySQL에 두 번 넣는 셈이고, 화면 주기(2초)마다 DB 쓰기가 되살아나
스로틀을 넣은 이유가 사라진다.
*/
func (s *QuotesService) refreshTerminalSnapshot(ctx context.Context, symbol string) (model.QuoteTerminalSnapshot, bool) {
	streamTrades, streamBook, streamed := s.streamTerminalSources(symbol)
	plan := planTerminalSources(streamed, streamBook, streamTrades)

	/*
		**시세 · 호가 · 체결을 동시에 받는다** (09-25 후속3).

		예전에는 차례로 불렀다 — 상세(종목 정보 → 가격) 다음 호가, 그다음 체결. 서로의
		결과를 쓰지 않는데 왕복이 줄지어 이어져, 종목을 처음 열 때(캐시가 빈 유일한 순간)
		272~540ms를 기다렸다(09-24~25 운영 로그). 호출 수는 그대로라 Toss 한도에는 차이가
		없고, 기다림만 가장 느린 하나로 준다. 상세가 실패하면 예전처럼 통째로 실패다.
	*/
	var (
		detail model.QuoteDetail
		ok     bool
		book   = model.OrderBook{Symbol: symbol}
		trades = []model.TradeTick{}
		wg     sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		detail, ok = s.getQuoteUncached(ctx, symbol)
	}()
	if plan.BookFromStream {
		book = *streamBook
	} else {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b, okBook := s.terminalOrderBook(ctx, symbol, streamed); okBook {
				book = b
			}
		}()
	}
	if plan.TradesFromStream {
		trades = streamTrades
	} else {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if t, okTrades := s.terminalTrades(ctx, symbol, streamed); okTrades {
				trades = t
			}
		}()
	}
	wg.Wait()
	if !ok {
		return model.QuoteTerminalSnapshot{}, false
	}

	s.rehydrateQuoteDetailWithSources(ctx, &detail, trades, &book)

	source := s.dataSource()
	if plan.anyFromStream() {
		source = streamDataSource
	}

	snap := model.QuoteTerminalSnapshot{
		Detail:    detail,
		OrderBook: book,
		Trades:    trades,
		Source:    source,
		CachedAt:  time.Now().UTC(),
	}
	/*
		**신선한 시간과 사는 시간을 나눈다** (2026-09-05 사용자 지적 6).

		Redis TTL은 `terminalSnapshotTTL`(60초)이고, 「신선하다」는 그중 앞의 1초뿐이다
		(`MarketDataQuoteCacheSec`, 위 `CachedAt` 참조). 예전에는 TTL 자체가 1초라
		2초가 지난 요청은 캐시에 **아무것도 없는** 상태로 상류를 동기로 불렀다.

		실측(2026-09-05, 마감 중 20회): 1.3~2ms가 대부분인데 **200~300ms가 서너 번**.
		느린 쪽이 정확히 캐시가 막 만료된 요청이고, 그 200ms는 Toss 왕복이다. 사용자가
		「가끔씩 엄청 느려진다」고 적은 모양이 이것이다.
	*/
	s.cacheSetJSON(ctx, fmt.Sprintf(quotesTerminalKeyFmt, symbol), snap, terminalSnapshotTTL)
	s.persistQuoteDetail(ctx, symbol, detail, source)
	if !plan.BookFromStream {
		s.persistOrderBook(ctx, symbol, book)
	}
	if !plan.TradesFromStream {
		s.persistTrades(ctx, symbol, trades)
	}
	s.publishQuoteUpdate(detail)
	return snap, true
}

/*
terminalSourcePlan — 스냅샷의 각 부분을 어디서 가져올지.

`refreshTerminalSnapshot`에서 떼어 낸 것은 이 판단이 **이 변경에서 가장 판단이
들어간 부분**이기 때문이다. 틀리면 두 가지 고장이 난다:

	너무 적극적 — 구독이 죽었는데 구독 데이터를 쓴다 → 화면이 조용히 멈춘다
	너무 소극적 — 구독이 살아 있는데 REST를 탄다 → 최적화가 통째로 무효

둘 다 「값이 좀 이상하다」로만 보여 로그로는 안 잡힌다. 그래서 네트워크 없이
순수 함수로 검증할 수 있어야 한다.
*/
type terminalSourcePlan struct {
	BookFromStream   bool
	TradesFromStream bool
}

func (p terminalSourcePlan) anyFromStream() bool {
	return p.BookFromStream || p.TradesFromStream
}

/*
planTerminalSources — 규칙은 세 줄이다.

	구독이 살아 있지 않다                    → 전부 예전 경로(REST)
	구독이 살아 있고 그 채널의 버퍼가 찼다    → 구독
	구독은 살아 있으나 그 채널이 비었다       → 예전 경로

세 번째가 필요한 이유: 슬롯이 모자라면 `planTopics`가 orderbook을 버리고 trade만
남긴다. 그때 호가는 구독이 아예 안 주므로 REST로 받아야 한다 — 채널별로 따로
판단하지 않으면 호가창이 빈 채로 남는다.
*/
func planTerminalSources(streamed bool, book *model.OrderBook, trades []model.TradeTick) terminalSourcePlan {
	if !streamed {
		return terminalSourcePlan{}
	}
	return terminalSourcePlan{
		BookFromStream:   book != nil && (len(book.Asks) > 0 || len(book.Bids) > 0),
		TradesFromStream: len(trades) > 0,
	}
}

/*
terminalOrderBook — 구독이 호가를 안 주는 경우의 호가.

`streamed`는 「이 종목의 구독이 살아 있다」는 뜻이다. 그때 호가가 없는 것은
슬롯 예산 때문에 orderbook 채널이 빠졌다는 뜻이므로(`planTopics`는 슬롯이
모자라면 trade만 남긴다) 캐시를 먼저 본다 — 폴링이 최근에 채웠을 수 있다.

구독이 아예 없으면 예전 동작 그대로 REST로 간다.
*/
func (s *QuotesService) terminalOrderBook(ctx context.Context, symbol string, streamed bool) (model.OrderBook, bool) {
	if streamed {
		if book, ok := s.loadOrderBookFromCache(ctx, symbol, true); ok {
			return book, true
		}
	}
	return s.getOrderBookUncached(ctx, symbol)
}

// terminalTrades mirrors terminalOrderBook for the execution list.
func (s *QuotesService) terminalTrades(ctx context.Context, symbol string, streamed bool) ([]model.TradeTick, bool) {
	if streamed {
		if trades, ok := s.loadTradesFromCache(ctx, symbol, true); ok {
			return trades, true
		}
	}
	return s.getTradesUncached(ctx, symbol)
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

	// 일봉은 1분봉과 달리 하루에 한 칸만 늘어난다. 30초 틱마다 200봉을 다시 받을
	// 이유가 없어 dailyRefetchInterval로 묶는다 — 같은 MARKET_DATA_CHART 버킷을
	// 사용자의 차트 요청과 나눠 쓰기 때문이다.
	var raw1d CandlesPage
	var ok1d bool
	if s.shouldFetchDailyCandles(symbol) {
		raw1d, ok1d = s.getCandlesPageUncached(ctx, symbol, model.Interval1d, "")
		if !ok1d {
			// 실패했으면 기록을 되돌린다 — 그대로 두면 5분 동안 재시도가 막힌다.
			s.forgetDailyFetch(symbol)
		}
	}

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
	// 쓰면서 프로세스 안 기억도 함께 갈아 끼운다 — 자기가 쓴 값을 낡게 보는 일이
	// 없어야 한다 (`quotes_raw_tail_memo.go`).
	s.storeRawCandles(ctx, key, page.Candles, time.Duration(model.MarketDataCandleTailCacheSec)*time.Second)

	if s.candles != nil && s.candles.HasTable(ctx) {
		completed, _ := splitFormingCandle(page.Candles, base)
		// 보관 한계 밖의 1분봉은 쓰지 않는다 — 다음 롤업이 곧바로 지운다.
		completed = dropUnretainedIntraday(base, completed)
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
	// 계층 시계열은 인트라데이 간격이 하나라도 있을 때 딱 한 번만 읽는다.
	var tieredSource []model.Candle
	tieredLoaded := false

	for _, interval := range allCandleIntervals {
		if interval == model.Interval1m || interval == model.Interval1d {
			continue
		}
		if !needsCandleAggregation(interval) {
			continue
		}

		var source []model.Candle
		var nextBefore *string
		base := model.Interval1d
		if intervalMinutes(interval) > 0 {
			base = model.Interval1m
			// 1분봉만으로는 깊은 간격을 채우지 못한다 — 롤업 뒤 1m 계층은 3일치라
			// 60분봉 120봉(7,200분)에 한참 못 미치고, 그러면
			// needsIntradayDepthRefresh가 매 틱 깊은 Toss 갱신을 부른다.
			// 계층 시계열은 같은 심볼을 계층당 한 번씩만 읽어 60일치를 준다.
			if !tieredLoaded {
				tieredSource = s.intradaySnapshotSource(ctx, symbol, raw1m)
				tieredLoaded = true
			}
			source = tieredSource
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

		// 커서는 **접은 결과**에서 뽑는다. 원본(1m/1d)의 커서를 그대로 쓰면
		// 다음 페이지가 이 페이지와 어긋난다 — 접힌 봉 수와 원본 봉 수가 다르기
		// 때문이다.
		//
		//   너무 과거를 가리키면(원본이 더 깊을 때) → 두 페이지 사이에 **구멍**
		//   너무 최근을 가리키면(접은 쪽이 더 깊을 때) → 페이지가 **겹침**
		//
		// 둘 다 실측했다. 계층 저장 이전에도 5분봉은 구멍이 났고(원본 1분봉
		// 7,200개 = 5일치인데 5분봉 120개는 10시간치), 계층 저장으로 스냅샷이
		// 60일까지 깊어지자 30·60분봉이 페이지당 60~106봉씩 겹쳤다.
		//
		// trimCandlesTail이 잘라낸 가장 오래된 버킷은 커서 바로 앞이라, 다음
		// 페이지가 그것부터 이어 받는다.
		if cursor := oldestBarCursor(agg, base); cursor != nil {
			nextBefore = cursor
		}

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

	// rev는 심볼 단위 카운터라 간격마다 다시 읽을 이유가 없다 — 예전에는 간격별
	// 스냅샷에 실려 오던 값이라 다섯 번 읽고 있었다.
	rev := s.getRev(ctx, symbol)

	for _, interval := range []model.CandleInterval{
		model.Interval1m, model.Interval5m, model.Interval10m, model.Interval30m,
		model.Interval1d,
	} {
		/*
			**형성봉 칸만 읽고 형성봉 칸만 쓴다.**

			예전에는 여기서 스냅샷 전체를 읽어 형성봉 하나를 고치고 통째로 되썼다.
			수백 봉짜리 JSON을 2초마다 간격 다섯 개씩 왕복시킨 셈이라, Redis 트래픽의
			대부분이 이 줄에서 나왔다(`cacheCandleSnapshot` 주석의 실측 참고).
			`rev`도 여기서는 안 건드린다 — 형성봉 갱신은 봉을 닫지 않는다.
		*/
		stored, ok := s.loadFormingCandle(ctx, symbol, interval)
		if !ok || stored == nil {
			continue
		}

		price := currentPrice
		forming := *stored
		forming.Close = price
		if price > forming.High {
			forming.High = price
		}
		if price < forming.Low || forming.Low == 0 {
			forming.Low = price
		}
		s.storeFormingCandle(ctx, symbol, interval, &forming)

		if interval == model.Interval1m && s.candles != nil && s.candles.HasTable(ctx) {
			_ = s.candles.UpsertBatch(ctx, symbol, model.Interval1m, []model.Candle{forming})
		}

		pub := forming

		s.streamHub.Publish(model.StreamEvent{
			Type:     "bar_update",
			Symbol:   symbol,
			Interval: interval,
			Forming:  &pub,
			Rev:      rev,
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
	s.touchTerminal(symbol)

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

	metrics.SSEOpened()
	defer metrics.SSEClosed()

	beat := time.NewTicker(sseHeartbeatInterval)
	defer beat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-beat.C:
			if !writeSSEHeartbeat(w, flusher) {
				return
			}
		case ev, open := <-ch:
			if !open {
				return
			}
			writeSSE(w, flusher, ev)
		}
	}
}

/*
sseHeartbeatInterval — 살아 있는지 확인하려고 주기적으로 보내는 주석 프레임 간격.

**끊긴 소켓을 조기에 정리하기 위한 것이다.** 이 스트림은 이벤트가 올 때만 쓰기를
하는데(허브 채널 수신), 종목이 조용하면 몇 분씩 아무것도 안 쓴다. 그동안 클라이언트가
사라져도(모바일 백그라운드 전환·네트워크 끊김) 서버는 알 방법이 없다 —
`RequestTimeout` 미들웨어는 SSE를 제외하고, `SetWriteDeadline(time.Time{})`으로
쓰기 마감도 없앴기 때문이다. 결과적으로 죽은 연결이 **TCP 타임아웃까지**
goroutine과 소켓·파일 디스크립터를 잡았다.

주기적으로 쓰면 그 쓰기가 실패하면서 바로 정리된다. 2 OCPU 박스에서는 goroutine보다
**파일 디스크립터**가 먼저 걸리므로 이게 값싼 보험이다.

`:`로 시작하는 줄은 SSE 명세상 **주석**이라 클라이언트 이벤트 핸들러를 깨우지 않는다.
30초는 흔한 프록시 유휴 타임아웃(60초)의 절반이다 — nginx가 중간에 끊는 것도 함께 막는다.
*/
const sseHeartbeatInterval = 30 * time.Second

// writeSSEHeartbeat reports whether the client is still there.
func writeSSEHeartbeat(w http.ResponseWriter, flusher http.Flusher) bool {
	if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
		return false
	}
	flusher.Flush()
	return true
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
	s.touchTerminal(symbol)

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

	metrics.SSEOpened()
	defer metrics.SSEClosed()

	beat := time.NewTicker(sseHeartbeatInterval)
	defer beat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-beat.C:
			if !writeSSEHeartbeat(w, flusher) {
				return
			}
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
