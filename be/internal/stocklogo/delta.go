package stocklogo

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

// HasDomesticLogo reports whether manifest and local PNG exist for a domestic symbol.
func HasDomesticLogo(m *Manifest, krDir, symbol string) bool {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return false
	}
	if m != nil {
		if entry, ok := m.Entries[sym]; ok && entry.Source == "naver" && entry.Path != "" {
			if _, err := os.Stat(filepath.Join(krDir, sym+".png")); err == nil {
				return true
			}
		}
	}
	return false
}

// HasUSFMPLogo reports whether manifest records a successful FMP probe.
func HasUSFMPLogo(m *Manifest, symbol string) bool {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" || m == nil {
		return false
	}
	entry, ok := m.Entries[sym]
	if !ok {
		return false
	}
	return entry.Region == "us" && entry.Source == "fmp" && entry.Reason == ""
}

// FilterDeltaLogoStocks returns rows that still need logo migration when DeltaOnly is set.
func FilterDeltaLogoStocks(stocks []model.Stock, m *Manifest, krDir string, deltaOnly bool) []model.Stock {
	if !deltaOnly {
		return stocks
	}
	out := make([]model.Stock, 0, len(stocks))
	for _, st := range stocks {
		sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
		if sym == "" {
			continue
		}
		if IsDomestic(st.Currency, st.AssetType) {
			if HasDomesticLogo(m, krDir, sym) {
				continue
			}
		} else if HasUSFMPLogo(m, sym) {
			continue
		}
		out = append(out, st)
	}
	return out
}
