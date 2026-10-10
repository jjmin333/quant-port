package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

func thesisRow(date string, b model.ScoreBreakdown) repository.ScoreLightRow {
	return repository.ScoreLightRow{Symbol: "A", TradeDate: date, Breakdown: b}
}

// 「성장 때문에 샀다」면 종합이 아니라 성장 축이 판정을 가른다.
func TestThesisDrivesScoreSignal(t *testing.T) {
	from := thesisRow("2026-09-01", model.ScoreBreakdown{Growth: f64(70), Technical: f64(40)})
	to := thesisRow("2026-09-20", model.ScoreBreakdown{Growth: f64(50), Technical: f64(80)})
	th := thesisTrend([]string{"growth"}, from, to)
	if th == nil || th.Delta == nil || *th.Delta != -20 {
		t.Fatalf("thesis delta = %+v", th)
	}
	sig := scoreTrendSignal(&model.DiagnosisScore{Thesis: th}, "")
	if !sig.Available || sig.Direction != -1 || !sig.Strong || sig.Label != "매수 이유(성장)" {
		t.Fatalf("signal = %+v", sig)
	}
}

// 한쪽 날에만 있는 축은 평균에 넣지 않는다 — 축이 생긴 것이 변화로 읽히면 안 된다.
func TestThesisIgnoresAxesMissingOnEitherDay(t *testing.T) {
	from := thesisRow("2026-09-01", model.ScoreBreakdown{Growth: f64(60)})
	to := thesisRow("2026-09-20", model.ScoreBreakdown{Growth: f64(60), Valuation: f64(90)})
	th := thesisTrend([]string{"growth", "valuation"}, from, to)
	if th == nil || th.Delta == nil || *th.Delta != 0 {
		t.Fatalf("valuation appeared only later and must not move the thesis: %+v", th)
	}
}
