package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

func (st *NewsStore) refreshNaver(ctx context.Context) {
	if st.naver == nil || !st.naver.Enabled() {
		return
	}
	st.rebuildNameIndex(ctx)
	keywords := st.nextKeywords(ctx, newsNaverKeywordsPerTick)
	var ingested []model.NewsArticle
	for _, kw := range keywords {
		items, err := st.naver.Search(ctx, kw.Text, 10)
		if err != nil {
			st.logFetchErr("naver", err)
			continue
		}
		for _, item := range items {
			article, ok := st.ingestItem(ctx, item, []string{kw.Text}, kw.Symbol)
			if ok {
				ingested = append(ingested, article)
			}
		}
	}
	st.mergeArticles(ctx, ingested)
}

func (st *NewsStore) refreshKakao(ctx context.Context) {
	if st.kakao == nil || !st.kakao.Enabled() {
		return
	}
	st.rebuildNameIndex(ctx)
	keywords := st.nextKeywords(ctx, newsKakaoKeywordsPerTick)
	var ingested []model.NewsArticle
	for _, kw := range keywords {
		items, err := st.kakao.Search(ctx, kw.Text+" 뉴스", 5)
		if err != nil {
			st.logFetchErr("kakao", err)
			continue
		}
		for _, item := range items {
			article, ok := st.ingestItem(ctx, item, []string{kw.Text}, kw.Symbol)
			if ok {
				ingested = append(ingested, article)
			}
		}
	}
	st.mergeArticles(ctx, ingested)
}

func (st *NewsStore) refreshRSS(ctx context.Context) {
	if st.rss == nil {
		return
	}
	st.rebuildNameIndex(ctx)
	items, err := st.rss.FetchAll(ctx)
	if err != nil {
		st.logFetchErr("rss", err)
		return
	}
	matchTerms := st.relevanceTerms(ctx)
	var ingested []model.NewsArticle
	for _, item := range items {
		if !st.matchesRelevance(item, matchTerms) {
			continue
		}
		article, ok := st.ingestItem(ctx, item, nil, "")
		if ok {
			ingested = append(ingested, article)
		}
	}
	st.mergeArticles(ctx, ingested)
}

func (st *NewsStore) ingestItem(
	ctx context.Context,
	item marketdata.RawNewsItem,
	keywords []string,
	linkedSymbol string,
) (model.NewsArticle, bool) {
	link := strings.TrimSpace(item.Link)
	title := strings.TrimSpace(item.Title)
	if link == "" || title == "" {
		return model.NewsArticle{}, false
	}

	urlHash := newsURLHash(link)
	exists := false
	if st.repo != nil {
		if ok, err := st.repo.ExistsByURLHash(ctx, urlHash); err == nil && ok {
			exists = true
		}
	}

	article, ok := st.normalizeItem(ctx, item, keywords, linkedSymbol)
	if !ok {
		return model.NewsArticle{}, false
	}
	if exists {
		return model.NewsArticle{}, false
	}
	return article, true
}

func (st *NewsStore) normalizeItem(
	ctx context.Context,
	item marketdata.RawNewsItem,
	keywords []string,
	linkedSymbol string,
) (model.NewsArticle, bool) {
	link := strings.TrimSpace(item.Link)
	title := strings.TrimSpace(item.Title)
	if link == "" || title == "" {
		return model.NewsArticle{}, false
	}

	urlHash := newsURLHash(link)
	symbols := st.extractSymbols(ctx, title+" "+item.Description)
	if linkedSymbol != "" {
		symbols = appendUnique(symbols, linkedSymbol)
	}

	tags := classifyNewsTags(title)
	if tags == nil {
		tags = []string{}
	}
	publishedAt := item.PublishedAt
	if publishedAt.IsZero() && st.repo != nil {
		if existing, err := st.repo.PublishedAtByURLHash(ctx, urlHash); err == nil && !existing.IsZero() {
			publishedAt = existing
		}
	}
	publisher := marketdata.ResolvePublisher(link)
	article := model.NewsArticle{
		ID:            newsArticleID(urlHash),
		Market:        "domestic",
		Headline:      title,
		Summary:       strings.TrimSpace(item.Description),
		SourceID:      item.SourceID,
		PublisherID:   publisher.ID,
		PublisherName: publisher.Name,
		Tags:          tags,
		Symbols:       symbols,
		PublishedAt:   publishedAt,
		URL:           link,
		EmbedPolicy:   string(marketdata.ResolveEmbedPolicy(ctx, st.embedHTTP, st.redis, link)),
	}

	if st.repo != nil {
		rec := repository.NewsArticleRecord{
			ID:          article.ID,
			Market:      article.Market,
			URLHash:     urlHash,
			URL:         link,
			Headline:    title,
			Summary:     article.Summary,
			SourceID:    article.SourceID,
			Tags:        tags,
			Symbols:     symbols,
			Keywords:    keywords,
			PublishedAt: publishedAt,
		}
		if err := st.repo.Upsert(ctx, rec); err != nil {
			st.logFetchErr("repo", err)
		}
	}
	return article, true
}

func (st *NewsStore) rebuildNameIndex(ctx context.Context) {
	if st.stocks == nil {
		return
	}
	symbols := st.trackedSymbols(ctx)
	if len(symbols) == 0 {
		return
	}
	stocks, err := st.stocks.ListBySymbols(ctx, symbols)
	if err != nil {
		return
	}
	idx := make(map[string]string, len(stocks))
	for _, s := range stocks {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			continue
		}
		idx[strings.ToLower(name)] = strings.ToUpper(s.Symbol)
	}
	st.mu.Lock()
	st.nameIndex = idx
	st.mu.Unlock()
}

func (st *NewsStore) extractSymbols(ctx context.Context, text string) []string {
	textLower := strings.ToLower(text)
	st.mu.RLock()
	idx := st.nameIndex
	st.mu.RUnlock()

	var out []string
	seen := map[string]struct{}{}
	for name, sym := range idx {
		if name == "" || sym == "" {
			continue
		}
		if strings.Contains(textLower, name) {
			if _, ok := seen[sym]; ok {
				continue
			}
			seen[sym] = struct{}{}
			out = append(out, sym)
		}
	}
	for _, sym := range st.trackedSymbols(ctx) {
		if strings.Contains(text, sym) {
			if _, ok := seen[sym]; ok {
				continue
			}
			seen[sym] = struct{}{}
			out = append(out, sym)
		}
	}
	return out
}

func (st *NewsStore) relevanceTerms(ctx context.Context) []string {
	terms := append([]string{}, adminNewsKeywords...)
	for _, kw := range st.buildKeywordQueue(ctx) {
		if kw.Text != "" {
			terms = append(terms, kw.Text)
		}
	}
	return uniqueStrings(terms)
}

func (st *NewsStore) matchesRelevance(item marketdata.RawNewsItem, terms []string) bool {
	blob := strings.ToLower(item.Title + " " + item.Description)
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term != "" && strings.Contains(blob, term) {
			return true
		}
	}
	return false
}

func appendUnique(items []string, v string) []string {
	v = strings.ToUpper(strings.TrimSpace(v))
	if v == "" {
		return items
	}
	for _, existing := range items {
		if existing == v {
			return items
		}
	}
	return append(items, v)
}

func uniqueStrings(items []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range items {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	return out
}
