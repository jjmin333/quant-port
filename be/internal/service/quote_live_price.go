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

func latestTradePrice(trades []model.TradeTick) (float64, bool) {
	return newestTradePrice(trades)
}

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

func (s *QuotesService) lastIntradayFormingClose(ctx context.Context, symbol string) (float64, bool) {
	snap, ok := s.loadCandleSnapshot(ctx, symbol, model.Interval1m)
	if !ok || snap.Forming == nil || snap.Forming.Close <= 0 {
		return 0, false
	}
	return snap.Forming.Close, true
}

func (s *QuotesService) lastDailyFormingClose(ctx context.Context, symbol string) (float64, bool) {
	snap, ok := s.loadCandleSnapshot(ctx, symbol, model.Interval1d)
	if !ok || snap.Forming == nil || snap.Forming.Close <= 0 {
		return 0, false
	}
	return snap.Forming.Close, true
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
			base := parseDecimal(out[idx].Price.BasePrice)
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

// ensurePriceMicrostructure warms trades/orderbook caches for cold symbols during live sessions.
func (s *QuotesService) ensurePriceMicrostructure(ctx context.Context, symbol string) {
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
		if len(trades) == 0 {
			trades = s.cachedTradesForPrice(ctx, symbol)
		}
		if book == nil {
			book = s.cachedOrderBookForPrice(ctx, symbol)
		}

		if price, ok := newestTradePrice(trades); ok {
			return price
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
	if price, ok := s.lastDailyBarClose(ctx, symbol); ok {
		return price
	}
	if fallback > 0 {
		return fallback
	}
	return 0
}
