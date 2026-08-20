package stocklogo

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

const FMPImageStockBase = "https://financialmodelingprep.com/image-stock"

// IsDomestic reports whether the FMP logo key should use the KRX `.KS` suffix.
func IsDomestic(currency model.QuoteCurrency, assetType model.QuoteAssetType) bool {
	if currency == model.CurrencyKRW {
		return true
	}
	return assetType == model.AssetDomestic
}

// FMPKey returns the symbol segment used in FMP image-stock URLs.
func FMPKey(symbol string, currency model.QuoteCurrency, assetType model.QuoteAssetType) string {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return ""
	}
	if IsDomestic(currency, assetType) {
		return sym + ".KS"
	}
	return sym
}

// FMPImageURL builds the runtime FMP CDN URL for a security (not persisted).
func FMPImageURL(symbol string, currency model.QuoteCurrency, assetType model.QuoteAssetType) string {
	key := FMPKey(symbol, currency, assetType)
	if key == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s.png", FMPImageStockBase, url.PathEscape(key))
}

// ApplyToQuoteSummary sets the computed logoUrl field on a quote row.
func ApplyToQuoteSummary(q *model.QuoteSummary) {
	if q == nil {
		return
	}
	q.LogoURL = LogoURL(q.Symbol, q.Currency, q.AssetType)
}

// ApplyToStock sets the computed logoUrl field on a catalog row.
func ApplyToStock(s *model.Stock) {
	if s == nil {
		return
	}
	s.LogoURL = LogoURL(s.Symbol, s.Currency, s.AssetType)
}

// ApplyToStocks sets logoUrl on each catalog row.
func ApplyToStocks(items []model.Stock) {
	for i := range items {
		ApplyToStock(&items[i])
	}
}
