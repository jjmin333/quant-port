package service

import (
	"context"
	"testing"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
)

func TestBackfillMaxSymbolsHeadroom(t *testing.T) {
	b := apibudget.NewManager(nil)
	b.SetLimits(apibudget.ProviderDART, apibudget.Limits{DailyQuota: 18000})

	ctx := context.Background()
	// 18000 - 5000 reserve = 13000 / 28 ≈ 464
	got := backfillMaxSymbols(ctx, b, apibudget.ProviderDART, 5000, 28, 602)
	if got != 464 {
		t.Fatalf("expected 464, got %d", got)
	}
	if backfillMaxSymbols(ctx, b, apibudget.ProviderDART, 5000, 28, 100) != 100 {
		t.Fatal("small want should pass through")
	}
	if backfillMaxSymbols(ctx, b, apibudget.ProviderDART, 20000, 28, 602) != 0 {
		t.Fatal("no headroom should yield 0")
	}
}

func TestBackfillMaxSymbolsNilBudget(t *testing.T) {
	got := backfillMaxSymbols(context.Background(), nil, apibudget.ProviderDART, 5000, 28, 602)
	if got != 602 {
		t.Fatalf("nil budget should not cap, got %d", got)
	}
}

func TestFinancialsBackfillOptionsNormalized(t *testing.T) {
	o := FinancialsBackfillOptions{}.Normalized()
	if o.DARTReserve != financialBackfillDARTReserve {
		t.Fatalf("reserve default: %d", o.DARTReserve)
	}
	if o.SymbolPause != financialBackfillSymbolPause {
		t.Fatalf("pause default: %v", o.SymbolPause)
	}
}

func TestBackfillProviderPeriodsOnly(t *testing.T) {
	p, n := backfillProvider(model.AssetDomestic, true)
	if p != apibudget.ProviderDART || n != financialBackfillDARTPeriodsPerSym {
		t.Fatalf("periods-only domestic: %v %d", p, n)
	}
	p, n = backfillProvider(model.AssetUS, true)
	if p != apibudget.ProviderEdgarBulk || n != financialBackfillEdgarPerSym {
		t.Fatalf("periods-only us: %v %d", p, n)
	}
}

func TestBackfillSucceeded(t *testing.T) {
	stock := &model.Stock{AssetType: model.AssetDomestic, SecurityType: "STOCK"}
	rev := int64(100)
	okFund := &model.StockFundamentals{CalcStatus: model.FundamentalsStatusOK, Revenue: &rev}

	if ok, _ := backfillSucceeded(okFund, stock, true); !ok {
		t.Fatal("ok fund + periods should succeed")
	}
	if ok, reason := backfillSucceeded(okFund, stock, false); ok || reason != "no periods after ingest" {
		t.Fatalf("missing periods: ok=%v reason=%q", ok, reason)
	}
	if ok, reason := backfillSucceeded(nil, stock, true); ok || reason != "fundamentals row missing after refresh" {
		t.Fatalf("nil fund: ok=%v reason=%q", ok, reason)
	}
	failed := &model.StockFundamentals{CalcStatus: model.FundamentalsStatusFailed}
	if ok, reason := backfillSucceeded(failed, stock, true); ok || reason == "" {
		t.Fatalf("failed empty fund should fail: ok=%v reason=%q", ok, reason)
	}
}
