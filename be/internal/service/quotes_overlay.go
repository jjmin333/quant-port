package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
)

func (s *QuotesService) watchlistSymbols(ctx context.Context) []string {
	if s.watchlist == nil {
		return nil
	}
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return nil
	}
	symbols, err := s.watchlist.List(ctx, userID)
	if err != nil {
		return nil
	}
	return symbols
}

func (s *QuotesService) watchSymbolSet(ctx context.Context) map[string]bool {
	out := map[string]bool{}
	for _, sym := range s.watchlistSymbols(ctx) {
		out[strings.ToUpper(sym)] = true
	}
	return out
}

func (s *QuotesService) heldSymbolSet(ctx context.Context) map[string]bool {
	out := map[string]bool{}
	for _, sym := range s.heldSymbols(ctx) {
		out[strings.ToUpper(sym)] = true
	}
	return out
}

func (s *QuotesService) heldSymbols(ctx context.Context) []string {
	if s.portfolio == nil {
		return nil
	}
	return s.portfolio.HeldSymbols(ctx)
}

func (s *QuotesService) overlayQuoteFlags(items []model.QuoteSummary, watchSet, heldSet map[string]bool) {
	for i := range items {
		sym := strings.ToUpper(items[i].Symbol)
		items[i].IsWatchlisted = watchSet[sym]
		items[i].IsHeld = heldSet[sym]
	}
}

func filterQuotesByHeld(ctx context.Context, s *QuotesService, items []model.QuoteSummary, heldSet map[string]bool) []model.QuoteSummary {
	if len(heldSet) == 0 {
		return []model.QuoteSummary{}
	}
	var out []model.QuoteSummary
	for _, item := range items {
		if heldSet[strings.ToUpper(item.Symbol)] {
			out = append(out, item)
		}
	}
	if len(out) > 0 {
		return out
	}
	return s.quotesForSymbols(ctx, keys(heldSet), heldSet, heldSet)
}

func (s *QuotesService) quotesForSymbols(ctx context.Context, symbols []string, watchSet, heldSet map[string]bool) []model.QuoteSummary {
	out := make([]model.QuoteSummary, 0, len(symbols))
	seen := map[string]struct{}{}
	for _, sym := range symbols {
		sym = strings.TrimSpace(strings.ToUpper(sym))
		if sym == "" {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}

		if s.TossEnabled() {
			stock, err := s.toss.GetStock(ctx, sym)
			if err != nil || stock == nil {
				continue
			}
			q := model.QuoteSummary{
				Symbol:        stock.Symbol,
				Name:          stock.Name,
				Market:        stock.Market,
				Currency:      modelCurrency(stock.Currency),
				AssetType:     assetTypeFromMarket(stock.Market, stock.SecurityType),
				IsWatchlisted: watchSet[sym],
				IsHeld:        heldSet[sym],
			}
			if s.marketStore != nil {
				if v, ok := s.marketStore.EnsurePrice(ctx, sym); ok {
					q.CurrentPrice = v
				}
			} else if price, err := s.toss.GetPrice(ctx, sym); err == nil && price != nil {
				q.CurrentPrice = parseDecimal(price.LastPrice)
			}
			out = append(out, q)
			continue
		}

		out = append(out, zeroQuoteSummary(sym, watchSet, heldSet))
	}
	return out
}

func zeroQuoteSummary(sym string, watchSet, heldSet map[string]bool) model.QuoteSummary {
	return model.QuoteSummary{
		Symbol:        sym,
		Name:          sym,
		IsWatchlisted: watchSet[sym],
		IsHeld:        heldSet[sym],
	}
}

func assetTypeFromMarket(market, securityType string) model.QuoteAssetType {
	if strings.Contains(strings.ToUpper(securityType), "ETF") {
		return model.AssetETF
	}
	m := strings.ToUpper(market)
	if m == "KRX" || m == "KOSDAQ" {
		return model.AssetDomestic
	}
	return model.AssetUS
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
