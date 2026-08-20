package textparse

import (
	"math"
	"testing"
)

func TestMergeCombinedFinanceSamsung(t *testing.T) {
	eval35 := 7_840_644.0
	pnl35 := -4_916_856.0
	eval109 := 23_816_500.0
	pnl109 := -15_343_045.0

	line35 := ParsedLine{
		Name:        "삼성전자",
		Quantity:    35,
		MarketValue: &eval35,
		ProfitLoss:  &pnl35,
		Currency:    "KRW",
	}
	line109 := ParsedLine{
		Name:        "삼성전자",
		Quantity:    109,
		MarketValue: &eval109,
		ProfitLoss:  &pnl109,
		Currency:    "KRW",
	}

	merged := MergeParsedLines([]ParsedLine{line35, line109})
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged line, got %+v", merged)
	}
	if merged[0].Quantity != 144 {
		t.Fatalf("qty=%v want 144", merged[0].Quantity)
	}

	res := merged[0]
	if res.MarketValue == nil || res.ProfitLoss == nil {
		t.Fatalf("missing merged finance: %+v", res)
	}
	cost := *res.MarketValue - *res.ProfitLoss
	wantCost := (eval35 - pnl35) + (eval109 - pnl109)
	if math.Abs(cost-wantCost) > 1 {
		t.Fatalf("cost=%v want %v", cost, wantCost)
	}
	avg := cost / res.Quantity
	wantAvg := wantCost / 144
	if math.Abs(avg-wantAvg) > 1 {
		t.Fatalf("avg=%v want %v", avg, wantAvg)
	}
}
