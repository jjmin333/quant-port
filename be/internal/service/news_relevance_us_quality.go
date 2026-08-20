package service

import (
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

var usPremiumSourceHosts = []string{
	"financialjuice.com",
	"reuters.com",
	"bloomberg.com",
	"wsj.com",
	"dj.com",
	"cnbc.com",
	"marketwatch.com",
	"ft.com",
	"financialtimes.com",
	"federalreserve.gov",
	"nasdaq.com",
}

var usLowQualitySourceHosts = []string{
	"fool.com",
	"motleyfool.com",
	"benzinga.com",
	"seekingalpha.com",
	"investing.com",
	"zacks.com",
	"investopedia.com",
}

var usGeopoliticsMarketTerms = []string{
	"tariff", "trade war", "sanctions", "geopolit", "opec", "oil price", "crude oil",
	"treasury yield", "treasury sell", "debt ceiling", "government shutdown",
	"central bank", "ministry of finance", "commerce ministry", "foreign exchange",
	"currency intervention", "fx intervention", "supply chain", "export ban", "import ban",
	"shipping lane", "strait of", "military strike", "ceasefire", "war risk",
	"trade dispute", "trade tension", "forced labor", "diplomatic",
}

func hasUSGeopoliticsMarketSignal(blob string) bool {
	return usContainsAnyTerm(blob, usGeopoliticsMarketTerms)
}

func usArticleURL(article model.NewsArticle) string {
	if u := strings.TrimSpace(article.OriginalURL); u != "" {
		return strings.ToLower(u)
	}
	return strings.ToLower(strings.TrimSpace(article.URL))
}

func isUSPremiumSource(url string) bool {
	url = strings.ToLower(strings.TrimSpace(url))
	if url == "" {
		return false
	}
	for _, host := range usPremiumSourceHosts {
		if strings.Contains(url, host) {
			return true
		}
	}
	return false
}

func isUSLowQualitySource(url string) bool {
	url = strings.ToLower(strings.TrimSpace(url))
	if url == "" {
		return false
	}
	for _, host := range usLowQualitySourceHosts {
		if strings.Contains(url, host) {
			return true
		}
	}
	return false
}

func hasUSTitleMarketAnchor(titleLower string) bool {
	return hasUSStrictFinanceSignal(titleLower) ||
		hasUSMacroMarketMovingSignal(titleLower) ||
		hasUSFedMarketSignal(titleLower) ||
		hasUSStrongMarketSignal(titleLower) ||
		hasUSGeopoliticsMarketSignal(titleLower)
}

func passesUSLowQualitySourceGate(title, articleURL string) bool {
	if !isUSLowQualitySource(articleURL) {
		return true
	}
	titleLower := strings.ToLower(strings.TrimSpace(title))
	return hasUSTitleMarketAnchor(titleLower)
}

// usNewsQualityScore ranks articles for overseas feed ordering (higher = more core market news).
func usNewsQualityScore(article model.NewsArticle) int {
	score := 0
	titleLower := strings.ToLower(strings.TrimSpace(article.Headline))
	blob := newsTextBlob(article.Headline, article.Summary)
	url := usArticleURL(article)

	if isUSPremiumSource(url) {
		score += 40
	}
	if article.SourceID == "reuters" || strings.Contains(url, "reuters.com") {
		score += 20
	}
	if article.SourceID == "financial-juice" || strings.Contains(url, "financialjuice.com") {
		score += 35
	}
	if hasUSMacroMarketMovingSignal(titleLower) || hasUSFedMarketSignal(titleLower) {
		score += 25
	}
	if hasUSGeopoliticsMarketSignal(titleLower) {
		score += 20
	}
	if hasUSStrongMarketSignal(titleLower) {
		score += 15
	}
	if hasUSCorporateFinanceSignal(titleLower) {
		score += 10
	}
	if article.SourceID == "finnhub" {
		score += 5
	}
	if isUSLowQualitySource(url) {
		score -= 35
	}
	if matchesUSPersonalFinanceNoise(blob) {
		score -= 50
	}
	return score
}
