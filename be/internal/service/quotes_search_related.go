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

	/*
		관련 상품 카탈로그 보강은 **요청 경로에서 빼고 배경으로 보낸다.**

		이 함수는 후보 중 `stocks`에 없는 것을 Toss에서 받아 채운다. 그 호출이
		요청 안에 있으면 `/stocks/search` 한 번이 외부 API 왕복 하나를 포함하게
		되는데, 이 경로는 **비로그인도 부를 수 있는 공개 엔드포인트**다. 게다가
		그 호출은 `STOCK` 한도를 `tickFast`의 `prefetchStockInfo`와 나눠 쓰므로
		검색 트래픽이 시세 갱신을 밀어낼 수 있었다(실측 178.7ms → 60~67ms).

		대가는 **미국 종목을 처음 검색할 때 관련 ETF가 한 박자 늦게 보이는 것**뿐이다
		(국내 종목은 애초에 추측 후보가 허구라 보강할 것이 없다 —
		`relatedSymbolCandidates` 헤더 참조). 관련 ETF는 부가 정보이지 검색의
		본질이 아니고, 두 번째 검색부터는 DB에 있어 정상적으로 나온다.

		`runOnce`가 같은 심볼의 중복 보강을 접는다 — 같은 종목을 여럿이 동시에
		검색해도 상류 호출은 하나다.
	*/
	s.triggerRelatedCatalogBackfill(primary)

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
/*
triggerRelatedCatalogBackfill — 관련 상품 보강을 배경에서 한 번만 돌린다.

`triggerCatalogBackfill`과 같은 패턴이다(`quotes_background.go`). 요청의
컨텍스트를 쓰지 않는 것이 핵심 — 그 컨텍스트는 응답을 쓰는 순간 취소되므로,
그대로 넘기면 보강이 시작하자마자 끊긴다.
*/
func (s *QuotesService) triggerRelatedCatalogBackfill(primary model.Stock) {
	sym := strings.ToUpper(strings.TrimSpace(primary.Symbol))
	if sym == "" {
		return
	}
	s.initQuoteRefresh()
	s.quoteRefresh.runOnce("related-catalog:"+sym, func(ctx context.Context) {
		s.ensureRelatedCatalogFromToss(ctx, primary)
	})
}

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
