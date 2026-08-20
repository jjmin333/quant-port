package service

import "github.com/enact/quant-port/be/internal/model"

// The 일별 점수 shown beside each close is computed here, on the server, from
// indicators we already calculate — it is never a number the LLM produced.
//
// This mirrors the rule the project already applies to news sentiment (the LLM
// labels articles, the server does the counting) and it matters more here: a
// score the model invents would drift between runs on identical input, which
// makes a dated series meaningless to compare, and it could not be audited
// after the fact. The LLM still writes the prose in ai_summary.
//
// Each component scores 0–100 around a neutral 50. Components without data are
// dropped and the remainder is re-weighted, so a missing indicator never reads
// as a bad signal.

type scoreComponent struct {
	value  float64
	weight float64
}

// stockSummaryScore returns nil when nothing measurable was available, which
// the UI renders as "-".
func stockSummaryScore(view *model.StockSummaryView, ti *TechnicalIndicators) *int {
	components := make([]scoreComponent, 0, 5)

	if ti != nil {
		if ti.RSI14 != nil {
			components = append(components, scoreComponent{value: rsiScore(*ti.RSI14), weight: 0.30})
		}
		if ti.MACD != nil && ti.MACDSignal != nil {
			components = append(components, scoreComponent{value: macdScore(*ti.MACD, *ti.MACDSignal), weight: 0.20})
		}
		if ti.LastClose != nil && ti.SMA20 != nil && ti.SMA50 != nil {
			components = append(components, scoreComponent{
				value:  trendScore(*ti.LastClose, *ti.SMA20, *ti.SMA50),
				weight: 0.25,
			})
		}
		if ti.PctFromLow52Week != nil && ti.PctFromHigh52Week != nil {
			components = append(components, scoreComponent{
				value:  rangePositionScore(*ti.PctFromLow52Week, *ti.PctFromHigh52Week),
				weight: 0.10,
			})
		}
	}

	if view != nil && view.NewsSentiment != nil && view.NewsSentiment.Total > 0 {
		// AverageScore is -100..100 → 0..100.
		components = append(components, scoreComponent{
			value:  clamp01to100((view.NewsSentiment.AverageScore + 100) / 2),
			weight: 0.15,
		})
	}

	if len(components) == 0 {
		return nil
	}

	var weighted, totalWeight float64
	for _, c := range components {
		weighted += c.value * c.weight
		totalWeight += c.weight
	}
	if totalWeight <= 0 {
		return nil
	}

	score := int(clamp01to100(weighted/totalWeight) + 0.5)
	return &score
}

// rsiScore maps RSI to a momentum reading. Deliberately not "high RSI = good":
// above ~70 the move is stretched, so the curve peaks in the strong-but-not-
// overbought band and falls away at both extremes.
func rsiScore(rsi float64) float64 {
	switch {
	case rsi <= 30:
		// Oversold — weak now, but not the worst possible reading.
		return 25 + rsi/30*15 // 25 → 40
	case rsi <= 55:
		return 40 + (rsi-30)/25*35 // 40 → 75
	case rsi <= 70:
		return 75 + (rsi-55)/15*15 // 75 → 90
	default:
		// Overbought — decay back toward neutral as it stretches further.
		return clamp01to100(90 - (rsi-70)/30*45) // 90 → 45
	}
}

// macdScore reads the histogram's sign and size relative to the signal line.
func macdScore(macd, signal float64) float64 {
	diff := macd - signal
	scale := absFloat(signal)
	if scale < 1e-9 {
		scale = absFloat(macd)
	}
	if scale < 1e-9 {
		return 50
	}
	// ±50% of the signal magnitude saturates the component.
	ratio := diff / scale
	return clamp01to100(50 + clampFloat(ratio, -1, 1)*40)
}

// trendScore rewards price above its moving averages and the 20d above the 50d.
func trendScore(last, sma20, sma50 float64) float64 {
	score := 50.0
	if sma20 > 0 {
		score += clampFloat((last-sma20)/sma20*100, -8, 8) * 1.5
	}
	if sma50 > 0 {
		score += clampFloat((last-sma50)/sma50*100, -8, 8) * 1.0
	}
	if sma20 > 0 && sma50 > 0 && sma20 > sma50 {
		score += 6
	}
	return clamp01to100(score)
}

// rangePositionScore places the price inside its 52-week band (0 = at the low,
// 100 = at the high).
func rangePositionScore(pctFromLow, pctFromHigh float64) float64 {
	// pctFromHigh is negative or zero below the high.
	span := pctFromLow + absFloat(pctFromHigh)
	if span <= 0 {
		return 50
	}
	return clamp01to100(pctFromLow / span * 100)
}

func clamp01to100(v float64) float64 { return clampFloat(v, 0, 100) }

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
