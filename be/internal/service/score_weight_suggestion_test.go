package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func suggestAxes(days int, ics map[string]float64, sig map[string]bool) []ScoreReportAxis {
	out := []ScoreReportAxis{}
	for _, a := range model.ScoreAxes {
		v := ics[string(a)]
		out = append(out, ScoreReportAxis{Axis: string(a), Days: days, MeanIC: &v, Significant: sig[string(a)]})
	}
	return out
}

func TestSuggestWeightsRefusesShortSample(t *testing.T) {
	got := suggestWeights(model.DefaultScoreWeights(), suggestAxes(30, map[string]float64{"quality": 0.05}, map[string]bool{"quality": true}))
	if got.Ready || got.Reason == "" {
		t.Fatalf("30일은 제안하면 안 된다: %+v", got)
	}
	for _, a := range got.Axes {
		if a.Suggested != a.Current {
			t.Fatalf("거절이면 제안 = 현재여야 한다: %+v", a)
		}
	}
}

func TestSuggestWeightsKeepsCurrentWithoutSignal(t *testing.T) {
	got := suggestWeights(model.DefaultScoreWeights(), suggestAxes(120, map[string]float64{"quality": 0.05}, nil))
	if got.Ready {
		t.Fatalf("유의한 축이 없으면 제안하지 않는다: %+v", got)
	}
}

func TestSuggestWeightsBlendsTowardSignificantAxes(t *testing.T) {
	cur := model.DefaultScoreWeights() // 30 25 20 15 10
	got := suggestWeights(cur, suggestAxes(120,
		map[string]float64{"quality": 0.04, "valuation": 0.02, "technical": -0.03},
		map[string]bool{"quality": true, "valuation": true, "technical": true}))
	if !got.Ready {
		t.Fatalf("not ready: %s", got.Reason)
	}
	sum := 0
	by := map[string]WeightSuggestionAxis{}
	for _, a := range got.Axes {
		sum += a.Suggested
		by[a.Axis] = a
		if a.Suggested < weightSuggestFloor {
			t.Errorf("%s below floor: %d", a.Axis, a.Suggested)
		}
	}
	if sum != 100 {
		t.Fatalf("sum = %d", sum)
	}
	if by["quality"].Suggested <= by["quality"].Current {
		t.Errorf("양의 유의 IC 축이 올라가야 한다: %+v", by["quality"])
	}
	if by["technical"].Suggested >= by["technical"].Current {
		t.Errorf("음의 IC 축은 근거가 아니다 — 비중이 줄어야 한다: %+v", by["technical"])
	}
	// 반씩 섞으므로 한 축이 과반을 넘지 않는다.
	if by["quality"].Suggested > 50 {
		t.Errorf("한 번의 제안이 너무 크다: %d", by["quality"].Suggested)
	}
}
