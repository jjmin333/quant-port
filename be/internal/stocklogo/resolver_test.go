package stocklogo

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestLogoURLWithManifest(t *testing.T) {
	SetManifest(&Manifest{
		Entries: map[string]ManifestEntry{
			"005930": {Region: "domestic", Source: "naver", Path: "/logos/kr/005930.png"},
			// 저장해 둔 해외 로고. source는 받아 온 곳이라 저장 후에도 "fmp"다 —
			// 이 항목이 원격 URL로 떨어지면 US 로고 11,205개가 통째로 죽는다.
			"AAPL": {Region: "us", Source: "fmp", Path: "/logos/us/AAPL.png"},
			// 아직 안 받아 둔 해외 종목 → 원격 URL.
			"NVDA": {Region: "us", Source: "fmp"},
			// 흰 불투명 이미지 → 쓸 수 있는 로고가 없다.
			"SOXS": {Region: "us", Source: "fmp", Path: "/logos/us/SOXS.png", Tone: "blank"},
		},
	})
	t.Cleanup(func() { SetManifest(nil) })

	cases := []struct {
		symbol    string
		currency  model.QuoteCurrency
		assetType model.QuoteAssetType
		want      string
	}{
		{"005930", model.CurrencyKRW, model.AssetDomestic, "/logos/kr/005930.png"},
		{"AAPL", model.CurrencyUSD, model.AssetUS, "/logos/us/AAPL.png"},
		{"aapl", model.CurrencyUSD, model.AssetUS, "/logos/us/AAPL.png"},
		{"NVDA", model.CurrencyUSD, model.AssetUS, FMPImageURL("NVDA", model.CurrencyUSD, model.AssetUS)},
		{"SOXS", model.CurrencyUSD, model.AssetUS, ""},
		// 매니페스트에 아예 없는 종목은 원격 URL이다 — 국내는 네이버(FMP .KS는 신규 상장이 404).
		{"069500", model.CurrencyKRW, model.AssetETF, NaverLogoURL("069500")},
		{"CBOE", model.CurrencyUSD, model.AssetUS, FMPImageURL("CBOE", model.CurrencyUSD, model.AssetUS)},
		{"", model.CurrencyUSD, model.AssetUS, ""},
	}
	for _, tc := range cases {
		if got := LogoURL(tc.symbol, tc.currency, tc.assetType); got != tc.want {
			t.Errorf("LogoURL(%q) = %q, want %q", tc.symbol, got, tc.want)
		}
	}
}
