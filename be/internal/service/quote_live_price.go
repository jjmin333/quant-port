package service

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

func newestTradePrice(trades []model.TradeTick) (float64, bool) {
	if len(trades) == 0 {
		return 0, false
	}
	var best model.TradeTick
	var bestT time.Time
	found := false
	for _, t := range trades {
		if t.Price <= 0 {
			continue
		}
		ts := parseQuoteCandleTime(t.Time)
		if ts.IsZero() {
			if !found {
				best = t
				found = true
			}
			continue
		}
		if !found || ts.After(bestT) {
			best = t
			bestT = ts
			found = true
		}
	}
	if !found {
		return 0, false
	}
	return best.Price, true
}

/*
newestTradePriceAt — `newestTradePrice`와 같은 값에 **그 체결의 시각**을 붙인다.

시각이 필요한 곳은 하나다: 배치 가격 캐시와 체결 중 어느 쪽이 새것인지 고를 때
(`resolveBestLivePrice`). 시각을 못 읽은 체결은 zero time으로 나가고, 호출부는
그걸 「비교할 수 없다」로 다뤄 예전 순서(체결 우선)를 그대로 쓴다 — 시각이 없다는
이유로 값을 버리면 시각 형식이 다른 상류 하나가 화면 전체를 멈춘다.
*/
func newestTradePriceAt(trades []model.TradeTick) (float64, time.Time, bool) {
	if len(trades) == 0 {
		return 0, time.Time{}, false
	}
	var best model.TradeTick
	var bestT time.Time
	found := false
	for _, t := range trades {
		if t.Price <= 0 {
			continue
		}
		ts := parseQuoteCandleTime(t.Time)
		if ts.IsZero() {
			if !found {
				best = t
				found = true
			}
			continue
		}
		if !found || bestT.IsZero() || ts.After(bestT) {
			best = t
			bestT = ts
			found = true
		}
	}
	if !found {
		return 0, time.Time{}, false
	}
	if bestT.IsZero() {
		return best.Price, time.Time{}, true
	}
	return best.Price, bestT.UTC(), true
}

func orderBookMid(book model.OrderBook) (float64, bool) {
	var bestBid, bestAsk float64
	for _, b := range book.Bids {
		if b.Price > 0 && (bestBid == 0 || b.Price > bestBid) {
			bestBid = b.Price
		}
	}
	for _, a := range book.Asks {
		if a.Price > 0 && (bestAsk == 0 || a.Price < bestAsk) {
			bestAsk = a.Price
		}
	}
	switch {
	case bestBid > 0 && bestAsk > 0:
		return (bestBid + bestAsk) / 2, true
	case bestBid > 0:
		return bestBid, true
	case bestAsk > 0:
		return bestAsk, true
	default:
		return 0, false
	}
}

func (s *QuotesService) cachedTradesForPrice(ctx context.Context, symbol string) []model.TradeTick {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	requireFresh := s.shouldLivePollSymbol(ctx, sym) &&
		quoteSessionHasLiveMicrostructure(s.marketSessionFor(ctx, sym))
	trades, ok := s.loadTradesFromCache(ctx, sym, requireFresh)
	if !ok {
		if persisted, ok := s.loadTradesPersisted(ctx, sym); ok {
			if !requireFresh || !s.terminalDataNeedsRefresh(ctx, sym, time.Time{}, persisted) {
				return persisted
			}
		}
		return nil
	}
	return trades
}

func (s *QuotesService) cachedOrderBookForPrice(ctx context.Context, symbol string) *model.OrderBook {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	requireFresh := s.shouldLivePollSymbol(ctx, sym) &&
		quoteSessionHasLiveMicrostructure(s.marketSessionFor(ctx, sym))
	book, ok := s.loadOrderBookFromCache(ctx, sym, requireFresh)
	if !ok {
		if persisted, ok := s.loadOrderBookPersisted(ctx, sym); ok {
			if !requireFresh || !s.terminalDataNeedsRefresh(ctx, sym, time.Time{}, nil) {
				return &persisted
			}
		}
		return nil
	}
	return &book
}

/*
형성봉 종가만 필요한 자리 둘. **작은 키만 읽는다**(`cacheCandleSnapshot` 주석).

예전에는 스냅샷을 통째로 받아 `Forming.Close` 하나만 꺼냈다. 이 둘은 현재가를
해석하는 경로라 종목마다 매 틱 불리므로, 수백 봉짜리 JSON을 그때마다 왕복시킬
이유가 없었다.
*/
func (s *QuotesService) lastIntradayFormingClose(ctx context.Context, symbol string) (float64, bool) {
	return s.formingCloseFor(ctx, symbol, model.Interval1m)
}

func (s *QuotesService) lastDailyFormingClose(ctx context.Context, symbol string) (float64, bool) {
	return s.formingCloseFor(ctx, symbol, model.Interval1d)
}

func (s *QuotesService) formingCloseFor(
	ctx context.Context,
	symbol string,
	interval model.CandleInterval,
) (float64, bool) {
	forming, ok := s.loadFormingCandle(ctx, symbol, interval)
	if !ok || forming == nil || forming.Close <= 0 {
		return 0, false
	}
	return forming.Close, true
}

func newestCandleByTime(candles []model.Candle) (model.Candle, bool) {
	if len(candles) == 0 {
		return model.Candle{}, false
	}
	newest := candles[0]
	newestT := parseQuoteCandleTime(newest.Time)
	for _, c := range candles[1:] {
		t := parseQuoteCandleTime(c.Time)
		if t.After(newestT) {
			newest = c
			newestT = t
		}
	}
	return newest, true
}

func (s *QuotesService) lastDailyBarClose(ctx context.Context, symbol string) (float64, bool) {
	snap, ok := s.loadCandleSnapshot(ctx, symbol, model.Interval1d)
	if !ok || len(snap.Completed) == 0 {
		return 0, false
	}
	newest, ok := newestCandleByTime(snap.Completed)
	if !ok || newest.Close <= 0 {
		return 0, false
	}
	return newest.Close, true
}

func quoteSessionHasLiveMicrostructure(session string) bool {
	switch session {
	case "regular", "pre", "after", "day":
		return true
	default:
		return false
	}
}

func (s *QuotesService) directTossPrice(ctx context.Context, symbol string) (float64, bool) {
	if s.toss == nil || !s.toss.Enabled() {
		return 0, false
	}
	price, err := s.toss.GetPrice(ctx, symbol)
	if err != nil || price == nil {
		return 0, false
	}
	v := parseDecimal(price.LastPrice)
	return v, v > 0
}

func (s *QuotesService) priceLooksLikePrevClose(ctx context.Context, symbol string, price float64) bool {
	if price <= 0 {
		return false
	}
	ref, ok := s.resolvePrevClose(ctx, symbol)
	if !ok || ref <= 0 {
		return false
	}
	return isSamePrice(price, ref)
}

// ResolveLiveQuotePrice is the single BE entry for current executable price (rankings, quotes, batch, terminal).
func (s *QuotesService) ResolveLiveQuotePrice(ctx context.Context, symbol string, fallback float64) float64 {
	return s.resolveBestLivePrice(ctx, symbol, fallback, nil, nil)
}

// rankingPriceParallelism bounds how many ranking rows resolve their price at
// once. See EnrichRankingPrices for why this is safe for upstream quotas.
const rankingPriceParallelism = 8

/*
EnrichRankingPrices overlays ranking lastPrice with ResolveLiveQuotePrice so the
list matches the quote endpoints.

**왜 병렬인가.** 「실시간 차트」(/market)는 열자마자 랭킹 100건을 받는다. 이 함수가
한 건씩 순서대로 돌면 그 100건이 그대로 직렬이 된다 — 실측으로 건수에 정확히
비례했다(count 8/25/50/100 → KR 0.15/0.29/0.61/1.44초, US 0.22/0.49/1.24/2.36초).

**외부 호출이 늘지 않는가.** 늘지 않는다. 이 경로의 비용은 Toss가 아니라 **Redis
왕복**이다 — count=100 요청 하나가 Redis 명령 약 700회를 냈고(항목당 7회), 왕복
2ms를 곱하면 1.4초로 실측치와 맞는다. 랭킹 상위 종목은 폴러가 계속 데워 두므로
EnsurePrice가 대부분 캐시에 적중하고 Toss까지 내려가지 않는다.

동시도를 8로 묶는 이유가 그 나머지다. 캐시가 전부 비어 모든 항목이 업스트림까지
내려가는 최악의 경우에도 동시 호출이 8을 넘지 않는다 — 지금(직렬로 100회를 길게
늘어뜨리는 것)보다 순간 부하가 커지지 않는다.

가격 해석 로직 자체는 건드리지 않았다. **표시되는 값은 이전과 같다.**
*/
func (s *QuotesService) EnrichRankingPrices(ctx context.Context, items []toss.RankingItem) []toss.RankingItem {
	if len(items) == 0 {
		return items
	}
	out := make([]toss.RankingItem, len(items))
	copy(out, items)

	// 각 goroutine은 자기 인덱스만 건드리므로 잠금이 필요 없다.
	sem := make(chan struct{}, rankingPriceParallelism)
	var wg sync.WaitGroup

	for i := range out {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// 요청이 이미 끊겼으면 남은 항목까지 해석할 이유가 없다.
			if ctx.Err() != nil {
				return
			}

			fallback := parseDecimal(out[idx].Price.LastPrice)
			live := s.ResolveLiveQuotePrice(ctx, out[idx].Symbol, fallback)
			if live <= 0 {
				return
			}
			/*
				기준가는 **종목 화면과 같은 값**(저장된 일봉의 직전 거래일 종가)이 먼저다
				(2026-09-26 외부 방문자 점검). 토스 순위의 basePrice는 그 세션(NXT 애프터
				마켓 포함)의 기준가라, 장이 닫힌 뒤에는 같은 종가 286,500원이 순위에서
				+3.62%(기준 276,500) · 종목 화면에서 +3.24%(기준 277,500)로 갈렸다. 장중
				오버레이(`RankingPrices` → `LiveChanges`)는 이미 일봉을 먼저 쓴다 — 마감 중인
				이 경로만 달랐다. 일봉 조회는 종목·날짜 키로 캐시돼 있다(prevCloseFromDailyBars).
			*/
			base := parseDecimal(out[idx].Price.BasePrice)
			if prev, ok := s.prevCloseFromDailyBars(ctx, out[idx].Symbol); ok && prev > 0 {
				base = prev
				out[idx].Price.BasePrice = formatRankingPrice(prev)
			}
			out[idx].Price.LastPrice = formatRankingPrice(live)
			if base > 0 {
				rate := (live - base) / base
				formatted := strconv.FormatFloat(rate, 'f', -1, 64)
				out[idx].Price.ChangeRate = &formatted
			}
		}(i)
	}
	wg.Wait()

	return out
}

func formatRankingPrice(v float64) string {
	if v <= 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

/*
ensurePriceMicrostructure — 캐시가 빈 종목의 체결·호가를 업스트림에서 미리 받아 둔다.

# 목록에서는 아무것도 하지 않는다 (2026-09-11 사용자 요청 7)

이 함수는 종목 하나에 `/trades` + `/orderbook` **2호출**을 쓴다. 상세 화면 한 종목에는
맞지만, 목록은 항목 수만큼 이 경로를 타므로 호출이 **목록 길이에 그대로 비례**한다.

같은 사고가 이미 한 번 있었고 `quotes_stocks.go`(`/stocks` 목록)에서 고쳤다 — 그 주석에
실측이 남아 있다: limit 5/10/20/40에 0.18/1.9/6.4/**58초**(Toss 한도에 걸리며 초선형으로
나빠진다). 그런데 그때 고친 것은 **그 호출 지점 하나**였고, `/quotes/batch`는 다른 문으로
같은 방에 들어와 있었다:

	BatchQuoteSummariesResult → EnsureQuoteDetail → ensureQuoteDetailCached
	  → loadQuoteDetailPersisted → finalizeQuoteDetail → ensurePriceMicrostructure

`ensureQuoteDetailCached`에는 `if !listMode(ctx)` 가드가 있는데, 그것이 막는 것은
`refreshTerminalSnapshot` 한 분기뿐이다. **캐시·DB에서 찾은 흔한 경우**는 그 가드를
지나 `finalizeQuoteDetail`로 가고, 거기서 이 함수가 불린다.

실측(2026-09-11, 마감 후 국내 뉴스 화면 1회 진입): `/quotes/batch` 한 요청이
`rate-limit exceeded`를 **90건**(orderbook 50 · trades 40) 만들었다. 그때 폴러의
터미널 집합은 **0**이었다 — 폴러가 아니라 이 경로다. 호출 스택으로 확인했다.

# 그래서 가드를 호출 지점이 아니라 **여기** 둔다

호출 지점은 셋이고(`finalizeQuoteDetail` · `finalizeTerminalSnapshot` · 직접 호출)
앞으로 늘어난다. 지점마다 가드를 복사하면 이번처럼 하나가 빠지고, 빠진 것은 화면이
아니라 상류 한도에서만 드러난다. 목록이 체결·호가를 그리지 않는다는 사실은 이 함수가
할 일 자체를 없애므로, 판단은 여기 한 곳에 있는 게 맞다.

상세 화면은 영향이 없다 — `listMode`는 목록 경로(`BatchQuoteSummariesResult`·`/stocks`·
`/rankings/prices`)만 켠다.
*/
func (s *QuotesService) ensurePriceMicrostructure(ctx context.Context, symbol string) {
	if listMode(ctx) {
		return
	}
	if !s.shouldLivePollSymbol(ctx, symbol) {
		return
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	s.initQuoteRefresh()

	if _, ok := s.loadTradesFromCache(ctx, sym, true); !ok {
		s.quoteRefresh.doSync(ctx, "trades:"+sym, func(refreshCtx context.Context) {
			if t, ok := s.getTradesUncached(refreshCtx, sym); ok {
				s.persistTrades(refreshCtx, sym, t)
			}
		})
	}

	if _, ok := s.loadOrderBookFromCache(ctx, sym, true); !ok {
		s.quoteRefresh.doSync(ctx, "book:"+sym, func(refreshCtx context.Context) {
			if b, ok := s.getOrderBookUncached(refreshCtx, sym); ok {
				s.persistOrderBook(refreshCtx, sym, b)
			}
		})
	}
}

// resolveBestLivePrice picks the freshest executable price when batch quotes lag trades/candles.
func (s *QuotesService) resolveBestLivePrice(
	ctx context.Context,
	symbol string,
	fallback float64,
	trades []model.TradeTick,
	book *model.OrderBook,
) float64 {
	session := s.marketSessionFor(ctx, symbol)
	if quoteSessionHasLiveMicrostructure(session) {
		/*
			**목록에서는 배치 가격 캐시로 끝낸다 — 체결·호가를 보지 않는다.**

			아래 `cachedTradesForPrice`·`cachedOrderBookForPrice`는 Redis가 비면
			**MySQL로 내려간다**(`loadTradesPersisted`·`loadOrderBookPersisted`).
			그런데 랭킹 종목은 상세 화면을 여는 종목이 아니라 그 Redis 캐시가 늘
			비어 있다 — 즉 목록 한 번이 종목마다 DB를 때린다. 실측(2026-09-02,
			`/rankings/prices` 100종목): **마감 세션 2 SELECT vs 장중 43 SELECT.**
			그 차이가 통째로 이 두 줄이다.

			**목록에 그 정밀도는 필요 없다.** 배치 가격 캐시는 1초 주기 폴러가 채우고
			웹소켓 체결도 같은 키에 쓰므로(`applyStreamPrice`), 구독 중인 종목은 체결
			순간 값이 이미 여기 있다. 체결 목록을 따로 읽어도 **같은 프레임에서 나온
			같은 값**이다.

			상세 화면(`!listMode`)은 이 분기를 타지 않는다 — 거기서는 체결가가 현재가이자
			형성봉의 재료라 아래 순서를 그대로 쓴다.
		*/
		if listMode(ctx) {
			// 같은 요청이 이미 배치로 받아 둔 값이 있으면 그것부터 — Redis 왕복이
			// 종목 수만큼 더 붙는 것을 막는다(withListPrices 참조).
			if live, ok := listPrice(ctx, symbol); ok {
				return live
			}
			if s.marketStore != nil {
				if live, ok := s.marketStore.GetPrice(ctx, symbol); ok && live > 0 {
					return live
				}
			}
		}
		if len(trades) == 0 {
			trades = s.cachedTradesForPrice(ctx, symbol)
		}
		if book == nil {
			book = s.cachedOrderBookForPrice(ctx, symbol)
		}

		/*
			**배치 가격 캐시가 기준이고, 체결은 그보다 새로울 때만 이긴다**
			(2026-09-04 사용자 지적 1 「데이터 통일성」).

			# 무엇이 갈라져 있었나

			증상은 「AI 랭킹 목록 $11.36 · 같은 종목 상세 $11.39」였다. 목록(listMode)은
			위에서 배치 가격 캐시로 끝내는데, 상세는 여기서 **체결 → 호가중간 →
			형성봉 → 배치 캐시** 순으로 내려갔다. 앞의 셋이 하나라도 값을 내면 배치
			캐시는 읽히지도 않는다.

			프리·애프터 세션에서 그 앞 셋이 전부 **정규장의 잔해**다. 체결 캐시에는
			정규장 마지막 체결이 남아 있고, 형성봉도 정규장 봉이다. 배치 가격만 시간외
			호가를 따라 계속 움직이므로, 상세는 낡은 값을 「현재가」로 붙잡고 다음
			정규장까지 놓지 않았다.

			실측(2026-09-04 21:13 KST, 미국 프리마켓): FDX 상세 323.17 · 목록 324.31이
			**3회 표본 내내 고정**이었고 그 종목의 체결 목록은 비어 있었다 — 즉 그
			갭은 체결이 아니라 그 아래 두 줄(호가중간·형성봉)이 만들고 있었다.

			# 왜 순서를 뒤집는가

			배치 가격 캐시는 **1초 주기 폴러가 채우고 웹소켓 체결도 같은 키에 쓴다**
			(`applyStreamPrice`). 즉 이 캐시는 파생값이 아니라 **가격이 모이는 곳**이고,
			목록 화면이 이미 그것 하나만 읽는다. 호가중간은 거래된 값이 아니라 매수·매도
			호가의 산술 평균이고, 형성봉은 캔들에서 되짚은 값이다. 둘 다 「진짜 가격이
			없을 때의 대용품」이지 캐시보다 나은 값이 아니다.

			체결만은 캐시보다 앞설 수 있다. 정규장 중 구독이 살아 있는 종목은 체결
			프레임이 먼저 도착하고 캐시 쓰기가 한 박자 뒤다. 그래서 체결은 **버리지
			않고 시각으로 견준다** — 어느 쪽이 원천인지가 아니라 어느 쪽이 새것인지가
			질문이다. 시각을 못 구하면(캐시 미스·봉투 없는 옛 값) 예전대로 체결이 이긴다.

			# 남은 폴백의 순서

			캐시가 아예 비었을 때만 호가중간·형성봉·랭킹 스냅샷으로 내려간다. 랭킹
			스냅샷을 맨 뒤에 두는 이유는 그것이 **5분 주기**이기 때문이다(실측
			2026-09-02: `/rankings/prices`가 6초 간격 두 번 모두 SPY 761.68인데 같은
			순간 배치 캐시는 761.36 → 761.43로 움직이고 있었다).

			`GetPriceAt`/`GetPrice`는 **읽기만 한다.** `EnsurePrice`와 달리 캐시가
			비어도 업스트림으로 내려가지 않으므로 목록에서 호출해도 안전하다.
		*/
		cached, cachedAt, hasCached := 0.0, time.Time{}, false
		if s.marketStore != nil {
			cached, cachedAt, hasCached = s.marketStore.GetPriceAt(ctx, symbol)
		}
		if price, at, ok := newestTradePriceAt(trades); ok {
			if hasCached && !at.IsZero() && cachedAt.After(at) {
				return cached
			}
			return price
		}
		if hasCached {
			return cached
		}
		if book != nil {
			if mid, ok := orderBookMid(*book); ok {
				return mid
			}
		}
		if price, ok := s.lastIntradayFormingClose(ctx, symbol); ok {
			return price
		}
		if price, ok := lookupRankingLastPrice(ctx, s.redis, s.rankingsRepo, symbol); ok {
			return price
		}
		// 여기부터 두 단계는 업스트림을 부른다. 목록에서는 건너뛴다.
		if !listMode(ctx) {
			if s.marketStore != nil {
				if live, ok := s.marketStore.EnsurePrice(ctx, symbol); ok && live > 0 {
					return live
				}
			}
			if price, ok := s.directTossPrice(ctx, symbol); ok {
				return price
			}
		}
		if price, ok := s.lastDailyFormingClose(ctx, symbol); ok {
			return price
		}
		if fallback > 0 && !s.priceLooksLikePrevClose(ctx, symbol, fallback) {
			return fallback
		}
		return 0
	}

	if !listMode(ctx) && s.marketStore != nil {
		if live, ok := s.marketStore.EnsurePrice(ctx, symbol); ok && live > 0 {
			return live
		}
	}
	/*
		**마감 세션에서 그날의 값은 「시간외까지 끝난 뒤의 최종가」다.**

		`fallback`은 호출부가 배치로 받아 둔 마지막 체결가다(목록은
		`fetchPriceMap`, 상세는 위의 `EnsurePrice`). 저장된 1일봉보다 **뒤에**
		두면 정규장 종가가 이기는데, 그 둘은 시간외에 거래가 있었던 날 갈린다 —
		실측(2026-09-01 마감 후 SK하이닉스): 1일봉 1,692,000 vs 최종가
		1,652,000이라 **목록은 +2.36%, 상세는 −0.06%**로 같은 종목이 두 값으로
		보였다.

		그래서 배치 가격을 먼저 본다. 1일봉은 그 값이 없을 때의 폴백으로 남는다
		(가격 캐시가 비었고 업스트림도 못 부르는 목록 경로).
	*/
	if fallback > 0 {
		return fallback
	}
	if price, ok := s.lastDailyBarClose(ctx, symbol); ok {
		return price
	}
	return 0
}
