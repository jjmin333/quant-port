package service

import (
	"math"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
)

func sale(date string, area float64, manwon int64, direct, cancelled bool) repository.ReTrade {
	p := manwon * 10000
	return repository.ReTrade{DealType: "sale", DealDate: date, Area: area, Price: &p, Direct: direct, Cancelled: cancelled}
}

func TestEstimateComparableRecentWindowSkipsIndexAndOutliers(t *testing.T) {
	today := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	trades := []repository.ReTrade{
		sale("2026-09-04", 84.9, 252000, false, false),
		sale("2026-08-07", 84.4, 261000, false, false),
		sale("2026-07-14", 84.9, 247000, false, false),
		sale("2026-06-20", 84.6, 250000, false, false),
		sale("2026-05-27", 84.4, 250000, false, false),
		sale("2026-06-01", 84.6, 150000, true, false),  // 직거래 — 빼야 한다
		sale("2026-06-02", 84.6, 990000, false, true),  // 해제 — 빼야 한다
		sale("2026-03-01", 59.9, 180000, false, false), // 다른 면적
	}
	called := false
	idx := func(time.Time) (float64, bool) { called = true; return 100, true }
	got, ok := EstimateComparable(trades, 84.6, today, idx)
	if !ok || got.N != 5 || got.WindowMonths != 6 || got.IndexAdjusted {
		t.Fatalf("got n=%d window=%d adjusted=%v ok=%v", got.N, got.WindowMonths, got.IndexAdjusted, ok)
	}
	if got.ExcludedDirect != 1 || got.ExcludedCancel != 1 {
		t.Fatalf("excluded direct=%d cancel=%d", got.ExcludedDirect, got.ExcludedCancel)
	}
	if got.Value < 24.9e8 || got.Value > 25.3e8 {
		t.Fatalf("value %v outside 24.9~25.3억", got.Value)
	}
	if got.Confidence != "high" || got.LastTradeDays != 27 {
		t.Fatalf("confidence %s lastDays %d", got.Confidence, got.LastTradeDays)
	}
	_ = called
}

func TestEstimateComparableLiquidUsesThreeMonths(t *testing.T) {
	today := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	var trades []repository.ReTrade
	for _, d := range []string{"2026-09-10", "2026-08-15", "2026-08-07", "2026-07-18", "2026-07-10"} {
		trades = append(trades, sale(d, 84.9, 300000, false, false))
	}
	for _, d := range []string{"2026-05-01", "2026-05-02", "2026-05-03", "2026-05-04", "2026-05-05", "2026-05-06", "2026-05-07"} {
		trades = append(trades, sale(d, 84.9, 270000, false, false))
	}
	got, _ := EstimateComparable(trades, 84.9, today, nil)
	if got.WindowMonths != 3 || got.Value != 3e9 {
		t.Fatalf("window %d value %v — 최근 3개월만 써야 한다", got.WindowMonths, got.Value)
	}
}

func TestEstimateComparableThinMarketUsesIndex(t *testing.T) {
	today := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	trades := []repository.ReTrade{sale("2025-12-10", 84, 100000, false, false), sale("2026-01-15", 84, 102000, false, false)}
	idx := func(d time.Time) (float64, bool) {
		if d.Before(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
			return 100, true
		}
		return 110, true
	}
	got, ok := EstimateComparable(trades, 84, today, idx)
	if !ok || !got.IndexAdjusted || got.WindowMonths != 12 {
		t.Fatalf("got %+v", got)
	}
	if math.Abs(got.Value-1.111e9) > 1e6 { // 중앙값 10.1억 × 1.1
		t.Fatalf("value %v", got.Value)
	}
	if got.Confidence != "low" { // 최근 거래가 180일 넘음
		t.Fatalf("confidence %s", got.Confidence)
	}
}

func TestFixedIncome(t *testing.T) {
	start := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(1, 0, 0)
	v, i := DepositValue(10_000_000, 4, start, end, end.AddDate(0, 1, 0), false, 15.4)
	if i != 338_400 || v != 10_338_400 {
		t.Fatalf("deposit %v %v (세후 338,400원이어야)", v, i)
	}
	if v, _, _ := SavingValue(500_000, 4, start, 12, end, 0); math.Abs(v-6_130_000) > 3_000 {
		t.Fatalf("saving %v (원금 600만 + 이자 약 13만)", v)
	}
	if b := BondValue(1_000_000, 0, 0, end, start, 5); math.Abs(b-952_381) > 2 {
		t.Fatalf("zero coupon %v", b)
	}
	if b := BondValue(1_000_000, 5, 1, end, start, 5); math.Abs(b-1_000_000) > 2 {
		t.Fatalf("par bond %v", b)
	}
	if y, _ := CurveYield(map[float64]float64{1: 2.5, 3: 2.9}, 2); math.Abs(y-2.7) > 1e-9 {
		t.Fatalf("curve %v", y)
	}
}

func TestScheduledLoan(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mat := start.AddDate(30, 0, 0)
	bal, pay, ok := ScheduledLoan(100_000_000, 4, "equal_payment", start, mat, start)
	if !ok || bal != 100_000_000 || math.Abs(pay-477_415) > 1 {
		t.Fatalf("equal payment %v %v %v", bal, pay, ok)
	}
	bal, _, _ = ScheduledLoan(120_000_000, 4, "equal_principal", start, start.AddDate(10, 0, 0), start.AddDate(1, 0, 0))
	if bal != 108_000_000 {
		t.Fatalf("equal principal after 1y %v", bal)
	}
	if g := GoldValue(31.1034768, 2000, 1350); g != 2_700_000 {
		t.Fatalf("gold %v", g)
	}
}
