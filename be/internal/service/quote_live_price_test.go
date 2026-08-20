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
