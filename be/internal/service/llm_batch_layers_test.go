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

func TestScoreDurationWeightRanksTodayFirst(t *testing.T) {
	// realtime과 1d는 실측으로 같은 목록이라 같은 무게다. 1개월 10위(16)는 실시간
	// 20위(20)보다 앞이고 실시간 5위(5)보다는 뒤여야 한다.
	if w := scoreDurationWeight(model.RankDurationRealtime); w != scoreDurationWeight(model.RankDuration1D) {
		t.Fatalf("realtime %v and 1d %v must weigh the same", w, scoreDurationWeight(model.RankDuration1D))
	}
	month := 10 * scoreDurationWeight(model.RankDuration1M)
	if !(month < 20*scoreDurationWeight(model.RankDurationRealtime)) {
		t.Fatalf("1개월 10위(%v) should outrank 실시간 20위", month)
	}
	if !(month > 5*scoreDurationWeight(model.RankDurationRealtime)) {
		t.Fatalf("1개월 10위(%v) should rank behind 실시간 5위", month)
	}
	if scoreDurationWeight(model.RankDuration1W) >= scoreDurationWeight(model.RankDuration1M) {
		t.Fatal("1주는 1개월보다 가벼워야 한다")
	}
}

func TestMergeScoreRanksKeepsBestWeightedPosition(t *testing.T) {
	rows := []repository.RankedSymbol{
		// 035420: 1개월 1위(1.6)와 실시간 40위(40) — 강한 쪽이 남는다.
		{Axis: model.RankAxisGrowth, Duration: model.RankDuration1M, Symbol: "035420", Rank: 1},
		{Axis: model.RankAxisGrowth, Duration: model.RankDurationRealtime, Symbol: "035420", Rank: 40},
		// 005930: 종합 실시간 3위 — head이면서 가장 앞이다.
		{Axis: model.RankAxisComposite, Duration: model.RankDurationRealtime, Symbol: "005930", Rank: 3},
		// 000660: 종합 1주 2위 — 종합축이지만 오늘 목록이 아니라 head가 아니다.
		{Axis: model.RankAxisComposite, Duration: model.RankDuration1W, Symbol: "000660", Rank: 2},
		// 잡음: 빈 심볼과 0순위는 버린다.
		{Axis: model.RankAxisQuality, Duration: model.RankDuration1D, Symbol: "  ", Rank: 1},
		{Axis: model.RankAxisQuality, Duration: model.RankDuration1D, Symbol: "051910", Rank: 0},
	}

	got := mergeScoreRanks(rows)

	if len(got) != 3 {
		t.Fatalf("got %+v, want 3 candidates", got)
	}
	// 정렬은 가중 순위만 본다: 035420(1×1.6) · 000660(2×1.3) · 005930(3×1.0).
	// head는 buildScoreTargets가 순서를 뒤집을 때 쓰는 표시일 뿐이다.
	if got[0].symbol != "035420" || got[0].weighted != 1.6 {
		t.Fatalf("head = %+v, want 035420 at 1.6", got[0])
	}
	if got[1].symbol != "000660" || got[1].head {
		t.Fatalf("000660 must not be head (1주 목록): %+v", got[1])
	}
	if got[2].symbol != "005930" || !got[2].head {
		t.Fatalf("005930 should be marked head: %+v", got[2])
	}
}

func TestMergeScoreRanksHeadOnlyWithinDepth(t *testing.T) {
	rows := []repository.RankedSymbol{
		{Axis: model.RankAxisComposite, Duration: model.RankDurationRealtime, Symbol: "005930", Rank: llmScoreHeadDepth},
		{Axis: model.RankAxisComposite, Duration: model.RankDurationRealtime, Symbol: "000660", Rank: llmScoreHeadDepth + 1},
	}
	got := mergeScoreRanks(rows)
	if !got[0].head {
		t.Fatalf("rank %d should be head: %+v", llmScoreHeadDepth, got[0])
	}
	if got[1].head {
		t.Fatalf("rank %d is past the head depth: %+v", llmScoreHeadDepth+1, got[1])
	}
}

func TestBuildScoreTargetsOrdersHeadThenUnanalysedThenOldest(t *testing.T) {
	now := time.Date(2026, 9, 8, 18, 0, 0, 0, time.UTC)
	candidates := []scoreCandidate{
		{symbol: "005930", weighted: 2, head: true},                 // ① 매일 갱신
		{symbol: "035420", weighted: 50},                            // ② 요약 없음
		{symbol: "051910", weighted: 10},                            // ② 요약 없음, 더 앞선 순위
		{symbol: "000660", weighted: 3},                             // ③ 오래된 순
		{symbol: "NVDA", weighted: 1},                               // 다른 시장 — 제외
		{symbol: "122630", weighted: 4},                             // 파생상품 — 제외
		{symbol: "900110", weighted: 5},                             // 거래정지 — 제외
		{symbol: "MISSING", weighted: 6},                            // 카탈로그에 없음 — 제외
	}
	analyzed := map[string]time.Time{
		"005930": now.Add(-2 * time.Hour),
		"000660": now.Add(-72 * time.Hour),
	}

	got := buildScoreTargets(candidates, layerCatalog(), toss.MarketKR, analyzed, map[string]bool{})

	assertOrder(t, got, "005930", "051910", "035420", "000660")
	for _, target := range got {
		if target.Source != model.LLMBatchSourceScore {
			t.Fatalf("source = %q, want score: %+v", target.Source, target)
		}
	}
}

func TestBuildScoreTargetsRotatesOldestFirstOnceBackfilled(t *testing.T) {
	// 한 바퀴 돌아 미분석이 사라지면 남는 규칙은 「가장 오래 안 본 종목부터」다.
	base := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	candidates := []scoreCandidate{
		{symbol: "005930", weighted: 1},
		{symbol: "000660", weighted: 2},
		{symbol: "035420", weighted: 3},
	}
	analyzed := map[string]time.Time{
		"005930": base.Add(-1 * time.Hour),
		"000660": base.Add(-100 * time.Hour),
		"035420": base.Add(-50 * time.Hour),
	}

	got := buildScoreTargets(candidates, layerCatalog(), toss.MarketKR, analyzed, map[string]bool{})

	assertOrder(t, got, "000660", "035420", "005930")
}

func TestBuildScoreTargetsSkipsSymbolsAlreadyTaken(t *testing.T) {
	seen := map[string]bool{"005930": true}
	candidates := []scoreCandidate{
		{symbol: "005930", weighted: 1, head: true},
		{symbol: "000660", weighted: 2},
	}

	got := buildScoreTargets(candidates, layerCatalog(), toss.MarketKR, map[string]time.Time{}, seen)

	assertOrder(t, got, "000660")
	if !seen["000660"] {
		t.Fatal("selected symbols must be recorded so a later layer skips them")
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
