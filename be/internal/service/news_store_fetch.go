package service

import (
	"context"
	"log"
	"maps"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

func (st *NewsStore) refreshNaver(ctx context.Context) {
	if st.naver == nil || !st.naver.Enabled() {
		return
	}
	st.rebuildNameIndex(ctx)
	keywords := st.nextKeywords(ctx, st.pollConfig(ctx).DomesticNaverKeywordsPerTick)
	var ingested []model.NewsArticle
	for _, kw := range keywords {
		items, err := st.naver.Search(ctx, kw.Text, 10)
		if err != nil {
			st.logFetchErr("naver", err)
			continue
		}
		for _, item := range items {
			article, ok := st.ingestItem(ctx, item, []string{kw.Text}, kw.Symbol, "domestic")
			if ok {
				ingested = append(ingested, article)
			}
		}
	}
	st.mergeIngestedArticles(ctx, ingested)
}

func (st *NewsStore) refreshKakao(ctx context.Context) {
	if st.kakao == nil || !st.kakao.Enabled() {
		return
	}
	st.rebuildNameIndex(ctx)
	keywords := st.nextKeywords(ctx, st.pollConfig(ctx).DomesticKakaoKeywordsPerTick)
	var ingested []model.NewsArticle
	for _, kw := range keywords {
		items, err := st.kakao.Search(ctx, kw.Text+" 뉴스", 5)
		if err != nil {
			st.logFetchErr("kakao", err)
			continue
		}
		for _, item := range items {
			article, ok := st.ingestItem(ctx, item, []string{kw.Text}, kw.Symbol, "domestic")
			if ok {
				ingested = append(ingested, article)
			}
		}
	}
	st.mergeIngestedArticles(ctx, ingested)
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
	var ingested []model.NewsArticle
	for _, item := range items {
		article, ok := st.ingestItem(ctx, item, nil, "", "domestic")
		if ok {
			ingested = append(ingested, article)
		}
	}
	st.mergeIngestedArticles(ctx, ingested)
}

func (st *NewsStore) ingestItem(
	ctx context.Context,
	item marketdata.RawNewsItem,
	keywords []string,
	linkedSymbol string,
	market string,
) (model.NewsArticle, bool) {
	if market == "" {
		market = "domestic"
	}
	canonical, original, _ := resolveNewsURLs(item)
	title := strings.TrimSpace(item.Title)
	if canonical == "" || title == "" {
		return model.NewsArticle{}, false
	}
	if original == "" {
		original = canonical
	}
	storedURL := canonicalNewsLink(original)
	if storedURL == "" {
		storedURL = canonicalNewsLink(canonical)
	}
	if storedURL == "" {
		return model.NewsArticle{}, false
	}

	defaultMarket := market
	inferredUS := false
	if defaultMarket == "domestic" && st.inferNewsMarket(ctx, title, item.Description, linkedSymbol, defaultMarket) == "us" {
		market = "us"
		inferredUS = true
	}

	if market == "us" {
		if isUSRSSNoiseItem(item) {
			return model.NewsArticle{}, false
		}
		if !isFinancialJuiceItem(item) && !inferredUS && !st.passesUSStockRelevance(ctx, item, keywords, linkedSymbol) {
			return model.NewsArticle{}, false
		}
		if inferredUS && !st.passesStockRelevance(ctx, item, keywords, linkedSymbol) {
			return model.NewsArticle{}, false
		}
	} else if !st.passesStockRelevance(ctx, item, keywords, linkedSymbol) {
		return model.NewsArticle{}, false
	}

	if st.shouldSkipDuplicateIngest(ctx, market, item, title, storedURL) {
		return model.NewsArticle{}, false
	}

	urlHash := newsURLHash(storedURL)
	if st.repo != nil {
		if ok, err := st.repo.ExistsByURLHash(ctx, urlHash); err == nil && ok {
			if len(keywords) > 0 {
				_ = st.repo.AppendKeywords(ctx, urlHash, keywords)
			}
			return model.NewsArticle{}, false
		}
	}

	article, ok := st.normalizeItem(ctx, item, keywords, linkedSymbol, market)
	if !ok {
		return model.NewsArticle{}, false
	}
	return article, true
}

func (st *NewsStore) normalizeItem(
	ctx context.Context,
	item marketdata.RawNewsItem,
	keywords []string,
	linkedSymbol string,
	market string,
) (model.NewsArticle, bool) {
	if market == "" {
		market = "domestic"
	}
	canonical, original, portal := resolveNewsURLs(item)
	title := strings.TrimSpace(item.Title)
	if canonical == "" || title == "" {
		return model.NewsArticle{}, false
	}
	if original == "" {
		original = canonical
	}
	storedURL := canonicalNewsLink(original)
	if storedURL == "" {
		storedURL = canonicalNewsLink(canonical)
	}
	if storedURL == "" {
		return model.NewsArticle{}, false
	}

	urlHash := newsURLHash(storedURL)
	symbolText := title + " " + item.Description
	// mergeTickerCandidates(≠ marketdata.MergeTickerSymbols)를 쓴다 — 후자는
	// 이미 해석된 심볼까지 "1~5자 영문 대문자" 규칙으로 다시 걸러서, 국내
	// 6자리 코드를 통째로 지워 버린다.
	symbols := st.extractSymbolsForMarket(ctx, symbolText, market)
	symbols = mergeTickerCandidates(symbols, item.FeedSymbols...)
	symbols = mergeTickerCandidates(symbols, marketdata.ExtractExchangeTickers(symbolText)...)
	symbols = mergeTickerCandidates(symbols, marketdata.ExtractCashtags(symbolText)...)
	symbols = mergeTickerCandidates(symbols, marketdata.ExtractHeadlineTickers(title)...)
	if linkedSymbol != "" {
		symbols = appendUnique(symbols, linkedSymbol)
	}
	if item.RelatedSymbol != "" {
		symbols = appendUnique(symbols, item.RelatedSymbol)
	}
	if market == "us" {
		if subject := marketdata.ExtractPossessiveHeadlineSubject(title); subject != "" {
			if sym := st.lookupUSSymbolByName(ctx, subject); sym != "" {
				symbols = appendUnique(symbols, sym)
			}
		}
	}

	topic, assetTags := st.topicFor(ctx, market, title, item.Description, symbols)
	tags := classifyNewsTags(title, item.SourceID, storedURL)
	if tags == nil {
		tags = []string{}
	}
	publishedAt := item.PublishedAt
	if publishedAt.IsZero() && st.repo != nil {
		if existing, err := st.repo.PublishedAtByURLHash(ctx, urlHash); err == nil && !existing.IsZero() {
			publishedAt = existing
		}
	}
	publishedAt = repository.NormalizeNewsPublishedAt(publishedAt)

	publisher := marketdata.ResolvePublisherFromArticle(portal, original)
	if name := strings.TrimSpace(item.PublisherName); name != "" && publisher.ID == "unknown" {
		publisher.Name = name
	}
	embedPolicy := marketdata.ResolveEmbedPolicy(ctx, st.embedHTTP, st.redis, original)
	previewMode := marketdata.DecidePreviewMode(ctx, st.embedHTTP, st.redis, original)
	if market == "us" && previewMode == marketdata.PreviewModeSummary && marketdata.ReaderAllowedHost(original) {
		previewMode = marketdata.PreviewModeReader
	}
	imageURL := marketdata.FetchOGImage(ctx, st.ogHTTP, st.redis, original)
	summary := strings.TrimSpace(item.Description)
	if market == "us" {
		summary = st.enrichUSThinSummary(ctx, original, summary)
		imageURL = st.resolveUSNewsPreviewImage(ctx, original, title, summary, symbols, imageURL)
	}

	article := model.NewsArticle{
		ID:            newsArticleID(urlHash),
		Market:        market,
		Headline:      title,
		Summary:       summary,
		SourceID:      item.SourceID,
		PublisherID:   publisher.ID,
		PublisherName: publisher.Name,
		Tags:          tags,
		Symbols:       symbols,
		Keywords:      append([]string(nil), keywords...),
		PublishedAt:   publishedAt,
		URL:           storedURL,
		OriginalURL:   original,
		PortalURL:     portal,
		ImageURL:      imageURL,
		PreviewMode:   string(previewMode),
		EmbedPolicy:   string(embedPolicy),
		Topic:         topic,
		AssetTags:     assetTags,
	}

	if market == "us" {
		st.applyUSTranslation(ctx, &article)
	}

	indexIDs := MatchDashboardIndexIDs(title, summary)
	if market == "us" {
		indexIDs = mergeUniqueStrings(indexIDs, MatchDashboardIndexIDs(article.HeadlineKo, article.SummaryKo))
	}
	if len(indexIDs) > 0 {
		tags = mergeUniqueStrings(tags, DashboardIndexTags(indexIDs))
	}
	article.Tags = tags

	if st.repo != nil {
		rec := repository.NewsArticleRecord{
			ID:            article.ID,
			Market:        article.Market,
			URLHash:       urlHash,
			URL:           storedURL,
			OriginalURL:   original,
			PortalURL:     portal,
			Headline:      title,
			HeadlineKo:    article.HeadlineKo,
			Summary:       article.Summary,
			ImageURL:      imageURL,
			PreviewMode:   string(previewMode),
			SourceID:      article.SourceID,
			PublisherID:   publisher.ID,
			PublisherName: publisher.Name,
			Tags:          tags,
			Symbols:       symbols,
			Keywords:      keywords,
			PublishedAt:   publishedAt,
			Topic:         topic,
			AssetTags:     assetTags,
		}
		if err := st.repo.Upsert(ctx, rec); err != nil {
			st.logFetchErr("repo", err)
		}
	}
	return article, true
}

func resolveNewsURLs(item marketdata.RawNewsItem) (canonical, original, portal string) {
	link := strings.TrimSpace(item.Link)
	original = strings.TrimSpace(item.OriginalLink)
	if original == "" {
		original = link
	}
	canonical = original
	if link != "" && link != original && isNaverPortalURL(link) {
		portal = link
	}
	return canonical, original, portal
}

func isNaverPortalURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.Contains(lower, "news.naver.com") || strings.Contains(lower, "n.news.naver.com")
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

// nameIndexTTL is how long a built name index is reused. rebuildNameIndex runs
// on every poll tick, and the catalog it reads changes maybe once a day — so
// without this the full 종목 마스터 would be re-read from MySQL every tick.
const nameIndexTTL = 30 * time.Minute

func (st *NewsStore) rebuildNameIndex(ctx context.Context) {
	if st.stocks == nil {
		return
	}
	st.mu.RLock()
	fresh := len(st.nameIndex) > 0 && time.Since(st.nameIndexAt) < nameIndexTTL
	st.mu.RUnlock()
	if fresh {
		return
	}

	// 전 종목 마스터를 읽는다. 예전에는 trackedSymbols(관심·보유에 담긴 종목)만
	// 넣어서, 아무도 담지 않은 종목은 기사에 이름이 나와도 태깅되지 않았다 —
	// 국내 기사 100건 중 종목코드가 붙은 것이 2건뿐이던 원인이다.
	stocks, err := st.stocks.ListDomesticNameCatalog(ctx)
	if err != nil || len(stocks) == 0 {
		return
	}
	idx := make(map[string]string, len(stocks))
	for _, s := range stocks {
		name := strings.ToLower(strings.TrimSpace(s.Name))
		if utf8.RuneCountInString(name) < 2 {
			continue
		}
		// 같은 이름이 둘이면(우선주 등) 먼저 온 쪽을 남긴다 — symbol ASC라
		// 보통 본주가 먼저다.
		if _, dup := idx[name]; dup {
			continue
		}
		idx[name] = strings.ToUpper(s.Symbol)
	}
	addHangulAliasNames(idx)
	st.mu.Lock()
	// 인덱스가 바뀌면 같은 본문도 다른 심볼을 낼 수 있다 (symbolMemo).
	// **내용이 같으면 세대를 올리지 않는다** — 재구성은 30분마다 돌지만 종목 마스터는
	// 하루에 한 번쯤 바뀐다. 매번 비우면 30분마다 첫 목록 재구성이 기사 전부를 다시
	// 훑었다(실측 해외 목록 1회 536ms → 메모가 살아 있으면 1.5ms).
	if !maps.Equal(st.nameIndex, idx) {
		st.nameIndexGen++
		st.symbolMemo = nil
	}
	st.nameIndex = idx
	st.nameIndexAt = time.Now()
	st.mu.Unlock()
	log.Printf("news name index (domestic): %d names from %d catalog rows", len(idx), len(stocks))
}

func (st *NewsStore) extractSymbolsForMarket(ctx context.Context, text, market string) []string {
	if market == "us" {
		return st.extractUSSymbols(ctx, text)
	}
	return st.extractSymbols(ctx, text)
}

// extractSymbols는 국내 이름 인덱스(4천여 종목)를 한 항목씩 훑는다. 미국 쪽과
// 같은 이유로 결과를 본문 단위로 들고 있는다 — 목록 요청 하나가 기사마다 이걸
// 부르는데, 인덱스도 본문도 그 사이에 바뀌지 않아 매번 같은 답이 나온다.
// 자세한 배경은 usSymbolMemo 주석 참고.
func (st *NewsStore) extractSymbols(ctx context.Context, text string) []string {
	textLower := strings.ToLower(text)
	st.mu.RLock()
	idx := st.nameIndex
	gen := st.nameIndexGen
	memo, hit := st.symbolMemo[text]
	st.mu.RUnlock()

	if hit && memo.gen == gen {
		return memo.symbols
	}

	var out []string
	seen := map[string]struct{}{}
	add := func(sym string) {
		if sym == "" {
			return
		}
		if _, ok := seen[sym]; ok {
			return
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}

	var matched []string
	for name, sym := range idx {
		if name == "" || sym == "" {
			continue
		}
		if !nameTokenMatch(textLower, name) {
			continue
		}
		// 두 글자 이름과 일상어와 겹치는 이름은 종목코드가 같이 나올 때만.
		// 그러지 않으면 "인수 대상은…"이 대상(001680) 기사가 된다.
		if domesticNameNeedsCode(name) && !containsDomesticCode(textLower, sym) {
			continue
		}
		matched = append(matched, name)
	}
	// 최장 일치. 한국어는 조사가 이름에 바로 붙어서, "두산에너빌리티"가 걸리면
	// "두산"도 같이 걸린다("에"가 조사이기도 하므로). 더 긴 이름이 같이 걸렸다면
	// 짧은 쪽은 그 이름의 머리였을 뿐이다.
	for _, name := range matched {
		if isPrefixOfLongerMatch(name, matched) {
			continue
		}
		add(idx[name])
	}
	/*
		상장명이 아닌 **통칭**도 푼다 (2026-09-04 사용자 지적 10).

		「삼전닉스」처럼 시장이 두 종목을 묶어 부르는 말은 이름 인덱스에 있을 수
		없다 — 회사가 등록한 이름이 아니다. 표와 그 판정 기준은
		`news_symbol_nickname.go`에 있다.

		**최장 일치 뒤에 둔다.** 앞의 루프가 상장명으로 이미 붙인 것이 있으면
		`add`가 중복을 걸러 주고, 상장명이 하나도 안 걸린 기사(바로 그 기사가
		그랬다)에서만 이 줄이 실제로 값을 낸다.
	*/
	for _, sym := range matchNicknameSymbols(textLower, idx) {
		add(sym)
	}
	for _, sym := range st.trackedSymbols(ctx) {
		if containsDomesticCode(textLower, strings.ToLower(sym)) {
			add(sym)
		}
	}
	st.rememberSymbols(text, gen, out)
	return out
}

func (st *NewsStore) rememberSymbols(text string, gen uint64, symbols []string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.symbolMemo == nil || len(st.symbolMemo) >= usSymbolMemoMaxEntries {
		st.symbolMemo = make(map[string]usSymbolMemoEntry, usSymbolMemoMaxEntries)
	}
	// nil 결과도 담는다. 종목이 안 붙는 기사가 다수인데, nil을 "캐시 없음"으로
	// 읽으면 정작 가장 흔한 경우가 매번 인덱스를 다시 훑는다.
	if symbols == nil {
		symbols = []string{}
	}
	st.symbolMemo[text] = usSymbolMemoEntry{gen: gen, symbols: symbols}
}

func (st *NewsStore) mergeIngestedArticles(ctx context.Context, ingested []model.NewsArticle) {
	if len(ingested) == 0 {
		return
	}
	var domestic, us []model.NewsArticle
	for _, a := range ingested {
		if a.Market == "us" {
			us = append(us, a)
		} else {
			domestic = append(domestic, a)
		}
	}
	st.mergeArticlesForMarket(ctx, domestic, "domestic")
	st.mergeArticlesForMarket(ctx, us, "us")
}
