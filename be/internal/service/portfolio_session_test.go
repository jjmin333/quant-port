package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

func TestUpdateSessionBreakdown_afterHours(t *testing.T) {
	h := &model.UnifiedHolding{
		PrevClose:     79.94,
		RegularClose:  77.12,
		CurrentPrice:  76.90,
		MarketSession: "after",
	}
	updateSessionBreakdown(h)

	if h.RegularChangeRate < -3.6 || h.RegularChangeRate > -3.4 {
		t.Fatalf("regular rate = %.2f want ~-3.52", h.RegularChangeRate)
	}
	if h.ExtendedChangeRate < -0.31 || h.ExtendedChangeRate > -0.27 {
		t.Fatalf("extended rate = %.2f want ~-0.28", h.ExtendedChangeRate)
	}
}

func TestUpdateSessionBreakdown_regularSession(t *testing.T) {
	h := &model.UnifiedHolding{
		PrevClose:     100,
		CurrentPrice:  105,
		MarketSession: "regular",
	}
	updateSessionBreakdown(h)

	if h.RegularChangeRate != 5 {
		t.Fatalf("regular rate = %v want 5", h.RegularChangeRate)
	}
	if h.ExtendedChangeRate != 0 {
		t.Fatalf("extended rate = %v want 0", h.ExtendedChangeRate)
	}
}

func TestDetectUsSession_afterMarket(t *testing.T) {
	day := tossUsDaySample()
	// After market: 2026-03-26T05:00 - 07:00 KST
	now := time.Date(2026, 3, 26, 6, 0, 0, 0, kst)
	if got := detectUsSession(day, now); got != "after" {
		t.Fatalf("session = %q want after", got)
	}
}

func tossUsDaySample() toss.UsMarketDay {
	return toss.UsMarketDay{
		RegularMarket: &toss.UsRegularMarketSession{
			StartTime: "2026-03-25T22:30:00+09:00",
			EndTime:   "2026-03-26T05:00:00+09:00",
		},
		AfterMarket: &toss.UsAfterMarketSession{
			StartTime: "2026-03-26T05:00:00+09:00",
			EndTime:   "2026-03-26T07:00:00+09:00",
		},
	}
}
