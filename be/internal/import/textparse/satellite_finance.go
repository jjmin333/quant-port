package textparse

import (
	"math"
	"strings"
)

// AttachSatelliteFinance copies evaluation/PnL from fragment rows (e.g. OCR "mete!")
// onto named holdings with the same quantity, then by list order for finance-only rows.
func AttachSatelliteFinance(named, all []ParsedLine) []ParsedLine {
	if len(named) == 0 || len(all) == 0 {
		return named
	}
	out := make([]ParsedLine, len(named))
	copy(out, named)
	for i := range out {
		for _, sat := range all {
			if sat.Quantity <= 0 || sat.Quantity != out[i].Quantity {
				continue
			}
			if !lineHasFinance(sat) {
				continue
			}
			if sat.Name == out[i].Name || isSatelliteFinanceDonor(sat) {
				out[i] = mergeFinanceInto(out[i], sat)
			}
		}
	}
	return attachOrderedFinance(out, all)
}

func attachOrderedFinance(named, all []ParsedLine) []ParsedLine {
	var financeOnly []ParsedLine
	for _, sat := range all {
		if !lineHasFinance(sat) {
			continue
		}
		if sat.Quantity > 0 && sat.Name != "" && !isSatelliteFinanceDonor(sat) {
			continue
		}
		if sat.Quantity > 0 && sat.Name == "" {
			continue
		}
		financeOnly = append(financeOnly, sat)
	}
	if len(financeOnly) == 0 {
		return named
	}
	for i := range named {
		if lineHasFinance(named[i]) || named[i].Quantity <= 0 {
			continue
		}
		for _, candidate := range financeOnly {
			if isPortfolioSummaryFinance(candidate) {
				continue
			}
			if !financeMatchesHolding(named[i], candidate) {
				continue
			}
			named[i] = mergeFinanceInto(named[i], candidate)
			break
		}
	}
	for i := range named {
		if named[i].ProfitLoss != nil || named[i].MarketValue == nil || named[i].Quantity <= 0 {
			continue
		}
		for _, candidate := range financeOnly {
			if candidate.ProfitLoss == nil || isPortfolioSummaryFinance(candidate) {
				continue
			}
			if !financeMatchesHolding(named[i], candidate) {
				continue
			}
			named[i] = mergeFinanceInto(named[i], candidate)
			break
		}
	}
	return named
}

func financeMatchesHolding(holding, fin ParsedLine) bool {
	if fin.MarketValue == nil || holding.Quantity <= 0 {
		return false
	}
	if isPortfolioScaleFinanceLeak(holding, fin) {
		return false
	}
	perShare := *fin.MarketValue / holding.Quantity
	cur := inferLineCurrency(fin)
	switch cur {
	case "KRW":
		return perShare >= 1_000 && perShare <= 5_000_000
	default:
		return perShare >= 0.5 && perShare <= 20_000
	}
}

func inferLineCurrency(line ParsedLine) string {
	if line.Currency != "" {
		return line.Currency
	}
	if line.MarketValue != nil {
		if *line.MarketValue >= 100_000 {
			return "KRW"
		}
		return "USD"
	}
	return ""
}

func isPortfolioScaleFinanceLeak(holding, fin ParsedLine) bool {
	if fin.ProfitLoss == nil {
		return false
	}
	pnl := math.Abs(*fin.ProfitLoss)
	cur := inferLineCurrency(fin)
	if cur == "KRW" && pnl >= 1_000_000 && holding.Quantity < 500 {
		return true
	}
	if fin.MarketValue != nil && pnl > 5*(*fin.MarketValue) {
		return true
	}
	return false
}

func isPortfolioSummaryFinance(line ParsedLine) bool {
	name := NormalizeDisplayName(line.Name)
	if strings.Contains(name, "가나다순") || strings.Contains(name, "가격순") || strings.Contains(name, "수량순") {
		return true
	}
	if line.ProfitLoss != nil && math.Abs(*line.ProfitLoss) >= 5_000_000 {
		return true
	}
	return false
}

func isSatelliteFinanceDonor(line ParsedLine) bool {
	name := NormalizeDisplayName(line.Name)
	if name == "" {
		return lineHasFinance(line)
	}
	if LooksLikeOCRFragment(name) {
		return true
	}
	if latinLetterRunes(name) > 0 && hangulRunes(name) == 0 && len([]rune(name)) <= 6 {
		return true
	}
	return !IsValidParsedLine(name, line.Quantity)
}

func lineHasFinance(line ParsedLine) bool {
	return line.MarketValue != nil || line.ProfitLoss != nil || line.ReturnRate != nil
}

func mergeFinanceInto(dst, src ParsedLine) ParsedLine {
	out := pickRicherFinance(dst, src)
	out.Name = dst.Name
	out.Quantity = dst.Quantity
	if out.Currency == "" {
		out.Currency = src.Currency
	}
	if out.MarketValue != nil && *out.MarketValue >= 100_000 && out.Currency == "USD" {
		out.Currency = "KRW"
	}
	return out
}
