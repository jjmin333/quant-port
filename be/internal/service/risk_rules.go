package service

import (
	"fmt"
	"math"

	"github.com/enact/quant-port/be/internal/model"
)

const (
	concentrationWarn = 0.30
	usLowInfo         = 0.15
	krHighInfo        = 0.85
	drawdownWarn      = -2.0
	drawdownCritical  = -5.0
)

func EvaluateRiskAlerts(summary *model.PortfolioSummary, stockWarnings map[string][]string) []model.RiskAlert {
	if summary == nil || summary.TotalAssetsKRW <= 0 {
		return nil
	}

	total := summary.TotalAssetsKRW
	var alerts []model.RiskAlert

	for _, h := range summary.Holdings {
		weight := h.MarketValueKRW / total
		if weight >= concentrationWarn {
			alerts = append(alerts, model.RiskAlert{
				Code:    "CONCENTRATION",
				Type:    "warning",
				Message: fmt.Sprintf("%s 비중 %.0f%% — 권장 20~30%%", h.Name, weight*100),
				Metrics: map[string]float64{"weight": weight, "symbolWeight": weight},
			})
		}
	}

	var usValue, krValue float64
	for _, h := range summary.Holdings {
		if h.Currency == "USD" || h.AssetType == "us" || h.AssetType == "etf" {
			usValue += h.MarketValueKRW
		} else {
			krValue += h.MarketValueKRW
		}
	}
	usWeight := usValue / total
	krWeight := krValue / total

	if usWeight > 0 && usWeight < usLowInfo && len(summary.Holdings) >= 3 {
		alerts = append(alerts, model.RiskAlert{
			Code:    "FX_EXPOSURE",
			Type:    "info",
			Message: fmt.Sprintf("미국 비중 %.0f%% — 분산 권장", usWeight*100),
			Metrics: map[string]float64{"usWeight": usWeight},
		})
	}
	if krWeight >= krHighInfo && len(summary.Holdings) >= 2 {
		alerts = append(alerts, model.RiskAlert{
			Code:    "COUNTRY",
			Type:    "info",
			Message: fmt.Sprintf("한국 비중 %.0f%% — 해외 분산 검토", krWeight*100),
			Metrics: map[string]float64{"krWeight": krWeight},
		})
	}

	if summary.TodayChangeRate <= drawdownCritical {
		alerts = append(alerts, model.RiskAlert{
			Code:    "DAILY_DRAWDOWN",
			Type:    "critical",
			Message: fmt.Sprintf("당일 손실 %.2f%% — 급격한 하락", summary.TodayChangeRate),
			Metrics: map[string]float64{"todayChangeRate": summary.TodayChangeRate},
		})
	} else if summary.TodayChangeRate <= drawdownWarn {
		alerts = append(alerts, model.RiskAlert{
			Code:    "DAILY_DRAWDOWN",
			Type:    "warning",
			Message: fmt.Sprintf("당일 손실 %.2f%%", summary.TodayChangeRate),
			Metrics: map[string]float64{"todayChangeRate": summary.TodayChangeRate},
		})
	}

	for symbol, warnings := range stockWarnings {
		for _, w := range warnings {
			alerts = append(alerts, model.RiskAlert{
				Code:    "STOCK_WARNING",
				Type:    "warning",
				Message: fmt.Sprintf("%s: %s", symbol, w),
				Metrics: map[string]float64{},
			})
		}
	}

	if len(alerts) == 0 {
		return alerts
	}
	return dedupeRiskAlerts(alerts)
}

func dedupeRiskAlerts(alerts []model.RiskAlert) []model.RiskAlert {
	seen := map[string]struct{}{}
	out := make([]model.RiskAlert, 0, len(alerts))
	for _, a := range alerts {
		key := a.Code + "|" + a.Message
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, a)
	}
	return out
}

func ComputeVolatility(summary *model.PortfolioSummary) string {
	if summary == nil || len(summary.Holdings) == 0 {
		return "낮음"
	}

	high := 0
	for _, h := range summary.Holdings {
		rate := math.Max(math.Abs(h.RegularChangeRate), math.Abs(h.SessionChangeRate))
		if rate >= 3 {
			high++
		}
	}
	ratio := float64(high) / float64(len(summary.Holdings))
	switch {
	case ratio >= 0.4:
		return "높음"
	case ratio >= 0.2:
		return "보통"
	default:
		return "낮음"
	}
}

func ruleBasedBriefing(summary *model.PortfolioSummary, risks []model.RiskAlert, volatility string) model.MorningBriefing {
	up, down := 0, 0
	for _, h := range summary.Holdings {
		if h.SessionChangeRate > 0 {
			up++
		} else if h.SessionChangeRate < 0 {
			down++
		}
	}

	summaryText := fmt.Sprintf("보유종목 %d개 상승, %d개 하락. 총자산 %s원.", up, down, formatKRWShort(summary.TotalAssetsKRW))
	highlights := []string{}
	if summary.TodayChangeRate != 0 {
		highlights = append(highlights, fmt.Sprintf("당일 수익률 %.2f%%", summary.TodayChangeRate))
	}
	if volatility != "" {
		highlights = append(highlights, fmt.Sprintf("예상 변동성 %s", volatility))
	}
	for i, r := range risks {
		if i >= 2 {
			break
		}
		highlights = append(highlights, r.Message)
	}

	return model.MorningBriefing{
		Summary:    summaryText,
		Highlights: highlights,
		Volatility: volatility,
		Source:     "rules",
	}
}

func formatKRWShort(v float64) string {
	if v >= 100_000_000 {
		return fmt.Sprintf("%.1f억", v/100_000_000)
	}
	if v >= 10_000 {
		return fmt.Sprintf("%.0f만", v/10_000)
	}
	return fmt.Sprintf("%.0f", v)
}
