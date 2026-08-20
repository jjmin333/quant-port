package stocklogo

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestLogoURLWithManifest(t *testing.T) {
	SetManifest(&Manifest{
		Entries: map[string]ManifestEntry{
			"005930": {Region: "domestic", Source: "naver", Path: "/logos/kr/005930.png"},
			"AAPL":   {Region: "us", Source: "fmp"},
		},
	})
	t.Cleanup(func() { SetManifest(nil) })

	got := LogoURL("005930", model.CurrencyKRW, model.AssetDomestic)
	want := "/logos/kr/005930.png"
	if got != want {
		t.Fatalf("LogoURL domestic = %q, want %q", got, want)
	}

	got = LogoURL("069500", model.CurrencyKRW, model.AssetETF)
	if got != FMPImageURL("069500", model.CurrencyKRW, model.AssetETF) {
		t.Fatalf("LogoURL ETF fallback should use FMP, got %q", got)
	}

	got = LogoURL("AAPL", model.CurrencyUSD, model.AssetUS)
	if got != FMPImageURL("AAPL", model.CurrencyUSD, model.AssetUS) {
		t.Fatalf("LogoURL US should use FMP, got %q", got)
	}
}

func TestFMPFallbackURL(t *testing.T) {
	SetManifest(&Manifest{
		Entries: map[string]ManifestEntry{
			"005930": {Region: "domestic", Source: "naver", Path: "/logos/kr/005930.png"},
		},
	})
	t.Cleanup(func() { SetManifest(nil) })

	fallback := FMPFallbackURL("005930", model.CurrencyKRW, model.AssetDomestic)
	if fallback == "" || fallback == "/logos/kr/005930.png" {
		t.Fatalf("expected FMP fallback URL, got %q", fallback)
	}
}
