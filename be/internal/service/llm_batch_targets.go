package service

import (
	"context"
	"sort"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

// Target selection for the scheduled AI 요약 batch.
//
// The universe is 마켓 레이더's 1일 기준 top 100, which the rankings poller
// already refreshes every 15 minutes into Redis and MySQL — selection reads
// that stored payload and never calls Toss, so choosing targets costs no
// external quota at all.
//
// Two ranking slots are merged per market (거래대금 + 거래량). They overlap by
// roughly a third, and 거래량 alone skews heavily toward leveraged ETFs, so the
// union after the derivative filter is a better "what actually traded today"
// list than either slot on its own.

// batchRankingTypes are the 마켓 레이더 slots a run draws from, in priority
// order: 거래대금 first, so a symbol appearing in both keeps the ordering the
// 레이더 default view shows.
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
		out = append(out, model.LLMBatchTarget{Symbol: stock.Symbol, Name: stock.Name, Pinned: true})
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

// selectBatchTargets builds one market's target list from the stored 레이더
// rankings. Pinned symbols outside the ranking are resolved separately, since
// the catalog lookup is driven by what the ranking returned.
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

	symbols := make([]string, 0, len(ranked)+len(pinned))
	for _, entry := range ranked {
		symbols = append(symbols, entry.symbol)
	}
	symbols = append(symbols, pinned...)

	catalog, err := s.catalogFor(ctx, symbols)
	if err != nil {
		return out, err
	}

	targets, excluded := filterBatchTargets(ranked, catalog)
	out.Excluded = excluded
	out.Targets = prependPinned(pinned, catalog, targets)
	return out, nil
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
