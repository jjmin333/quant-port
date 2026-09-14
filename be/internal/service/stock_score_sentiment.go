package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

/*
심리·수급 축을 이미 저장된 점수 행에 얹는다.

배치(`Recompute`)는 네 축만 매긴다 — 뉴스 감성과 내부자 거래는 전 종목 17,900개에
돌릴 수 없는 재료이기 때문이다(`sentiment_axis.go` 헤더 참조). 그 둘을 이미 모아
둔 곳은 요약 파이프라인 하나뿐이라, 그쪽이 한 종목을 다 만들고 나서 이 함수를
부른다. **LLM 호출은 늘지 않는다** — 프롬프트에 넣던 재료를 한 번 더 쓸 뿐이다.

# 왜 덮어쓰기가 안전한가

이 축은 배치가 다시 돌면 사라진다(배치는 심리를 `nil`로 두고 행을 통째로 덮는다).
그래도 괜찮은 이유는 요약 재빌드가 **읽을 때마다** 일어나기 때문이다 — 캐시가
낡으면 그 종목을 다시 조립하고, 그 끝에서 이 함수가 다시 붙는다. 몇 시간 안에
스스로 복구된다.

# 왜 커버리지를 inputs에 적어 두는가

같은 행에 이 함수가 두 번 불릴 수 있다(요약이 하루에 여러 번 재빌드된다).
커버리지는 **누적값**이라, 직전에 이 축이 얼마를 보탰는지 모르면 부를 때마다
조금씩 부풀어 오른다. 그래서 보탠 몫을 `inputs.sentimentConfidence`에 남기고,
다음 번에 그만큼 빼고 다시 더한다 — 몇 번을 불러도 결과가 같다.
*/

// sentimentInputsKey — inputs blob에 남기는 열쇠. 점수가 왜 그 값인지 되짚을 때
// 이 둘만 있으면 이 축의 기여분을 그대로 재현할 수 있다.
const (
	sentimentInputsScore = "sentiment"
	sentimentInputsConf  = "sentimentConfidence"
)

// ApplySentiment recomputes one symbol's stored composite with the 심리·수급
// axis filled in from the material the summary pipeline already gathered.
//
// A symbol the batch has not scored yet is skipped, not inserted — see
// StockScoreRepo.UpdateAxes.
func (s *ScoreService) ApplySentiment(
	ctx context.Context,
	symbol string,
	news *model.NewsSentimentSummary,
	insider []model.InsiderTransaction,
) error {
	if s == nil || s.scores == nil {
		return nil
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return nil
	}

	value, confidence, ok := sentimentAxis(news, insider)
	if !ok {
		// 잴 재료가 없다. 있던 값을 지우지도 않는다 — 오늘 기사가 없다는 것이
		// 어제 잰 심리가 틀렸다는 뜻은 아니다.
		return nil
	}

	view, err := s.scores.Latest(ctx, symbol)
	if err != nil || view == nil {
		return err
	}

	settings := model.DefaultScoreSettings()
	if s.settings != nil {
		settings = s.settings.Get(ctx)
	}
	weights := settings.Weights
	if s.settings != nil {
		if w, ok := s.settings.SymbolOverrides(ctx)[symbol]; ok {
			weights = w
		}
	}

	breakdown := view.Breakdown
	breakdown.Sentiment = &value

	coverage := recoverCoverage(view, weights, confidence)
	score, grade := compositeFromBreakdown(breakdown, weights, coverage, settings)

	inputs := view.Inputs
	if inputs == nil {
		inputs = map[string]any{}
	}
	inputs[sentimentInputsScore] = math.Round(value*10) / 10
	inputs[sentimentInputsConf] = math.Round(confidence*100) / 100

	breakdownJSON, err := json.Marshal(breakdown)
	if err != nil {
		return fmt.Errorf("sentiment breakdown: %w", err)
	}
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return fmt.Errorf("sentiment inputs: %w", err)
	}

	return s.scores.UpdateAxes(
		ctx, symbol, view.TradeDate, score, grade, coverage,
		string(breakdownJSON), string(inputsJSON),
	)
}

/*
recoverCoverage — 이 축이 보탤 커버리지를 다시 계산한다.

저장된 커버리지는 `Σ(축 가중치 × 축 confidence) / 총 가중치`다. 심리 축은 배치가
비워 두므로 배치 직후 값에는 그 몫이 없고, 이 함수가 한 번 다녀간 뒤에는 있다.
그래서 **직전에 보탠 몫을 먼저 빼고** 새 몫을 더한다 — 그 값은 inputs에 남겨 둔
`sentimentConfidence`다. 없으면 0으로 본다(=배치 직후).
*/
func recoverCoverage(view *model.StockScoreView, weights model.ScoreWeights, confidence float64) float64 {
	total := float64(weights.Total())
	if total <= 0 {
		return view.Coverage
	}
	w := float64(weights.Get(model.AxisSentiment))
	base := view.Coverage - w*previousSentimentConfidence(view)/total
	if base < 0 {
		base = 0
	}
	return clampFloat(base+w*confidence/total, 0, 1)
}

func previousSentimentConfidence(view *model.StockScoreView) float64 {
	if view == nil || view.Inputs == nil {
		return 0
	}
	// inputs는 JSON에서 되살린 map이라 숫자는 float64로 온다. 그 밖의 타입이면
	// 남이 쓴 값이므로 0으로 본다 — 커버리지를 잘못 빼느니 다시 더하는 편이 낫다.
	if v, ok := view.Inputs[sentimentInputsConf].(float64); ok && v > 0 {
		return v
	}
	return 0
}

/*
compositeFromBreakdown — 축 점수만으로 종합 점수를 다시 낸다.

`buildScoreRow`의 마지막 단계와 **같은 식**이다: 값이 있는 축만 가중 평균하고,
커버리지가 얕으면 중립(50) 쪽으로 당긴다. 두 곳이 갈라지면 배치가 낸 점수와
이 함수가 낸 점수가 달라지므로, 식이 바뀔 때는 둘을 같이 고쳐야 한다.
*/
func compositeFromBreakdown(
	breakdown model.ScoreBreakdown,
	weights model.ScoreWeights,
	coverage float64,
	settings model.ScoreSettings,
) (sql.NullInt64, string) {
	var weighted, usedWeight float64
	for _, axis := range model.ScoreAxes {
		w := float64(weights.Get(axis))
		v := breakdown.Get(axis)
		if v == nil || w <= 0 {
			continue
		}
		weighted += *v * w
		usedWeight += w
	}
	if usedWeight <= 0 || coverage < settings.CoverageFloor {
		return sql.NullInt64{}, ""
	}
	raw := weighted / usedWeight
	final := applyShrink(raw, shrinkFactor(coverage, settings.ShrinkFull))
	score := int(clamp01to100(final) + 0.5)
	return sql.NullInt64{Int64: int64(score), Valid: true}, model.ScoreGrade(score)
}
