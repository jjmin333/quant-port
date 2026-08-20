package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/stocklogo"
	"github.com/enact/quant-port/be/internal/toss"
)

const defaultQuoteListLimit = 100

func stockToQuoteSummary(st model.Stock, watchSet, heldSet map[string]bool) model.QuoteSummary {
	sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
	q := model.QuoteSummary{
		Symbol:        st.Symbol,
		Name:          st.Name,
		Market:        st.Market,
		Currency:      st.Currency,
		AssetType:     st.AssetType,
		IsWatchlisted: watchSet[sym],
		IsHeld:        heldSet[sym],
	}
	stocklogo.ApplyToQuoteSummary(&q)
	return q
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
	if p.Limit > 0 && len(items) > 0 {
		if sym := pickSearchPrimary(items).Symbol; sym != "" && repository.ContainsHangul(p.Query) {
			s.triggerCatalogBackfill(sym)
		}
		expanded := s.expandSearchWithRelatedProducts(ctx, p.Query, p.Limit, items)
		if len(expanded) > len(items) {
			items = expanded
			if total < len(items) {
				total = len(items)
			}
		}
	}
	stocklogo.ApplyToStocks(items)
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

	// 이 아래는 항목마다 도는 경로다. 종목별 업스트림을 금지한다 — 배치로 받은
	// priceMap과 캐시·DB만 쓴다. 자세한 배경은 withListMode 주석.
	ctx = withListMode(ctx)

	priceMap, _ := s.fetchPriceMap(ctx, base)

	out := make([]model.QuoteSummary, len(base))
	for i, item := range base {
		if p, ok := priceMap[item.Symbol]; ok {
			if v := parseDecimal(p.LastPrice); v > 0 && item.CurrentPrice <= 0 {
				item.CurrentPrice = v
			}
			if p.Currency != "" {
				item.Currency = modelCurrency(p.Currency)
			}
		}
		s.enrichSummaryLivePrice(ctx, &item)
		out[i] = item
	}

	s.overlayQuoteFlags(out, watchSet, heldSet)
	return out
}

// enrichSummaryLivePrice overlays list quotes with the same live-price path used by
// GET /quotes/{symbol} (trades → book → forming bar), not the batch last-price cache alone.
func (s *QuotesService) enrichSummaryLivePrice(ctx context.Context, item *model.QuoteSummary) {
	if item == nil {
		return
	}
	sym := strings.ToUpper(strings.TrimSpace(item.Symbol))
	if sym == "" {
		return
	}
	// 여기서 poller.Touch()를 부르지 않는다.
	//
	// Touch는 "지금 누가 이 종목을 보고 있다"는 표시이고, 폴러는 그 표시가 붙은
	// 종목을 1초마다 **체결·호가까지 통째로** 다시 받는다. 그런데 이 함수는 목록의
	// 항목마다 호출된다 — 관심 종목 목록 한 번이 그 종목 전부를 "보는 중"으로
	// 만들었고, 목록은 체결·호가를 그리지도 않는다.
	//
	// 실측: 사용자 요청이 30초에 2건(헬스체크)뿐인데도 Toss `rate-limit exceeded`가
	// 분당 약 600건이었고, 분포는 /trades 145 · /orderbook 139(각 한도 15/s)였다.
	// 마지막 목록 요청으로부터 marketActiveTTL(5분)이 지난 뒤에야 126건/분으로
	// 떨어졌다 — 사용자가 계속 보고 있으면 영원히 포화된다는 뜻이다.
	//
	// market_poller.go의 tickFast 주석도 목록에는 이 처리를 하지 않는다고 적고
	// 있었다. 주석의 의도와 코드가 어긋나 있었다.
	//
	// 같은 이유로 ensurePriceMicrostructure도 부르지 않는다.
	//
	// 그 함수는 캐시에 없는 종목의 **체결과 호가를 업스트림에서 받아 온다**.
	// 목록은 그 둘을 그리지 않으므로 받아 봐야 쓰이는 곳이 없고, 종목마다
	// 한 번씩이라 목록 길이에 그대로 비례한다. Touch를 뗀 뒤로는 폴러가 미리
	// 데워 주지도 않아 매 요청이 그 값을 다시 치렀다 — 실측 limit=5/10/20/40에
	// 0.18/1.9/6.4/**58초**였다(Toss 한도에 걸리며 초선형으로 나빠진다).
	// 이 엔드포인트는 관심 종목 화면이 filter=watchlist로 쓰고 5초마다 폴링한다.
	//
	// 대신 아래 해석 경로가 캐시된 체결·호가를 **있으면** 쓰고, 없으면
	// applyLivePrices가 이미 받아 둔 배치 가격으로 떨어진다. 실제로 보고 있는
	// 종목(상세·스트림)은 폴러가 계속 갱신하므로 캐시가 살아 있어 정밀도가
	// 그대로다.
	//
	// 트레이드오프: 아무도 보고 있지 않은 종목은 목록에서 "최신 체결가"가 아니라
	// "배치 마지막가"로 보인다. 급변동 중이면 한두 틱 차이가 날 수 있다.

	fallback := item.CurrentPrice
	// EnsurePrice는 캐시에 없으면 업스트림을 부른다. 목록에서는 위 priceMap이
	// 이미 배치로 받아 왔으므로 여기서 다시 부를 이유가 없다.
	if fallback <= 0 && !listMode(ctx) && s.marketStore != nil {
		if v, ok := s.marketStore.EnsurePrice(ctx, sym); ok {
			fallback = v
		}
	}
	live := s.ResolveLiveQuotePrice(ctx, sym, fallback)
	if live <= 0 {
		return
	}
	item.CurrentPrice = live

	prevClose, ok := s.resolvePrevClose(ctx, sym)
	if !ok || prevClose <= 0 {
		return
	}
	detail := model.QuoteDetail{QuoteSummary: *item}
	s.applyQuoteChange(ctx, sym, live, prevClose, &detail)
	*item = detail.QuoteSummary
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
