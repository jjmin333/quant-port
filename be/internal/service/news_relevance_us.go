package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

var usStrongMarketTerms = []string{
	"stock", "stocks", "shares", "share price", "equity", "equities",
	"nasdaq", "dow", "s&p", "sp500", "s&p 500", "wall street", "nyse",
	"stock market", "trading", "market rally", "market selloff", "market cap",
	"earnings", "ipo", "sec filing", "etf", "short selling", "short squeeze",
	"buyback", "dividend", "guidance", "outlook", "analyst", "upgrade", "downgrade",
	"premarket", "after hours", "futures", "options", "bull market", "bear market",
}

var usCorporateFinanceTerms = []string{
	"revenue", "profit", "eps", "ebitda", "margin", "forecast", "quarter", "quarterly",
	"annual report", "10-k", "10-q", "merger", "acquisition", "m&a", "layoff", "ceo", "cfo",
}

var usMacroFinanceTerms = []string{
	"inflation", "interest rate", "rate cut", "rate hike", "fed", "federal reserve",
	"treasury", "gdp", "cpi", "jobs report", "unemployment", "bond yield", "yield curve",
	"monetary policy", "quantitative", "basis point", "pce", "nonfarm", "labor market",
}

// Market-moving macro terms (excludes bare "fed"/"federal reserve" institution mentions).
var usMacroMarketMovingTerms = []string{
	"inflation", "interest rate", "rate cut", "rate hike", "fomc",
	"treasury", "gdp", "cpi", "jobs report", "unemployment", "bond yield", "yield curve",
	"monetary policy", "quantitative", "basis point", "pce", "nonfarm", "labor market",
}

var usFedMarketContextTerms = []string{
	"rate", "rates", "inflation", "fomc", "monetary policy", "interest",
	"powell", "dissent", "dot plot", "statement", "minutes", "basis point",
	"tightening", "easing", "hold steady", "hold rates", "cut rates", "hike rates",
}

var usOperationalNoiseTerms = []string{
	"triennial payment", "payments study", "payment study",
	"enforcement action", "requests comment on a proposal",
	"passing of", "obituar", "discount rate meeting",
	"anti-money laundering program", "mutual banking organization",
	"bank examination", "data download program",
}

var usSectorTerms = []string{
	"semiconductor", "semiconductors", "ai chip", "biotech", "electric vehicle",
	"cloud computing", "software sector", "energy sector", "banking sector",
}

var usIndustryContextTerms = []string{
	"industry", "outlook", "export", "import", "orders", "company", "corporate",
	"production", "factory", "capacity", "investment", "stake", "takeover",
}

var usAmbiguousQueryTerms = map[string]struct{}{
	"ai": {}, "trump": {}, "tech": {}, "bank": {}, "banks": {},
	"semiconductor": {}, "semiconductors": {}, "biotech": {}, "energy": {},
}

var usPersonalFinanceNoiseTerms = []string{
	"passive income", "personal finance", "retire early", "save money", "money tip",
	"death certificate", "ex-husband", "ex-wife", "so why is", "things every",
	"has in common", "simpler than you might think", "financial planning tip",
	"money habit", "budget hack", "credit card debt", "student loan forgiveness",
	"customer service", "asking me for", "my ex-", "sister died", "brother died",
}

var usExcludeTopicTerms = []string{
	"sports", "nba", "nfl", "mlb", "soccer", "olympics", "celebrity", "movie", "music",
	"weather", "hurricane", "earthquake", "recipe", "fashion", "wedding", "divorce",
	"covid vaccine", "hospital", "school shooting",
}

func usContainsAnyTerm(blob string, terms []string) bool {
	for _, term := range terms {
		if containsUSTerm(blob, term) {
			return true
		}
	}
	return false
}

func containsUSTerm(blob, term string) bool {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return false
	}
	if strings.Contains(term, " ") || strings.Contains(term, "&") || strings.Contains(term, "-") {
		return strings.Contains(blob, term)
	}
	padded := " " + blob + " "
	return strings.Contains(padded, " "+term+" ")
}

func hasUSStrongMarketSignal(blob string) bool {
	return usContainsAnyTerm(blob, usStrongMarketTerms)
}

func hasUSCorporateFinanceSignal(blob string) bool {
	return usContainsAnyTerm(blob, usCorporateFinanceTerms)
}

func hasUSMacroFinanceSignal(blob string) bool {
	return usContainsAnyTerm(blob, usMacroFinanceTerms)
}

func hasUSMacroMarketMovingSignal(blob string) bool {
	return usContainsAnyTerm(blob, usMacroMarketMovingTerms)
}

func hasUSFedMarketSignal(blob string) bool {
	if !containsUSTerm(blob, "fed") && !containsUSTerm(blob, "federal reserve") {
		return false
	}
	return usContainsAnyTerm(blob, usFedMarketContextTerms)
}

func matchesUSOperationalNoise(blob string) bool {
	return usContainsAnyTerm(blob, usOperationalNoiseTerms)
}

func hasUSFinanceContentSignal(blob string) bool {
	return hasUSStrongMarketSignal(blob) || hasUSCorporateFinanceSignal(blob)
}

func hasUSStrictFinanceSignal(blob string) bool {
	return hasUSFinanceContentSignal(blob)
}

func matchesUSPersonalFinanceNoise(blob string) bool {
	return usContainsAnyTerm(blob, usPersonalFinanceNoiseTerms)
}

func hasUSSectorWithIndustryContext(blob string) bool {
	if !usContainsAnyTerm(blob, usSectorTerms) {
		return false
	}
	return usContainsAnyTerm(blob, usIndustryContextTerms) ||
		hasUSStrongMarketSignal(blob) ||
		hasUSCorporateFinanceSignal(blob) ||
		hasUSMacroFinanceSignal(blob)
}

func isUSAmbiguousSearchTerm(term string) bool {
	_, ok := usAmbiguousQueryTerms[strings.ToLower(strings.TrimSpace(term))]
	return ok
}

func hasUSRelevanceAnchor(title, blob string) bool {
	titleLower := strings.ToLower(strings.TrimSpace(title))
	if hasUSGeopoliticsMarketSignal(titleLower) {
		return true
	}
	if hasUSStrictFinanceSignal(blob) {
		return true
	}
	if hasUSMacroFinanceSignal(blob) && hasUSStrongMarketSignal(blob) {
		return true
	}
	if hasUSMacroFinanceSignal(titleLower) && hasUSStrongMarketSignal(titleLower) {
		return true
	}
	if hasUSMacroMarketMovingSignal(titleLower) || hasUSFedMarketSignal(titleLower) {
		return true
	}
	if hasUSGeopoliticsMarketSignal(blob) && (hasUSMacroFinanceSignal(blob) || hasUSStrongMarketSignal(blob)) {
		return true
	}
	return hasUSSectorWithIndustryContext(blob)
}

func passesUSKeywordSearchGate(title, blob string, searchTerms []string, linkedSymbol string) bool {
	if linkedSymbol != "" {
		return true
	}
	if len(searchTerms) == 0 {
		return true
	}

	for _, term := range searchTerms {
		if isUSAmbiguousSearchTerm(term) && containsSearchTerm(blob, term) {
			if !hasUSStrictFinanceSignal(blob) {
				return false
			}
		}
	}

	titleLower := strings.ToLower(strings.TrimSpace(title))
	if hasUSStrictFinanceSignal(titleLower) {
		return true
	}
	if hasUSMacroFinanceSignal(titleLower) && hasUSStrongMarketSignal(titleLower) {
		return true
	}
	if hasUSSectorWithIndustryContext(titleLower) {
		return true
	}

	for _, term := range searchTerms {
		if containsSearchTerm(blob, term) && !containsSearchTerm(titleLower, term) {
			return false
		}
	}

	return hasUSStrictFinanceSignal(blob) ||
		(hasUSMacroFinanceSignal(blob) && hasUSStrongMarketSignal(blob))
}

func isUSStockRelated(
	ctx context.Context,
	st *NewsStore,
	title, summary string,
	linkedSymbol string,
	searchTerms []string,
	storedSymbols []string,
	articleURL string,
) bool {
	blob := newsTextBlob(title, summary)

	if matchesUSNasdaqSyndicationNoise(title, summary, articleURL) {
		return false
	}

	symbols := st.extractUSSymbols(ctx, title+" "+summary)
	for _, sym := range storedSymbols {
		symbols = appendUnique(symbols, sym)
	}
	if linkedSymbol != "" {
		symbols = appendUnique(symbols, linkedSymbol)
	}

	hasSymbols := len(symbols) > 0
	hasStrong := hasUSStrongMarketSignal(blob)
	hasCorporate := hasUSCorporateFinanceSignal(blob)
	hasFinance := hasUSStrictFinanceSignal(blob)

	if usContainsAnyTerm(blob, usExcludeTopicTerms) && !hasStrong && !hasSymbols && !hasCorporate {
		return false
	}

	if (matchesUSPersonalFinanceNoise(blob) || usContainsAnyTerm(blob, usExcludeTopicTerms)) && hasSymbols && !hasFinance {
		return false
	}

	if matchesUSPersonalFinanceNoise(blob) && !hasFinance {
		return false
	}

	if matchesUSOperationalNoise(blob) && !hasStrong && !hasSymbols {
		return false
	}

	if hasSymbols && !hasUSFinanceContentSignal(blob) && !hasUSSectorWithIndustryContext(blob) {
		return false
	}

	if linkedSymbol != "" {
		if !hasUSRelevanceAnchor(title, blob) {
			return false
		}
		return passesUSKeywordSearchGate(title, blob, searchTerms, linkedSymbol)
	}

	for _, term := range searchTerms {
		if isUSAmbiguousSearchTerm(term) && containsSearchTerm(blob, term) {
			if !hasUSStrictFinanceSignal(blob) {
				return false
			}
		}
	}

	if !hasUSRelevanceAnchor(title, blob) {
		return false
	}
	if !passesUSLowQualitySourceGate(title, articleURL) {
		return false
	}
	return passesUSKeywordSearchGate(title, blob, searchTerms, linkedSymbol)
}

func (st *NewsStore) passesUSStockRelevance(
	ctx context.Context,
	item marketdata.RawNewsItem,
	searchTerms []string,
	linkedSymbol string,
) bool {
	titleLower := strings.ToLower(strings.TrimSpace(item.Title))
	if item.SourceID == "google_news" && !hasUSTitleMarketAnchor(titleLower) {
		return false
	}
	linked := linkedSymbol
	if linked == "" {
		linked = strings.ToUpper(strings.TrimSpace(item.RelatedSymbol))
	}
	articleURL := strings.TrimSpace(item.OriginalLink)
	if articleURL == "" {
		articleURL = strings.TrimSpace(item.Link)
	}
	return isUSStockRelated(
		ctx,
		st,
		item.Title,
		item.Description,
		linked,
		searchTerms,
		nil,
		articleURL,
	)
}

func isUSStockRelatedRecord(ctx context.Context, st *NewsStore, rec repository.NewsArticleRecord) bool {
	linked := inferLinkedUSSymbolFromRecord(ctx, st, rec)
	return isUSStockRelated(
		ctx,
		st,
		rec.Headline,
		rec.Summary,
		linked,
		rec.Keywords,
		rec.Symbols,
		rec.URL,
	)
}

func isUSStockRelatedArticle(ctx context.Context, st *NewsStore, article model.NewsArticle) bool {
	titleLower := strings.ToLower(strings.TrimSpace(article.Headline))
	if article.SourceID == "google_news" && !hasUSTitleMarketAnchor(titleLower) {
		return false
	}
	linked := ""
	if len(article.Symbols) == 1 {
		linked = article.Symbols[0]
	}
	return isUSStockRelated(
		ctx,
		st,
		article.Headline,
		article.Summary,
		linked,
		article.MatchedKeys,
		article.Symbols,
		usArticleURL(article),
	)
}

func inferLinkedUSSymbolFromRecord(ctx context.Context, st *NewsStore, rec repository.NewsArticleRecord) string {
	if len(rec.Symbols) != 1 || len(rec.Keywords) == 0 {
		return ""
	}
	sym := strings.ToUpper(strings.TrimSpace(rec.Symbols[0]))
	if sym == "" || !isLikelyUSTicker(sym) {
		return ""
	}
	st.rebuildUSNameIndex(ctx)
	st.mu.RLock()
	idx := st.usNameIndex
	st.mu.RUnlock()
	for name, mapped := range idx {
		if mapped != sym {
			continue
		}
		for _, kw := range rec.Keywords {
			if strings.EqualFold(strings.TrimSpace(kw), name) {
				return sym
			}
		}
	}
	return ""
}

func isLikelyUSTicker(sym string) bool {
	if len(sym) < 1 || len(sym) > 5 {
		return false
	}
	for _, r := range sym {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}
