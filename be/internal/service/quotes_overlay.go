package service

import (
	"context"
	"strings"
	"sync"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/stocklogo"
	"github.com/enact/quant-port/be/internal/toss"
)

// overlayCache memoizes the per-user watchlist / holdings lookups for the span
// of one fan-out. Both lookups hit MySQL (and Redis for holdings), and they were
// previously repeated once per symbol — up to 100x per batch quote request.
type overlayCache struct {
	watchOnce sync.Once
	watch     []string
	watchSet  map[string]bool

	heldOnce sync.Once
	held     []string
	heldSet  map[string]bool
}

type overlayCacheKeyType struct{}

var overlayCacheKey overlayCacheKeyType

// withOverlayCache attaches a fan-out scoped cache. It is deliberately not
// installed per-request in middleware: long-lived SSE handlers must keep seeing
// live watchlist/holdings changes.
func withOverlayCache(ctx context.Context) context.Context {
	if ctx.Value(overlayCacheKey) != nil {
		return ctx
	}
	return context.WithValue(ctx, overlayCacheKey, &overlayCache{})
}

func overlayCacheFrom(ctx context.Context) *overlayCache {
	c, _ := ctx.Value(overlayCacheKey).(*overlayCache)
	return c
}

func upperSet(symbols []string) map[string]bool {
	out := make(map[string]bool, len(symbols))
	for _, sym := range symbols {
		out[strings.ToUpper(sym)] = true
	}
	return out
}

func (s *QuotesService) watchlistSymbols(ctx context.Context) []string {
	if c := overlayCacheFrom(ctx); c != nil {
		c.watchOnce.Do(func() {
			c.watch = s.loadWatchlistSymbols(ctx)
			c.watchSet = upperSet(c.watch)
		})
		return c.watch
	}
	return s.loadWatchlistSymbols(ctx)
}

func (s *QuotesService) loadWatchlistSymbols(ctx context.Context) []string {
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
	if c := overlayCacheFrom(ctx); c != nil {
		s.watchlistSymbols(ctx) // populates the cache via watchOnce
		return c.watchSet
	}
	return upperSet(s.loadWatchlistSymbols(ctx))
}

func (s *QuotesService) heldSymbolSet(ctx context.Context) map[string]bool {
	if c := overlayCacheFrom(ctx); c != nil {
		s.heldSymbols(ctx) // populates the cache via heldOnce
		return c.heldSet
	}
	return upperSet(s.loadHeldSymbols(ctx))
}

func (s *QuotesService) heldSymbols(ctx context.Context) []string {
	if c := overlayCacheFrom(ctx); c != nil {
		c.heldOnce.Do(func() {
			c.held = s.loadHeldSymbols(ctx)
			c.heldSet = upperSet(c.held)
		})
		return c.held
	}
	return s.loadHeldSymbols(ctx)
}

func (s *QuotesService) loadHeldSymbols(ctx context.Context) []string {
	seen := map[string]struct{}{}
	var out []string
	appendSyms := func(symbols []string) {
		for _, sym := range symbols {
			sym = strings.ToUpper(strings.TrimSpace(sym))
			if sym == "" {
				continue
			}
			if _, dup := seen[sym]; dup {
				continue
			}
			seen[sym] = struct{}{}
			out = append(out, sym)
		}
	}
	if s.userHoldings != nil {
		appendSyms(s.userHoldings.HeldSymbols(ctx))
	}
	return out
}

func (s *QuotesService) overlayUserQuoteFlags(ctx context.Context, detail *model.QuoteDetail) {
	if detail == nil {
		return
	}
	watchSet := s.watchSymbolSet(ctx)
	heldSet := s.heldSymbolSet(ctx)
	sym := strings.ToUpper(strings.TrimSpace(detail.Symbol))
	detail.IsWatchlisted = watchSet[sym]
	detail.IsHeld = heldSet[sym]
}

func stripQuoteListForCache(result QuoteListResult) QuoteListResult {
	if len(result.Items) == 0 {
		return result
	}
	items := make([]model.QuoteSummary, len(result.Items))
	for i, item := range result.Items {
		items[i] = item
		items[i].IsWatchlisted = false
		items[i].IsHeld = false
		items[i].WatchlistMetrics = nil
	}
	return QuoteListResult{Items: items, Total: result.Total}
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
	seen := map[string]struct{}{}
	var ordered []string
	for _, sym := range symbols {
		sym = strings.TrimSpace(strings.ToUpper(sym))
		if sym == "" {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		ordered = append(ordered, sym)
	}
	if len(ordered) == 0 {
		return nil
	}
	ctx = withOverlayCache(ctx)

	// Resolve every price with one batched Redis read (plus at most one batched
	// upstream refresh) instead of a single-key lookup per goroutine.
	var prices map[string]toss.PriceResponse
	if s.marketStore != nil {
		prices = s.marketStore.EnsurePrices(ctx, ordered)
	}

	out := make([]model.QuoteSummary, len(ordered))
	var wg sync.WaitGroup
	sem := make(chan struct{}, watchlistMetricsParallelism)
	for i, sym := range ordered {
		wg.Add(1)
		go func(idx int, symbol string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[idx] = s.buildQuoteSummaryForSymbol(ctx, symbol, watchSet, heldSet, prices)
		}(i, sym)
	}
	wg.Wait()
	return out
}

func (s *QuotesService) buildQuoteSummaryForSymbol(
	ctx context.Context,
	sym string,
	watchSet, heldSet map[string]bool,
	prices map[string]toss.PriceResponse,
) model.QuoteSummary {
	// prefetchedPrice reads the batch result; it is authoritative when
	// marketStore is wired, so no per-symbol fallback lookup is needed.
	prefetchedPrice := func() (float64, bool) {
		p, ok := prices[sym]
		if !ok {
			return 0, false
		}
		v := parseDecimal(p.LastPrice)
		return v, v > 0
	}

	if s.TossEnabled() {
		stock, err := s.toss.GetStock(ctx, sym)
		if err != nil || stock == nil {
			q := zeroQuoteSummary(sym, watchSet, heldSet)
			if s.marketStore != nil {
				if v, ok := prefetchedPrice(); ok {
					q.CurrentPrice = v
				}
			} else if price, priceErr := s.toss.GetPrice(ctx, sym); priceErr == nil && price != nil {
				q.CurrentPrice = parseDecimal(price.LastPrice)
			}
			return q
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
			if v, ok := prefetchedPrice(); ok {
				q.CurrentPrice = v
			}
		} else if price, err := s.toss.GetPrice(ctx, sym); err == nil && price != nil {
			q.CurrentPrice = parseDecimal(price.LastPrice)
		}
		stocklogo.ApplyToQuoteSummary(&q)
		return q
	}

	return zeroQuoteSummary(sym, watchSet, heldSet)
}

func zeroQuoteSummary(sym string, watchSet, heldSet map[string]bool) model.QuoteSummary {
	q := model.QuoteSummary{
		Symbol:        sym,
		Name:          sym,
		IsWatchlisted: watchSet[sym],
		IsHeld:        heldSet[sym],
	}
	stocklogo.ApplyToQuoteSummary(&q)
	return q
}

func assetTypeFromMarket(market, securityType string) model.QuoteAssetType {
	if strings.Contains(strings.ToUpper(securityType), "ETF") {
		return model.AssetETF
	}
	if isDomesticKRMarket(market) {
		return model.AssetDomestic
	}
	return model.AssetUS
}

func isDomesticKRMarket(market string) bool {
	m := strings.ToUpper(strings.TrimSpace(market))
	switch m {
	case "KRX", "KOSPI", "KOSDAQ", "KOSDAQ GLOBAL", "KONEX":
		return true
	}
	return strings.Contains(m, "KRX") ||
		strings.Contains(m, "KOSDAQ") ||
		strings.Contains(m, "KOSPI")
}

func isDomesticKRSymbol(symbol string) bool {
	sym := strings.TrimSpace(symbol)
	if len(sym) != 6 {
		return false
	}
	for _, r := range sym {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

type listModeKeyType struct{}

var listModeKey listModeKeyType

/*
withListMode marks a context as serving a **list**, where per-symbol upstream
calls are not allowed.

목록은 항목 수만큼 이 경로를 탄다. 종목마다 업스트림을 부르면 길이에 그대로
비례하고, Toss 한도에 걸리면 초선형으로 나빠진다 — 실측으로 `/quotes`가
limit 5/10/20/40/100에서 0.18/1.9/6.4/58/90초였다. 관심 종목 화면이 이
엔드포인트를 5초마다 부른다.

이 표시가 붙으면 가격 해석은 **이미 받아 둔 배치 가격과 캐시·DB만** 쓴다.
상세 화면과 스트림은 이 표시를 붙이지 않으므로 예전 그대로 동작한다.
*/
func withListMode(ctx context.Context) context.Context {
	return context.WithValue(ctx, listModeKey, true)
}

// listMode reports whether the caller forbids per-symbol upstream fetches.
func listMode(ctx context.Context) bool {
	v, _ := ctx.Value(listModeKey).(bool)
	return v
}
