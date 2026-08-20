package service

import (
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

var domesticPremiumPublisherIDs = map[string]struct{}{
	"hankyung":       {},
	"maeil":          {},
	"sedaily":        {},
	"yonhap":         {},
	"financial-news": {},
	"herald":         {},
	"etnews":         {},
	"mt":             {},
	"naver-news":     {},
}

var domesticLowQualitySourceIDs = map[string]struct{}{
	"coinreaders": {},
	"tokenpost":   {},
	"blockmedia":  {},
}

func isDomesticPremiumPublisher(article model.NewsArticle) bool {
	if _, ok := domesticPremiumPublisherIDs[strings.TrimSpace(article.PublisherID)]; ok {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(article.SourceID)) {
	case "naver", "yonhap":
		return true
	default:
		return false
	}
}

func isDomesticLowQualitySource(article model.NewsArticle) bool {
	if _, ok := domesticLowQualitySourceIDs[strings.ToLower(strings.TrimSpace(article.SourceID))]; ok {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(article.PublisherID)) {
	case "coinreaders", "tokenpost", "blockmedia":
		return true
	default:
		return false
	}
}

// domesticNewsQualityScore ranks domestic feed articles (higher = more core market news).
func domesticNewsQualityScore(article model.NewsArticle) int {
	score := 0
	titleLower := strings.ToLower(strings.TrimSpace(article.Headline))
	blob := newsTextBlob(article.Headline, article.Summary)

	if isDomesticPremiumPublisher(article) {
		score += 35
	}
	if hasExplicitDomesticStockSignal(titleLower) {
		score += 25
	} else if hasExplicitDomesticStockSignal(blob) {
		score += 15
	}
	if hasCorporateFinanceSignal(titleLower) {
		score += 10
	}
	if len(article.Symbols) > 0 {
		score += 10
	}
	switch strings.ToLower(strings.TrimSpace(article.SourceID)) {
	case "naver", "kakao":
		score += 8
	case "rss", "yonhap":
		score += 5
	}
	if isDomesticLowQualitySource(article) {
		score -= 45
	}
	if isDomesticCryptoOnlyArticle(blob) {
		score -= 60
	}
	return score
}
