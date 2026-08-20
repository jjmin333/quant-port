package service

import (
	"fmt"
	"sort"

	"github.com/enact/quant-port/be/internal/model"
)

// Rule-based 시장 분석, generated from data alone.
//
// Two things it must never do: state a figure it did not receive, and name a
// price level it did not derive from real history. Every level below is an
// actual moving average, swing extreme or 52-week bound — there are no invented
// targets, because a made-up target is exactly what makes a briefing
// untrustworthy.

// maxBriefingPoints keeps the block scannable. Beyond four points a reader
// stops ranking them, which defeats the purpose of ordering by importance.
const maxBriefingPoints = 4

// buildMarketBriefing assembles the deterministic briefing. Returns nil when
// there is not even a price to anchor on.
func buildMarketBriefing(
	stock *model.Stock,
	fundamentals *model.FundamentalsView,
	history []model.FinancialHistoryPeriod,
	insiderTxns []model.InsiderTransaction,
	technical *TechnicalIndicators,
	score *model.StockScoreView,
) *model.MarketBriefing {
	if technical == nil || technical.LastClose == nil || *technical.LastClose <= 0 {
		return nil
	}
	price := *technical.LastClose

	out := &model.MarketBriefing{
		Price:    price,
		Currency: string(stock.Currency),
	}
	if score != nil {
		out.AsOf = score.TradeDate
	}

	out.KeyPoints = briefingKeyPoints(fundamentals, history, insiderTxns, technical, score)
	out.Upside, out.Downside = briefingScenarios(price, technical)
	return out
}

// briefingKeyPoints ranks what matters. Order is deliberate: the score's own
// extremes first (they already weigh every axis against peers), then the
// standout raw signals that a reader would otherwise have to dig for.
func briefingKeyPoints(
	fundamentals *model.FundamentalsView,
	history []model.FinancialHistoryPeriod,
	insiderTxns []model.InsiderTransaction,
	technical *TechnicalIndicators,
	score *model.StockScoreView,
) []model.BriefingPoint {
	var points []model.BriefingPoint

	// 1. Where the stock stands overall, in peer terms.
	if score != nil && score.Score != nil {
		text := fmt.Sprintf("종합 %d점(%s)", *score.Score, score.Grade)
		if score.PeerPct != nil {
			text += fmt.Sprintf(" · 동종 상위 %d%%", *score.PeerPct)
		}
		tone := model.ToneNeutral
		if *score.Score >= 65 {
			tone = model.TonePositive
		} else if *score.Score <= 40 {
			tone = model.ToneNegative
		}
		points = append(points, model.BriefingPoint{Label: "종합", Text: text, Tone: tone})
	}

	// 2. Profitability, quoted with the figures that drive it.
	if fundamentals != nil {
		var quality []string
		if fundamentals.ROE != nil && *fundamentals.ROE > 0 {
			quality = append(quality, fmt.Sprintf("ROE %.1f%%", *fundamentals.ROE))
		}
		if fundamentals.OperatingMargin != nil {
			quality = append(quality, fmt.Sprintf("영업이익률 %.1f%%", *fundamentals.OperatingMargin))
		}
		if len(quality) > 0 {
			tone := model.ToneNeutral
			if score != nil && score.Breakdown.Quality != nil {
				if *score.Breakdown.Quality >= 70 {
					tone = model.TonePositive
				} else if *score.Breakdown.Quality <= 30 {
					tone = model.ToneNegative
				}
			}
			points = append(points, model.BriefingPoint{
				Label: "수익성", Text: joinKorean(quality), Tone: tone,
			})
		}

		// 3. Valuation, judged by peer rank rather than an absolute cutoff —
		// the same reason the score itself ranks instead of thresholding.
		var valuation []string
		if fundamentals.PER != nil && *fundamentals.PER > 0 {
			valuation = append(valuation, fmt.Sprintf("PER %.1f배", *fundamentals.PER))
		}
		if fundamentals.PBR != nil && *fundamentals.PBR > 0 {
			valuation = append(valuation, fmt.Sprintf("PBR %.1f배", *fundamentals.PBR))
		}
		if len(valuation) > 0 {
			text := joinKorean(valuation)
			tone := model.ToneNeutral
			if score != nil && score.Breakdown.Valuation != nil {
				switch {
				case *score.Breakdown.Valuation >= 70:
					tone = model.TonePositive
					text += " — 동종 대비 저평가 구간"
				case *score.Breakdown.Valuation <= 30:
					tone = model.ToneNegative
					text += " — 동종 대비 고평가 구간"
				}
			}
			points = append(points, model.BriefingPoint{Label: "밸류에이션", Text: text, Tone: tone})
		}
	}

	// 4. Earnings direction, from the latest quarter's real deltas.
	if len(history) > 0 {
		latest := history[len(history)-1]
		trend := classifyEarningsTrend(latest.RevenueGrowthQoQ, latest.OperatingIncomeGrowthQoQ, latest.NetIncomeGrowthQoQ)
		if trend != "" {
			var parts []string
			if latest.RevenueGrowthQoQ != nil {
				parts = append(parts, fmt.Sprintf("매출 %s", signedPercent(*latest.RevenueGrowthQoQ)))
			}
			if latest.OperatingIncomeGrowthQoQ != nil {
				parts = append(parts, fmt.Sprintf("영업이익 %s", signedPercent(*latest.OperatingIncomeGrowthQoQ)))
			}
			if len(parts) > 0 {
				tone := model.ToneNeutral
				switch trend {
				case "strong_improving", "improving":
					tone = model.TonePositive
				case "deteriorating":
					tone = model.ToneNegative
				}
				points = append(points, model.BriefingPoint{
					Label: "실적", Text: "직전 분기 대비 " + joinKorean(parts), Tone: tone,
				})
			}
		}
	}

	// 5. Price position — the drawdown a reader feels most directly.
	if technical.PctFromHigh52Week != nil && *technical.PctFromHigh52Week < -15 {
		points = append(points, model.BriefingPoint{
			Label: "주가",
			Text:  fmt.Sprintf("52주 최고가 대비 %s", signedPercent(*technical.PctFromHigh52Week)),
			Tone:  model.ToneNegative,
		})
	}

	// 6. Insider direction, only when it is decisive enough to mean something.
	if len(insiderTxns) > 0 {
		buy, sell := 0, 0
		for _, tx := range insiderTxns {
			if tx.SharesChange == nil {
				continue
			}
			if *tx.SharesChange > 0 {
				buy++
			} else if *tx.SharesChange < 0 {
				sell++
			}
		}
		switch insiderDirection(buy, sell) {
		case "net_buying":
			points = append(points, model.BriefingPoint{
				Label: "내부자", Text: fmt.Sprintf("최근 매수 %d건 / 매도 %d건 — 순매수 우위", buy, sell),
				Tone: model.TonePositive,
			})
		case "net_selling":
			points = append(points, model.BriefingPoint{
				Label: "내부자", Text: fmt.Sprintf("최근 매수 %d건 / 매도 %d건 — 순매도 우위", buy, sell),
				Tone: model.ToneNegative,
			})
		}
	}

	if len(points) > maxBriefingPoints {
		points = points[:maxBriefingPoints]
	}
	return points
}

// briefingScenarios turns the indicator set into the levels that would confirm
// a move in either direction.
func briefingScenarios(price float64, t *TechnicalIndicators) (up, down *model.BriefingScenario) {
	above, below := candidateLevels(price, t)

	if len(above) > 0 {
		up = &model.BriefingScenario{
			Summary: upsideSummary(t),
			Levels:  above,
		}
	}
	if len(below) > 0 {
		down = &model.BriefingScenario{
			Summary: downsideSummary(t),
			Levels:  below,
		}
	}
	return up, down
}

// maxScenarioLevels — the nearest few levels are the ones that matter; a longer
// ladder is noise at this distance.
const maxScenarioLevels = 3

// candidateLevels collects every real reference price and splits them around
// the current price, nearest first.
func candidateLevels(price float64, t *TechnicalIndicators) (above, below []model.PriceLevel) {
	type candidate struct {
		label string
		value *float64
	}
	candidates := []candidate{
		{"20일선", t.SMA20},
		{"50일선", t.SMA50},
		{"200일선", t.SMA200},
		{"최근 스윙 고점", t.RecentSwingHigh},
		{"최근 스윙 저점", t.RecentSwingLow},
		{"52주 최고", t.High52Week},
		{"52주 최저", t.Low52Week},
	}

	seen := map[string]bool{}
	for _, c := range candidates {
		if c.value == nil || *c.value <= 0 {
			continue
		}
		v := *c.value
		// A level within 0.2% of the price is not actionable, and two levels
		// that round to the same price would read as duplicates.
		key := fmt.Sprintf("%.2f", v)
		if seen[key] {
			continue
		}
		seen[key] = true

		dist := (v - price) / price * 100
		if dist > 0.2 {
			above = append(above, model.PriceLevel{Label: c.label, Price: round2f(v), DistancePct: round2f(dist)})
		} else if dist < -0.2 {
			below = append(below, model.PriceLevel{Label: c.label, Price: round2f(v), DistancePct: round2f(dist)})
		}
	}

	sort.Slice(above, func(i, j int) bool { return above[i].DistancePct < above[j].DistancePct })
	sort.Slice(below, func(i, j int) bool { return below[i].DistancePct > below[j].DistancePct })

	if len(above) > maxScenarioLevels {
		above = above[:maxScenarioLevels]
	}
	if len(below) > maxScenarioLevels {
		below = below[:maxScenarioLevels]
	}
	return above, below
}

func upsideSummary(t *TechnicalIndicators) string {
	if t.RSI14 != nil && *t.RSI14 <= 35 {
		return "과매도 구간에서의 반등 시 아래 저항선 회복 여부가 관건입니다."
	}
	if t.MACD != nil && t.MACDSignal != nil && *t.MACD > *t.MACDSignal {
		return "MACD가 시그널선을 상회하는 국면으로, 아래 저항선 돌파 여부가 추세 전환의 기준이 됩니다."
	}
	return "상승 시 아래 가격대가 차례로 저항으로 작용할 수 있습니다."
}

func downsideSummary(t *TechnicalIndicators) string {
	if t.RSI14 != nil && *t.RSI14 >= 70 {
		return "과매수 구간이라 조정 시 아래 지지선까지 되돌림이 나올 수 있습니다."
	}
	if t.LastClose != nil && t.SMA20 != nil && *t.LastClose < *t.SMA20 {
		return "단기 이동평균 아래에 있어 하락 지속 시 아래 지지선이 1차 방어선입니다."
	}
	return "하락 시 아래 가격대가 차례로 지지선으로 작용할 수 있습니다."
}

func round2f(v float64) float64 {
	return float64(int64(v*100+sign(v)*0.5)) / 100
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// joinKorean renders a short list as "a, b" — plain enough that the figures
// stay the focus.
func joinKorean(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
