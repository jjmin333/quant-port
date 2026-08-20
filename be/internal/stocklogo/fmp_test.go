package stocklogo

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestFMPImageURL(t *testing.T) {
	tests := []struct {
		symbol    string
		currency  model.QuoteCurrency
		assetType model.QuoteAssetType
		want      string
	}{
		{"SNDK", model.CurrencyUSD, model.AssetUS, "https://financialmodelingprep.com/image-stock/SNDK.png"},
		{"005930", model.CurrencyKRW, model.AssetDomestic, "https://financialmodelingprep.com/image-stock/005930.KS.png"},
		{"TQQQ", model.CurrencyUSD, model.AssetETF, "https://financialmodelingprep.com/image-stock/TQQQ.png"},
		{"069500", model.CurrencyKRW, model.AssetETF, "https://financialmodelingprep.com/image-stock/069500.KS.png"},
		{"BRK.B", model.CurrencyUSD, model.AssetUS, "https://financialmodelingprep.com/image-stock/BRK.B.png"},
	}
	for _, tc := range tests {
		got := FMPImageURL(tc.symbol, tc.currency, tc.assetType)
		if got != tc.want {
			t.Errorf("FMPImageURL(%q) = %q, want %q", tc.symbol, got, tc.want)
		}
	}
}
