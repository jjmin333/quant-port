package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/stocklogo"
)

func (st *NewsStore) inferUSPreviewSymbols(ctx context.Context, headline, summary string, stored []string) []string {
	st.ensureUSNameIndex(ctx)
	blob := strings.TrimSpace(headline + " " + summary)
	symbols := marketdata.MergeTickerSymbols(stored)
	symbols = marketdata.MergeTickerSymbols(symbols, marketdata.ExtractExchangeTickers(blob)...)
	symbols = marketdata.MergeTickerSymbols(symbols, st.extractUSSymbols(ctx, blob)...)

	if subject := marketdata.ExtractPossessiveHeadlineSubject(headline); subject != "" {
		if sym := st.lookupUSSymbolByName(ctx, subject); sym != "" {
			symbols = marketdata.MergeTickerSymbols(symbols, sym)
		}
	}
	return symbols
}

func (st *NewsStore) lookupUSSymbolByName(ctx context.Context, name string) string {
	nameLower := strings.ToLower(strings.TrimSpace(name))
	if nameLower == "" {
		return ""
	}
	st.mu.RLock()
	if sym := st.usNameIndex[nameLower]; sym != "" {
		st.mu.RUnlock()
		return sym
	}
	st.mu.RUnlock()

	if st.stocks == nil {
		return ""
	}
	results, err := st.stocks.Search(ctx, model.StockSearchParams{
		Query:  name,
		Market: "us",
		Limit:  5,
	})
	if err != nil || len(results) == 0 {
		return ""
	}
	for _, s := range results {
		if s.Currency != model.CurrencyUSD {
			continue
		}
		return strings.ToUpper(strings.TrimSpace(s.Symbol))
	}
	return ""
}

func pickPrimaryUSSymbol(symbols []string, headline string, nameIndex map[string]string) string {
	headlineLower := strings.ToLower(strings.TrimSpace(headline))
	if headlineLower == "" {
		if len(symbols) == 1 {
			return strings.ToUpper(strings.TrimSpace(symbols[0]))
		}
		return ""
	}

	allowed := map[string]struct{}{}
	for _, sym := range symbols {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym != "" {
			allowed[sym] = struct{}{}
		}
	}
	restrict := len(allowed) > 0

	bestSym := ""
	bestPos := len(headlineLower) + 1
	for name, sym := range nameIndex {
		if len(name) < 3 {
			continue
		}
		if restrict {
			if _, ok := allowed[sym]; !ok {
				continue
			}
		}
		pos := strings.Index(headlineLower, name)
		if pos < 0 || pos >= bestPos {
			continue
		}
		bestPos = pos
		bestSym = sym
	}
	if bestSym != "" {
		return bestSym
	}

	for _, sym := range symbols {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym != "" && strings.Contains(strings.ToUpper(headline), sym) {
			return sym
		}
	}
	if len(symbols) > 0 {
		return strings.ToUpper(strings.TrimSpace(symbols[0]))
	}
	return ""
}

func usSymbolLogoURL(symbol string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return ""
	}
	return stocklogo.LogoURL(symbol, model.CurrencyUSD, model.AssetUS)
}

func (st *NewsStore) resolveUSNewsPreviewImage(
	ctx context.Context,
	articleURL, headline, summary string,
	symbols []string,
	rawImage string,
) string {
	rawImage = strings.TrimSpace(rawImage)

	/*
	 * 값싼 관문을 먼저 통과시킨다.
	 *
	 * inferUSPreviewSymbols는 미국 종목 이름 인덱스(1만 3천여 종목 × 이름 변형)를
	 * 통째로 훑는다 — 기사 한 건당이다. 그런데 그 결과는 아래 ShouldReplace…에
	 * `len(inferred)`로만 들어가고, 이미 기사 고유 이미지가 있거나 알려진 범용
	 * 이미지가 아니면 심볼이 몇 개든 **원본을 그대로 돌려준다**.
	 *
	 * 즉 대부분의 기사에서 그 스캔은 결과를 바꾸지 못한 채 버려졌다. 아래 두
	 * 분기는 ShouldReplaceNewsImageWithSymbolLogo가 false를 내는 조건 그대로라
	 * 판정 결과는 달라지지 않는다.
	 */
	if rawImage != "" {
		if marketdata.IsArticleSpecificNewsImage(articleURL, rawImage, headline) {
			return rawImage
		}
		if !marketdata.IsKnownGenericNewsImage(rawImage) {
			return rawImage
		}
	}

	inferred := st.inferUSPreviewSymbols(ctx, headline, summary, symbols)
	if !marketdata.ShouldReplaceNewsImageWithSymbolLogo(articleURL, rawImage, headline, len(inferred)) {
		return rawImage
	}

	st.mu.RLock()
	idx := st.usNameIndex
	st.mu.RUnlock()

	if logo := usSymbolLogoURL(pickPrimaryUSSymbol(inferred, headline, idx)); logo != "" {
		return logo
	}
	return rawImage
}

func isUSRSSNoiseItem(item marketdata.RawNewsItem) bool {
	creator := strings.ToLower(strings.TrimSpace(item.PublisherName))
	if strings.Contains(creator, "motley fool") {
		return true
	}
	return matchesUSNasdaqSyndicationNoise(item.Title, item.Description, articleURLFromRaw(item))
}

func articleURLFromRaw(item marketdata.RawNewsItem) string {
	if u := strings.TrimSpace(item.OriginalLink); u != "" {
		return u
	}
	return strings.TrimSpace(item.Link)
}

func matchesUSNasdaqSyndicationNoise(title, summary, articleURL string) bool {
	link := strings.ToLower(strings.TrimSpace(articleURL))
	if !strings.Contains(link, "nasdaq.com") {
		return false
	}
	blob := strings.ToLower(title + " " + summary)
	for _, term := range []string{
		"which stock is the better buy",
		"better buy today",
		"here's what billionaire",
		"if you'd invested",
		"passive income investor",
		"has in common",
		"so why is",
	} {
		if strings.Contains(blob, term) {
			return true
		}
	}
	return false
}

func (st *NewsStore) enrichUSThinSummary(ctx context.Context, originalURL, summary string) string {
	if !marketdata.IsThinNewsSummary(summary) {
		return summary
	}
	meta := marketdata.FetchArticlePreviewMeta(ctx, st.ogHTTP, st.redis, originalURL)
	if desc := strings.TrimSpace(meta.Description); desc != "" && len(desc) > len(summary) {
		return desc
	}
	return summary
}
