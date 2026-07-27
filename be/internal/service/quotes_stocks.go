package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

const defaultQuoteListLimit = 100

func stockToQuoteSummary(st model.Stock, watchSet, heldSet map[string]bool) model.QuoteSummary {
	sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
	return model.QuoteSummary{
		Symbol:        st.Symbol,
		Name:          st.Name,
		Market:        st.Market,
		Currency:      st.Currency,
		AssetType:     st.AssetType,
		IsWatchlisted: watchSet[sym],
		IsHeld:        heldSet[sym],
	}
}

func stockMarketFilter(filter string) string {
	switch filter {
	case "domestic", "us":
		return filter
	default:
		return ""
	}
}

func (s *QuotesService) listQuotesFromCatalog(
	ctx context.Context,
	filter, query string,
	limit, offset int,
	watchSet, heldSet map[string]bool,
) ([]model.QuoteSummary, int) {
	if s.stocks == nil {
		return nil, 0
	}
	if limit <= 0 {
		limit = defaultQuoteListLimit
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	p := model.StockSearchParams{
		Query:  query,
		Market: stockMarketFilter(filter),
		Limit:  limit,
		Offset: offset,
	}

	total, err := s.stocks.Count(ctx, p)
	if err != nil || total == 0 {
		return nil, 0
	}

	stocks, err := s.stocks.Search(ctx, p)
	if err != nil || len(stocks) == 0 {
		return nil, total
	}

	base := make([]model.QuoteSummary, len(stocks))
	for i, st := range stocks {
		base[i] = stockToQuoteSummary(st, watchSet, heldSet)
	}
	return s.applyLivePrices(ctx, base, watchSet, heldSet), total
}

func (s *QuotesService) SearchStocksCatalog(
	ctx context.Context,
	p model.StockSearchParams,
) ([]model.Stock, int, error) {
	if s.stocks == nil {
		return nil, 0, nil
	}
	if p.Limit <= 0 {
		p.Limit = defaultQuoteListLimit
	}
	if p.Limit > 200 {
		p.Limit = 200
	}
	total, err := s.stocks.Count(ctx, p)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.stocks.Search(ctx, p)
	if err != nil {
		return nil, 0, err
	}
	items, total = s.mergeTickerCatalogSearch(ctx, p.Query, p.Limit, items, total)
	return items, total, nil
}

func (s *QuotesService) mergeTickerCatalogSearch(
	ctx context.Context,
	query string,
	limit int,
	items []model.Stock,
	total int,
) ([]model.Stock, int) {
	q := strings.TrimSpace(query)
	if q == "" || !repository.IsLatinOnlyQuery(q) || !repository.LooksLikeTickerSymbol(q) {
		return items, total
	}
	sym := strings.ToUpper(q)
	for _, st := range items {
		if strings.EqualFold(st.Symbol, sym) {
			return items, total
		}
	}
	if !s.TossEnabled() {
		return items, total
	}

	s.triggerCatalogBackfill(sym)
	return items, total
}

func (s *QuotesService) applyLivePrices(
	ctx context.Context,
	base []model.QuoteSummary,
	watchSet, heldSet map[string]bool,
) []model.QuoteSummary {
	if len(base) == 0 {
		return base
	}

	if !s.TossEnabled() {
		s.overlayQuoteFlags(base, watchSet, heldSet)
		return base
	}

	priceMap, err := s.fetchPriceMap(ctx, base)
	if err != nil || len(priceMap) == 0 {
		s.overlayQuoteFlags(base, watchSet, heldSet)
		return base
	}

	out := make([]model.QuoteSummary, len(base))
	for i, item := range base {
		if p, ok := priceMap[item.Symbol]; ok {
			item.CurrentPrice = parseDecimal(p.LastPrice)
			if p.Currency != "" {
				item.Currency = modelCurrency(p.Currency)
			}
		}
		out[i] = item
	}

	fx := s.usdKrwRate(ctx)
	for i := range out {
		s.normalizeQuoteSummaryKRW(&out[i], fx)
	}
	s.overlayQuoteFlags(out, watchSet, heldSet)
	return out
}

func (s *QuotesService) fetchPriceMap(ctx context.Context, items []model.QuoteSummary) (map[string]toss.PriceResponse, error) {
	symbols := make([]string, len(items))
	for i, q := range items {
		symbols[i] = q.Symbol
	}

	if s.marketStore != nil {
		return s.marketStore.EnsurePrices(ctx, symbols), nil
	}

	return map[string]toss.PriceResponse{}, nil
}
