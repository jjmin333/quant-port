package service

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/enact/quant-port/be/internal/model"
)

const usTranslationBatchSize = 24

func isUSTranslationReady(article model.NewsArticle) bool {
	return strings.TrimSpace(article.HeadlineKo) != ""
}

func containsHangulText(s string) bool {
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7A3 {
			return true
		}
	}
	return false
}

func needsEnglishTranslation(headline, summary string) bool {
	if containsHangulText(headline) {
		return false
	}
	for _, r := range headline + summary {
		if unicode.IsLetter(r) && r <= unicode.MaxASCII {
			return true
		}
	}
	return false
}

func (st *NewsStore) translateENToKO(ctx context.Context, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	if st.papago != nil && st.papago.Enabled() {
		ko, err := st.papago.TranslateENToKO(ctx, text)
		if err == nil {
			return ko, nil
		}
		if !strings.Contains(err.Error(), "403") && !strings.Contains(err.Error(), "forbidden") {
			st.logFetchErr("papago", err)
		}
		return "", err
	}
	return "", nil
}

func (st *NewsStore) applyUSTranslation(ctx context.Context, article *model.NewsArticle) {
	if article == nil || article.Market != "us" {
		return
	}
	if strings.TrimSpace(article.HeadlineKo) != "" {
		return
	}
	if !needsEnglishTranslation(article.Headline, article.Summary) {
		article.HeadlineKo = article.Headline
		article.SummaryKo = article.Summary
		return
	}
	if st.papago == nil || !st.papago.Enabled() {
		return
	}

	headlineKo, err := st.translateENToKO(ctx, article.Headline)
	if err != nil || strings.TrimSpace(headlineKo) == "" {
		if err != nil {
			st.logFetchErr("translate_headline", err)
		}
		return
	}
	article.HeadlineKo = fixUSStockTranslation(article.Headline, headlineKo)
}

func fixUSStockTranslation(headlineEn, translated string) string {
	translated = strings.TrimSpace(translated)
	if translated == "" {
		return translated
	}
	en := strings.ToLower(strings.TrimSpace(headlineEn))
	if !strings.Contains(en, "stock") && !strings.Contains(en, "stocks") && !strings.Contains(en, "share") {
		return translated
	}
	out := translated
	out = strings.ReplaceAll(out, "재고가", "주식이")
	out = strings.ReplaceAll(out, " 재고 ", " 주식 ")
	out = strings.ReplaceAll(out, "재고는", "주식은")
	if strings.HasSuffix(out, " 재고") {
		out = strings.TrimSuffix(out, " 재고") + " 주식"
	}
	return out
}

func (st *NewsStore) backfillUSTranslations(ctx context.Context) {
	st.runUSTranslationBatch(ctx)
	ticker := time.NewTicker(newsPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			st.runUSTranslationBatch(ctx)
		}
	}
}

func (st *NewsStore) runUSTranslationBatch(ctx context.Context) {
	if st.repo == nil {
		return
	}
	if st.papago == nil || !st.papago.Enabled() {
		return
	}
	recs, err := st.repo.ListMissingTranslation(ctx, "us", usTranslationBatchSize)
	if err != nil || len(recs) == 0 {
		return
	}
	for _, rec := range recs {
		article := model.NewsArticle{
			Market:   rec.Market,
			Headline: rec.Headline,
			Summary:  rec.Summary,
		}
		st.applyUSTranslation(ctx, &article)
		if strings.TrimSpace(article.HeadlineKo) == "" {
			continue
		}
		if err := st.repo.UpdateTranslation(ctx, rec.ID, article.HeadlineKo, article.SummaryKo); err != nil {
			st.logFetchErr("repo_translate", err)
			continue
		}
		st.patchUSArticleTranslation(rec.ID, article.HeadlineKo, article.SummaryKo)
	}
}

func (st *NewsStore) patchUSArticleTranslation(id, headlineKo, summaryKo string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	for i, a := range st.usArticles {
		if a.ID != id {
			continue
		}
		st.usArticles[i].HeadlineKo = headlineKo
		st.usArticles[i].SummaryKo = summaryKo
		break
	}
}
