package service

import (
	"math"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

func TestInferCurrentHoldStart_rebuyAfterFullSell(t *testing.T) {
	jan := time.Date(2025, 1, 10, 10, 0, 0, 0, kst)
	feb := time.Date(2025, 2, 10, 10, 0, 0, 0, kst)
	mar := time.Date(2025, 3, 10, 10, 0, 0, 0, kst)

	fills := []orderFill{
		{at: jan, side: toss.OrderSideBuy, qty: 100},
		{at: feb, side: toss.OrderSideSell, qty: 100},
		{at: mar, side: toss.OrderSideBuy, qty: 50},
	}

	start, ok := inferCurrentHoldStart(fills, 50)
	if !ok {
		t.Fatal("expected hold start")
	}
	if !start.Equal(mar) {
		t.Fatalf("got %v want %v", start, mar)
	}
}

func TestInferCurrentHoldStart_partialSellSameCycle(t *testing.T) {
	jan := time.Date(2025, 1, 10, 10, 0, 0, 0, kst)
	feb := time.Date(2025, 2, 10, 10, 0, 0, 0, kst)

	fills := []orderFill{
		{at: jan, side: toss.OrderSideBuy, qty: 100},
		{at: feb, side: toss.OrderSideSell, qty: 40},
	}

	start, ok := inferCurrentHoldStart(fills, 60)
	if !ok {
		t.Fatal("expected hold start")
	}
	if !start.Equal(jan) {
		t.Fatalf("got %v want %v", start, jan)
	}
}

func TestApplyPurchaseBaselines_highAtLeastAvg(t *testing.T) {
	var tracker priceRangeTracker
	buyTime := time.Date(2026, 4, 28, 10, 0, 0, 0, kst)

	tracker.applyPurchaseBaselines(buyTime, 365000, nil)
	tracker.considerOHLC(time.Date(2026, 6, 26, 0, 0, 0, 0, kst), 361000, 350000)

	if tracker.high < 365000 {
		t.Fatalf("high = %v, want at least avg purchase 365000", tracker.high)
	}
}

func TestApplyPurchaseBaselines_usesMaxFillPrice(t *testing.T) {
	var tracker priceRangeTracker
	buyTime := time.Date(2026, 4, 28, 10, 0, 0, 0, kst)
	buyFills := []orderFill{
		{at: buyTime, side: toss.OrderSideBuy, qty: 10, price: 380000},
		{at: buyTime.Add(24 * time.Hour), side: toss.OrderSideBuy, qty: 25, price: 358000},
	}

	tracker.applyPurchaseBaselines(buyTime, 365000, buyFills)
	tracker.considerOHLC(time.Date(2026, 6, 26, 0, 0, 0, 0, kst), 361000, 350000)

	if tracker.high < 380000 {
		t.Fatalf("high = %v, want at least max fill 380000", tracker.high)
	}
}

func TestPriceRangeTracker_intradayLowAfterBuy(t *testing.T) {
	var tracker priceRangeTracker
	buyTime := time.Date(2026, 7, 16, 10, 0, 0, 0, kst)

	tracker.considerOHLC(buyTime.Add(5*time.Minute), 150, 140)
	tracker.considerOHLC(buyTime.Add(10*time.Minute), 160, 155)

	if tracker.low != 140 {
		t.Fatalf("low = %v want 140", tracker.low)
	}
	if tracker.high != 160 {
		t.Fatalf("high = %v want 160", tracker.high)
	}
}

func TestPriceRangeTracker_ignoresPrePurchaseDailyLow(t *testing.T) {
	var tracker priceRangeTracker
	buyTime := time.Date(2026, 7, 16, 14, 30, 0, 0, kst)

	tracker.considerOHLC(buyTime.Add(1*time.Minute), 152, 148)
	tracker.considerOHLC(buyTime.Add(2*time.Minute), 155, 149)

	if tracker.low != 148 {
		t.Fatalf("low = %v want 148 (post-buy), not pre-buy daily low", tracker.low)
	}
}

func TestPriceRangeTracker_excludesPreFillHighInSameMinute(t *testing.T) {
	var tracker priceRangeTracker
	buyTime := time.Date(2026, 7, 16, 10, 30, 45, 0, kst)
	tracker.consider(buyTime, 150)

	candles := []toss.Candle{
		{
			Timestamp:  time.Date(2026, 7, 16, 10, 30, 0, 0, kst).Format(time.RFC3339),
			HighPrice:  "170",
			LowPrice:   "120",
			ClosePrice: "145",
		},
	}
	tracker.considerCandlesSince(candles, buyTime, time.Minute)

	if tracker.high != 150 {
		t.Fatalf("high = %v want 150 (fill price), not pre-fill 170", tracker.high)
	}
}

func TestResolvePerSharePrice_correctsPositionTotal(t *testing.T) {
	got := resolvePerSharePrice(365000, 365000, 35)
	want := 365000.0 / 35
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestNormalizePerShareExtreme(t *testing.T) {
	got := normalizePerShareExtreme(361000, 10428, 35)
	want := 361000.0 / 35
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestInferCurrentHoldStart_dcaCurrentCycle(t *testing.T) {
	mar := time.Date(2025, 3, 10, 10, 0, 0, 0, kst)
	apr := time.Date(2025, 4, 10, 10, 0, 0, 0, kst)

	fills := []orderFill{
		{at: mar, side: toss.OrderSideBuy, qty: 50},
		{at: apr, side: toss.OrderSideBuy, qty: 30},
	}

	start, ok := inferCurrentHoldStart(fills, 80)
	if !ok {
		t.Fatal("expected hold start")
	}
	if !start.Equal(mar) {
		t.Fatalf("got %v want %v", start, mar)
	}
}
