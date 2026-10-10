package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

// 20-F(현지 통화) 재무를 미국 ADR 가격과 나누면 SONY PER 0.025가 나왔다(2026-09-28).
func TestDropForeignCurrencyAbsolutes(t *testing.T) {
	i64 := func(v int64) *int64 { return &v }
	f64 := func(v float64) *float64 { return &v }
	fresh := func() model.StockFundamentals {
		return model.StockFundamentals{
			Revenue: i64(2_894_307_700_000), NetIncome: i64(1_157_523_900_000),
			TotalEquity: i64(4_000_000_000_000), EPSBasic: f64(44.68), EPSDiluted: f64(44.68),
			ROE: f64(28.9), OperatingMargin: f64(45.7),
		}
	}

	row := fresh()
	if !dropForeignCurrencyAbsolutes(&row, "twd", "USD") {
		t.Fatal("TWD filing on a USD listing must be dropped")
	}
	if row.Revenue != nil || row.NetIncome != nil || row.TotalEquity != nil || row.EPSBasic != nil || row.EPSDiluted != nil {
		t.Fatalf("absolutes must be cleared: %+v", row)
	}
	if row.ROE == nil || row.OperatingMargin == nil {
		t.Fatal("currency-neutral ratios must survive")
	}

	// TSM: 매출은 달러 환산치, EPS는 대만 달러 — 하나라도 다르면 버린다.
	mixed := fresh()
	if !dropForeignCurrencyAbsolutes(&mixed, "TWD,USD", "USD") || mixed.EPSBasic != nil || mixed.ROE != nil {
		t.Fatal("mixed currencies must drop absolutes and ratios")
	}

	for _, tc := range []struct{ filing, listing string }{{"USD", "USD"}, {"", "USD"}, {"KRW", "KRW"}} {
		row := fresh()
		if dropForeignCurrencyAbsolutes(&row, tc.filing, tc.listing) || row.EPSBasic == nil {
			t.Fatalf("%q on %q must be kept", tc.filing, tc.listing)
		}
	}
}
