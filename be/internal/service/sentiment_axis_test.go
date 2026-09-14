package service

import (
	"database/sql"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func i64(v int64) *int64 { return &v }

// 재료가 없으면 **점수가 아니라 「없음」**이다. 0을 돌려주면 뉴스가 없다는 이유로
// 심리 최하위가 되고, 그 축의 가중치만큼 종합 점수가 통째로 깎인다.
func TestSentimentAxisAbsentWithoutMaterial(t *testing.T) {
	if _, _, ok := sentimentAxis(nil, nil); ok {
		t.Fatal("expected no axis without news or insider data")
	}
	if _, _, ok := sentimentAxis(&model.NewsSentimentSummary{}, nil); ok {
		t.Fatal("expected no axis from an empty sentiment tally")
	}
	// SharesChange가 없는 신고는 방향을 말하지 않으므로 재료가 아니다.
	blank := []model.InsiderTransaction{{Date: "2026-09-01"}, {Date: "2026-09-02"}}
	if _, _, ok := sentimentAxis(nil, blank); ok {
		t.Fatal("expected no axis from insider rows without share counts")
	}
}

func TestSentimentAxisNewsDirection(t *testing.T) {
	pos, _, ok := sentimentAxis(&model.NewsSentimentSummary{Total: 10, AverageScore: 80}, nil)
	if !ok {
		t.Fatal("expected an axis from positive news")
	}
	neg, _, ok := sentimentAxis(&model.NewsSentimentSummary{Total: 10, AverageScore: -80}, nil)
	if !ok {
		t.Fatal("expected an axis from negative news")
	}
	neutral, _, _ := sentimentAxis(&model.NewsSentimentSummary{Total: 10, AverageScore: 0}, nil)

	if pos <= neutral || neutral <= neg {
		t.Fatalf("expected positive(%.1f) > neutral(%.1f) > negative(%.1f)", pos, neutral, neg)
	}
	if neutral < 49 || neutral > 51 {
		t.Fatalf("all-neutral coverage should land on 50, got %.1f", neutral)
	}
}

// 표본이 얇으면 confidence가 낮아야 한다 — 그 값이 커버리지를 통해 점수를 중립
// 쪽으로 당기는 유일한 장치다.
func TestSentimentConfidenceGrowsWithSample(t *testing.T) {
	_, thin, _ := sentimentAxis(&model.NewsSentimentSummary{Total: 1, AverageScore: 100}, nil)
	_, thick, _ := sentimentAxis(&model.NewsSentimentSummary{Total: 20, AverageScore: 100}, nil)
	if thin >= thick {
		t.Fatalf("1 article (%.2f) should be less confident than 20 (%.2f)", thin, thick)
	}
	if thick != 1 {
		t.Fatalf("a full sample should reach confidence 1, got %.2f", thick)
	}
}

/*
내부자 거래는 **건수가 아니라 주식 수로** 잰다. 1주짜리 매수 세 건이 100만 주
매도 한 건을 이기면, 대주주 대량 매도가 소액 신고에 묻힌다.
*/
func TestInsiderSentimentWeighsShareVolumeNotRowCount(t *testing.T) {
	txns := []model.InsiderTransaction{
		{SharesChange: i64(1)},
		{SharesChange: i64(1)},
		{SharesChange: i64(1)},
		{SharesChange: i64(-1_000_000)},
	}
	score, _, ok := insiderSentimentComponent(txns)
	if !ok {
		t.Fatal("expected an insider component")
	}
	if score >= 50 {
		t.Fatalf("a large net sale should read bearish, got %.1f", score)
	}
}

// 한 축이 0이나 100으로 튀면 종합 점수가 신고 두 건에 몇 점씩 움직인다.
func TestInsiderSentimentStaysOffTheExtremes(t *testing.T) {
	allBuy, _, _ := insiderSentimentComponent([]model.InsiderTransaction{{SharesChange: i64(500)}})
	allSell, _, _ := insiderSentimentComponent([]model.InsiderTransaction{{SharesChange: i64(-500)}})
	if allBuy > 90 || allSell < 10 {
		t.Fatalf("extremes should be damped, got buy=%.1f sell=%.1f", allBuy, allSell)
	}
	if allBuy <= allSell {
		t.Fatalf("all-buy (%.1f) should read above all-sell (%.1f)", allBuy, allSell)
	}
}

/*
커버리지 보정은 **몇 번을 불러도 같은 값**이어야 한다.

이 축은 요약이 재빌드될 때마다 붙는데, 직전 기여분을 빼지 않으면 부를 때마다
커버리지가 조금씩 부풀어 결국 얕은 데이터로 매긴 점수가 완전한 점수 행세를 한다.
*/
func TestSentimentCoverageIsIdempotent(t *testing.T) {
	weights := model.DefaultScoreWeights() // sentiment 10 / total 100
	base := &model.StockScoreView{Coverage: 0.5}

	first := recoverCoverage(base, weights, 1.0)
	if first <= base.Coverage {
		t.Fatalf("a fresh sentiment reading should raise coverage, got %.3f", first)
	}

	// 두 번째 호출은 첫 번째가 남긴 기여분을 알고 있다.
	second := recoverCoverage(&model.StockScoreView{
		Coverage: first,
		Inputs:   map[string]any{sentimentInputsConf: 1.0},
	}, weights, 1.0)

	if diff := second - first; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("re-applying the same reading changed coverage: %.4f → %.4f", first, second)
	}
}

// 축이 붙으면 종합 점수도 그 방향으로 움직여야 한다 — 아니면 이 축은 저장만
// 되고 아무 일도 하지 않는 셈이다.
func TestCompositeMovesWithSentiment(t *testing.T) {
	settings := model.DefaultScoreSettings()
	weights := model.DefaultScoreWeights()
	mid := 50.0

	withoutScore, _ := compositeFromBreakdown(model.ScoreBreakdown{
		Technical: &mid, Quality: &mid, Growth: &mid, Valuation: &mid,
	}, weights, 0.9, settings)

	high := 90.0
	withHigh, _ := compositeFromBreakdown(model.ScoreBreakdown{
		Technical: &mid, Quality: &mid, Growth: &mid, Valuation: &mid, Sentiment: &high,
	}, weights, 1.0, settings)

	low := 10.0
	withLow, _ := compositeFromBreakdown(model.ScoreBreakdown{
		Technical: &mid, Quality: &mid, Growth: &mid, Valuation: &mid, Sentiment: &low,
	}, weights, 1.0, settings)

	if !withoutScore.Valid || !withHigh.Valid || !withLow.Valid {
		t.Fatal("expected scores for a fully covered breakdown")
	}
	if withHigh.Int64 <= withoutScore.Int64 {
		t.Fatalf("bullish sentiment (%d) should lift the composite above %d", withHigh.Int64, withoutScore.Int64)
	}
	if withLow.Int64 >= withoutScore.Int64 {
		t.Fatalf("bearish sentiment (%d) should pull the composite below %d", withLow.Int64, withoutScore.Int64)
	}
}

// 커버리지 하한 아래는 점수를 내지 않는다 — 배치와 같은 규칙이어야 한다.
func TestCompositeWithheldBelowCoverageFloor(t *testing.T) {
	settings := model.DefaultScoreSettings()
	mid := 50.0
	score, grade := compositeFromBreakdown(
		model.ScoreBreakdown{Technical: &mid},
		model.DefaultScoreWeights(),
		settings.CoverageFloor/2,
		settings,
	)
	if score.Valid || grade != "" {
		t.Fatalf("expected a withheld score below the coverage floor, got %v/%q", score, grade)
	}
	var zero sql.NullInt64
	if score != zero {
		t.Fatalf("a withheld score must be the zero NullInt64, got %v", score)
	}
}
