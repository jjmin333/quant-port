package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
)

const (
	marketScoreTitleWeight        = 3.0
	marketScoreSummaryWeight      = 1.0
	marketScoreStructuralUnknown  = 2.0
	marketScoreUnknownUSHint      = 0.5
)

type symbolMarketEvidence struct {
	symbol      string
	currency    model.QuoteCurrency
	inTitle     bool
	known       bool
	structural  bool
}

// inferNewsMarket decides whether a newly ingested article belongs in domestic or US feeds.
// defaultMarket is returned when evidence is mixed or weak (ambiguous cases lean domestic).
func (st *NewsStore) inferNewsMarket(
	ctx context.Context,
	title, summary, linkedSymbol, defaultMarket string,
) string {
	defaultMarket = normalizeNewsMarket(defaultMarket)
	if defaultMarket != "domestic" {
		return defaultMarket
	}

	title = strings.TrimSpace(title)
	summary = strings.TrimSpace(summary)
	if title == "" {
		return defaultMarket
	}

	st.ensureUSNameIndex(ctx)

	evidence := st.collectSymbolMarketEvidence(ctx, title, summary, linkedSymbol)
	usScore, krScore := scoreSymbolMarketEvidence(evidence)
	return decideInferredNewsMarket(usScore, krScore, defaultMarket)
}

func normalizeNewsMarket(market string) string {
	market = strings.ToLower(strings.TrimSpace(market))
	if market == "us" {
		return "us"
	}
	return "domestic"
}

func (st *NewsStore) collectSymbolMarketEvidence(
	ctx context.Context,
	title, summary, linkedSymbol string,
) []symbolMarketEvidence {
	blob := title + " " + summary
	titleUpper := strings.ToUpper(title)

	candidates := marketdata.MergeTickerSymbols(nil, marketdata.ExtractExchangeTickers(blob)...)
	structural := map[string]struct{}{}
	for _, sym := range marketdata.ExtractCashtags(blob) {
		structural[sym] = struct{}{}
		candidates = marketdata.MergeTickerSymbols(candidates, sym)
	}
	for _, sym := range marketdata.ExtractHeadlineTickers(title) {
		structural[sym] = struct{}{}
		candidates = marketdata.MergeTickerSymbols(candidates, sym)
	}
	candidates = marketdata.MergeTickerSymbols(candidates, st.extractSymbols(ctx, blob)...)
	candidates = marketdata.MergeTickerSymbols(candidates, st.extractUSSymbols(ctx, blob)...)

	if subject := marketdata.ExtractPossessiveHeadlineSubject(title); subject != "" {
		if sym := st.lookupUSSymbolByName(ctx, subject); sym != "" {
			candidates = marketdata.MergeTickerSymbols(candidates, sym)
		}
	}

	linkedSymbol = strings.ToUpper(strings.TrimSpace(linkedSymbol))
	if linkedSymbol != "" {
		candidates = marketdata.MergeTickerSymbols(candidates, linkedSymbol)
	}

	currencies := st.resolveSymbolCurrencies(ctx, candidates)
	trackedUS := st.trackedUSSymbolSet(ctx)
	titleMentioned := st.symbolsMentionedInTitle(ctx, title)

	seen := map[string]struct{}{}
	evidence := make([]symbolMarketEvidence, 0, len(candidates))
	for _, sym := range candidates {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}

		cur, known := currencies[sym]
		if !known {
			if _, ok := trackedUS[sym]; ok {
				cur = model.CurrencyUSD
				known = true
			}
		}

		_, isStructural := structural[sym]
		_, inTitle := titleMentioned[sym]
		if !inTitle {
			inTitle = strings.Contains(titleUpper, sym)
		}
		evidence = append(evidence, symbolMarketEvidence{
			symbol:     sym,
			currency:   cur,
			inTitle:    inTitle,
			known:      known,
			structural: isStructural,
		})
	}
	return evidence
}

// symbolsMentionedInTitle finds tickers and company names that appear in the
// headline (including Korean aliases like "엔비디아" → NVDA). Title-only
// matching drives market inference weight — a US name in the headline should
// count as strongly as a cashtag even when the ticker itself is absent.
func (st *NewsStore) symbolsMentionedInTitle(ctx context.Context, title string) map[string]struct{} {
	out := map[string]struct{}{}
	title = strings.TrimSpace(title)
	if title == "" {
		return out
	}
	add := func(sym string) {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym != "" {
			out[sym] = struct{}{}
		}
	}
	for _, sym := range st.extractUSSymbols(ctx, title) {
		add(sym)
	}
	for _, sym := range st.extractSymbols(ctx, title) {
		add(sym)
	}
	for _, sym := range marketdata.ExtractExchangeTickers(title) {
		add(sym)
	}
	for _, sym := range marketdata.ExtractCashtags(title) {
		add(sym)
	}
	for _, sym := range marketdata.ExtractHeadlineTickers(title) {
		add(sym)
	}
	return out
}

func (st *NewsStore) resolveSymbolCurrencies(ctx context.Context, symbols []string) map[string]model.QuoteCurrency {
	out := map[string]model.QuoteCurrency{}
	if st.stocks == nil || len(symbols) == 0 {
		return out
	}
	stocks, err := st.stocks.ListBySymbols(ctx, symbols)
	if err != nil {
		return out
	}
	for _, s := range stocks {
		sym := strings.ToUpper(strings.TrimSpace(s.Symbol))
		if sym == "" {
			continue
		}
		out[sym] = s.Currency
	}
	return out
}

func (st *NewsStore) trackedUSSymbolSet(ctx context.Context) map[string]struct{} {
	out := map[string]struct{}{}
	for _, sym := range st.usIndexSymbols(ctx) {
		out[sym] = struct{}{}
	}
	return out
}

func scoreSymbolMarketEvidence(evidence []symbolMarketEvidence) (usScore, krScore float64) {
	for _, hit := range evidence {
		weight := marketScoreSummaryWeight
		if hit.inTitle {
			weight = marketScoreTitleWeight
		}

		switch hit.currency {
		case model.CurrencyUSD:
			usScore += weight
		case model.CurrencyKRW:
			krScore += weight
		default:
			if !hit.known {
				if hit.structural {
					usScore += marketScoreStructuralUnknown
				} else if hit.inTitle {
					usScore += marketScoreUnknownUSHint
				}
			}
		}
	}
	return usScore, krScore
}

func decideInferredNewsMarket(usScore, krScore float64, defaultMarket string) string {
	defaultMarket = normalizeNewsMarket(defaultMarket)

	if usScore >= marketScoreTitleWeight && krScore == 0 {
		return "us"
	}
	if krScore >= marketScoreTitleWeight && usScore == 0 {
		return "domestic"
	}
	if usScore >= 2*marketScoreSummaryWeight && usScore > krScore*1.5 {
		return "us"
	}
	if krScore > usScore {
		return "domestic"
	}
	return defaultMarket
}
