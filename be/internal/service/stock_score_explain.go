package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
점수 설명 — 저장된 배치 산출물만 읽어 「왜 이 점수인가 · 왜 바뀌었나」를 조립한다.

# 이 파일이 지키는 선

**아무것도 다시 계산하지 않는다.** 지표를 다시 긁지 않고, 피어 순위를 다시 매기지
않고, LLM을 부르지 않는다. 재료는 전부 `stock_scores`의 그날 행 안에 있다 —
축 점수(`breakdown`), 지표 원값과 백분위와 순위 그룹(`inputs`), 커버리지. 조회
경로에서 무거운 일을 하면 이 화면은 종목 상세를 여는 모든 사람에게 비용이 된다
(PRINCIPLES 「응답 속도 원칙」).

**따라서 쿼리는 하나다** — 최근 N거래일 이력 한 번. 전일·전주 비교도 그 안에서 고른다.

# 귀인이 정확한 이유

`model/score_explain.go` 머리글 참조. 가중치·축 유무·축소율을 고정하면 raw는 축
점수에 선형이므로, 두 날의 차이를 「축 값 변화 · 축 유무 변화 · 축소율 변화」로
잔차 없이 가를 수 있다. 남는 것은 정수 반올림뿐이고 그것도 적어서 내보낸다.
*/

// scoreExplainHistoryDays is how far back the single query reaches.
//
// 30은 `repository.ScoreRetentionDates`와 같다 — 그보다 길게 물어도 행이 없다.
// 전주 비교(달력 7일)와 추세선이 둘 다 이 한 번의 조회로 끝난다.
const scoreExplainHistoryDays = 30

// scoreMetricMeta is the display catalogue for the metrics in `scoreMetrics`.
//
// 카탈로그를 둘로 나눈 이유: 점수 계산에 필요한 것(축·방향·도메인)과 화면에 필요한
// 것(라벨·단위)은 바뀌는 이유가 다르다. 대신 **빠짐없이 짝이 맞는지 시험이 센다**
// (`stock_score_explain_test.go`) — 지표를 새로 넣고 라벨을 잊으면 시험이 막는다.
var scoreMetricMeta = map[string]struct {
	label string
	unit  string
}{
	"roe":                {"ROE", "%"},
	"operatingMargin":    {"영업이익률", "%"},
	"netProfitMargin":    {"순이익률", "%"},
	"debtRatio":          {"부채비율", "%"},
	"currentRatio":       {"유동비율", "x"},
	"revenueGrowthYoY":   {"매출 성장률(YoY)", "%"},
	"epsGrowthYoY":       {"EPS 성장률(YoY)", "%"},
	"per":                {"PER", "x"},
	"pbr":                {"PBR", "x"},
	"psr":                {"PSR", "x"},
	"technicalComposite": {"기술적 종합", "score"},
}

// scoreTechnicalIndicatorMeta is the curated subset of `inputs.technical` that
// the 추세 축 driver reports.
//
// 지표 전체(17개)를 늘어놓으면 「무엇이 움직였나」가 안 읽힌다. 여기 있는 것은
// technicalAxis가 실제로 점수를 만들 때 쓰는 커브들과, 그 커브를 사람이 납득할 때
// 필요한 위치값(이동평균 대비·52주 대비)뿐이다.
var scoreTechnicalIndicatorMeta = []struct {
	key   string
	label string
	unit  string
}{
	{"rsi14", "RSI(14)", "score"},
	{"macdHistogram", "MACD 히스토그램", "raw"},
	{"adx14", "ADX(14)", "score"},
	{"return5d", "5일 수익률", "%"},
	{"return20d", "20일 수익률", "%"},
	{"sma20", "20일 이동평균", "price"},
	{"sma50", "50일 이동평균", "price"},
	{"sma200", "200일 이동평균", "price"},
	{"pctFromHigh52Week", "52주 고점 대비", "%"},
	{"lastClose", "종가", "price"},
}

// scoreExplainSources is the provenance list. Kept here rather than on the
// screen so it cannot drift from what the batch actually reads.
var scoreExplainSources = []string{
	"일별 종가",
	"분기 재무제표",
	"시가총액·종목 마스터",
}

// Explain builds the full explanation payload for one symbol's latest score.
//
// Returns (nil, nil) when the symbol has no stored score — a normal state, not
// an error, exactly like ScoreService.Get.
func (s *ScoreService) Explain(ctx context.Context, symbol string) (*model.ScoreExplain, error) {
	if s == nil || s.scores == nil {
		return nil, fmt.Errorf("score service not configured")
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return nil, fmt.Errorf("symbol required")
	}

	rows, err := s.scores.History(ctx, symbol, scoreExplainHistoryDays)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	settings := model.DefaultScoreSettings()
	if s.settings != nil {
		settings = s.settings.Get(ctx)
	}
	weights := settings.Weights
	weightSource := "global"
	if s.settings != nil {
		if w, ok := s.settings.SymbolOverrides(ctx)[symbol]; ok {
			weights = w
			weightSource = "symbol"
		}
	}

	return buildScoreExplain(rows, weights, weightSource, settings), nil
}

// buildScoreExplain is the pure core — no DB, no clock. Everything it needs is
// the stored rows (newest first) and the settings in force.
func buildScoreExplain(
	rows []model.StockScoreView,
	weights model.ScoreWeights,
	weightSource string,
	settings model.ScoreSettings,
) *model.ScoreExplain {
	if len(rows) == 0 {
		return nil
	}
	latest := rows[0]
	in := readScoreInputs(latest.Inputs)

	out := &model.ScoreExplain{
		Symbol:       latest.Symbol,
		Name:         latest.Name,
		TradeDate:    latest.TradeDate,
		Score:        latest.Score,
		Grade:        latest.Grade,
		ScoreKind:    latest.ScoreKind,
		Coverage:     latest.Coverage,
		PeerGroup:    latest.PeerGroup,
		PeerPct:      latest.PeerPct,
		BarCount:     in.barCount,
		Close:        in.close,
		MarketCap:    in.marketCap,
		ComputedAt:   latest.ComputedAt,
		Weights:      weights,
		WeightSource: weightSource,
	}

	comp := compositeOf(latest.Breakdown, weights, latest.Coverage, settings)
	out.Composite = comp
	out.Axes = explainAxes(latest, in, weights, comp)
	out.Rejected = explainRejected(in)
	out.Changes = explainChanges(rows, weights, settings)
	out.History = explainHistory(rows)
	out.Provenance = explainProvenance(latest, comp)
	return out
}

// ---------------------------------------------------------------------------
// composite arithmetic
// ---------------------------------------------------------------------------

// compositeParts is the composite reconstructed from a stored row. It mirrors
// buildScoreRow exactly; `stock_score_explain_test.go` pins that equality
// against real stored rows so the two can never drift apart silently.
type compositeParts struct {
	raw         float64
	final       float64
	shrink      float64
	usedWeight  float64
	totalWeight float64
	available   bool
}

func compositeReconstruct(
	b model.ScoreBreakdown,
	w model.ScoreWeights,
	coverage float64,
	shrinkFull float64,
) compositeParts {
	var weighted, used, total float64
	for _, axis := range model.ScoreAxes {
		wt := float64(w.Get(axis))
		total += wt
		v := b.Get(axis)
		if v == nil || wt <= 0 {
			continue
		}
		weighted += *v * wt
		used += wt
	}
	parts := compositeParts{usedWeight: used, totalWeight: total}
	if used <= 0 || total <= 0 {
		return parts
	}
	parts.available = true
	parts.raw = weighted / used
	parts.shrink = shrinkFactor(coverage, shrinkFull)
	parts.final = applyShrink(parts.raw, parts.shrink)
	return parts
}

func compositeOf(
	b model.ScoreBreakdown,
	w model.ScoreWeights,
	coverage float64,
	settings model.ScoreSettings,
) model.ScoreComposite {
	p := compositeReconstruct(b, w, coverage, settings.ShrinkFull)
	return model.ScoreComposite{
		Raw:           scoreRound2(p.raw),
		Final:         scoreRound2(p.final),
		ShrinkFactor:  scoreRound4(p.shrink),
		UsedWeight:    int(math.Round(p.usedWeight)),
		TotalWeight:   int(math.Round(p.totalWeight)),
		CoverageFloor: settings.CoverageFloor,
		ShrinkFull:    settings.ShrinkFull,
		Withheld:      !p.available || coverage < settings.CoverageFloor,
	}
}

// ---------------------------------------------------------------------------
// axes
// ---------------------------------------------------------------------------

func explainAxes(
	row model.StockScoreView,
	in scoreInputsBlob,
	weights model.ScoreWeights,
	comp model.ScoreComposite,
) []model.ScoreAxisExplain {
	used := float64(comp.UsedWeight)
	out := make([]model.ScoreAxisExplain, 0, len(model.ScoreAxes))

	for _, axis := range model.ScoreAxes {
		wt := weights.Get(axis)
		v := row.Breakdown.Get(axis)
		item := model.ScoreAxisExplain{
			Axis:      axis,
			Label:     model.ScoreAxisLabels[axis],
			Score:     scoreRoundPtr2(v),
			Weight:    wt,
			Available: v != nil && wt > 0,
			Metrics:   axisMetrics(axis, in),
		}
		if v != nil && wt > 0 && used > 0 {
			share := float64(wt) / used * 100
			item.WeightShare = scoreRound2(share)
			// Contribution is in points of the FINAL score, so the numbers a
			// reader adds up are the ones they can see. The shrink is applied
			// because that is what the displayed score went through.
			contrib := comp.ShrinkFactor * (float64(wt) / used) * *v
			item.Contribution = scorePtr(scoreRound2(contrib))
		}
		if v == nil {
			item.MissingReason = axisMissingReason(axis, row.ScoreKind)
		} else if wt <= 0 {
			item.MissingReason = "weight_zero"
		}
		out = append(out, item)
	}
	return out
}

// axisMissingReason distinguishes 「데이터가 없다」 from 「애초에 해당 없다」.
//
// ETF·ETN에는 재무제표가 없다. 그 종목의 수익성 축을 「데이터 없음」이라고만 하면
// 언젠가 채워질 것처럼 읽히는데, 채워질 일이 없다.
func axisMissingReason(axis model.ScoreAxis, scoreKind string) string {
	if scoreKind == "technical_only" && axis != model.AxisTechnical {
		return "not_applicable"
	}
	return "no_data"
}

func axisMetrics(axis model.ScoreAxis, in scoreInputsBlob) []model.ScoreMetricExplain {
	out := []model.ScoreMetricExplain{}
	for _, spec := range scoreMetrics {
		if spec.axis != axis {
			continue
		}
		meta := scoreMetricMeta[spec.name]
		raw, hasRaw := in.raw[spec.name]
		pct, hasPct := in.percentile[spec.name]
		if !hasRaw && !hasPct {
			continue
		}
		m := model.ScoreMetricExplain{
			Key:            spec.name,
			Label:          meta.label,
			Axis:           axis,
			Unit:           meta.unit,
			HigherIsBetter: spec.higherIsBetter,
			RankedIn:       in.rankedIn[spec.name],
		}
		if hasRaw {
			m.Raw = scorePtr(scoreRound4(raw))
		}
		if hasPct {
			m.Percentile = scorePtr(scoreRound2(pct))
		}
		out = append(out, m)
	}
	return out
}

func explainRejected(in scoreInputsBlob) []model.ScoreRejectedMetric {
	if len(in.rejected) == 0 {
		return nil
	}
	out := make([]model.ScoreRejectedMetric, 0, len(in.rejected))
	for _, spec := range scoreMetrics {
		v, ok := in.rejected[spec.name]
		if !ok {
			continue
		}
		meta := scoreMetricMeta[spec.name]
		out = append(out, model.ScoreRejectedMetric{
			Key:      spec.name,
			Label:    meta.label,
			Axis:     spec.axis,
			Unit:     meta.unit,
			Value:    scoreRound4(v),
			DomainLo: spec.lo,
			DomainHi: spec.hi,
			Reason:   "out_of_domain",
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// change attribution
// ---------------------------------------------------------------------------

func explainChanges(
	rows []model.StockScoreView,
	weights model.ScoreWeights,
	settings model.ScoreSettings,
) []model.ScoreChange {
	latest := rows[0]
	out := make([]model.ScoreChange, 0, 2)

	// 전일: the immediately preceding stored row. 점수 배치가 매일 도는 것이
	// 아니므로(실측 27일 중 16일) 「어제 날짜」가 아니라 **직전 회차**다.
	var prev *model.StockScoreView
	if len(rows) > 1 {
		prev = &rows[1]
	}
	out = append(out, attributeChange(model.ScoreChangePrevDay, prev, latest, weights, settings))

	// 전주: 달력 7일 이전에서 가장 가까운 회차. 거래일로 세면 종목마다 다른 날을
	// 가리킨다(model/score_ranking.go LookbackDays 주석과 같은 이유).
	week := rowOnOrBefore(rows, latest.TradeDate, 7)
	out = append(out, attributeChange(model.ScoreChangePrevWeek, week, latest, weights, settings))
	return out
}

// rowOnOrBefore finds the newest row at least `days` calendar days before the
// anchor date. Nil when nothing reaches back that far.
func rowOnOrBefore(rows []model.StockScoreView, anchor string, days int) *model.StockScoreView {
	base, err := time.Parse("2006-01-02", anchor)
	if err != nil {
		return nil
	}
	cutoff := base.AddDate(0, 0, -days)
	for i := range rows {
		d, err := time.Parse("2006-01-02", rows[i].TradeDate)
		if err != nil {
			continue
		}
		if !d.After(cutoff) {
			return &rows[i]
		}
	}
	return nil
}

func attributeChange(
	window model.ScoreChangeWindow,
	from *model.StockScoreView,
	to model.StockScoreView,
	weights model.ScoreWeights,
	settings model.ScoreSettings,
) model.ScoreChange {
	ch := model.ScoreChange{
		Window:  window,
		Label:   model.ScoreChangeWindowLabels[window],
		ToDate:  to.TradeDate,
		ToScore: to.Score,
		Drivers: []model.ScoreChangeDriver{},
	}
	if from == nil {
		ch.Reason = "no_prior_row"
		return ch
	}
	ch.FromDate = from.TradeDate
	ch.FromScore = from.Score

	p0 := compositeReconstruct(from.Breakdown, weights, from.Coverage, settings.ShrinkFull)
	p1 := compositeReconstruct(to.Breakdown, weights, to.Coverage, settings.ShrinkFull)
	if !p0.available || !p1.available || from.Score == nil || to.Score == nil {
		// 한쪽이 보류된 날이면 「몇 점 올랐다」가 성립하지 않는다.
		ch.Reason = "score_withheld"
		return ch
	}
	ch.Available = true

	delta := *to.Score - *from.Score
	ch.Delta = &delta
	ch.ExactDelta = scoreRound2(p1.final - p0.final)

	inFrom := readScoreInputs(from.Inputs)
	inTo := readScoreInputs(to.Inputs)

	// ① 축 값 변화 — 양쪽에 다 있는 축만. 가중치·분모·축소율은 t0의 것으로 고정하므로
	//    축마다 효과가 정확히 `shrink0 × (w/W0) × Δv`이고 서로 독립이다.
	for _, axis := range model.ScoreAxes {
		v0 := from.Breakdown.Get(axis)
		v1 := to.Breakdown.Get(axis)
		wt := float64(weights.Get(axis))
		if v0 == nil || v1 == nil || wt <= 0 {
			continue
		}
		diff := *v1 - *v0
		if math.Abs(diff) < 1e-9 {
			continue
		}
		effect := p0.shrink * (wt / p0.usedWeight) * diff
		ch.Drivers = append(ch.Drivers, model.ScoreChangeDriver{
			Kind:    model.DriverAxis,
			Axis:    axis,
			Label:   model.ScoreAxisLabels[axis],
			Detail:  fmt.Sprintf("%s %.1f → %.1f", model.ScoreAxisLabels[axis], *v0, *v1),
			Effect:  scoreRound2(effect),
			From:    scorePtr(scoreRound2(*v0)),
			To:      scorePtr(scoreRound2(*v1)),
			Metrics: axisMetricDeltas(axis, inFrom, inTo),
		})
	}

	// ② 축 유무 변화 — 분모가 바뀐다. 한 축이 사라지면 남은 축들의 비중이 커져,
	//    아무 지표도 안 움직였는데 종합이 몇 점 뛴다. 그 날의 진짜 원인이다.
	mixedRaw := rawWith(to.Breakdown, from.Breakdown, weights, from.Breakdown)
	availEffect := p0.shrink * (p1.raw - mixedRaw)
	if math.Abs(availEffect) > 1e-9 {
		ch.Drivers = append(ch.Drivers, model.ScoreChangeDriver{
			Kind:   model.DriverAvailability,
			Label:  "평가 축 구성 변화",
			Detail: availabilityDetail(from.Breakdown, to.Breakdown, weights),
			Effect: scoreRound2(availEffect),
			From:   scorePtr(float64(int(math.Round(p0.usedWeight)))),
			To:     scorePtr(float64(int(math.Round(p1.usedWeight)))),
		})
	}

	// ③ 축소율 변화 — 커버리지가 달라지면 중립으로 당기는 정도가 달라진다.
	shrinkEffect := (p1.raw - 50) * (p1.shrink - p0.shrink)
	if math.Abs(shrinkEffect) > 1e-9 {
		ch.Drivers = append(ch.Drivers, model.ScoreChangeDriver{
			Kind:  model.DriverShrink,
			Label: "데이터 충실도(커버리지) 변화",
			Detail: fmt.Sprintf("커버리지 %.0f%% → %.0f%%",
				from.Coverage*100, to.Coverage*100),
			Effect: scoreRound2(shrinkEffect),
			From:   scorePtr(scoreRound2(from.Coverage * 100)),
			To:     scorePtr(scoreRound2(to.Coverage * 100)),
		})
	}

	sort.SliceStable(ch.Drivers, func(i, j int) bool {
		return math.Abs(ch.Drivers[i].Effect) > math.Abs(ch.Drivers[j].Effect)
	})

	// 잔차는 정수 반올림뿐이다. 숨기지 않고 적는다 — 드라이버 합과 표시 숫자가
	// 어긋나 보일 때 그것이 버그가 아니라 반올림임을 화면이 말할 수 있어야 한다.
	ch.RoundingResidual = scoreRound2(float64(delta) - ch.ExactDelta)
	return ch
}

/*
rawWith — t1의 값을 쓰되 **t0의 축 구성**으로 매긴 raw.

축 유무 효과를 값 효과에서 떼어 내려면 중간 단계가 필요하다: 「값만 바뀌고 구성은
그대로였다면」의 raw. t0에 있던 축만 세고, 그 축의 값은 t1 것을 쓰되 t1에서 사라진
축은 t0 값을 그대로 둔다.
*/
func rawWith(
	newVals model.ScoreBreakdown,
	shape model.ScoreBreakdown,
	w model.ScoreWeights,
	fallback model.ScoreBreakdown,
) float64 {
	var weighted, used float64
	for _, axis := range model.ScoreAxes {
		wt := float64(w.Get(axis))
		if shape.Get(axis) == nil || wt <= 0 {
			continue
		}
		v := newVals.Get(axis)
		if v == nil {
			v = fallback.Get(axis)
		}
		if v == nil {
			continue
		}
		weighted += *v * wt
		used += wt
	}
	if used <= 0 {
		return 0
	}
	return weighted / used
}

func availabilityDetail(from, to model.ScoreBreakdown, w model.ScoreWeights) string {
	var added, removed []string
	for _, axis := range model.ScoreAxes {
		if w.Get(axis) <= 0 {
			continue
		}
		had := from.Get(axis) != nil
		has := to.Get(axis) != nil
		switch {
		case !had && has:
			added = append(added, model.ScoreAxisLabels[axis])
		case had && !has:
			removed = append(removed, model.ScoreAxisLabels[axis])
		}
	}
	var parts []string
	if len(added) > 0 {
		parts = append(parts, strings.Join(added, "·")+" 축이 새로 반영되었습니다")
	}
	if len(removed) > 0 {
		parts = append(parts, strings.Join(removed, "·")+" 축이 빠졌습니다")
	}
	if len(parts) == 0 {
		return "평가에 쓰인 축 구성이 달라졌습니다"
	}
	return strings.Join(parts, ", ")
}

// axisMetricDeltas explains one axis's move with the figures underneath it.
//
// 축 점수는 그 축 지표들의 **백분위 평균**이므로, 축이 움직였다면 반드시 어느
// 백분위가 움직인 것이다. 원값도 같이 실어야 「백분위가 내렸다」가 「회사가 나빠졌다」
// 인지 「남들이 더 좋아졌다」인지 갈린다 — 원값이 그대로인데 백분위만 내렸으면 후자다.
func axisMetricDeltas(axis model.ScoreAxis, from, to scoreInputsBlob) []model.ScoreMetricDelta {
	out := []model.ScoreMetricDelta{}
	for _, spec := range scoreMetrics {
		if spec.axis != axis {
			continue
		}
		meta := scoreMetricMeta[spec.name]
		p0, ok0 := from.percentile[spec.name]
		p1, ok1 := to.percentile[spec.name]
		r0, hr0 := from.raw[spec.name]
		r1, hr1 := to.raw[spec.name]
		if !ok0 && !ok1 && !hr0 && !hr1 {
			continue
		}
		d := model.ScoreMetricDelta{
			Key:            spec.name,
			Label:          meta.label,
			Axis:           axis,
			Unit:           meta.unit,
			HigherIsBetter: spec.higherIsBetter,
		}
		if hr0 {
			d.FromRaw = scorePtr(scoreRound4(r0))
		}
		if hr1 {
			d.ToRaw = scorePtr(scoreRound4(r1))
		}
		if ok0 {
			d.FromPercentile = scorePtr(scoreRound2(p0))
		}
		if ok1 {
			d.ToPercentile = scorePtr(scoreRound2(p1))
		}
		if ok0 && ok1 {
			d.PercentileDelta = scorePtr(scoreRound2(p1 - p0))
		}
		out = append(out, d)
	}

	// 추세 축은 지표가 `technicalComposite` 하나뿐이라 백분위만으로는 「왜」가 안
	// 나온다. 그 합성값을 만든 커브들을 같이 싣는다.
	if axis == model.AxisTechnical {
		out = append(out, technicalIndicatorDeltas(from, to)...)
	}
	return out
}

func technicalIndicatorDeltas(from, to scoreInputsBlob) []model.ScoreMetricDelta {
	out := []model.ScoreMetricDelta{}
	for _, meta := range scoreTechnicalIndicatorMeta {
		v0, ok0 := from.technical[meta.key]
		v1, ok1 := to.technical[meta.key]
		if !ok0 && !ok1 {
			continue
		}
		if ok0 && ok1 && math.Abs(v1-v0) < 1e-9 {
			continue
		}
		d := model.ScoreMetricDelta{
			Key:            meta.key,
			Label:          meta.label,
			Axis:           model.AxisTechnical,
			Unit:           meta.unit,
			HigherIsBetter: true,
		}
		if ok0 {
			d.FromRaw = scorePtr(scoreRound4(v0))
		}
		if ok1 {
			d.ToRaw = scorePtr(scoreRound4(v1))
		}
		out = append(out, d)
	}
	return out
}

// ---------------------------------------------------------------------------
// history & provenance
// ---------------------------------------------------------------------------

// explainHistory returns the trend oldest-first, which is the order a chart
// draws. The stored rows come newest-first.
func explainHistory(rows []model.StockScoreView) []model.ScoreHistoryPoint {
	out := make([]model.ScoreHistoryPoint, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, model.ScoreHistoryPoint{
			TradeDate: rows[i].TradeDate,
			Score:     rows[i].Score,
			Coverage:  rows[i].Coverage,
			Breakdown: rows[i].Breakdown,
		})
	}
	return out
}

func explainProvenance(row model.StockScoreView, comp model.ScoreComposite) model.ScoreProvenance {
	p := model.ScoreProvenance{
		AsOf:       row.TradeDate,
		ComputedAt: row.ComputedAt,
		Basis:      model.BasisActual,
		Sources:    scoreExplainSources,
	}
	if comp.UsedWeight < comp.TotalWeight || row.Coverage < 1 {
		p.Basis = model.BasisPartial
	}
	if row.ScoreKind == "technical_only" {
		p.Note = "재무제표가 없는 종목(ETF·ETN)이라 추세 축만으로 평가했습니다"
	}
	return p
}

// ---------------------------------------------------------------------------
// inputs blob reader
// ---------------------------------------------------------------------------

// scoreInputsBlob is the typed view of `stock_scores.inputs`.
//
// 저장 당시의 JSON을 그대로 읽으므로 **없을 수 있는 키는 전부 없을 수 있다** —
// 지표가 추가·삭제된 날 이전 행에는 그 키가 아예 없고, 그것이 정상이다.
type scoreInputsBlob struct {
	barCount   int
	close      *float64
	marketCap  *float64
	raw        map[string]float64
	percentile map[string]float64
	rejected   map[string]float64
	rankedIn   map[string]string
	technical  map[string]float64
}

func readScoreInputs(m map[string]any) scoreInputsBlob {
	out := scoreInputsBlob{
		raw:        map[string]float64{},
		percentile: map[string]float64{},
		rejected:   map[string]float64{},
		rankedIn:   map[string]string{},
		technical:  map[string]float64{},
	}
	if m == nil {
		return out
	}
	if v, ok := scoreNumber(m["barCount"]); ok {
		out.barCount = int(v)
	}
	if v, ok := scoreNumber(m["close"]); ok {
		out.close = scorePtr(v)
	}
	if v, ok := scoreNumber(m["marketCap"]); ok {
		out.marketCap = scorePtr(v)
	}
	out.raw = scoreNumberMap(m["raw"])
	out.percentile = scoreNumberMap(m["percentile"])
	out.rejected = scoreNumberMap(m["rejected"])
	out.technical = scoreNumberMap(m["technical"])
	if sub, ok := m["rankedIn"].(map[string]any); ok {
		for k, v := range sub {
			if s, ok := v.(string); ok {
				out.rankedIn[k] = s
			}
		}
	}
	return out
}

func scoreNumberMap(v any) map[string]float64 {
	out := map[string]float64{}
	sub, ok := v.(map[string]any)
	if !ok {
		return out
	}
	for k, raw := range sub {
		if f, ok := scoreNumber(raw); ok {
			out[k] = f
		}
	}
	return out
}

// numberFrom accepts every shape a JSON number can arrive in. json.Unmarshal
// into `any` gives float64, but the same blob can reach here from a Go map the
// batch built (tests do exactly that), where an int stays an int.
func scoreNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return 0, false
		}
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// ---------------------------------------------------------------------------
// small helpers
// ---------------------------------------------------------------------------

func scorePtr[T any](v T) *T { return &v }

func scoreRound2(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*100) / 100
}

func scoreRound4(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*10000) / 10000
}

func scoreRoundPtr2(v *float64) *float64 {
	if v == nil {
		return nil
	}
	return scorePtr(scoreRound2(*v))
}

// ---------------------------------------------------------------------------
// 목록용 변화 요약 (행마다 부르는 자리)
// ---------------------------------------------------------------------------

/*
scoreChangeLookbackDays — 직전 회차를 찾을 창(달력 일수).

점수 배치가 매일 도는 것은 아니라(실측 27일 중 16일) 「어제」로는 못 찾는다. 열흘은
연휴를 끼고도 직전 회차가 들어오는 폭이면서, 윈도 함수가 훑는 파티션을 보관분
30거래일의 3분의 1로 줄인다.
*/
const scoreChangeLookbackDays = 10

/*
Changes returns one-line score deltas for a list of symbols.

목록 화면(보유·관심)이 행마다 부르는 자리다. **쿼리는 한 번**이고 행마다 지표 표도
이력도 싣지 않는다 — 변화량과 가장 큰 원인 하나뿐이다.
*/
func (s *ScoreService) Changes(ctx context.Context, symbols []string) ([]model.ScoreChangeSummary, error) {
	if s == nil || s.scores == nil {
		return nil, fmt.Errorf("score service not configured")
	}
	if len(symbols) == 0 {
		return []model.ScoreChangeSummary{}, nil
	}

	latest, err := s.LatestScoredDate(ctx)
	if err != nil {
		return nil, err
	}
	since := latest
	if d, err := time.Parse("2006-01-02", latest); err == nil {
		since = d.AddDate(0, 0, -scoreChangeLookbackDays).Format("2006-01-02")
	}

	pairs, err := s.scores.RecentPairs(ctx, symbols, since)
	if err != nil {
		return nil, err
	}

	settings := model.DefaultScoreSettings()
	if s.settings != nil {
		settings = s.settings.Get(ctx)
	}
	var overrides map[string]model.ScoreWeights
	if s.settings != nil {
		overrides = s.settings.SymbolOverrides(ctx)
	}

	out := make([]model.ScoreChangeSummary, 0, len(pairs))
	seen := map[string]struct{}{}
	for _, raw := range symbols {
		sym := strings.ToUpper(strings.TrimSpace(raw))
		if sym == "" {
			continue
		}
		if _, dup := seen[sym]; dup {
			continue
		}
		seen[sym] = struct{}{}

		rows := pairs[sym]
		if len(rows) == 0 {
			continue
		}
		weights := settings.Weights
		if w, ok := overrides[sym]; ok {
			weights = w
		}
		out = append(out, summarizeScoreChange(rows, weights, settings))
	}
	return out, nil
}

func summarizeScoreChange(
	rows []repository.ScoreLightRow,
	weights model.ScoreWeights,
	settings model.ScoreSettings,
) model.ScoreChangeSummary {
	to := lightToView(rows[0])
	sum := model.ScoreChangeSummary{
		Symbol:    to.Symbol,
		TradeDate: to.TradeDate,
		Score:     to.Score,
	}
	if len(rows) < 2 {
		sum.Reason = "no_prior_row"
		return sum
	}
	from := lightToView(rows[1])
	ch := attributeChange(model.ScoreChangePrevDay, &from, to, weights, settings)
	sum.Available = ch.Available
	sum.Reason = ch.Reason
	sum.FromDate = ch.FromDate
	sum.FromScore = ch.FromScore
	sum.Delta = ch.Delta
	// 드라이버는 이미 절대 효과 순으로 정렬돼 있다. 목록에는 맨 앞 하나만 나가고,
	// 그 하나도 지표 표는 떼고 보낸다 — 행 하나가 60B를 넘지 않아야 한다.
	if len(ch.Drivers) > 0 {
		top := ch.Drivers[0]
		top.Metrics = nil
		sum.TopDriver = &top
	}
	return sum
}

// lightToView adapts the inputs-free row to the shape the attribution code
// reads. Inputs stay nil on purpose — the metric tables are exactly what this
// path must not pay for, and `axisMetricDeltas` returns empty for a nil blob.
func lightToView(r repository.ScoreLightRow) model.StockScoreView {
	v := model.StockScoreView{
		Symbol:    r.Symbol,
		TradeDate: r.TradeDate,
		Coverage:  r.Coverage,
		Breakdown: r.Breakdown,
	}
	if r.Score.Valid {
		score := int(r.Score.Int64)
		v.Score = &score
	}
	return v
}

// ---------------------------------------------------------------------------
// 가중치 바꿔 보기 (관리자 전용)
// ---------------------------------------------------------------------------

/*
ExplainWeights answers "이 종목은 가중치를 이렇게 바꾸면 몇 점이 되나" for one
stored row, without recomputing anything.

관리자가 가중치를 저장하기 **전에** 그 결과를 보는 자리다. 저장 후 배치를 돌려
확인하면 17,906종목이 한 번 더 계산되고, 되돌리려면 또 한 번이다.
*/
func (s *ScoreService) ExplainWeights(
	ctx context.Context, symbol string, to model.ScoreWeights,
) (*model.ScoreWeightChange, error) {
	if s == nil || s.scores == nil {
		return nil, fmt.Errorf("score service not configured")
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	row, err := s.scores.Latest(ctx, symbol)
	if err != nil || row == nil {
		return nil, err
	}
	settings := model.DefaultScoreSettings()
	if s.settings != nil {
		settings = s.settings.Get(ctx)
	}
	from := settings.Weights
	if s.settings != nil {
		if w, ok := s.settings.SymbolOverrides(ctx)[symbol]; ok {
			from = w
		}
	}
	out := attributeWeightChange(*row, from, to.Clamped(), settings)
	return &out, nil
}

/*
attributeWeightChange walks the axes in canonical order, moving one axis's
weight at a time and recording the increment.

축 값 변화와 달리 **순서에 기댄다** — 가중치는 분모에도 들어가 선형이 아니기
때문이다(`model.ScoreWeightChange` 머리글). 합은 정확하고, 순서는 응답에 적는다.
*/
func attributeWeightChange(
	row model.StockScoreView,
	from, to model.ScoreWeights,
	settings model.ScoreSettings,
) model.ScoreWeightChange {
	p0 := compositeReconstruct(row.Breakdown, from, row.Coverage, settings.ShrinkFull)
	p1 := compositeReconstruct(row.Breakdown, to, row.Coverage, settings.ShrinkFull)

	out := model.ScoreWeightChange{
		Symbol:      row.Symbol,
		TradeDate:   row.TradeDate,
		FromWeights: from,
		ToWeights:   to,
		FromScore:   int(p0.final + 0.5),
		ToScore:     int(p1.final + 0.5),
		ExactDelta:  scoreRound2(p1.final - p0.final),
		Order:       model.ScoreAxes,
		Drivers:     []model.ScoreChangeDriver{},
	}
	out.Delta = out.ToScore - out.FromScore
	out.RoundingResidual = scoreRound2(float64(out.Delta) - out.ExactDelta)

	running := from
	prev := p0.final
	for _, axis := range model.ScoreAxes {
		if from.Get(axis) == to.Get(axis) {
			continue
		}
		running.Set(axis, to.Get(axis))
		step := compositeReconstruct(row.Breakdown, running, row.Coverage, settings.ShrinkFull)
		effect := step.final - prev
		prev = step.final
		if math.Abs(effect) < 1e-9 {
			continue
		}
		out.Drivers = append(out.Drivers, model.ScoreChangeDriver{
			Kind:  model.DriverWeight,
			Axis:  axis,
			Label: model.ScoreAxisLabels[axis] + " 가중치",
			Detail: fmt.Sprintf("%s 가중치 %d → %d",
				model.ScoreAxisLabels[axis], from.Get(axis), to.Get(axis)),
			Effect: scoreRound2(effect),
			From:   scorePtr(float64(from.Get(axis))),
			To:     scorePtr(float64(to.Get(axis))),
		})
	}
	sort.SliceStable(out.Drivers, func(i, j int) bool {
		return math.Abs(out.Drivers[i].Effect) > math.Abs(out.Drivers[j].Effect)
	})
	return out
}
