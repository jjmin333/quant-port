package stocklogo

import (
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

// LogoURL returns the preferred logo URL: migrated local path (domestic) or FMP CDN.
func LogoURL(symbol string, currency model.QuoteCurrency, assetType model.QuoteAssetType) string {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return ""
	}

	m := GetManifest()
	if m != nil {
		if entry, ok := m.Entries[sym]; ok {
			if entry.Source == "naver" && entry.Path != "" {
				return entry.Path
			}
		}
	}
	return FMPImageURL(symbol, currency, assetType)
}

// FMPFallbackURL returns the FMP CDN URL for img onError fallback (empty when primary is already FMP).
func FMPFallbackURL(symbol string, currency model.QuoteCurrency, assetType model.QuoteAssetType) string {
	primary := LogoURL(symbol, currency, assetType)
	fmp := FMPImageURL(symbol, currency, assetType)
	if primary != "" && primary != fmp {
		return fmp
	}
	return ""
}
