package model

import "testing"

func TestComputeCoverageTier(t *testing.T) {
	rev := int64(1000)
	ni := int64(100)
	eq := int64(500)
	roe := 10.0
	etf := Stock{AssetType: AssetETF, SecurityType: "ETF"}

	cases := []struct {
		name  string
		f     StockFundamentals
		stock *Stock
		want  CoverageTier
	}{
		{
			name:  "etf is not applicable",
			f:     StockFundamentals{CalcStatus: FundamentalsStatusOK, Revenue: &rev},
			stock: &etf,
			want:  CoverageTierNotApplicable,
		},
		{
			name:  "ok is full",
			f:     StockFundamentals{CalcStatus: FundamentalsStatusOK},
			want:  CoverageTierFull,
		},
		{
			name:  "partial with revenue is basic",
			f:     StockFundamentals{CalcStatus: FundamentalsStatusPartial, Revenue: &rev},
			want:  CoverageTierBasic,
		},
		{
			name:  "failed empty is unavailable",
			f:     StockFundamentals{CalcStatus: FundamentalsStatusFailed},
			want:  CoverageTierUnavailable,
		},
		{
			name:  "failed with net income is basic",
			f:     StockFundamentals{CalcStatus: FundamentalsStatusFailed, NetIncome: &ni, TotalEquity: &eq},
			want:  CoverageTierBasic,
		},
		{
			name:  "partial with roe only is basic",
			f:     StockFundamentals{CalcStatus: FundamentalsStatusPartial, ROE: &roe},
			want:  CoverageTierBasic,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ComputeCoverageTier(tc.f, tc.stock); got != tc.want {
				t.Fatalf("ComputeCoverageTier() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsFundamentalsApplicable(t *testing.T) {
	if IsFundamentalsApplicable(Stock{AssetType: AssetETF}) {
		t.Fatal("etf should not be applicable")
	}
	if !IsFundamentalsApplicable(Stock{AssetType: AssetUS, SecurityType: "STOCK"}) {
		t.Fatal("common stock should be applicable")
	}
}
