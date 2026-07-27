package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestPickSharesOutstandingFallback(t *testing.T) {
	facts := map[string]map[string][]XBRLFact{
		"dei": {
			"EntityCommonStockSharesOutstanding": {
				{End: "2025-10-26", Val: 793_000_000, Form: "10-K", FP: "FY"},
			},
		},
	}
	got := PickSharesOutstanding(facts)
	if got == nil || *got != 793_000_000 {
		t.Fatalf("PickSharesOutstanding() = %v, want 793000000", got)
	}
}

func TestPickLatestAnnualPair(t *testing.T) {
	items := []XBRLFact{
		{End: "2023-09-30", FY: 2023, FP: "FY", Form: "10-K", Val: 100},
		{End: "2022-09-30", FY: 2022, FP: "FY", Form: "10-K", Val: 80},
		{End: "2024-06-29", FY: 2024, FP: "Q2", Form: "10-Q", Val: 55},
	}
	cur, prior := pickLatestAnnualPair(items)
	if cur == nil || cur.Val != 100 || prior == nil || prior.Val != 80 {
		t.Fatalf("pickLatestAnnualPair() = (%v, %v)", cur, prior)
	}
}

func TestComputeStoredFundamentals(t *testing.T) {
	shares := int64(10)
	current := model.FinancialSnapshot{
		Revenue:            int64Ptr(1000),
		OperatingIncome:    int64Ptr(200),
		NetIncome:          int64Ptr(150),
		TotalAssets:        int64Ptr(5000),
		TotalEquity:        int64Ptr(1000),
		TotalLiabilities:   int64Ptr(3000),
		CurrentAssets:      int64Ptr(800),
		CurrentLiabilities: int64Ptr(400),
		EPSBasic:           float64Ptr(1.5),
		EPSDiluted:         float64Ptr(1.4),
	}
	prior := model.FinancialSnapshot{
		Revenue:     int64Ptr(800),
		NetIncome:   int64Ptr(120),
		TotalAssets: int64Ptr(4500),
		TotalEquity: int64Ptr(900),
		EPSBasic:    float64Ptr(1.0),
		EPSDiluted:  float64Ptr(0.9),
	}
	computed := ComputeStoredFundamentals(current, prior, &shares)

	if computed.ROE == nil || *computed.ROE < 15.7 || *computed.ROE > 15.9 {
		t.Fatalf("ROE = %v, want ~15.79", computed.ROE)
	}
	if computed.OperatingMargin == nil || *computed.OperatingMargin != 20 {
		t.Fatalf("OperatingMargin = %v", computed.OperatingMargin)
	}
	if computed.RevenueGrowthYoY == nil || *computed.RevenueGrowthYoY != 25 {
		t.Fatalf("RevenueGrowthYoY = %v", computed.RevenueGrowthYoY)
	}
	if computed.EPSGrowthYoY == nil || *computed.EPSGrowthYoY < 55 || *computed.EPSGrowthYoY > 56 {
		t.Fatalf("EPSGrowthYoY = %v", computed.EPSGrowthYoY)
	}
	if computed.DebtRatio == nil || *computed.DebtRatio != 300 {
		t.Fatalf("DebtRatio = %v", computed.DebtRatio)
	}
	if computed.CurrentRatio == nil || *computed.CurrentRatio != 2 {
		t.Fatalf("CurrentRatio = %v", computed.CurrentRatio)
	}
}

func TestComputeValuationMetrics(t *testing.T) {
	price := 100.0
	shares := int64(10)
	current := model.FinancialSnapshot{
		TotalEquity: int64Ptr(1000),
		EPSDiluted:  float64Ptr(1.4),
	}
	val := ComputeValuationMetrics(&price, current, &shares)
	if val.PER == nil || *val.PER != 100.0/1.4 {
		t.Fatalf("PER = %v", val.PER)
	}
	if val.PBR == nil || *val.PBR != 1.0 {
		t.Fatalf("PBR = %v", val.PBR)
	}
}

func TestEnrichDerivedEPS(t *testing.T) {
	shares := int64(100)
	ni := int64(500)
	snap := model.FinancialSnapshot{
		NetIncome:         &ni,
		SharesOutstanding: &shares,
	}
	enrichDerivedEPS(&snap)
	if snap.EPSBasic == nil || *snap.EPSBasic != 5 {
		t.Fatalf("EPSBasic = %v, want 5", snap.EPSBasic)
	}
}

func TestCommonStockCandidate(t *testing.T) {
	if alt, ok := commonStockCandidate("005935"); !ok || alt != "005930" {
		t.Fatalf("commonStockCandidate(005935) = (%q, %v)", alt, ok)
	}
	if _, ok := commonStockCandidate("005930"); ok {
		t.Fatal("expected no candidate for common stock")
	}
}

func TestCalcStatusExcludesValuation(t *testing.T) {
	stored := StoredFundamentals{}
	status := calcStatus(stored)
	if status != model.FundamentalsStatusOK {
		t.Fatalf("calcStatus() = %v, want ok without per/pbr", status)
	}
}
