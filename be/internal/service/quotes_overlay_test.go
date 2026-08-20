package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestAssetTypeFromMarket_KOSPI(t *testing.T) {
	if got := assetTypeFromMarket("KOSPI", "STOCK"); got != model.AssetDomestic {
		t.Fatalf("KOSPI STOCK = %q, want domestic", got)
	}
}

func TestAssetTypeFromMarket_KRX(t *testing.T) {
	if got := assetTypeFromMarket("KRX", "STOCK"); got != model.AssetDomestic {
		t.Fatalf("KRX STOCK = %q, want domestic", got)
	}
}

func TestAssetTypeFromMarket_NASDAQ(t *testing.T) {
	if got := assetTypeFromMarket("NASDAQ", "STOCK"); got != model.AssetUS {
		t.Fatalf("NASDAQ STOCK = %q, want us", got)
	}
}

func TestIsDomesticKRSymbol(t *testing.T) {
	if !isDomesticKRSymbol("005930") {
		t.Fatal("expected 005930 domestic")
	}
	if isDomesticKRSymbol("AAPL") {
		t.Fatal("expected AAPL not domestic symbol pattern")
	}
}
