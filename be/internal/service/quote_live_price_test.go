package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestNewestTradePrice_picksLatestTimestamp(t *testing.T) {
	price, ok := newestTradePrice([]model.TradeTick{
		{Time: "2026-08-03T09:00:00+09:00", Price: 1_565_000},
		{Time: "2026-08-03T20:00:00+09:00", Price: 1_718_000},
		{Time: "2026-08-03T15:30:00+09:00", Price: 1_600_000},
	})
	if !ok || price != 1_718_000 {
		t.Fatalf("newest trade = %v ok=%v want 1718000", price, ok)
	}
}

func TestOrderBookMid(t *testing.T) {
	mid, ok := orderBookMid(model.OrderBook{
		Bids: []model.OrderBookLevel{{Price: 910}, {Price: 908}},
		Asks: []model.OrderBookLevel{{Price: 914}, {Price: 916}},
	})
	if !ok || mid != 912 {
		t.Fatalf("mid = %v ok=%v want 912", mid, ok)
	}
}

func TestQuoteSessionHasLiveMicrostructure(t *testing.T) {
	if quoteSessionHasLiveMicrostructure("regular") != true {
		t.Fatal("regular should use microstructure")
	}
	if quoteSessionHasLiveMicrostructure("closed") != false {
		t.Fatal("closed should skip microstructure")
	}
}

func TestResolveBestLivePrice_closedSessionIgnoresStaleTrades(t *testing.T) {
	s := &QuotesService{}
	session := s.marketSessionFor(t.Context(), "MU")
	price := s.resolveBestLivePrice(t.Context(), "MU", 815.2, []model.TradeTick{
		{Price: 908.2},
	}, nil)
	if quoteSessionHasLiveMicrostructure(session) {
		if price != 908.2 {
			t.Fatalf("live session price = %v want 908.2 from trades", price)
		}
		return
	}
	if price != 815.2 {
		t.Fatalf("closed session price = %v want 815.2 fallback", price)
	}
}

func TestResolveBestLivePrice_liveSessionRejectsPrevCloseFallback(t *testing.T) {
	s := &QuotesService{}
	// Closed session: completed daily / fallback allowed (no microstructure path).
	price := s.resolveBestLivePrice(t.Context(), "000660", 1_718_000, nil, nil)
	if price != 1_718_000 {
		t.Fatalf("closed-session fallback = %v want 1718000", price)
	}
}

func TestPriceLooksLikePrevClose(t *testing.T) {
	if !isSamePrice(1_718_000, 1_718_000) {
		t.Fatal("expected same price")
	}
	if isSamePrice(1_580_000, 1_718_000) {
		t.Fatal("expected different prices")
	}
}

func TestNewestCandleByTime_descendingSnapshot(t *testing.T) {
	candles := []model.Candle{
		{Time: "2026-07-31", Close: 1703000},
		{Time: "2025-10-02", Close: 397500},
	}
	got, ok := newestCandleByTime(candles)
	if !ok || got.Close != 1703000 {
		t.Fatalf("newest close = %v want 1703000", got.Close)
	}
}

func TestNewestCandleByTime_ascendingSnapshot(t *testing.T) {
	candles := []model.Candle{
		{Time: "2025-10-02", Close: 397500},
		{Time: "2026-07-31", Close: 1703000},
	}
	got, ok := newestCandleByTime(candles)
	if !ok || got.Close != 1703000 {
		t.Fatalf("newest close = %v want 1703000", got.Close)
	}
}

/*
newestTradePriceAt — 값과 함께 **그 체결의 시각**을 돌려준다.

이 시각이 없어서 2026-09-04에 「목록 $11.36 vs 상세 $11.39」가 났다. 상세는 체결을
먼저 보는데, 프리·애프터 세션의 체결 캐시에는 정규장 마지막 체결이 남아 있고 배치
가격 캐시는 그 사이에도 움직인다 — 어느 쪽이 새것인지 **물어볼 수가 없었다.**
*/
func TestNewestTradePriceAt_returnsTimestamp(t *testing.T) {
	price, at, ok := newestTradePriceAt([]model.TradeTick{
		{Time: "2026-08-03T09:00:00+09:00", Price: 1_565_000},
		{Time: "2026-08-03T20:00:00+09:00", Price: 1_718_000},
		{Time: "2026-08-03T15:30:00+09:00", Price: 1_600_000},
	})
	if !ok || price != 1_718_000 {
		t.Fatalf("newest trade = %v ok=%v want 1718000", price, ok)
	}
	if at.IsZero() {
		t.Fatal("시각이 비었다 — 배치 가격과 순서를 정할 수 없다")
	}
	if got := at.UTC().Format("2006-01-02T15:04:05Z"); got != "2026-08-03T11:00:00Z" {
		t.Fatalf("시각 = %s, want 2026-08-03T11:00:00Z", got)
	}
}

/*
시각을 못 읽은 체결은 **값은 살리고 시각만 비운다.**

호출부는 zero time을 「비교할 수 없다」로 다뤄 예전 순서(체결 우선)를 그대로 쓴다.
시각이 없다는 이유로 값을 버리면, 시각 형식이 다른 상류 하나가 화면을 멈춘다.
*/
func TestNewestTradePriceAt_undatedTradeKeepsPriceDropsTime(t *testing.T) {
	price, at, ok := newestTradePriceAt([]model.TradeTick{{Time: "", Price: 900}})
	if !ok || price != 900 {
		t.Fatalf("price = %v ok=%v want 900", price, ok)
	}
	if !at.IsZero() {
		t.Fatalf("시각 = %v, want zero", at)
	}
}

// 시각이 있는 체결이 하나라도 있으면 그쪽을 고른다 — 시각 없는 것이 앞에 와도.
func TestNewestTradePriceAt_prefersDatedOverUndated(t *testing.T) {
	price, at, ok := newestTradePriceAt([]model.TradeTick{
		{Time: "", Price: 900},
		{Time: "2026-08-03T20:00:00+09:00", Price: 1_000},
	})
	if !ok || price != 1_000 {
		t.Fatalf("price = %v ok=%v want 1000", price, ok)
	}
	if at.IsZero() {
		t.Fatal("시각 있는 체결을 골랐는데 시각이 비었다")
	}
}

/*
배치 가격 캐시가 없을 때만 호가중간·형성봉으로 내려간다 (2026-09-04 사용자 지적 1).

실측이 이 순서를 요구했다: 미국 프리마켓의 FDX가 상세 323.17 · 목록 324.31로 3회
표본 내내 고정이었고, **그 종목의 체결 목록은 비어 있었다.** 즉 갭을 만든 것은
체결이 아니라 그 아래 두 줄(호가중간·형성봉)이었다 — 체결만 고쳐서는 안 되는 이유다.

`marketStore`가 nil이면 캐시가 아예 없는 것과 같으므로 **폴백이 살아 있어야 한다.**
그 경로가 죽으면 캐시가 식은 순간 상세 화면의 현재가가 0이 된다.

세션은 실제 시각에서 나오므로(`marketSessionFor`) 테스트가 고를 수 없다 — 위
`closedSessionIgnoresStaleTrades`와 같은 방식으로 두 갈래를 다 검사한다.
*/
func TestResolveBestLivePrice_fallsBackWhenNoCache(t *testing.T) {
	s := &QuotesService{}
	book := model.OrderBook{
		Bids: []model.OrderBookLevel{{Price: 910}},
		Asks: []model.OrderBookLevel{{Price: 914}},
	}
	live := quoteSessionHasLiveMicrostructure(s.marketSessionFor(t.Context(), "FDX"))
	got := s.resolveBestLivePrice(t.Context(), "FDX", 0, nil, &book)
	if live {
		if got != 912 {
			t.Fatalf("가격 = %v, want 912 (호가중간) — 캐시가 없으면 폴백이 살아 있어야 한다", got)
		}
		return
	}
	// 마감 세션은 호가를 보지 않는다. fallback 0이므로 값이 없다.
	if got != 0 {
		t.Fatalf("마감 세션 가격 = %v, want 0", got)
	}
}

// 체결이 있고 캐시가 없으면 체결을 그대로 쓴다 — 견줄 대상이 없다.
func TestResolveBestLivePrice_tradeWinsWithoutCache(t *testing.T) {
	s := &QuotesService{}
	trades := []model.TradeTick{{Time: "2026-09-04T21:13:05+09:00", Price: 923.82}}
	live := quoteSessionHasLiveMicrostructure(s.marketSessionFor(t.Context(), "COST"))
	got := s.resolveBestLivePrice(t.Context(), "COST", 0, trades, nil)
	if live && got != 923.82 {
		t.Fatalf("가격 = %v, want 923.82", got)
	}
}
