package service

import (
	"fmt"
	"strings"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/model"
)

// ruleBasedStockSummary builds a plain, numbers-only fallback for every
// section using only values actually computed/fetched — no invented figures.
// It is always computed first; the LLM (if enabled) overlays richer prose on
// top, and this is what gets returned untouched if the LLM call fails.
func ruleBasedStockSummary(
	fundamentals *model.FundamentalsView,
	history []model.FinancialHistoryPeriod,
	insiderTxns []model.InsiderTransaction,
	technical *TechnicalIndicators,
	articleCount int,
) *model.StockSummaryView {
	return &model.StockSummaryView{
		Valuation: ruleBasedValuation(fundamentals, history),
		Technical: ruleBasedTechnical(technical),
		Insider:   ruleBasedInsider(insiderTxns),
		News:      ruleBasedNews(articleCount),
		Source:    "rule",
	}
}

func ruleBasedValuation(fundamentals *model.FundamentalsView, history []model.FinancialHistoryPeriod) string {
	var parts []string
	if fundamentals != nil {
		if fundamentals.PER != nil && *fundamentals.PER > 0 {
			parts = append(parts, fmt.Sprintf("PER %.1f배", *fundamentals.PER))
		}
		if fundamentals.PBR != nil && *fundamentals.PBR > 0 {
			parts = append(parts, fmt.Sprintf("PBR %.1f배", *fundamentals.PBR))
		}
		if fundamentals.OperatingMargin != nil {
			parts = append(parts, fmt.Sprintf("영업이익률 %.1f%%", *fundamentals.OperatingMargin))
		}
		if fundamentals.RevenueGrowthYoY != nil {
			parts = append(parts, fmt.Sprintf("매출 YoY %s", signedPercent(*fundamentals.RevenueGrowthYoY)))
		}
	}
	if len(history) > 0 {
		latest := history[len(history)-1]
		if latest.RevenueGrowthQoQ != nil {
			parts = append(parts, fmt.Sprintf("직전 분기 대비 매출 %s", signedPercent(*latest.RevenueGrowthQoQ)))
		}
	}
	if len(parts) == 0 {
		return "밸류에이션 데이터가 충분하지 않습니다."
	}
	return strings.Join(parts, ", ") + "."
}

func ruleBasedTechnical(t *TechnicalIndicators) string {
	if t == nil || t.LastClose == nil {
		return "기술적 지표를 계산할 캔들 데이터가 부족합니다."
	}
	var parts []string
	last := *t.LastClose
	if t.SMA20 != nil {
		pos := "위"
		if last < *t.SMA20 {
			pos = "아래"
		}
		parts = append(parts, fmt.Sprintf("20일선 %s", pos))
	}
	if t.RSI14 != nil {
		parts = append(parts, fmt.Sprintf("RSI(14) %.1f", *t.RSI14))
	}
	if t.ADX14 != nil {
		parts = append(parts, fmt.Sprintf("ADX(14) %.1f", *t.ADX14))
	}
	if t.PctFromHigh52Week != nil {
		parts = append(parts, fmt.Sprintf("52주 최고가 대비 %s", signedPercent(*t.PctFromHigh52Week)))
	}
	if len(parts) == 0 {
		return "기술적 지표를 계산할 캔들 데이터가 부족합니다."
	}
	return strings.Join(parts, ", ") + "."
}

func ruleBasedInsider(txns []model.InsiderTransaction) string {
	if len(txns) == 0 {
		return "최근 내부자 거래 내역이 없습니다."
	}
	buy, sell := 0, 0
	for _, t := range txns {
		if t.SharesChange == nil {
			continue
		}
		if *t.SharesChange > 0 {
			buy++
		} else if *t.SharesChange < 0 {
			sell++
		}
	}
	return fmt.Sprintf("최근 내부자 거래 %d건 중 증가 %d건, 감소 %d건입니다.", len(txns), buy, sell)
}

func ruleBasedNews(articleCount int) string {
	if articleCount == 0 {
		return "최근 관련 뉴스가 없습니다."
	}
	return fmt.Sprintf("최근 관련 뉴스 %d건이 있습니다.", articleCount)
}

// ruleBasedKeywords builds a short, non-fabricated tag list from values the
// service actually has: the symbol and name always, plus at most two
// threshold-derived tags from real computed indicators (never an invented
// theme/sector — the codebase has no sector data to draw one from).
func ruleBasedKeywords(stock *model.Stock, fundamentals *model.FundamentalsView, technical *TechnicalIndicators) []string {
	if stock == nil {
		return nil
	}
	keywords := make([]string, 0, 4)
	if stock.Symbol != "" {
		keywords = append(keywords, stock.Symbol)
	}
	if stock.Name != "" && stock.Name != stock.Symbol {
		keywords = append(keywords, stock.Name)
	}
	if technical != nil && technical.RSI14 != nil {
		switch {
		case *technical.RSI14 >= 70:
			keywords = append(keywords, "과매수")
		case *technical.RSI14 <= 30:
			keywords = append(keywords, "과매도")
		}
	}
	if fundamentals != nil && fundamentals.PER != nil && *fundamentals.PER > 0 {
		switch {
		case *fundamentals.PER < 10:
			keywords = append(keywords, "저PER")
		case *fundamentals.PER > 30:
			keywords = append(keywords, "고PER")
		}
	}
	return keywords
}

func signedPercent(v float64) string {
	if v > 0 {
		return fmt.Sprintf("+%.1f%%", v)
	}
	return fmt.Sprintf("%.1f%%", v)
}

// applyStockSummaryLLMOutput overlays the LLM's prose onto the rule-based
// fallback (only non-empty fields win) and computes the news sentiment
// aggregate from its per-article labels.
func applyStockSummaryLLMOutput(out *model.StockSummaryView, llmOut *llm.StockSummaryLLMOutput, articles []model.NewsArticle) {
	if llmOut.Valuation != "" {
		out.Valuation = llmOut.Valuation
	}
	if llmOut.Technical != "" {
		out.Technical = llmOut.Technical
	}
	if llmOut.Insider != "" {
		out.Insider = llmOut.Insider
	}
	if llmOut.News != "" {
		out.News = llmOut.News
	}
	if sentiment := aggregateNewsSentiment(articles, llmOut.NewsSentiments); sentiment != nil {
		out.NewsSentiment = sentiment
	}
	if len(llmOut.Keywords) > 0 {
		out.Keywords = llmOut.Keywords
	}
}

func aggregateNewsSentiment(articles []model.NewsArticle, labels []llm.ArticleSentimentLabel) *model.NewsSentimentSummary {
	if len(articles) == 0 {
		return nil
	}
	byID := make(map[string]string, len(labels))
	for _, l := range labels {
		byID[l.ID] = l.Sentiment
	}
	var pos, neg, neu int
	var scoreSum float64
	for _, a := range articles {
		switch byID[a.ID] {
		case "positive":
			pos++
			scoreSum++
		case "negative":
			neg++
			scoreSum--
		default:
			neu++
		}
	}
	total := len(articles)
	avg := 0.0
	if total > 0 {
		avg = scoreSum / float64(total) * 100
	}
	return &model.NewsSentimentSummary{
		Positive:     pos,
		Negative:     neg,
		Neutral:      neu,
		Total:        total,
		AverageScore: avg,
	}
}
