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
	if llmOut.Overview != "" {
		out.Overview = llmOut.Overview
	}
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

/*
composeOverview — 네 섹션에서 개요 한 문단을 엮는다.

# 언제 쓰이나

LLM이 `overview`를 써 주면 그쪽이 이긴다. 이 함수는 그 밖의 모든 경우다:
LLM이 꺼져 있을 때, 호출이 실패했을 때, 그리고 **이 필드가 생기기 전에 만들어진
캐시**(며칠은 남아 있다). 그때도 개요는 네 갈래를 다 거쳐야 한다 — 그러지 않으면
「밸류에이션만 보여 주던」 예전 화면으로 조용히 되돌아간다.

# 왜 첫 문장만인가

섹션마다 최대 세 문장이라 통째로 이으면 열두 문장이 되는데, 개요 카드는 네 줄에서
잘린다(`line-clamp-4`). 각 섹션의 첫 문장이 그 섹션의 결론인 구조라 — 프롬프트가
「우선순위: 가장 크게 바꾸는 것부터」를 요구한다 — 첫 문장만 모아도 네 갈래가 다
한 번씩 말한다.

「최근 관련 뉴스가 없습니다」류의 규칙 기반 공백 문장은 **뺀다.** 개요 네 줄 중
한 줄을 「없다」에 쓰는 것은 손해다.
*/
func composeOverview(view *model.StockSummaryView) string {
	if view == nil {
		return ""
	}
	sections := []string{view.Valuation, view.Technical, view.Insider, view.News}
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		if s := firstSentence(section); s != "" && !isEmptySectionText(s) {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " ")
}

// firstSentence cuts at the first sentence end. Korean prose here always ends
// sentences with a period, so a plain scan is enough — no tokenizer.
func firstSentence(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	if idx := strings.Index(trimmed, ". "); idx >= 0 {
		return strings.TrimSpace(trimmed[:idx+1])
	}
	return trimmed
}

/*
isEmptySectionText — 「데이터가 없다」고만 말하는 규칙 기반 문장인가.

`ruleBasedStockSummary`가 재료가 없을 때 내놓는 문장들이다. 정보를 담고 있지
않으므로 개요에 넣지 않는다. 접두사로 보는 이유는 뒤에 건수 등이 붙는 변형이
있어서다.
*/
func isEmptySectionText(text string) bool {
	for _, prefix := range []string{
		"밸류에이션 데이터가 충분하지 않습니다",
		"기술적 지표를 계산할 캔들 데이터가 부족합니다",
		"최근 내부자 거래 내역이 없습니다",
		"최근 관련 뉴스가 없습니다",
	} {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
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
