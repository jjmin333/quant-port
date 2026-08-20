package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

const relatedSearchFetchCap = 40

func (s *QuotesService) expandSearchWithRelatedProducts(
	ctx context.Context,
	query string,
	limit int,
	items []model.Stock,
) []model.Stock {
	if s.stocks == nil || limit <= 0 || len(items) == 0 {
		return items
	}

	primary := pickSearchPrimary(items)
	anchors := relatedSearchAnchors(query, primary)
	if len(anchors) == 0 && primary.Symbol == "" {
		return items
	}

	s.ensureRelatedCatalogFromToss(ctx, primary)

	exclude := make([]string, 0, len(items))
	for _, st := range items {
		exclude = append(exclude, st.Symbol)
	}

	var related []model.Stock
	if len(anchors) > 0 {
		byAnchor, err := s.stocks.SearchByNameAnchors(ctx, anchors, exclude, relatedSearchFetchCap)
		if err == nil {
			related = append(related, byAnchor...)
		}
	}
	if sym := strings.ToUpper(strings.TrimSpace(primary.Symbol)); sym != "" {
		bySymbol, err := s.stocks.SearchRelatedByUnderlyingSymbol(ctx, sym, exclude, relatedSearchFetchCap)
		if err == nil {
			related = append(related, bySymbol...)
		}
	}

	related = dedupeStocks(related)
	if len(related) == 0 {
		return items
	}

	related = repository.RankRelatedProducts(anchors, related)
	if len(related) == 0 {
		return items
	}

	return mergeSearchResults(items, related, limit)
}

func pickSearchPrimary(items []model.Stock) model.Stock {
	if len(items) == 0 {
		return model.Stock{}
	}
	best := items[0]
	bestPri := repository.SearchInstrumentPriority(best)
	for _, st := range items {
		pri := repository.SearchInstrumentPriority(st)
		if pri > bestPri {
			bestPri = pri
			best = st
		}
	}
	return best
}

func dedupeStocks(items []model.Stock) []model.Stock {
	if len(items) == 0 {
		return items
	}
	seen := map[string]struct{}{}
	out := make([]model.Stock, 0, len(items))
	for _, st := range items {
		sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
		if sym == "" {
			continue
		}
		if _, dup := seen[sym]; dup {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, st)
	}
	return out
}

// ensureRelatedCatalogFromToss upserts guessed related ETF symbols via Toss when missing locally.
func (s *QuotesService) ensureRelatedCatalogFromToss(ctx context.Context, primary model.Stock) {
	if !s.TossEnabled() || s.toss == nil || s.stocks == nil {
		return
	}

	candidates := relatedSymbolCandidates(primary.Symbol)
	if len(candidates) == 0 {
		return
	}

	existing, err := s.stocks.ListBySymbols(ctx, candidates)
	if err != nil {
		return
	}
	have := make(map[string]struct{}, len(existing))
	for _, st := range existing {
		have[strings.ToUpper(st.Symbol)] = struct{}{}
	}

	missing := make([]string, 0, len(candidates))
	for _, sym := range candidates {
		if _, ok := have[sym]; !ok {
			missing = append(missing, sym)
		}
	}
	if len(missing) == 0 {
		return
	}

	for i := 0; i < len(missing); i += 200 {
		end := i + 200
		if end > len(missing) {
			end = len(missing)
		}
		fromToss, err := s.toss.GetStocks(ctx, missing[i:end])
		if err != nil {
			continue
		}
		for _, info := range fromToss {
			st := stockFromTossInfo(info)
			if st.Symbol == "" {
				continue
			}
			_ = s.stocks.Upsert(ctx, st)
		}
	}
}

func relatedSearchAnchors(query string, primary model.Stock) []string {
	var anchors []string
	q := strings.TrimSpace(query)
	if repository.ContainsHangul(q) && len([]rune(q)) >= 2 {
		anchors = append(anchors, q)
	}
	if primary.Symbol != "" {
		anchors = append(anchors, strings.ToLower(strings.TrimSpace(primary.Symbol)))
	}
	if primary.Name != "" && repository.ContainsHangul(primary.Name) {
		anchors = append(anchors, strings.TrimSpace(primary.Name))
	}
	anchors = append(anchors, repository.CompanyNameAnchors(primary.EnglishName)...)

	seen := map[string]struct{}{}
	out := make([]string, 0, len(anchors))
	for _, a := range anchors {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		key := strings.ToLower(a)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, a)
	}
	return out
}

func mergeSearchResults(primary, related []model.Stock, limit int) []model.Stock {
	if limit <= 0 {
		limit = len(primary) + len(related)
	}
	seen := map[string]struct{}{}
	out := make([]model.Stock, 0, limit)

	for _, st := range primary {
		sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
		if sym == "" {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, st)
		if len(out) >= limit {
			return out
		}
	}

	for _, st := range related {
		sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
		if sym == "" {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, st)
		if len(out) >= limit {
			break
		}
	}
	return out
}
