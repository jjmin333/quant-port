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

/*
HasUSLogo reports whether an overseas symbol is already done.

「이미 됐다」의 뜻이 모드에 따라 다르다. 원격 URL만 쓰는 모드에서는 **FMP에 그림이
있다고 확인된 것**이면 충분하지만, 내려받아 저장하는 모드에서는 **파일이 실제로 있어야**
한다 — 확인만 해 둔 종목까지 건너뛰면 저장 모드로 처음 도는 실행이 아무것도 받지 않고
끝난다(매니페스트에는 이미 11,204종목이 「확인됨」으로 적혀 있다).
*/
func HasUSLogo(m *Manifest, usDir, symbol string, storeUS bool) bool {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" || m == nil {
		return false
	}
	entry, ok := m.Entries[sym]
	if !ok || entry.Region != "us" || entry.Source != "fmp" || entry.Reason != "" {
		return false
	}
	if !storeUS {
		return true
	}
	if entry.Path == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(usDir, sym+".png"))
	return err == nil
}

// FilterDeltaLogoStocks returns rows that still need logo migration when DeltaOnly is set.
func FilterDeltaLogoStocks(
	stocks []model.Stock, m *Manifest, krDir, usDir string, deltaOnly, storeUS bool,
) []model.Stock {
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
		} else if HasUSLogo(m, usDir, sym, storeUS) {
			continue
		}
		out = append(out, st)
	}
	return out
}
