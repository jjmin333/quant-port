package service

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

/*
Target selection for the scheduled AI 요약 batch.

선정은 **저장분만 읽는다** — 레이더 랭킹은 폴러가 Redis·MySQL에 넣어 둔 것을,
AI 추천 랭킹은 00:25 배치가 계산해 둔 표를, 보유·관심은 사용자 표를 읽는다.
외부 API를 한 번도 부르지 않으므로 대상을 고르는 데 드는 쿼터는 0이다.

# 세 층 (2026-09-08, 2층은 2026-10-10 사용자 결정으로 바뀜)

	순위  층                      무엇을 데려오나                                   회전
	1    실시간 차트(레이더)        거래대금 top 100 ∪ 거래량 top 100 (ETF·ETN 제외)   매일 같은 얼굴이 정상
	2    AI 추천 랭킹              종합축 · 오늘 목록 top 100 (1층과 겹치면 뺀다)       매일 같은 얼굴
	3    사용자 보유·관심          요약이 없거나 일주일 넘은 것                        생기는 대로

1층 둘은 3분의 1쯤 겹치고 레버리지 ETF가 많아, 걸러내면 실측 약 109종목이다(2026-10-09 국내).
2층은 예전에 6축 × 4기간 합집합(국내 약 680)을 오래된 순으로 돌며 상한까지 채웠다 — 대상 수가 상한(350)
그 자체였고 KR 키 토큰이 하루 96%였다. 지금은 사용자가 아는 「100 + 100 + 100」 그대로다.

# 왜 3층에 최소 보장분이 있나

`applyBudget`은 목록을 **앞에서부터** 자른다. 3층을 순서대로 맨 뒤에 두면 예산이
조금만 줄어도 가장 먼저 잘리는 자리인데, 1·2층은 목록이라 한 종목이 빠져도 옆
종목이 보이는 반면 **보유 종목은 그 사용자에게 대체재가 없다.** 그래서 3층의 앞
llmInterestFloor개를 2층보다 앞에 세운다 — 나머지는 예정대로 꼬리에 붙는다.

# 건너뛴 종목의 화면은 비지 않는다

회전에서 오늘 차례가 아닌 종목도 화면은 그대로다. 숫자는 매 빌드가 다시 계산하고
글은 `carryOverNarrative`가 지난 것을 이어 받으며, 그 글이 언제 쓰였는지는
`analyzedAt`이 말한다. 「어제 글 + 오늘 숫자」라고 정직하게 보여주는 구조라서
회전이 성립한다.

# 남는 축

Two ranking slots are merged per market (거래대금 + 거래량). They overlap by
roughly a third, and 거래량 alone skews heavily toward leveraged ETFs, so the
union after the derivative filter is a better "what actually traded today"
list than either slot on its own.
*/

// batchRankingTypes are the 마켓 레이더 slots a run draws from, in priority
// order: 거래대금 first, so a symbol appearing in both keeps the ordering the
// 레이더 default view shows.
const (
	// llmInterestFloor — 3순위(보유·관심)가 2순위보다 앞에 확보하는 자리 수.
	// 예산이 줄어도 여기까지는 잘리지 않는다.
	llmInterestFloor = 30

	// llmInterestStaleAfter — 보유·관심 종목의 글을 다시 쓰는 주기. 주 1회다.
	llmInterestStaleAfter = 7 * 24 * time.Hour

)

var batchRankingTypes = []toss.RankingType{
	toss.RankingMarketTradingAmount,
	toss.RankingMarketTradingVolume,
}

// derivativeSecurityMarkers match `stocks.security_type` values that are not
// individual companies. Substring matching covers the variants the catalog
// actually carries (ETF, ETN, ETP, FOREIGN_ETF) without enumerating them.
var derivativeSecurityMarkers = []string{"ETF", "ETN", "ETP"}

// isDerivativeProduct reports whether a stock row is a pooled/derivative
// product rather than a single company. model.IsFundamentalsApplicable is
// deliberately not reused here: it only knows ETF and ETN, and the US top 100
// carries a double-digit number of ETPs that would otherwise slip through.
func isDerivativeProduct(stock model.Stock) bool {
	if stock.AssetType == model.AssetETF {
		return true
	}
	sec := strings.ToUpper(strings.TrimSpace(stock.SecurityType))
	for _, marker := range derivativeSecurityMarkers {
		if strings.Contains(sec, marker) {
			return true
		}
	}
	return false
}

// isTradableStatus reports whether the symbol is currently listed and trading.
// An empty status means the catalog never recorded one — treated as tradable,
// since the ranking itself is evidence that it traded today.
func isTradableStatus(stock model.Stock) bool {
	status := strings.ToUpper(strings.TrimSpace(stock.Status))
	return status == "" || status == "NORMAL"
}

// rankedSymbol is one symbol's best position across the merged ranking slots.
type rankedSymbol struct {
	symbol string
	rank   int
}

// mergeRankedSymbols folds several ranking payloads into one list ordered by
// best rank. Symbols in more than one slot keep their strongest position, so
// the head of the list is what traded most heavily by either measure.
func mergeRankedSymbols(payloads []rankingsStorePayload) []rankedSymbol {
	best := make(map[string]int)
	for _, payload := range payloads {
		for _, item := range payload.Rankings {
			symbol := strings.ToUpper(strings.TrimSpace(item.Symbol))
			if symbol == "" || item.Rank <= 0 {
				continue
			}
			if prev, ok := best[symbol]; !ok || item.Rank < prev {
				best[symbol] = item.Rank
			}
		}
	}

	out := make([]rankedSymbol, 0, len(best))
	for symbol, rank := range best {
		out = append(out, rankedSymbol{symbol: symbol, rank: rank})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].rank != out[j].rank {
			return out[i].rank < out[j].rank
		}
		return out[i].symbol < out[j].symbol
	})
	return out
}

// filterBatchTargets drops everything that is not an individually analysable
// listed company and reports why, so the admin page can show "100 ranked → 72
// analysed" with the difference accounted for.
func filterBatchTargets(ranked []rankedSymbol, catalog map[string]model.Stock) ([]model.LLMBatchTarget, map[string]int) {
	targets := make([]model.LLMBatchTarget, 0, len(ranked))
	excluded := make(map[string]int)

	for _, entry := range ranked {
		stock, ok := catalog[entry.symbol]
		if !ok {
			excluded[model.LLMBatchExcludeUnlisted]++
			continue
		}
		if isDerivativeProduct(stock) {
			excluded[model.LLMBatchExcludeDerivative]++
			continue
		}
		if !isTradableStatus(stock) {
			excluded[model.LLMBatchExcludeSuspended]++
			continue
		}
		targets = append(targets, model.LLMBatchTarget{
			Symbol: stock.Symbol,
			Name:   stock.Name,
			Rank:   entry.rank,
			Source: model.LLMBatchSourceRadar,
		})
	}
	return targets, excluded
}

// prependPinned puts the admin's always-include symbols at the head of the
// list, ahead of the ranking. They are the ones an admin explicitly cares
// about, so they must survive a budget cut that trims the tail.
func prependPinned(pinned []string, catalog map[string]model.Stock, targets []model.LLMBatchTarget) []model.LLMBatchTarget {
	if len(pinned) == 0 {
		return targets
	}
	seen := make(map[string]bool, len(targets))
	out := make([]model.LLMBatchTarget, 0, len(targets)+len(pinned))
	for _, symbol := range pinned {
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		stock, ok := catalog[symbol]
		if !ok || seen[symbol] {
			continue
		}
		seen[symbol] = true
		out = append(out, model.LLMBatchTarget{
			Symbol: stock.Symbol,
			Name:   stock.Name,
			Pinned: true,
			Source: model.LLMBatchSourcePinned,
		})
	}
	for _, target := range targets {
		if seen[target.Symbol] {
			continue
		}
		seen[target.Symbol] = true
		out = append(out, target)
	}
	return out
}

/*
selectBatchTargets builds one market's target list — 세 층을 한 줄로 세운다.

	[관리자 핀] [1순위 레이더] [3순위 보유·관심 앞 30] [2순위 AI 랭킹 회전] [3순위 나머지]

3순위가 두 토막으로 나뉘는 이유는 파일 머리글의 「최소 보장분」 절에 있다.
`Considered`와 `Excluded`는 **1순위에 대해서만** 센다 — 관리자 화면의 「155개 중
106개 분석」이 뜻하는 바가 레이더 목록이고, 여기에 2·3순위의 탈락까지 섞으면 그
숫자가 무엇을 재는지 알 수 없게 된다. 층별 기여는 `BySource`가 따로 말한다.
*/
func (s *StockSummaryService) selectBatchTargets(
	ctx context.Context,
	market toss.MarketCountry,
	pinned []string,
) (model.LLMBatchSelection, error) {
	out := model.LLMBatchSelection{Market: string(market)}

	payloads := make([]rankingsStorePayload, 0, len(batchRankingTypes))
	for _, rankingType := range batchRankingTypes {
		payload, ok := s.loadBatchRankings(ctx, market, rankingType)
		if !ok {
			continue
		}
		if out.RankedAt == "" && payload.TossRankedAt != nil {
			out.RankedAt = *payload.TossRankedAt
		}
		payloads = append(payloads, payload)
	}

	ranked := mergeRankedSymbols(payloads)
	out.Considered = len(ranked)

	scoreRanked := s.loadScoreRankCandidates(ctx, market)
	interested := s.loadInterestSymbols(ctx)

	// 카탈로그는 세 층의 심볼을 **한 번에** 조회한다. 층마다 부르면 같은 표를 세 번
	// 훑게 되고, 겹치는 종목이 많아 대부분이 중복 조회다.
	symbols := make([]string, 0, len(ranked)+len(scoreRanked)+len(interested)+len(pinned))
	for _, entry := range ranked {
		symbols = append(symbols, entry.symbol)
	}
	for _, entry := range scoreRanked {
		symbols = append(symbols, entry.symbol)
	}
	symbols = append(symbols, interested...)
	symbols = append(symbols, pinned...)

	catalog, err := s.catalogFor(ctx, symbols)
	if err != nil {
		return out, err
	}

	targets, excluded := filterBatchTargets(ranked, catalog)
	out.Excluded = excluded

	// 이미 앞 층이 데려간 종목은 뒤 층에서 뺀다. 레이더 상위와 AI 랭킹 상위는 크게
	// 겹치므로(실측 국내 681 중 109) 이 걸러내기가 2순위 몫의 대부분을 정한다.
	seen := make(map[string]bool, len(targets))
	for _, target := range targets {
		seen[target.Symbol] = true
	}

	analyzed := s.narrativeAnalyzedAt(ctx)
	interestTargets := buildInterestTargets(interested, catalog, market, analyzed, seen, time.Now())
	scoreTargets := buildScoreTargets(scoreRanked, catalog, market, seen)

	ordered := composeTargetLayers(targets, scoreTargets, interestTargets, llmInterestFloor)
	out.Targets = prependPinned(pinned, catalog, ordered)
	out.BySource = model.CountBySource(out.Targets)
	return out, nil
}

/*
composeTargetLayers — 세 층을 우선순위 순서로 한 줄에 세운다.

	[1순위 레이더] [3순위 앞 floor개] [2순위 AI 랭킹] [3순위 나머지]

`applyBudget`이 이 목록을 **앞에서부터** 자르므로 순서가 곧 우선순위다. 3순위를 두
토막으로 나누는 것이 「최소 보장분」의 전부다 — 예산 계산에 예외를 넣는 대신 자리를
앞에 잡아 준다. 예산이 그 앞(1순위)에서 이미 바닥나면 보장분도 함께 잘리는데, 그건
의도한 결과다: 오늘 실제로 거래된 종목보다 앞에 설 층은 없다.
*/
func composeTargetLayers(radar, score, interest []model.LLMBatchTarget, floor int) []model.LLMBatchTarget {
	if floor < 0 {
		floor = 0
	}
	if floor > len(interest) {
		floor = len(interest)
	}
	out := make([]model.LLMBatchTarget, 0, len(radar)+len(score)+len(interest))
	out = append(out, radar...)
	out = append(out, interest[:floor]...)
	out = append(out, score...)
	out = append(out, interest[floor:]...)
	return out
}

/*
2순위 · AI 추천 랭킹 종합 top 100 (2026-10-10 사용자 결정 — 「거래대금 100 · 거래량 100 · AI 랭킹 100」).

예전에는 6축 × 4기간 저장분의 합집합(국내 약 680종목)을 오래된 순으로 **회전**해 상한(350)까지 채웠다 —
그래서 대상 수가 사용자가 아는 300이 아니라 상한 그 자체였고, KR 키 토큰이 하루 96%까지 찼다.
이제는 첫 화면이 보여 주는 **종합축 · 오늘 목록(실시간·1일) 상위 100**만, 순위대로 매일 다시 쓴다.
다른 축·기간 탭 종목은 그 종목을 다시 고를 때까지 지난 글을 이어 쓴다(carryOverNarrative · analyzedAt).
*/
const llmScoreTopN = 100

// scoreCandidate is one symbol's best 종합 순위 in today's lists.
type scoreCandidate struct {
	symbol string
	rank   int
}

// loadScoreRankCandidates reads the stored 종합 ranking of one market. Returns
// nil when the ranking batch has not run yet, which leaves the 2순위 layer empty.
func (s *StockSummaryService) loadScoreRankCandidates(
	ctx context.Context,
	market toss.MarketCountry,
) []scoreCandidate {
	if s.scoreRankings == nil {
		return nil
	}
	rows, err := s.scoreRankings.ListSymbolRanks(ctx, scoreRankingMarketFor(market))
	if err != nil {
		// 2순위가 없으면 1·3순위로 돈다 — 대상 선정 전체를 실패시킬 일이 아니다.
		log.Printf("llm batch %s: score rankings unavailable: %v", market, err)
		return nil
	}
	return mergeScoreRanks(rows)
}

// mergeScoreRanks keeps 종합축 · 오늘 목록(실시간·1일)의 상위 llmScoreTopN, symbol마다 더 높은 순위로.
func mergeScoreRanks(rows []repository.RankedSymbol) []scoreCandidate {
	best := make(map[string]int, llmScoreTopN)
	for _, row := range rows {
		symbol := strings.ToUpper(strings.TrimSpace(row.Symbol))
		today := row.Duration == model.RankDurationRealtime || row.Duration == model.RankDuration1D
		if symbol == "" || row.Rank <= 0 || row.Rank > llmScoreTopN || row.Axis != model.RankAxisComposite || !today {
			continue
		}
		if cur, ok := best[symbol]; !ok || row.Rank < cur {
			best[symbol] = row.Rank
		}
	}
	out := make([]scoreCandidate, 0, len(best))
	for symbol, rank := range best {
		out = append(out, scoreCandidate{symbol: symbol, rank: rank})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].rank != out[j].rank {
			return out[i].rank < out[j].rank
		}
		return out[i].symbol < out[j].symbol
	})
	return out
}

// buildScoreTargets — 순위 순. 1순위(실시간 차트)가 이미 데려간 종목과 다른 시장·파생상품은 뺀다.
func buildScoreTargets(
	candidates []scoreCandidate,
	catalog map[string]model.Stock,
	market toss.MarketCountry,
	seen map[string]bool,
) []model.LLMBatchTarget {
	out := make([]model.LLMBatchTarget, 0, len(candidates))
	for _, candidate := range candidates {
		stock, ok := eligibleStock(candidate.symbol, catalog, market, seen)
		if !ok {
			continue
		}
		seen[candidate.symbol] = true
		out = append(out, model.LLMBatchTarget{Symbol: stock.Symbol, Name: stock.Name, Source: model.LLMBatchSourceScore})
	}
	return out
}

/*
3순위 · 사용자 보유·관심.

「요약이 없는 것」만이 아니라 **일주일 넘은 것**까지 담는다. 한 번 메우고 나면 이
층은 거의 비므로(실측 보유 208종목 중 190개가 미분석이었고, 메우고 나면 새로 담긴
종목만 남는다) 주 1회 갱신을 보장해도 하루 몇십 건을 넘지 않는다.

순서는 요약 없는 것 먼저, 그다음 오래된 순이다 — 빈 화면이 오래된 화면보다 급하다.
*/
func buildInterestTargets(
	symbols []string,
	catalog map[string]model.Stock,
	market toss.MarketCountry,
	analyzed map[string]time.Time,
	seen map[string]bool,
	now time.Time,
) []model.LLMBatchTarget {
	type entry struct {
		target model.LLMBatchTarget
		at     time.Time
		has    bool
	}

	rows := make([]entry, 0, len(symbols))
	for _, raw := range symbols {
		symbol := strings.ToUpper(strings.TrimSpace(raw))
		stock, ok := eligibleStock(symbol, catalog, market, seen)
		if !ok {
			continue
		}
		at, has := analyzed[symbol]
		if has && now.Sub(at) < llmInterestStaleAfter {
			continue
		}
		seen[symbol] = true
		rows = append(rows, entry{
			target: model.LLMBatchTarget{
				Symbol: stock.Symbol,
				Name:   stock.Name,
				Source: model.LLMBatchSourceInterest,
			},
			at:  at,
			has: has,
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].has != rows[j].has {
			return !rows[i].has
		}
		if !rows[i].at.Equal(rows[j].at) {
			return rows[i].at.Before(rows[j].at)
		}
		return rows[i].target.Symbol < rows[j].target.Symbol
	})

	out := make([]model.LLMBatchTarget, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.target)
	}
	return out
}

// eligibleStock applies the same "individually analysable listed company of
// this market" test the 레이더 layer uses, plus the cross-layer dedupe.
//
// 시장 확인이 여기에만 있는 이유: 레이더 목록은 애초에 시장별로 받아 오지만 AI 랭킹의
// 「해외」 묶음에는 국내 ETF가 함께 들어가고(scoreRankingMarkets), 보유·관심은 두
// 시장이 한 목록에 섞여 있다.
func eligibleStock(
	symbol string,
	catalog map[string]model.Stock,
	market toss.MarketCountry,
	seen map[string]bool,
) (model.Stock, bool) {
	if symbol == "" || seen[symbol] {
		return model.Stock{}, false
	}
	stock, ok := catalog[symbol]
	if !ok || isDerivativeProduct(stock) || !isTradableStatus(stock) {
		return model.Stock{}, false
	}
	if marketForStock(stock) != market {
		return model.Stock{}, false
	}
	return stock, true
}

// scoreRankingMarketFor maps a ranking market onto the key AI 추천 랭킹 stores
// its leaderboards under (scoreRankingMarkets).
func scoreRankingMarketFor(market toss.MarketCountry) string {
	if market == toss.MarketUS {
		return "overseas"
	}
	return "domestic"
}

// loadInterestSymbols is every symbol some user holds or watches, both markets
// mixed — buildInterestTargets splits them by catalog.
func (s *StockSummaryService) loadInterestSymbols(ctx context.Context) []string {
	out := make([]string, 0, 64)
	if s.holdings != nil {
		symbols, err := s.holdings.ListAllSymbols(ctx)
		if err != nil {
			log.Printf("llm batch: holdings symbols: %v", err)
		} else {
			out = append(out, symbols...)
		}
	}
	if s.watchlist != nil {
		symbols, err := s.watchlist.ListAllSymbols(ctx)
		if err != nil {
			log.Printf("llm batch: watchlist symbols: %v", err)
		} else {
			out = append(out, symbols...)
		}
	}
	return out
}

// narrativeAnalyzedAt reads when each symbol's prose was last written. An empty
// map (no store, or a failed read) makes every candidate look unanalysed, which
// degrades the rotation to "best rank first" rather than breaking it.
func (s *StockSummaryService) narrativeAnalyzedAt(ctx context.Context) map[string]time.Time {
	if s.narratives == nil {
		return map[string]time.Time{}
	}
	analyzed, err := s.narratives.AnalyzedAt(ctx)
	if err != nil {
		log.Printf("llm batch: narrative analyzed_at: %v", err)
		return map[string]time.Time{}
	}
	return analyzed
}

// catalogFor resolves symbols to catalog rows in one query, keyed uppercase.
func (s *StockSummaryService) catalogFor(ctx context.Context, symbols []string) (map[string]model.Stock, error) {
	if s.stocks == nil || len(symbols) == 0 {
		return map[string]model.Stock{}, nil
	}
	rows, err := s.stocks.ListBySymbols(ctx, symbols)
	if err != nil {
		return nil, err
	}
	out := make(map[string]model.Stock, len(rows))
	for _, row := range rows {
		out[strings.ToUpper(strings.TrimSpace(row.Symbol))] = row
	}
	return out, nil
}

// loadBatchRankings reads one stored ranking slot, Redis first then MySQL —
// the same order the 레이더 read path uses, and never a live Toss call.
func (s *StockSummaryService) loadBatchRankings(
	ctx context.Context,
	market toss.MarketCountry,
	rankingType toss.RankingType,
) (rankingsStorePayload, bool) {
	params := toss.GetRankingsParams{
		MarketCountry: market,
		Type:          rankingType,
		Duration:      toss.Ranking1d,
		Count:         rankingsPollFetchCount,
	}
	if payload, ok := loadRankingsPayloadFromRedis(ctx, s.redis, params); ok && len(payload.Rankings) > 0 {
		return payload, true
	}
	if s.rankings == nil {
		return rankingsStorePayload{}, false
	}
	row, err := s.rankings.Get(ctx, string(market), string(rankingType), string(toss.Ranking1d))
	if err != nil || row == nil || len(row.Payload) == 0 {
		return rankingsStorePayload{}, false
	}
	payload, ok := decodeRankingsStore(row.Payload)
	return payload, ok && len(payload.Rankings) > 0
}
