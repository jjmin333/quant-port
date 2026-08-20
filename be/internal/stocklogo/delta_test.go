package stocklogo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestFilterDeltaLogoStocks(t *testing.T) {
	dir := t.TempDir()
	krDir := filepath.Join(dir, "kr")
	if err := os.MkdirAll(krDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(krDir, "0193T0.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &Manifest{
		Entries: map[string]ManifestEntry{
			"0193T0": {Region: "domestic", Source: "naver", Path: "/logos/kr/0193T0.png"},
			"TQQQ":   {Region: "us", Source: "fmp"},
			"FAIL":   {Region: "us", Source: "fmp", Reason: "fmp 404"},
		},
	}

	stocks := []model.Stock{
		{Symbol: "0193T0", Currency: model.CurrencyKRW, AssetType: model.AssetETF},
		{Symbol: "114800", Currency: model.CurrencyKRW, AssetType: model.AssetETF},
		{Symbol: "TQQQ", Currency: model.CurrencyUSD, AssetType: model.AssetETF},
		{Symbol: "FAIL", Currency: model.CurrencyUSD, AssetType: model.AssetETF},
	}

	out := FilterDeltaLogoStocks(stocks, m, krDir, true)
	if len(out) != 2 {
		t.Fatalf("want 2 delta rows, got %d", len(out))
	}
	if out[0].Symbol != "114800" || out[1].Symbol != "FAIL" {
		t.Fatalf("unexpected symbols: %+v", out)
	}

	all := FilterDeltaLogoStocks(stocks, m, krDir, false)
	if len(all) != len(stocks) {
		t.Fatalf("deltaOnly=false should keep all rows")
	}
}
