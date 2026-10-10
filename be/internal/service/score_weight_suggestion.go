package service

import (
	"context"
	"fmt"
	"math"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
가중치 제안 — 성적표(축별 IC)로 종합 점수의 가중치를 **제안만** 한다 (2026-09-24 사용자 결정:
「도구만 지금」).

# 적용은 사람이 한다

이 함수는 아무것도 저장하지 않는다. 관리자 화면이 제안과 근거를 보여 주고, 적용은 기존
가중치 편집기로 사람이 한다. 가중치를 자동으로 따라 바꾸면 성적표가 자기 자신을 채점하게
된다 — 오늘의 IC로 가중치를 고치고, 그 가중치로 만든 점수의 IC를 다시 잰다.

# 규칙

  - **표본 90일 미만이면 제안하지 않는다.** 5일 수익률 창이 겹쳐 90일이 독립 표본 약 18개다.
    그보다 짧으면 한 달의 장세가 가중치를 정한다.
  - **유의한(|t| ≥ 2) 양(+)의 IC만** 근거로 센다. 음의 IC 축을 0으로 내리는 것은 제안하지 않는다
    — 축을 빼는 것은 가중치가 아니라 설계 문제다.
  - **현재 값과 반씩 섞는다.** IC 비례만 쓰면 한 축이 60~70%를 가져간다. 반씩 섞으면
    한 번의 제안이 움직이는 폭이 절반이 된다.
  - 축마다 최소 5. 합은 100.
*/

const (
	weightSuggestMinDays = 90
	weightSuggestFloor   = 5
	weightSuggestBlend   = 0.5
)

// WeightSuggestionAxis is one axis's evidence and the proposal.
type WeightSuggestionAxis struct {
	Axis        string   `json:"axis"`
	Days        int      `json:"days"`
	MeanIC      *float64 `json:"meanIC,omitempty"`
	TStat       *float64 `json:"tStat,omitempty"`
	Significant bool     `json:"significant"`
	Current     int      `json:"current"`
	Suggested   int      `json:"suggested"`
}

// WeightSuggestion is the whole answer. Ready=false carries Reason and no proposal.
type WeightSuggestion struct {
	Market  string                 `json:"market"`
	Ready   bool                   `json:"ready"`
	Reason  string                 `json:"reason,omitempty"`
	MinDays int                    `json:"minDays"`
	Axes    []WeightSuggestionAxis `json:"axes"`
}

// SuggestWeights reads the stored report card for market and proposes weights.
func (s *ScoreReportCardService) SuggestWeights(ctx context.Context, market string, current model.ScoreWeights) (WeightSuggestion, error) {
	if s == nil || s.repo == nil {
		return WeightSuggestion{}, fmt.Errorf("score report card not configured")
	}
	rows, err := s.repo.List(ctx, market, scoreICHorizon)
	if err != nil {
		return WeightSuggestion{}, err
	}
	by := map[string][]repository.ScoreICRow{}
	for _, r := range rows {
		by[r.Axis] = append(by[r.Axis], r)
	}
	summaries := make([]ScoreReportAxis, 0, len(model.ScoreAxes))
	for _, axis := range model.ScoreAxes {
		summaries = append(summaries, summarizeAxis(string(axis), by[string(axis)]))
	}
	out := suggestWeights(current, summaries)
	out.Market = market
	return out, nil
}

// suggestWeights is the pure rule (see file header).
func suggestWeights(current model.ScoreWeights, axes []ScoreReportAxis) WeightSuggestion {
	out := WeightSuggestion{MinDays: weightSuggestMinDays, Axes: make([]WeightSuggestionAxis, 0, len(axes))}
	curTotal := 0
	for _, a := range model.ScoreAxes {
		curTotal += current.Get(a)
	}
	minDays := math.MaxInt
	var signalSum float64
	signals := map[string]float64{}
	for _, a := range axes {
		row := WeightSuggestionAxis{
			Axis: a.Axis, Days: a.Days, MeanIC: a.MeanIC, TStat: a.TStat, Significant: a.Significant,
			Current: current.Get(model.ScoreAxis(a.Axis)),
		}
		row.Suggested = row.Current
		out.Axes = append(out.Axes, row)
		if a.Days < minDays {
			minDays = a.Days
		}
		if a.Significant && a.MeanIC != nil && *a.MeanIC > 0 {
			signals[a.Axis] = *a.MeanIC
			signalSum += *a.MeanIC
		}
	}
	switch {
	case len(axes) == 0 || minDays < weightSuggestMinDays:
		if minDays == math.MaxInt {
			minDays = 0
		}
		out.Reason = fmt.Sprintf("표본이 %d일입니다 — 모든 축이 %d일 이상 쌓여야 제안합니다(약 %d일 남음).",
			minDays, weightSuggestMinDays, weightSuggestMinDays-minDays)
		return out
	case signalSum <= 0:
		out.Reason = "유의하게 앞날의 수익률을 맞힌 축이 없습니다 — 현재 가중치를 유지하는 것이 제안입니다."
		return out
	case curTotal <= 0:
		out.Reason = "현재 가중치 합이 0입니다."
		return out
	}

	raw := make([]float64, len(out.Axes))
	total := 0
	for i, a := range out.Axes {
		cur := float64(a.Current) / float64(curTotal) * 100
		ic := signals[a.Axis] / signalSum * 100
		v := math.Max(weightSuggestFloor, weightSuggestBlend*cur+(1-weightSuggestBlend)*ic)
		raw[i] = v
	}
	var rawSum float64
	for _, v := range raw {
		rawSum += v
	}
	largest := 0
	for i := range out.Axes {
		out.Axes[i].Suggested = int(math.Round(raw[i] / rawSum * 100))
		total += out.Axes[i].Suggested
		if out.Axes[i].Suggested > out.Axes[largest].Suggested {
			largest = i
		}
	}
	// 반올림 잔차는 가장 큰 축이 흡수한다 — 합이 정확히 100이어야 화면의 「현재 → 제안」이 읽힌다.
	out.Axes[largest].Suggested += 100 - total
	out.Ready = true
	return out
}
