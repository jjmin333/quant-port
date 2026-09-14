package service

import (
	"database/sql"
	"math"
	"strconv"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// The values here are real readings measured on the live universe
// (docs/STOCK_SCORE_DESIGN.md §2.2), not invented edge cases.

func TestMetricDomainRejectsMeasuredArtifacts(t *testing.T) {
	specs := map[string]metricSpec{}
	for _, s := range scoreMetrics {
		specs[s.name] = s
	}

	cases := []struct {
		metric string
		value  float64
		want   bool // true = accepted
		why    string
	}{
		{"roe", 227162.55, false, "측정된 ROE 최대값 — 껍데기 회사의 분모 붕괴"},
		{"roe", -14957.07, false, "측정된 ROE 최소값"},
		{"roe", 13.7, true, "p75 수준의 정상 ROE"},
		{"roe", -24.24, true, "p25 — 적자지만 실재하는 값"},
		{"operatingMargin", -2112684.87, false, "측정된 영업이익률 최소값"},
		{"operatingMargin", 28698.51, false, "측정된 영업이익률 최대값"},
		{"operatingMargin", 39.98, true, "p95 정상"},
		{"epsGrowthYoY", 2276367794, false, "측정된 EPS 성장률 최대값 (음수 기저 효과)"},
		{"epsGrowthYoY", 66.94, true, "p75 정상"},
		{"debtRatio", -7169000, false, "음수 부채비율 = 자본잠식, 반전하면 최우량으로 뒤집힘"},
		{"debtRatio", -344.70, false, "p5 음수 — 동일 사유"},
		{"debtRatio", 196.40, true, "p75 정상"},
	}

	for _, tc := range cases {
		spec, ok := specs[tc.metric]
		if !ok {
			t.Fatalf("metric %s not in catalogue", tc.metric)
		}
		v := tc.value
		in := &scoreInput{symbol: "T"}
		switch tc.metric {
		case "roe":
			in.roe = &v
		case "operatingMargin":
			in.opMargin = &v
		case "epsGrowthYoY":
			in.epsGrowth = &v
		case "debtRatio":
			in.debtRatio = &v
		}
		got := spec.value(in) != nil
		if got != tc.want {
			t.Errorf("%s=%g accepted=%v want=%v (%s)", tc.metric, tc.value, got, tc.want, tc.why)
		}
	}
}

// A garbage ROE must not win its peer group. Winsorizing alone cannot deliver
// this — the clipped value still sorts first — so this pins the domain guard.
func TestGarbageROEDoesNotTakeTopPercentile(t *testing.T) {
	mk := func(sym string, roe float64) *scoreInput {
		v := roe
		return &scoreInput{symbol: sym, assetType: "domestic", market: "KRX", roe: &v, peer: "p"}
	}
	inputs := []*scoreInput{mk("SHELL", 227162.55)}
	for i, roe := range []float64{2, 4, 6, 8, 10, 12, 14, 16, 18, 20} {
		inputs = append(inputs, mk(string(rune('A'+i)), roe))
	}

	norm := normalizeUniverse(inputs)
	if _, ok := norm["SHELL"]["roe"]; ok {
		t.Fatalf("껍데기 종목의 ROE가 정규화에 포함됨 — 도메인 가드 실패")
	}
	if pct, ok := norm["J"]["roe"]; !ok || pct < 99 {
		t.Errorf("정상 최고 ROE 종목이 최상위여야 함, got %v (ok=%v)", pct, ok)
	}
}

func TestPercentileRanksTiesShareRank(t *testing.T) {
	samples := []metricSample{
		{"a", 10}, {"b", 10}, {"c", 10}, {"d", 20},
	}
	got := percentileRanks(samples, true)
	if got["a"] != got["b"] || got["b"] != got["c"] {
		t.Errorf("동일 값은 같은 백분위여야 함: %v", got)
	}
	if got["d"] <= got["a"] {
		t.Errorf("더 큰 값이 더 높은 백분위여야 함: %v", got)
	}
}

func TestPercentileRanksInverted(t *testing.T) {
	samples := []metricSample{{"cheap", 5}, {"mid", 20}, {"rich", 80}}
	got := percentileRanks(samples, false)
	if got["cheap"] <= got["rich"] {
		t.Errorf("낮을수록 좋은 지표는 반전되어야 함: %v", got)
	}
}

// Locks the four scenarios from the design doc §5.2 table.
func TestCoverageShrinkScenarios(t *testing.T) {
	settings := model.DefaultScoreSettings()

	cases := []struct {
		name     string
		coverage float64
		raw      float64
		want     int
	}{
		{"5축 전부", 1.00, 82, 82},
		{"재무 있음", 0.75, 82, 82},
		{"추세만·일봉40", 0.24, 90, 66},
		{"추세만·일봉20", 0.15, 90, 60},
	}
	for _, tc := range cases {
		got := int(applyShrink(tc.raw, shrinkFactor(tc.coverage, settings.ShrinkFull)) + 0.5)
		if got != tc.want {
			t.Errorf("%s: coverage=%.2f raw=%.0f → %d, want %d", tc.name, tc.coverage, tc.raw, got, tc.want)
		}
	}
}

// The single most important invariant: thin data can never produce an extreme
// score, no matter how strong the one available signal is.
func TestThinCoverageCannotReachExtremes(t *testing.T) {
	settings := model.DefaultScoreSettings()
	for _, coverage := range []float64{0.05, 0.10, 0.20, 0.30} {
		got := applyShrink(100, shrinkFactor(coverage, settings.ShrinkFull))
		if got > 95 {
			t.Errorf("coverage %.2f 에서 %.1f점 — 데이터 부족한데 극단값 도달", coverage, got)
		}
	}
}

func TestMissingAxisIsNotZero(t *testing.T) {
	// An axis with no data must be dropped, never scored 0 — a 0 would read as
	// the worst possible company rather than "unknown".
	if v := axisScore(nil); v != nil {
		t.Errorf("결측 축은 nil이어야 함, got %v", *v)
	}
	v := axisScore([]float64{80, 60})
	if v == nil || math.Abs(*v-70) > 1e-9 {
		t.Errorf("가용 지표 평균이어야 함, got %v", v)
	}
}

func TestTechnicalAxisConfidenceScalesWithHistory(t *testing.T) {
	mkTI := func() *TechnicalIndicators {
		rsi, macd, sig, last, sma20 := 55.0, 1.2, 1.0, 100.0, 98.0
		return &TechnicalIndicators{RSI14: &rsi, MACD: &macd, MACDSignal: &sig, LastClose: &last, SMA20: &sma20}
	}
	cases := []struct {
		bars     int
		wantConf float64
	}{{20, 0.5}, {40, 0.8}, {80, 0.9}, {220, 1.0}}
	for _, tc := range cases {
		in := &scoreInput{symbol: "T", ti: mkTI(), barCount: tc.bars}
		score, conf := technicalAxis(in)
		if score == nil {
			t.Fatalf("bars=%d: 점수가 나와야 함", tc.bars)
		}
		if conf != tc.wantConf {
			t.Errorf("bars=%d: 신뢰도 %.2f, want %.2f", tc.bars, conf, tc.wantConf)
		}
	}
	// Too little history for even RSI → axis unavailable.
	if score, _ := technicalAxis(&scoreInput{symbol: "T", ti: mkTI(), barCount: 10}); score != nil {
		t.Errorf("일봉 10개면 추세 축이 결측이어야 함, got %v", *score)
	}
}

func TestETFGetsTechnicalOnlyKind(t *testing.T) {
	in := &scoreInput{
		symbol: "SPY", assetType: string(model.AssetETF), barCount: 250,
		peer: "etf:ARCX", techConf: 1.0,
	}
	row := buildScoreRow(in, normalizedMetrics{"technicalComposite": 70},
		model.DefaultScoreSettings(), map[string]model.ScoreWeights{}, "2026-08-07")
	if row.ScoreKind != "technical_only" {
		t.Errorf("ETF는 technical_only 라벨이어야 함, got %q", row.ScoreKind)
	}
}

func TestPerSymbolWeightOverrideApplies(t *testing.T) {
	mk := func() *scoreInput {
		return &scoreInput{
			symbol: "005930", assetType: "domestic", market: "KRX",
			barCount: 250, peer: "p", techConf: 1.0,
		}
	}
	// Strong chart, weak fundamentals.
	metrics := normalizedMetrics{"technicalComposite": 90, "roe": 10, "operatingMargin": 10}

	base := buildScoreRow(mk(), metrics, model.DefaultScoreSettings(),
		map[string]model.ScoreWeights{}, "2026-08-07")

	// Same inputs, but this symbol is configured to score on the chart alone.
	overrides := map[string]model.ScoreWeights{
		"005930": {Technical: 100, Quality: 0, Growth: 0, Valuation: 0, Sentiment: 0},
	}
	overridden := buildScoreRow(mk(), metrics, model.DefaultScoreSettings(), overrides, "2026-08-07")

	if !base.Score.Valid || !overridden.Score.Valid {
		t.Fatal("두 경우 모두 점수가 나와야 함")
	}
	if base.Score.Int64 >= overridden.Score.Int64 {
		t.Errorf("수익성 비중을 뺀 쪽 점수가 더 높아야 함: base=%d override=%d",
			base.Score.Int64, overridden.Score.Int64)
	}
	if overridden.WeightSource != "symbol" {
		t.Errorf("가중치 출처가 symbol이어야 함, got %q", overridden.WeightSource)
	}
}

func TestCoverageFloorWithholdsScore(t *testing.T) {
	settings := model.DefaultScoreSettings()
	settings.CoverageFloor = 0.5

	// Technical only, and a thin reading at that (20 bars → confidence 0.5),
	// so weighted coverage is 30×0.5/100 = 0.15.
	in := &scoreInput{
		symbol: "X", assetType: "domestic", market: "KRX",
		barCount: 20, peer: "p", techConf: 0.5,
	}
	row := buildScoreRow(in, normalizedMetrics{"technicalComposite": 55},
		settings, map[string]model.ScoreWeights{}, "2026-08-07")
	if row.Score.Valid {
		t.Errorf("커버리지 미달이면 점수를 내지 않아야 함, got %d", row.Score.Int64)
	}
}

// The technical axis must be centred after ranking — an absolute curve that
// sits above 50 by construction pulled the whole universe's mean to 53.1.
func TestTechnicalCompositeIsRankNormalized(t *testing.T) {
	inputs := make([]*scoreInput, 0, 40)
	for i := 0; i < 40; i++ {
		v := float64(60 + i) // every symbol reads "good" on the absolute curve
		inputs = append(inputs, &scoreInput{
			symbol: string(rune('A' + i)), assetType: "domestic", market: "KRX",
			peer: "p", techRaw: &v, techConf: 1,
		})
	}
	norm := normalizeUniverse(inputs)

	var sum float64
	for _, in := range inputs {
		sum += norm[in.symbol]["technicalComposite"]
	}
	mean := sum / float64(len(inputs))
	if mean < 45 || mean > 55 {
		t.Errorf("순위 정규화 후 평균이 50 부근이어야 함, got %.1f", mean)
	}
}

func TestScoreGradeBands(t *testing.T) {
	cases := map[int]string{95: "A+", 80: "A+", 79: "A", 70: "A", 69: "B+", 55: "B", 45: "C+", 35: "C", 10: "D"}
	for score, want := range cases {
		if got := model.ScoreGrade(score); got != want {
			t.Errorf("score %d → %q, want %q", score, got, want)
		}
	}
}

// makeScoreRows builds a synthetic batch result centred on `mean` with a
// triangular spread of `spread` points either side.
func makeScoreRows(n int, mean, spread int) []repository.StockScoreRow {
	rows := make([]repository.StockScoreRow, 0, n)
	for i := 0; i < n; i++ {
		v := mean
		if spread > 0 {
			// Deterministic zig-zag around the mean.
			offset := (i % (2*spread + 1)) - spread
			v = mean + offset
		}
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
		rows = append(rows, repository.StockScoreRow{
			Symbol: "S", TradeDate: "2026-08-07",
			Score: sql.NullInt64{Int64: int64(v), Valid: true},
		})
	}
	return rows
}

func TestSummarizeDistributionFlagsUnhealthyShape(t *testing.T) {
	healthy := summarizeDistribution(makeScoreRows(400, 50, 20))
	if !healthy.Healthy {
		t.Errorf("정상 분포인데 경고 발생: %v (mean=%.1f sd=%.1f)",
			healthy.Warnings, healthy.Mean, healthy.StdDev)
	}

	// Everything jammed at the ceiling is exactly the failure the batch gate
	// exists to catch before a user ever sees it.
	broken := summarizeDistribution(makeScoreRows(400, 99, 0))
	if broken.Healthy {
		t.Error("천장에 몰린 분포는 unhealthy 여야 함")
	}
}

/*
분위는 **바꿀 이유가 분명할 때만** 바뀐다.

시총은 주가만큼 매일 움직이고 분위 경계선도 그날 분포에서 다시 잡히므로, 경계 근처
종목은 아무 일이 없어도 하루걸러 무리를 옮겨 다녔다 — 실측 08-27→08-28에 17,913종목
중 9,936(55%)이 소속을 바꿨고, 그때마다 백분위가 바뀌어 재무제표가 그대로인 종목의
수익성 점수까지 움직였다.
*/
func TestStickyCapTercile(t *testing.T) {
	// 90개 표본: 1..90. 경계는 sortedCaps[30]=31, sortedCaps[60]=61.
	caps := make([]float64, 0, 90)
	for i := 1; i <= 90; i++ {
		caps = append(caps, float64(i))
	}

	cases := []struct {
		name string
		cap  float64
		prev int
		want int
	}{
		{"어제가 없으면 오늘 값 그대로", 70, 0, 3},
		{"경계 바로 위로 올라온 정도로는 안 옮긴다", 31.5, 1, 1},
		{"경계를 여유만큼 넘으면 옮긴다", 40, 1, 2},
		{"경계 바로 아래로 내려온 정도로는 안 옮긴다", 60.5, 3, 3},
		{"여유만큼 내려가면 옮긴다", 50, 3, 2},
		{"시총을 못 받은 날은 어제 자리를 지킨다", 0, 2, 2},
		{"같은 분위면 그대로", 70, 3, 3},
	}
	for _, tc := range cases {
		if got := stickyCapTercile(caps, tc.cap, tc.prev); got != tc.want {
			t.Errorf("%s: stickyCapTercile(cap=%v, prev=%d) = %d, want %d",
				tc.name, tc.cap, tc.prev, got, tc.want)
		}
	}
}

// 저장된 그룹 문자열에서 분위를 꺼낼 때, **다른 base의 값은 이어 붙이지 않는다** —
// 시장을 옮긴 종목의 어제 분위는 다른 분포에서 매긴 값이다.
func TestPrevTercileFor(t *testing.T) {
	cases := []struct {
		prev, base string
		want       int
	}{
		{"us:NASDAQ:cap2", "us:NASDAQ", 2},
		{"us:NASDAQ", "us:NASDAQ", 0},         // 분위가 없던 그룹
		{"domestic:KRX:cap3", "us:NASDAQ", 0}, // base가 다르다
		{"", "us:NASDAQ", 0},
		{"us:NASDAQ:cap9", "us:NASDAQ", 0}, // 범위 밖은 버린다
	}
	for _, tc := range cases {
		if got := prevTercileFor(tc.prev, tc.base); got != tc.want {
			t.Errorf("prevTercileFor(%q, %q) = %d, want %d", tc.prev, tc.base, got, tc.want)
		}
	}
}

// 「동종 상위 0%」는 화면에 나갈 수 없는 표현이다. 피어 그룹의 최고 종목은 배치
// 백분위에서 100을 받고 그것이 상위 0%로 뒤집히므로, 저장 전에 1로 막는다.
func TestPeerPctNeverReportsZero(t *testing.T) {
	rows := make([]repository.StockScoreRow, 0, 5)
	scores := []int64{10, 30, 50, 70, 90}
	for _, v := range scores {
		rows = append(rows, repository.StockScoreRow{
			Symbol:    "S" + strconv.FormatInt(v, 10),
			PeerGroup: "g",
			Score:     sql.NullInt64{Int64: v, Valid: true},
		})
	}
	assignPeerPercentiles(rows, map[string][]float64{"g": {10, 30, 50, 70, 90}})

	best := rows[len(rows)-1]
	if !best.PeerPct.Valid || best.PeerPct.Int64 != 1 {
		t.Errorf("최고 종목은 상위 1%%여야 함(0%% 금지), got %v", best.PeerPct)
	}
	for _, r := range rows {
		if r.PeerPct.Valid && r.PeerPct.Int64 < 1 {
			t.Errorf("%s 상위 %d%% — 1 미만이 저장됐다", r.Symbol, r.PeerPct.Int64)
		}
	}
}
