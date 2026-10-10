package service

import (
	"context"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
)

func canonicalNewsLink(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)
	if !strings.Contains(lower, "financialjuice.com") {
		return strings.ToLower(raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return lower
	}
	u.Fragment = ""
	u.RawQuery = ""
	u.User = nil
	return strings.ToLower(strings.TrimRight(u.String(), "/"))
}

func newsArticleURL(article model.NewsArticle) string {
	if u := strings.TrimSpace(article.OriginalURL); u != "" {
		return u
	}
	return strings.TrimSpace(article.URL)
}

func newsArticleDedupeKeys(article model.NewsArticle) []string {
	seen := map[string]struct{}{}
	var keys []string
	add := func(prefix, raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		key := prefix + raw
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for _, key := range marketdata.NewsHeadlineMatchKeys(article.Headline, article.HeadlineKo) {
		add("h:", key)
	}
	if u := canonicalNewsLink(newsArticleURL(article)); u != "" {
		add("u:", u)
	}
	if len(keys) == 0 {
		add("id:", article.ID)
	}
	return keys
}

func newsSourcePriority(sourceID string) int {
	switch strings.ToLower(strings.TrimSpace(sourceID)) {
	case "financial-juice":
		return 4
	case "reuters":
		return 3
	case "finnhub":
		return 2
	case "google_news":
		return 1
	default:
		return 0
	}
}

func preferNewsArticleDuplicate(candidate, current model.NewsArticle) bool {
	candidatePri := newsSourcePriority(candidate.SourceID)
	currentPri := newsSourcePriority(current.SourceID)
	if candidatePri != currentPri {
		return candidatePri > currentPri
	}
	candidateKo := strings.TrimSpace(candidate.HeadlineKo)
	currentKo := strings.TrimSpace(current.HeadlineKo)
	if candidateKo != "" && currentKo == "" {
		return true
	}
	if candidateKo == "" && currentKo != "" {
		return false
	}
	if candidate.ViewCount > current.ViewCount {
		return true
	}
	if candidate.ViewCount < current.ViewCount {
		return false
	}
	return candidate.PublishedAt.After(current.PublishedAt)
}

func articlesOverlap(a, b model.NewsArticle) bool {
	keysA := newsArticleDedupeKeys(a)
	keysB := newsArticleDedupeKeys(b)
	for _, ka := range keysA {
		for _, kb := range keysB {
			if ka == kb {
				return true
			}
		}
	}
	return false
}

func dedupeNewsArticles(items []model.NewsArticle) []model.NewsArticle {
	if len(items) < 2 {
		return items
	}
	groups := make([][]model.NewsArticle, 0, len(items))
	for _, item := range items {
		merged := false
		for i := range groups {
			if articlesOverlap(item, groups[i][0]) {
				groups[i] = append(groups[i], item)
				merged = true
				break
			}
		}
		if !merged {
			groups = append(groups, []model.NewsArticle{item})
		}
	}
	out := make([]model.NewsArticle, 0, len(groups))
	for _, group := range groups {
		best := group[0]
		for _, candidate := range group[1:] {
			if preferNewsArticleDuplicate(candidate, best) {
				best = candidate
			}
		}
		for _, member := range group {
			best.Tags = mergeUniqueStrings(best.Tags, member.Tags)
			best.Keywords = mergeUniqueStrings(best.Keywords, member.Keywords)
			best.Symbols = mergeUniqueStrings(best.Symbols, member.Symbols)
		}
		out = append(out, best)
	}
	return out
}

func newsHeadlineDuplicateSince() time.Duration {
	return 72 * time.Hour
}

func sortNewsArticlesLatest(items []model.NewsArticle) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].PublishedAt.After(items[j].PublishedAt)
	})
}

func isSyndicatedFinancialJuiceIngest(item marketdata.RawNewsItem, storedURL string) bool {
	if strings.EqualFold(strings.TrimSpace(item.SourceID), "financial-juice") {
		return false
	}
	return isFinancialJuiceSource("", storedURL)
}

func (st *NewsStore) shouldSkipDuplicateIngest(
	ctx context.Context,
	market string,
	item marketdata.RawNewsItem,
	title, storedURL string,
) bool {
	if isSyndicatedFinancialJuiceIngest(item, storedURL) {
		return true
	}
	if st.repo == nil {
		return false
	}
	dup, err := st.repo.ExistsRecentHeadlineMatch(ctx, market, title, "", newsHeadlineDuplicateSince())
	return err == nil && dup
}

func mergeUniqueStrings(base, extra []string) []string {
	if len(extra) == 0 {
		return base
	}
	seen := map[string]struct{}{}
	for _, v := range base {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		seen[v] = struct{}{}
	}
	out := append([]string(nil), base...)
	for _, v := range extra {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
