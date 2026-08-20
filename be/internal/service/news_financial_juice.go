package service

import (
	"strings"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

func isFinancialJuiceSource(sourceID, articleURL string) bool {
	if strings.EqualFold(strings.TrimSpace(sourceID), "financial-juice") {
		return true
	}
	link := strings.ToLower(strings.TrimSpace(articleURL))
	return strings.Contains(link, "financialjuice.com")
}

func isFinancialJuiceItem(item marketdata.RawNewsItem) bool {
	return isFinancialJuiceSource(item.SourceID, articleURLFromRaw(item))
}

func isFinancialJuiceArticle(article model.NewsArticle) bool {
	url := strings.TrimSpace(article.OriginalURL)
	if url == "" {
		url = strings.TrimSpace(article.URL)
	}
	return isFinancialJuiceSource(article.SourceID, url)
}

func isFinancialJuiceRecord(rec repository.NewsArticleRecord) bool {
	url := strings.TrimSpace(rec.OriginalURL)
	if url == "" {
		url = strings.TrimSpace(rec.URL)
	}
	return isFinancialJuiceSource(rec.SourceID, url)
}

func financialJuiceNewsTags() []string {
	return []string{"속보"}
}
