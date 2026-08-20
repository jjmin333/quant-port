package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

func rankingPayload(symbols ...string) rankingsStorePayload {
	items := make([]toss.RankingItem, 0, len(symbols))
	for i, symbol := range symbols {
		items = append(items, toss.RankingItem{Rank: i + 1, Symbol: symbol})
	}
	return rankingsStorePayload{Rankings: items}
}

func TestMergeRankedSymbolsKeepsBestRank(t *testing.T) {
	// 000660 is 1st by 거래대금 and 3rd by 거래량; the stronger position wins.
	got := mergeRankedSymbols([]rankingsStorePayload{
		rankingPayload("000660", "005930", "035420"),
		rankingPayload("122630", "252670", "000660"),
	})

	want := []rankedSymbol{
		{symbol: "000660", rank: 1},
		{symbol: "122630", rank: 1},
		{symbol: "005930", rank: 2},
		{symbol: "252670", rank: 2},
		{symbol: "035420", rank: 3},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d symbols, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("position %d = %+v, want %+v (full: %+v)", i, got[i], want[i], got)
		}
	}
}

func TestMergeRankedSymbolsIgnoresBlanksAndBadRanks(t *testing.T) {
	payload := rankingsStorePayload{Rankings: []toss.RankingItem{
		{Rank: 1, Symbol: "005930"},
		{Rank: 0, Symbol: "000660"},
		{Rank: 2, Symbol: "   "},
		{Rank: 3, Symbol: "aapl"},
	}}
	got := mergeRankedSymbols([]rankingsStorePayload{payload})
	if len(got) != 2 {
		t.Fatalf("got %+v, want 2 usable symbols", got)
	}
	if got[1].symbol != "AAPL" {
		t.Fatalf("symbol not uppercased: %+v", got[1])
	}
}

func TestFilterBatchTargetsExcludesDerivatives(t *testing.T) {
	catalog := map[string]model.Stock{
		"005930": {Symbol: "005930", Name: "삼성전자", SecurityType: "주권", Status: "NORMAL"},
		"122630": {Symbol: "122630", Name: "KODEX 레버리지", SecurityType: "ETF", Status: "NORMAL"},
		"530063": {Symbol: "530063", Name: "ETN 상품", SecurityType: "ETN", Status: "NORMAL"},
		"MSTU":   {Symbol: "MSTU", Name: "MSTU", SecurityType: "ETP", Status: "NORMAL"},
		"SPY":    {Symbol: "SPY", Name: "SPY", SecurityType: "FOREIGN_ETF", Status: "NORMAL"},
		"TSM":    {Symbol: "TSM", Name: "TSM", SecurityType: "DEPOSITARY_RECEIPT", Status: "NORMAL"},
		"DEAD":   {Symbol: "DEAD", Name: "정리매매", SecurityType: "STOCK", Status: "DELISTED"},
		"QQQ":    {Symbol: "QQQ", Name: "QQQ", SecurityType: "", AssetType: model.AssetETF, Status: "NORMAL"},
	}
	ranked := []rankedSymbol{
		{symbol: "005930", rank: 1},
		{symbol: "122630", rank: 2},
		{symbol: "530063", rank: 3},
		{symbol: "MSTU", rank: 4},
		{symbol: "SPY", rank: 5},
		{symbol: "TSM", rank: 6},
		{symbol: "DEAD", rank: 7},
		{symbol: "QQQ", rank: 8},
		{symbol: "GHOST", rank: 9},
	}

	targets, excluded := filterBatchTargets(ranked, catalog)

	wantSymbols := []string{"005930", "TSM"}
	if len(targets) != len(wantSymbols) {
		t.Fatalf("targets = %+v, want %v", targets, wantSymbols)
	}
	for i, symbol := range wantSymbols {
		if targets[i].Symbol != symbol {
			t.Fatalf("target %d = %s, want %s", i, targets[i].Symbol, symbol)
		}
	}
	// ETF, ETN, ETP, FOREIGN_ETF, and the asset-type-only ETF all count.
	if got := excluded[model.LLMBatchExcludeDerivative]; got != 5 {
		t.Fatalf("derivative exclusions = %d, want 5 (%v)", got, excluded)
	}
	if got := excluded[model.LLMBatchExcludeSuspended]; got != 1 {
		t.Fatalf("suspended exclusions = %d, want 1 (%v)", got, excluded)
	}
	if got := excluded[model.LLMBatchExcludeUnlisted]; got != 1 {
		t.Fatalf("unlisted exclusions = %d, want 1 (%v)", got, excluded)
	}
}

func TestFilterBatchTargetsKeepsBlankStatus(t *testing.T) {
	// The catalog does not always carry a status; appearing in today's
	// ranking is itself evidence the symbol trades.
	catalog := map[string]model.Stock{
		"035420": {Symbol: "035420", Name: "NAVER", SecurityType: "주권"},
	}
	targets, excluded := filterBatchTargets([]rankedSymbol{{symbol: "035420", rank: 1}}, catalog)
	if len(targets) != 1 {
		t.Fatalf("targets = %+v, excluded = %v", targets, excluded)
	}
}

func TestPrependPinnedLeadsAndDedupes(t *testing.T) {
	catalog := map[string]model.Stock{
		"005930": {Symbol: "005930", Name: "삼성전자"},
		"000660": {Symbol: "000660", Name: "SK하이닉스"},
	}
	targets := []model.LLMBatchTarget{
		{Symbol: "035420", Name: "NAVER", Rank: 1},
		{Symbol: "005930", Name: "삼성전자", Rank: 2},
	}

	got := prependPinned([]string{"000660", "005930", "UNKNOWN"}, catalog, targets)

	want := []string{"000660", "005930", "035420"}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %v", got, want)
	}
	for i, symbol := range want {
		if got[i].Symbol != symbol {
			t.Fatalf("position %d = %s, want %s (full %+v)", i, got[i].Symbol, symbol, got)
		}
	}
	if !got[0].Pinned || !got[1].Pinned {
		t.Fatalf("pinned symbols must be flagged: %+v", got)
	}
	if got[2].Pinned {
		t.Fatalf("ranking symbol must not be flagged pinned: %+v", got[2])
	}
}
