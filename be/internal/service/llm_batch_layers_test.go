package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

// layerCatalog — 세 층 테스트가 함께 쓰는 카탈로그. 국내 주권 넷, 해외 하나,
// 파생상품 하나, 거래정지 하나.
func layerCatalog() map[string]model.Stock {
	return map[string]model.Stock{
		"005930": {Symbol: "005930", Name: "삼성전자", SecurityType: "주권", Status: "NORMAL", AssetType: model.AssetDomestic},
		"000660": {Symbol: "000660", Name: "SK하이닉스", SecurityType: "주권", Status: "NORMAL", AssetType: model.AssetDomestic},
		"035420": {Symbol: "035420", Name: "NAVER", SecurityType: "주권", Status: "NORMAL", AssetType: model.AssetDomestic},
		"051910": {Symbol: "051910", Name: "LG화학", SecurityType: "주권", Status: "NORMAL", AssetType: model.AssetDomestic},
		"NVDA":   {Symbol: "NVDA", Name: "NVIDIA", SecurityType: "COMMON_STOCK", Status: "NORMAL", AssetType: model.AssetUS},
		"122630": {Symbol: "122630", Name: "KODEX 레버리지", SecurityType: "ETF", Status: "NORMAL", AssetType: model.AssetETF},
		"900110": {Symbol: "900110", Name: "정리매매", SecurityType: "주권", Status: "DELISTED", AssetType: model.AssetDomestic},
	}
}

func layerSymbols(targets []model.LLMBatchTarget) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		out = append(out, target.Symbol)
	}
	return out
}

func assertOrder(t *testing.T, got []model.LLMBatchTarget, want ...string) {
	t.Helper()
	gotSymbols := layerSymbols(got)
	if len(gotSymbols) != len(want) {
		t.Fatalf("got %v, want %v", gotSymbols, want)
	}
	for i := range want {
		if gotSymbols[i] != want[i] {
			t.Fatalf("position %d = %s, want %s (full %v)", i, gotSymbols[i], want[i], gotSymbols)
		}
	}
}

func TestMergeScoreRanksKeepsCompositeTodayTop100(t *testing.T) {
	rows := []repository.RankedSymbol{
		// 종합 실시간 3위와 1일 5위 — 높은 쪽(3)이 남는다.
		{Axis: model.RankAxisComposite, Duration: model.RankDurationRealtime, Symbol: "005930", Rank: 3},
		{Axis: model.RankAxisComposite, Duration: model.RankDuration1D, Symbol: "005930", Rank: 5},
		{Axis: model.RankAxisComposite, Duration: model.RankDuration1D, Symbol: "000660", Rank: 1},
		// 빠지는 것: 다른 축 · 1주·1개월 · 100위 밖 · 빈 심볼 · 0순위.
		{Axis: model.RankAxisGrowth, Duration: model.RankDurationRealtime, Symbol: "035420", Rank: 1},
		{Axis: model.RankAxisComposite, Duration: model.RankDuration1W, Symbol: "051910", Rank: 1},
		{Axis: model.RankAxisComposite, Duration: model.RankDurationRealtime, Symbol: "NVDA", Rank: llmScoreTopN + 1},
		{Axis: model.RankAxisComposite, Duration: model.RankDurationRealtime, Symbol: "  ", Rank: 2},
		{Axis: model.RankAxisComposite, Duration: model.RankDurationRealtime, Symbol: "122630", Rank: 0},
	}
	got := mergeScoreRanks(rows)
	if len(got) != 2 || got[0].symbol != "000660" || got[1].symbol != "005930" || got[1].rank != 3 {
		t.Fatalf("got %+v, want [000660#1 005930#3]", got)
	}
}

func TestBuildScoreTargetsKeepsRankOrderAndSkipsIneligible(t *testing.T) {
	candidates := []scoreCandidate{
		{symbol: "051910", rank: 1},
		{symbol: "NVDA", rank: 2},    // 다른 시장
		{symbol: "122630", rank: 3},  // 파생상품
		{symbol: "900110", rank: 4},  // 거래정지
		{symbol: "MISSING", rank: 5}, // 카탈로그에 없음
		{symbol: "005930", rank: 6},
		{symbol: "000660", rank: 7}, // 1층이 이미 데려감
	}
	seen := map[string]bool{"000660": true}
	got := buildScoreTargets(candidates, layerCatalog(), toss.MarketKR, seen)
	assertOrder(t, got, "051910", "005930")
	if !seen["005930"] {
		t.Fatal("selected symbols must be recorded so a later layer skips them")
	}
	for _, target := range got {
		if target.Source != model.LLMBatchSourceScore {
			t.Fatalf("source = %q, want score", target.Source)
		}
	}
}

func TestBuildInterestTargetsTakesMissingAndStaleOnly(t *testing.T) {
	now := time.Date(2026, 9, 8, 18, 0, 0, 0, time.UTC)
	analyzed := map[string]time.Time{
		"005930": now.Add(-1 * time.Hour),               // 최근 — 건너뛴다
		"000660": now.Add(-llmInterestStaleAfter - time.Hour), // 일주일 넘음 — 담는다
		"051910": now.Add(-llmInterestStaleAfter * 2),   // 더 오래됨 — 앞에 선다
	}
	symbols := []string{"005930", "000660", "051910", "035420", "NVDA", "122630", "MISSING"}

	got := buildInterestTargets(symbols, layerCatalog(), toss.MarketKR, analyzed, map[string]bool{}, now)

	// 요약이 아예 없는 035420이 먼저, 그다음 오래된 순.
	assertOrder(t, got, "035420", "051910", "000660")
	for _, target := range got {
		if target.Source != model.LLMBatchSourceInterest {
			t.Fatalf("source = %q, want interest: %+v", target.Source, target)
		}
	}
}

func TestBuildInterestTargetsSplitsByMarket(t *testing.T) {
	now := time.Now()
	symbols := []string{"005930", "NVDA"}

	kr := buildInterestTargets(symbols, layerCatalog(), toss.MarketKR, map[string]time.Time{}, map[string]bool{}, now)
	us := buildInterestTargets(symbols, layerCatalog(), toss.MarketUS, map[string]time.Time{}, map[string]bool{}, now)

	assertOrder(t, kr, "005930")
	assertOrder(t, us, "NVDA")
}

func TestComposeTargetLayersGivesInterestAFloorAheadOfScore(t *testing.T) {
	radar := []model.LLMBatchTarget{{Symbol: "R1"}, {Symbol: "R2"}}
	score := []model.LLMBatchTarget{{Symbol: "S1"}, {Symbol: "S2"}}
	interest := []model.LLMBatchTarget{{Symbol: "I1"}, {Symbol: "I2"}, {Symbol: "I3"}}

	got := composeTargetLayers(radar, score, interest, 2)

	assertOrder(t, got, "R1", "R2", "I1", "I2", "S1", "S2", "I3")
}

func TestComposeTargetLayersHandlesShortInterestList(t *testing.T) {
	radar := []model.LLMBatchTarget{{Symbol: "R1"}}
	score := []model.LLMBatchTarget{{Symbol: "S1"}}
	interest := []model.LLMBatchTarget{{Symbol: "I1"}}

	// floor가 목록보다 크면 전부 앞으로 오고, 뒤 토막은 비어야 한다.
	got := composeTargetLayers(radar, score, interest, llmInterestFloor)

	assertOrder(t, got, "R1", "I1", "S1")
}

func TestCountBySourceLabelsEveryLayer(t *testing.T) {
	got := model.CountBySource([]model.LLMBatchTarget{
		{Symbol: "A", Source: model.LLMBatchSourceRadar},
		{Symbol: "B", Source: model.LLMBatchSourceScore},
		{Symbol: "C", Source: model.LLMBatchSourceScore},
		{Symbol: "D", Source: model.LLMBatchSourceInterest},
		{Symbol: "E", Source: model.LLMBatchSourcePinned},
		// 층 표시가 없는 옛 기록은 레이더로 센다.
		{Symbol: "F"},
	})

	want := map[string]int{
		model.LLMBatchSourceRadar:    2,
		model.LLMBatchSourceScore:    2,
		model.LLMBatchSourceInterest: 1,
		model.LLMBatchSourcePinned:   1,
	}
	for source, count := range want {
		if got[source] != count {
			t.Fatalf("%s = %d, want %d (full %v)", source, got[source], count, got)
		}
	}
}

func TestScoreRankingMarketForMatchesStoredKeys(t *testing.T) {
	if got := scoreRankingMarketFor(toss.MarketKR); got != "domestic" {
		t.Fatalf("KR = %q, want domestic", got)
	}
	if got := scoreRankingMarketFor(toss.MarketUS); got != "overseas" {
		t.Fatalf("US = %q, want overseas", got)
	}
	// 저장 키는 scoreRankingMarkets가 정의한 것과 같아야 한다 — 갈라지면 2순위가
	// 조용히 빈 목록을 읽는다.
	for _, market := range []string{"domestic", "overseas"} {
		if _, ok := scoreRankingMarkets[market]; !ok {
			t.Fatalf("%q is not a stored ranking market", market)
		}
	}
}
