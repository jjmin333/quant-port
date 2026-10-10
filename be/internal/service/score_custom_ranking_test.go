package service

import (
	"database/sql"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)


// 기본 가중치로 다시 합산하면 저장된 점수가 그대로 나와야 한다 — 그래야 「기본 대비 ±」가
// 가중치 차이만 말한다. 저장 점수는 buildScoreRow와 같은 산식으로 만든다.
func TestCustomRankingDefaultWeightsReproduceStored(t *testing.T) {
	settings := model.DefaultScoreSettings()
	b := model.ScoreBreakdown{Technical: f64(80), Quality: f64(60), Growth: f64(40), Valuation: f64(20)}
	coverage := 0.9
	p := compositeReconstruct(b, settings.Weights, coverage, settings.ShrinkFull)
	stored := int(clamp01to100(p.final) + 0.5)
	row := repository.CustomRankingRow{Symbol: "A", Score: sql.NullInt64{Int64: int64(stored), Valid: true}, Coverage: coverage, Breakdown: b}
	if !reproducesStored(row, settings) {
		t.Fatalf("default weights did not reproduce stored score %d", stored)
	}
}

// 가치만 보는 가중치에서는 가치 축이 높은 종목이 위로 간다.
func TestCustomRankingWeightsReorder(t *testing.T) {
	settings := model.DefaultScoreSettings()
	valueOnly := model.ScoreWeights{Valuation: 100}
	cheap := compositeReconstruct(model.ScoreBreakdown{Technical: f64(10), Valuation: f64(90)}, valueOnly, 1, settings.ShrinkFull)
	hot := compositeReconstruct(model.ScoreBreakdown{Technical: f64(95), Valuation: f64(20)}, valueOnly, 1, settings.ShrinkFull)
	if cheap.final <= hot.final {
		t.Fatalf("value-only weights should favour the cheap stock: %.1f vs %.1f", cheap.final, hot.final)
	}
}
