package costestimate

import (
	"math"

	"github.com/enact/quant-port/be/internal/import/textparse"
)

const crossCheckTolerance = 0.02

// Result holds an optional average purchase price derived from OCR financials.
type Result struct {
	AvgPrice   *float64
	Currency   string
	Confidence float64
	Method     string
}

// Estimate derives avg price from parsed OCR row. Returns empty Result when insufficient data.
func Estimate(line textparse.ParsedLine) Result {
	if line.Quantity <= 0 {
		return Result{}
	}
	line = reconcileParsedFinance(line)
	currency := inferCurrency(line)

	var fromEvalPnL *float64
	var fromEvalReturn *float64

	// Ignore tiny P&L lines (often daily change, not total return since purchase).
	if line.MarketValue != nil && line.ProfitLoss != nil && pnlIsMaterial(*line.MarketValue, *line.ProfitLoss, line.ReturnRate, currency) {
		cost := *line.MarketValue - *line.ProfitLoss
		if cost > 0 {
			avg := cost / line.Quantity
			if avg > 0 && !math.IsNaN(avg) && !math.IsInf(avg, 0) {
				fromEvalPnL = &avg
			}
		}
	}

	if line.MarketValue != nil && line.ReturnRate != nil && math.Abs(*line.ReturnRate) >= 0.0001 {
		cur := *line.MarketValue / line.Quantity
		denom := 1 + *line.ReturnRate
		if cur > 0 && denom != 0 {
			avg := cur / denom
			if avg > 0 && !math.IsNaN(avg) && !math.IsInf(avg, 0) {
				fromEvalReturn = &avg
			}
		}
	}

	switch {
	case fromEvalPnL != nil && fromEvalReturn != nil:
		diff := math.Abs(*fromEvalPnL-*fromEvalReturn) / *fromEvalPnL
		if diff <= crossCheckTolerance {
			return finalizeEstimate(fromEvalPnL, currency, 0.95, "eval_pnl_return")
		}
		return finalizeEstimate(fromEvalPnL, currency, 0.85, "eval_pnl")
	case fromEvalPnL != nil:
		return finalizeEstimate(fromEvalPnL, currency, 0.9, "eval_pnl")
	case fromEvalReturn != nil:
		return finalizeEstimate(fromEvalReturn, currency, 0.8, "eval_return")
	default:
		return Result{}
	}
}

func finalizeEstimate(avg *float64, currency string, confidence float64, method string) Result {
	if avg == nil {
		return Result{}
	}
	rounded := roundAvgPrice(*avg, currency)
	return Result{
		AvgPrice:   &rounded,
		Currency:   currency,
		Confidence: confidence,
		Method:     method,
	}
}

func roundAvgPrice(avg float64, currency string) float64 {
	if currency == "USD" {
		return math.Round(avg*100) / 100
	}
	// Domestic KRW avg prices align to exchange tick (5원 from 1,000원 tier upward).
	if avg >= 1000 {
		return math.Round(avg/5) * 5
	}
	return math.Round(avg)
}

func inferCurrency(line textparse.ParsedLine) string {
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

func pnlIsMaterial(eval, pnl float64, returnRate *float64, currency string) bool {
	if eval <= 0 {
		return false
	}
	if pnl != 0 {
		if currency == "USD" || eval < 100_000 {
			if math.Abs(pnl) >= 0.01 {
				if returnRate == nil || (*returnRate >= 0) == (pnl >= 0) {
					return true
				}
			}
		}
	}
	if returnRate != nil && math.Abs(*returnRate) >= 0.0001 {
		if pnl == 0 || (*returnRate >= 0) == (pnl >= 0) {
			return true
		}
	}
	if pnl != 0 && math.Abs(pnl)/eval >= 0.0001 {
		return true
	}
	return false
}

// reconcileParsedFinance fixes common OCR sign/noise issues before avg-price math.
func reconcileParsedFinance(line textparse.ParsedLine) textparse.ParsedLine {
	if line.MarketValue == nil || *line.MarketValue <= 0 {
		return line
	}
	if line.Currency == "" && *line.MarketValue >= 100_000 {
		line.Currency = "KRW"
	} else if line.Currency == "USD" && *line.MarketValue >= 100_000 {
		line.Currency = "KRW"
	}
	eval := *line.MarketValue

	// KRW OCR often drops the minus on loss percentages when PnL amount is missing.
	if line.Currency == "KRW" && line.ProfitLoss == nil && line.ReturnRate != nil && *line.ReturnRate > 0 && line.Quantity > 0 {
		neg := -*line.ReturnRate
		line.ReturnRate = &neg
	}

	if line.ProfitLoss != nil && line.ReturnRate != nil {
		pnl := *line.ProfitLoss
		rate := *line.ReturnRate

		// Large return % with tiny PnL amount is almost always OCR column bleed (e.g. 45% vs -29k on 36M eval).
		if math.Abs(rate) > 0.05 && math.Abs(pnl)/eval < 0.02 {
			line.ReturnRate = nil
		} else {
			pnlFlipped := false

			if pnl < 0 && rate > 0 {
				corrected := -rate
				line.ReturnRate = &corrected
				rate = corrected
			} else if pnl > 0 && rate < 0 {
				corrected := -rate
				line.ReturnRate = &corrected
				rate = corrected
			}

			// OCR often drops the minus on loss lines (e.g. "89,9908! (4.63%)" for a loss).
			if pnl > 0 && rate > 0 {
				impliedLoss := eval * (-rate) / (1 - rate)
				if math.Abs(pnl+impliedLoss)/eval < 0.001 {
					flipped := -pnl
					line.ProfitLoss = &flipped
					pnl = flipped
					pnlFlipped = true
				}
			}

			rateFlipped := false
			if pnl*rate < 0 && math.Abs(rate) >= 0.01 {
				corrected := -math.Abs(rate)
				if pnl > 0 {
					corrected = math.Abs(rate)
				}
				line.ReturnRate = &corrected
				rate = corrected
				rateFlipped = true
			}

			// Return % lost its minus but PnL sign is known (e.g. NAVER "-892,2218! (18.06%)").
			if rateFlipped && !pnlFlipped {
				implied := eval * rate / (1 + rate)
				if line.ProfitLoss == nil {
					line.ProfitLoss = &implied
				} else if math.Abs(*line.ProfitLoss-implied)/eval > 0.005 {
					line.ProfitLoss = &implied
				}
			}
		}
	}

	if line.ProfitLoss != nil && line.ReturnRate != nil && math.Abs(*line.ReturnRate) >= 0.0001 {
		implied := eval * (*line.ReturnRate) / (1 + *line.ReturnRate)
		existing := *line.ProfitLoss
		if math.Abs(existing-implied)/eval > 0.015 {
			ratio := math.Abs(implied) / math.Max(math.Abs(existing), 1)
			switch {
			case ratio > 20:
				// Parsed PnL is tiny vs return-implied (garbage return % from column bleed).
			case ratio < 0.05 && math.Abs(*line.ReturnRate) <= 0.5:
				line.ProfitLoss = &implied
			case ratio >= 0.05 && ratio <= 20:
				line.ProfitLoss = &implied
			}
		}
	}

	if line.MarketValue != nil && line.ProfitLoss != nil && *line.MarketValue-*line.ProfitLoss <= 0 {
		line.ProfitLoss = nil
	}

	return line
}
