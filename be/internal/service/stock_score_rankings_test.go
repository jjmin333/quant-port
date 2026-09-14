package service

import (
	"database/sql"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

func rankF(v float64) sql.NullFloat64 { return sql.NullFloat64{Float64: v, Valid: true} }

func rankNull() sql.NullFloat64 { return sql.NullFloat64{} }

func windowRow(symbol string, composite sql.NullFloat64, axes map[model.ScoreAxis]sql.NullFloat64) repository.ScoreWindowRow {
	full := map[model.ScoreAxis]sql.NullFloat64{
		model.AxisTechnical: rankNull(),
		model.AxisQuality:   rankNull(),
		model.AxisGrowth:    rankNull(),
		model.AxisValuation: rankNull(),
		model.AxisSentiment: rankNull(),
	}
	for k, v := range axes {
		full[k] = v
	}
	return repository.ScoreWindowRow{Symbol: symbol, Composite: composite, Axes: full}
}

func symbolsOf(list []scoreRankedSymbol) []string {
	out := make([]string, 0, len(list))
	for _, r := range list {
		out = append(out, r.symbol)
	}
	return out
}

func TestRankWindowSortsEachAxisIndependently(t *testing.T) {
	rows := []repository.ScoreWindowRow{
		windowRow("AAA", rankF(60), map[model.ScoreAxis]sql.NullFloat64{
			model.AxisTechnical: rankF(10), model.AxisGrowth: rankF(90),
		}),
		windowRow("BBB", rankF(80), map[model.ScoreAxis]sql.NullFloat64{
			model.AxisTechnical: rankF(95), model.AxisGrowth: rankF(20),
		}),
		windowRow("CCC", rankF(70), map[model.ScoreAxis]sql.NullFloat64{
			model.AxisTechnical: rankF(50), model.AxisGrowth: rankF(50),
		}),
	}

	got := rankWindow(rows, nil)

	if want := []string{"BBB", "CCC", "AAA"}; !equalStrings(symbolsOf(got[model.RankAxisComposite]), want) {
		t.Errorf("composite = %v, want %v", symbolsOf(got[model.RankAxisComposite]), want)
	}
	if want := []string{"BBB", "CCC", "AAA"}; !equalStrings(symbolsOf(got[model.RankAxisTechnical]), want) {
		t.Errorf("technical = %v, want %v", symbolsOf(got[model.RankAxisTechnical]), want)
	}
	// 성장은 순서가 뒤집힌다 — 축마다 독립적으로 줄 세운다는 것이 이 기능의 요지다.
	if want := []string{"AAA", "CCC", "BBB"}; !equalStrings(symbolsOf(got[model.RankAxisGrowth]), want) {
		t.Errorf("growth = %v, want %v", symbolsOf(got[model.RankAxisGrowth]), want)
	}
}

func TestRankWindowDropsSymbolsWithoutThatAxis(t *testing.T) {
	/*
		심리·수급 축은 요약 배치가 다녀간 종목에만 붙는다 — 실측 2026-09-02 기준
		17,916종목 중 250종목뿐이다. 값 없는 종목을 0점으로 채우면 그 목록이
		「데이터 없음」 종목으로 가득 찬다.
	*/
	rows := []repository.ScoreWindowRow{
		windowRow("AAA", rankF(60), map[model.ScoreAxis]sql.NullFloat64{model.AxisSentiment: rankF(40)}),
		windowRow("BBB", rankF(90), nil), // 심리 값 없음
		windowRow("CCC", rankF(70), map[model.ScoreAxis]sql.NullFloat64{model.AxisSentiment: rankF(80)}),
	}

	got := rankWindow(rows, nil)

	if want := []string{"CCC", "AAA"}; !equalStrings(symbolsOf(got[model.RankAxisSentiment]), want) {
		t.Errorf("sentiment = %v, want %v", symbolsOf(got[model.RankAxisSentiment]), want)
	}
	// 종합에는 셋 다 있다.
	if n := len(got[model.RankAxisComposite]); n != 3 {
		t.Errorf("composite length = %d, want 3", n)
	}
}

func TestRankWindowBreaksTiesBySymbol(t *testing.T) {
	// 값이 그대로인 날에도 순위가 흔들리면 「어제와 오늘 랭킹이 다르다」로 읽힌다.
	rows := []repository.ScoreWindowRow{
		windowRow("ZZZ", rankF(70), nil),
		windowRow("AAA", rankF(70), nil),
		windowRow("MMM", rankF(70), nil),
	}
	got := rankWindow(rows, nil)
	if want := []string{"AAA", "MMM", "ZZZ"}; !equalStrings(symbolsOf(got[model.RankAxisComposite]), want) {
		t.Errorf("tie order = %v, want %v", symbolsOf(got[model.RankAxisComposite]), want)
	}
}

func TestRankWindowCapsAtTopN(t *testing.T) {
	rows := make([]repository.ScoreWindowRow, 0, scoreRankingTopN+50)
	for i := 0; i < scoreRankingTopN+50; i++ {
		rows = append(rows, windowRow(string(rune('A'+i%26))+string(rune('a'+i/26)), rankF(float64(i)), nil))
	}
	got := rankWindow(rows, nil)
	if n := len(got[model.RankAxisComposite]); n != scoreRankingTopN {
		t.Errorf("capped length = %d, want %d", n, scoreRankingTopN)
	}
}

func TestBuildRankingItemsNumbersConsecutivelyAndCarriesRankValue(t *testing.T) {
	ranked := []scoreRankedSymbol{
		{symbol: "AAA", value: 91.6},
		{symbol: "GONE", value: 88.0}, // 최신일 표시 행이 없다
		{symbol: "CCC", value: 70.4},
	}
	display := map[string]model.StockScoreListItem{
		"AAA": {Symbol: "AAA", Name: "가", Score: 55},
		"CCC": {Symbol: "CCC", Name: "다", Score: 61},
	}

	items := buildRankingItems(ranked, display, "2026-09-02")

	if len(items) != 2 {
		t.Fatalf("items = %d, want 2 (표시 행 없는 종목은 버린다)", len(items))
	}
	// 순위는 1,2로 이어진다 — 빠진 자리를 비워 두지 않는다.
	if items[0].Rank != 1 || items[1].Rank != 2 {
		t.Errorf("ranks = %d,%d, want 1,2", items[0].Rank, items[1].Rank)
	}
	// 화면에 적히는 숫자는 종합 점수가 아니라 **줄 세운 값**이다.
	if items[0].RankValue == nil || *items[0].RankValue != 92 {
		t.Errorf("rankValue[0] = %v, want 92 (91.6 반올림)", items[0].RankValue)
	}
	if items[1].RankValue == nil || *items[1].RankValue != 70 {
		t.Errorf("rankValue[1] = %v, want 70", items[1].RankValue)
	}
	if items[0].TradeDate != "2026-09-02" {
		t.Errorf("tradeDate = %q", items[0].TradeDate)
	}
}

func TestScoreRankingDurationWindows(t *testing.T) {
	cases := map[model.ScoreRankingDuration]int{
		model.RankDurationRealtime: 0,
		model.RankDuration1D:       1,
		model.RankDuration1W:       7,
		model.RankDuration1M:       30,
	}
	for duration, want := range cases {
		if got := duration.LookbackDays(); got != want {
			t.Errorf("%s lookback = %d, want %d", duration, got, want)
		}
	}
}

func TestParseScoreRankingParamsFallBackToDefaults(t *testing.T) {
	if got := model.ParseScoreRankingAxis("growth"); got != model.RankAxisGrowth {
		t.Errorf("axis = %q", got)
	}
	// 모르는 값은 기본값으로 — 주소창을 손댄 요청이 500이 되면 안 된다.
	if got := model.ParseScoreRankingAxis("nonsense"); got != model.RankAxisComposite {
		t.Errorf("unknown axis = %q, want composite", got)
	}
	if got := model.ParseScoreRankingDuration("1w"); got != model.RankDuration1W {
		t.Errorf("duration = %q", got)
	}
	if got := model.ParseScoreRankingDuration("10y"); got != model.RankDurationRealtime {
		t.Errorf("unknown duration = %q, want realtime", got)
	}
}

func TestScoreRankingAxisMapsToBreakdownAxis(t *testing.T) {
	if _, ok := model.RankAxisComposite.Axis(); ok {
		t.Error("composite should not map to a breakdown axis — it is the weighted total")
	}
	axis, ok := model.RankAxisValuation.Axis()
	if !ok || axis != model.AxisValuation {
		t.Errorf("valuation → %q (ok=%v)", axis, ok)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

/*
자격 집합은 **상위 N을 자르기 전에** 걸러야 한다.

랭킹 자격이 없는 종목이 상위 100 안에 들어 있으면, 나중에 걸러 봐야 그 자리는
이미 비어 있다. 실측 09-04 해외 「수익성·1개월」 상위 6위 중 셋(ADSEW·IRAB·IMSRW)이
자격 미달이었으니, 나중에 거르는 구현이었다면 목록이 그만큼 짧아졌을 것이다.
*/
func TestRankWindowFiltersToEligibleBeforeCapping(t *testing.T) {
	rows := make([]repository.ScoreWindowRow, 0, scoreRankingTopN+10)
	eligible := map[string]struct{}{}
	// 값이 높은 순서대로 심볼을 만들되, 홀수 번째만 자격을 준다.
	for i := 0; i < scoreRankingTopN+10; i++ {
		sym := "S" + string(rune('A'+i/26)) + string(rune('a'+i%26))
		rows = append(rows, windowRow(sym, rankF(float64(1000-i)), nil))
		if i%2 == 1 {
			eligible[sym] = struct{}{}
		}
	}

	got := rankWindow(rows, eligible)

	list := got[model.RankAxisComposite]
	if len(list) != len(eligible) {
		t.Fatalf("length = %d, want %d (자격 있는 종목 전부)", len(list), len(eligible))
	}
	for _, r := range list {
		if _, ok := eligible[r.symbol]; !ok {
			t.Errorf("%s는 자격 집합에 없는데 목록에 있다", r.symbol)
		}
	}
}

// 자격 집합이 비어 있으면 목록도 비어야 한다 — 「비었으니 전부 통과」로 물러서면
// 배치가 실패한 날 걸러지지 않은 목록이 저장된다.
func TestRankWindowEmptyEligibleYieldsEmptyList(t *testing.T) {
	rows := []repository.ScoreWindowRow{
		windowRow("AAA", rankF(90), nil),
		windowRow("BBB", rankF(80), nil),
	}
	if n := len(rankWindow(rows, map[string]struct{}{})[model.RankAxisComposite]); n != 0 {
		t.Errorf("length = %d, want 0", n)
	}
}

// 자격 필터는 축마다 따로가 아니라 **모든 축에** 걸린다.
func TestRankWindowEligibilityAppliesToEveryAxis(t *testing.T) {
	rows := []repository.ScoreWindowRow{
		windowRow("KEEP", rankF(60), map[model.ScoreAxis]sql.NullFloat64{
			model.AxisTechnical: rankF(10), model.AxisQuality: rankF(20),
		}),
		windowRow("DROP", rankF(99), map[model.ScoreAxis]sql.NullFloat64{
			model.AxisTechnical: rankF(99), model.AxisQuality: rankF(99),
		}),
	}
	got := rankWindow(rows, map[string]struct{}{"KEEP": {}})

	for _, axis := range model.ScoreRankingAxes {
		for _, r := range got[axis] {
			if r.symbol == "DROP" {
				t.Errorf("%s 축에 자격 없는 DROP이 남아 있다", axis)
			}
		}
	}
	if want := []string{"KEEP"}; !equalStrings(symbolsOf(got[model.RankAxisTechnical]), want) {
		t.Errorf("technical = %v, want %v", symbolsOf(got[model.RankAxisTechnical]), want)
	}
}
