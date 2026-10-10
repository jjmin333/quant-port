package service

import (
	"database/sql"
	"math"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
이 시험이 지키는 것은 하나다 — **설명이 점수와 같은 산수를 쓴다.**

설명 화면은 배치가 이미 계산해 저장한 숫자를 풀어 보여 주는 자리라, 여기서 쓰는
합성 공식이 `buildScoreRow`의 것과 조금이라도 어긋나면 화면은 「48점인데 근거를
더하면 51점」을 말하게 된다. 그래서 고정값은 지어내지 않고 **운영 DB에서 실제로
읽은 행**(005930, 2026-09-15·09-16)을 그대로 쓴다.
*/

func scoreInt(v int) *int { return &v }

// sec005930Sep16 — 실측 행. score 48, coverage 0.90, 심리·수급 축 없음.
func sec005930Sep16() model.StockScoreView {
	return model.StockScoreView{
		Symbol:    "005930",
		Name:      "삼성전자",
		TradeDate: "2026-09-16",
		Score:     scoreInt(48),
		Grade:     "C+",
		ScoreKind: "composite",
		Coverage:  0.9,
		PeerGroup: "domestic:KOSPI:cap3",
		PeerPct:   scoreInt(50),
		Breakdown: model.ScoreBreakdown{
			Technical: f(26.923076923076923),
			Quality:   f(75.93333333333334),
			Growth:    f(61.684782608695656),
			Valuation: f(23.21428571428572),
		},
		Inputs: map[string]any{
			"barCount":  260.0,
			"close":     253500.0,
			"peerGroup": "domestic:KOSPI:cap3",
			"raw": map[string]any{
				"roe": 10.7826, "debtRatio": 29.9371, "currentRatio": 2.3276,
				"per": 38.391640163562016, "pbr": 3.3966595215753146,
				"technicalComposite": 51.00455125935287,
			},
			"percentile": map[string]any{
				"roe": 62.5, "debtRatio": 87.5, "currentRatio": 88.0,
				"per": 23.80952380952381, "pbr": 25.0,
				"technicalComposite": 26.923076923076923,
			},
			"rankedIn": map[string]any{
				"roe": "domestic:KOSPI:cap3", "technicalComposite": "domestic:KOSPI:cap3",
			},
			"technical": map[string]any{
				"rsi14": 47.086740554280425, "sma20": 259975.0, "lastClose": 253500.0,
				"return5d": -5.233644859813085,
			},
		},
	}
}

// sec005930Sep15 — 하루 전 실측 행. score 46, coverage 1.00, 심리·수급 축 **있음**.
func sec005930Sep15() model.StockScoreView {
	return model.StockScoreView{
		Symbol:    "005930",
		TradeDate: "2026-09-15",
		Score:     scoreInt(46),
		Grade:     "C+",
		ScoreKind: "composite",
		Coverage:  1,
		Breakdown: model.ScoreBreakdown{
			Technical: f(19.230769230769234),
			Quality:   f(75.93333333333334),
			Growth:    f(61.684782608695656),
			Valuation: f(26.388888888888896),
			Sentiment: f(54.17665876123766),
		},
		Inputs: map[string]any{
			"percentile": map[string]any{
				"roe": 62.5, "technicalComposite": 19.230769230769234,
				"per": 26.0,
			},
			"raw": map[string]any{
				"roe": 10.7826, "technicalComposite": 44.0, "per": 37.1,
			},
			"technical": map[string]any{
				"rsi14": 44.2, "sma20": 260100.0, "lastClose": 254800.0,
				"return5d": -3.1,
			},
		},
	}
}

// 설명이 저장된 점수를 **정확히** 되짚는다. 48이 아니라 47.9나 48.2가 나오면
// 화면의 막대 합과 머리줄 숫자가 어긋난다.
func TestCompositeReconstructMatchesStoredScore(t *testing.T) {
	settings := model.DefaultScoreSettings()
	for _, tc := range []struct {
		name string
		row  model.StockScoreView
	}{
		{"2026-09-16 (심리축 없음, 커버리지 0.9)", sec005930Sep16()},
		{"2026-09-15 (다섯 축 전부)", sec005930Sep15()},
	} {
		p := compositeReconstruct(tc.row.Breakdown, settings.Weights, tc.row.Coverage, settings.ShrinkFull)
		got := int(p.final + 0.5)
		if got != *tc.row.Score {
			t.Errorf("%s: 재구성 %d, 저장 %d (raw=%.4f shrink=%.4f)",
				tc.name, got, *tc.row.Score, p.raw, p.shrink)
		}
	}
}

// 커버리지가 축소 임계(0.6) 아래면 점수를 중립으로 당긴다 — 그 사실이 응답에 드러나야
// 「왜 이렇게 미지근한 점수인가」에 답할 수 있다.
func TestCompositeExposesShrink(t *testing.T) {
	settings := model.DefaultScoreSettings()
	row := model.StockScoreView{
		Coverage:  0.3,
		Breakdown: model.ScoreBreakdown{Technical: f(90)},
	}
	comp := compositeOf(row.Breakdown, settings.Weights, row.Coverage, settings)
	if comp.ShrinkFactor >= 1 {
		t.Fatalf("축소율이 1이면 안 된다: %v", comp.ShrinkFactor)
	}
	if comp.Raw <= comp.Final {
		t.Errorf("raw %.2f는 축소 후 %.2f보다 커야 한다", comp.Raw, comp.Final)
	}
	if comp.UsedWeight != 30 || comp.TotalWeight != 100 {
		t.Errorf("쓰인 가중치 30/100을 기대했다: %d/%d", comp.UsedWeight, comp.TotalWeight)
	}
	if comp.Withheld {
		t.Error("커버리지 0.3은 보류 기준(0.2)보다 위다")
	}
}

/*
귀인의 합이 실제 변화와 **같아야** 한다.

이것이 이 기능 전체의 주장이다. 드라이버를 다 더해도 점수 변화와 다르면 「데이터
기반 설명」이 아니라 추측이고, 사용자는 그 차이를 알 방법이 없다.
*/
func TestAttributeChangeDriversSumToExactDelta(t *testing.T) {
	settings := model.DefaultScoreSettings()
	from := sec005930Sep15()
	to := sec005930Sep16()

	ch := attributeChange(model.ScoreChangePrevDay, &from, to, settings.Weights, settings)
	if !ch.Available {
		t.Fatalf("비교 가능한 두 행인데 available=false (%s)", ch.Reason)
	}

	var sum float64
	for _, d := range ch.Drivers {
		sum += d.Effect
	}
	// 드라이버는 소수 둘째 자리로 반올림해 나가므로 그만큼의 여유만 둔다.
	if math.Abs(sum-ch.ExactDelta) > 0.05 {
		t.Errorf("드라이버 합 %.4f ≠ 실제 변화 %.4f", sum, ch.ExactDelta)
	}
	if *ch.Delta != 2 {
		t.Errorf("표시 변화는 46 → 48 = +2여야 한다: %d", *ch.Delta)
	}
	// 표시값은 정수라 실제 변화(+1.18)와 어긋난다. 그 어긋남을 숨기지 않는지 본다.
	if math.Abs(ch.RoundingResidual-(2-ch.ExactDelta)) > 0.001 {
		t.Errorf("반올림 잔차가 보고되지 않았다: %v", ch.RoundingResidual)
	}
}

/*
심리·수급 축이 빠진 날은 **아무 지표도 안 움직였는데** 종합이 오른다 — 분모가
작아지기 때문이다. 그 날의 진짜 원인을 「추세가 좋아졌다」로 적으면 거짓말이 된다.
*/
func TestAttributeChangeReportsAxisDisappearing(t *testing.T) {
	settings := model.DefaultScoreSettings()
	from := sec005930Sep15()
	to := sec005930Sep16()

	ch := attributeChange(model.ScoreChangePrevDay, &from, to, settings.Weights, settings)

	var avail *model.ScoreChangeDriver
	for idx := range ch.Drivers {
		if ch.Drivers[idx].Kind == model.DriverAvailability {
			avail = &ch.Drivers[idx]
		}
	}
	if avail == nil {
		t.Fatal("심리·수급 축이 사라졌는데 구성 변화 드라이버가 없다")
	}
	// 54.2점이던 축이 빠졌고 그 값은 나머지 평균(48.3)보다 높았으므로, 빠진 효과는
	// 점수를 **올린다**. 방향이 뒤집히면 설명이 정반대가 된다.
	if avail.Effect >= 0 {
		t.Errorf("빠진 축(54.2)이 나머지 평균보다 높으니 효과는 음수여야 한다: %v", avail.Effect)
	}
	if avail.Detail == "" {
		t.Error("어느 축이 빠졌는지 문장이 비어 있다")
	}
}

// 비교할 행이 아예 없는 것과 변화가 0인 것은 다른 답이다.
func TestAttributeChangeWithoutPriorRow(t *testing.T) {
	settings := model.DefaultScoreSettings()
	ch := attributeChange(model.ScoreChangePrevWeek, nil, sec005930Sep16(), settings.Weights, settings)
	if ch.Available {
		t.Error("이전 행이 없으면 available=false여야 한다")
	}
	if ch.Reason != "no_prior_row" {
		t.Errorf("사유가 적혀야 한다: %q", ch.Reason)
	}
	if ch.Delta != nil {
		t.Error("비교 대상이 없는데 변화량을 지어내면 안 된다")
	}
}

// 한쪽 날의 점수가 보류됐으면 「몇 점 올랐다」가 성립하지 않는다.
func TestAttributeChangeWithheldScore(t *testing.T) {
	settings := model.DefaultScoreSettings()
	from := sec005930Sep15()
	from.Score = nil
	from.Breakdown = model.ScoreBreakdown{}
	ch := attributeChange(model.ScoreChangePrevDay, &from, sec005930Sep16(), settings.Weights, settings)
	if ch.Available || ch.Reason != "score_withheld" {
		t.Errorf("보류된 날과의 비교는 거절해야 한다: available=%v reason=%q", ch.Available, ch.Reason)
	}
}

// 전주 비교는 달력 7일 이전의 가장 가까운 회차를 고른다 — 점수 배치가 매일 도는
// 것이 아니라서 「7회차 전」으로 세면 종목마다 다른 날을 가리킨다.
func TestRowOnOrBeforePicksCalendarWindow(t *testing.T) {
	rows := []model.StockScoreView{
		{TradeDate: "2026-09-16"},
		{TradeDate: "2026-09-15"},
		{TradeDate: "2026-09-11"},
		{TradeDate: "2026-09-09"}, // 7일 이전에서 가장 가까운 회차
		{TradeDate: "2026-09-04"},
	}
	got := rowOnOrBefore(rows, "2026-09-16", 7)
	if got == nil || got.TradeDate != "2026-09-09" {
		t.Fatalf("2026-09-09를 기대했다: %+v", got)
	}
	if rowOnOrBefore(rows, "2026-09-16", 90) != nil {
		t.Error("90일 이전 행이 없으면 nil이어야 한다")
	}
}

/*
지표를 새로 넣고 라벨을 잊으면 화면에 빈 이름이 뜬다.

점수 카탈로그(`scoreMetrics`)와 표시 카탈로그(`scoreMetricMeta`)를 따로 두기로 한
대가는 이 시험 하나다 — 짝이 안 맞는 순간 여기서 막힌다.
*/
func TestEveryScoreMetricHasDisplayMeta(t *testing.T) {
	for _, spec := range scoreMetrics {
		meta, ok := scoreMetricMeta[spec.name]
		if !ok {
			t.Errorf("%s: scoreMetricMeta에 라벨이 없다", spec.name)
			continue
		}
		if meta.label == "" || meta.unit == "" {
			t.Errorf("%s: 라벨/단위가 비어 있다 (%+v)", spec.name, meta)
		}
	}
	for name := range scoreMetricMeta {
		found := false
		for _, spec := range scoreMetrics {
			if spec.name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: 점수에 쓰이지 않는 지표의 라벨이 남아 있다", name)
		}
	}
}

// 축이 비었을 때 0이 아니라 「없음」으로 나가는지. 0은 「최하위」라 뜻이 정반대다.
func TestExplainAxesKeepsMissingNil(t *testing.T) {
	settings := model.DefaultScoreSettings()
	row := sec005930Sep16()
	in := readScoreInputs(row.Inputs)
	comp := compositeOf(row.Breakdown, settings.Weights, row.Coverage, settings)
	axes := explainAxes(row, in, settings.Weights, comp)

	var sentiment *model.ScoreAxisExplain
	for idx := range axes {
		if axes[idx].Axis == model.AxisSentiment {
			sentiment = &axes[idx]
		}
	}
	if sentiment == nil {
		t.Fatal("다섯 축은 비어 있어도 전부 응답에 있어야 한다")
	}
	if sentiment.Score != nil {
		t.Errorf("심리·수급은 데이터가 없다 — nil이어야 한다: %v", *sentiment.Score)
	}
	if sentiment.Available || sentiment.MissingReason != "no_data" {
		t.Errorf("없음의 사유가 적혀야 한다: %+v", sentiment)
	}
	if sentiment.Contribution != nil {
		t.Error("없는 축은 기여도를 갖지 않는다")
	}

	// 기여도 합 + 축소 보정이 최종 점수를 되짚는지.
	var sum float64
	for _, a := range axes {
		if a.Contribution != nil {
			sum += *a.Contribution
		}
	}
	expected := comp.Final - 50*(1-comp.ShrinkFactor)
	if math.Abs(sum-expected) > 0.05 {
		t.Errorf("축 기여도 합 %.2f ≠ 최종 점수 성분 %.2f", sum, expected)
	}
}

// ETF는 재무제표가 없다. 「아직 데이터가 없다」와 「원래 해당 없다」를 같은 말로
// 적으면 언젠가 채워질 것처럼 읽힌다.
func TestExplainAxesMarksNotApplicable(t *testing.T) {
	row := model.StockScoreView{
		TradeDate: "2026-09-16",
		Score:     scoreInt(55),
		ScoreKind: "technical_only",
		Coverage:  0.3,
		Breakdown: model.ScoreBreakdown{Technical: f(70)},
	}
	settings := model.DefaultScoreSettings()
	comp := compositeOf(row.Breakdown, settings.Weights, row.Coverage, settings)
	axes := explainAxes(row, readScoreInputs(row.Inputs), settings.Weights, comp)
	for _, a := range axes {
		if a.Axis == model.AxisTechnical {
			continue
		}
		if a.MissingReason != "not_applicable" {
			t.Errorf("%s: technical_only에서는 not_applicable이어야 한다 (%q)", a.Axis, a.MissingReason)
		}
	}
}

// 버린 값은 숨기지 않는다 — 「수익성 점수가 왜 없나」의 답이 대개 그것이다.
func TestExplainRejectedSurfacesOutOfDomain(t *testing.T) {
	in := readScoreInputs(map[string]any{
		"rejected": map[string]any{"roe": 227162.0},
	})
	out := explainRejected(in)
	if len(out) != 1 {
		t.Fatalf("버린 지표 하나를 기대했다: %+v", out)
	}
	if out[0].Key != "roe" || out[0].Label != "ROE" || out[0].Reason != "out_of_domain" {
		t.Errorf("예상 밖의 항목: %+v", out[0])
	}
	if out[0].DomainHi != 200 {
		t.Errorf("도메인 상한도 같이 나가야 한다: %v", out[0].DomainHi)
	}
}

// 추세 축이 움직인 이유는 백분위 한 줄로는 안 읽힌다 — 커브도 같이 나가야 한다.
func TestAxisMetricDeltasIncludeTechnicalCurves(t *testing.T) {
	from := readScoreInputs(sec005930Sep15().Inputs)
	to := readScoreInputs(sec005930Sep16().Inputs)
	deltas := axisMetricDeltas(model.AxisTechnical, from, to)

	var sawComposite, sawRSI bool
	for _, d := range deltas {
		switch d.Key {
		case "technicalComposite":
			sawComposite = true
			if d.PercentileDelta == nil {
				t.Error("백분위 변화가 계산되지 않았다")
			}
		case "rsi14":
			sawRSI = true
		}
	}
	if !sawComposite || !sawRSI {
		t.Errorf("합성값과 커브가 둘 다 있어야 한다 (composite=%v rsi=%v)", sawComposite, sawRSI)
	}
}

// 이력은 차트가 그리는 순서(오래된 것부터)로 나간다.
func TestExplainHistoryIsChronological(t *testing.T) {
	rows := []model.StockScoreView{
		{TradeDate: "2026-09-16", Score: scoreInt(48)},
		{TradeDate: "2026-09-15", Score: scoreInt(46)},
		{TradeDate: "2026-09-14", Score: scoreInt(58)},
	}
	got := explainHistory(rows)
	if len(got) != 3 || got[0].TradeDate != "2026-09-14" || got[2].TradeDate != "2026-09-16" {
		t.Fatalf("오래된 것부터여야 한다: %+v", got)
	}
}

// 커버리지가 100%가 아니면 「일부 축 결측」임을 스탬프가 말해야 한다 (요구 8).
func TestProvenanceMarksPartialBasis(t *testing.T) {
	settings := model.DefaultScoreSettings()
	row := sec005930Sep16()
	comp := compositeOf(row.Breakdown, settings.Weights, row.Coverage, settings)
	p := explainProvenance(row, comp)
	if p.Basis != model.BasisPartial {
		t.Errorf("심리축이 빠진 행은 partial이어야 한다: %v", p.Basis)
	}
	if p.AsOf != "2026-09-16" || len(p.Sources) == 0 {
		t.Errorf("기준일·출처가 비어 있다: %+v", p)
	}
}

// 전체 조립이 한 번에 도는지 — 두 비교 창이 항상 응답에 있어야 화면이 자리를 비워
// 두지 않는다.
func TestBuildScoreExplainShape(t *testing.T) {
	settings := model.DefaultScoreSettings()
	rows := []model.StockScoreView{sec005930Sep16(), sec005930Sep15()}
	out := buildScoreExplain(rows, settings.Weights, "global", settings)
	if out == nil {
		t.Fatal("nil 응답")
	}
	if len(out.Axes) != len(model.ScoreAxes) {
		t.Errorf("축 다섯 개가 전부 나와야 한다: %d", len(out.Axes))
	}
	if len(out.Changes) != 2 {
		t.Fatalf("전일·전주 두 창을 기대했다: %d", len(out.Changes))
	}
	if out.Changes[0].Window != model.ScoreChangePrevDay ||
		out.Changes[1].Window != model.ScoreChangePrevWeek {
		t.Errorf("창 순서가 다르다: %+v", out.Changes)
	}
	// 09-15는 09-16의 달력 7일 이전이 아니므로 전주 창은 비교 대상이 없다.
	if out.Changes[1].Available {
		t.Error("7일 이전 행이 없는데 전주 비교가 성립했다")
	}
	if out.Composite.UsedWeight != 90 {
		t.Errorf("쓰인 가중치 90을 기대했다: %d", out.Composite.UsedWeight)
	}
}

// ---------------------------------------------------------------------------
// 목록용 요약
// ---------------------------------------------------------------------------

func lightRow(date string, score int, coverage float64, b model.ScoreBreakdown) repository.ScoreLightRow {
	return repository.ScoreLightRow{
		Symbol:    "005930",
		TradeDate: date,
		Score:     sql.NullInt64{Int64: int64(score), Valid: true},
		Coverage:  coverage,
		Breakdown: b,
	}
}

/*
목록 행은 **드라이버 하나만, 지표 표 없이** 나가야 한다.

행마다 지표 표를 실으면 20종목짜리 보유 목록이 설명 패널 스무 개가 된다 —
「목록 응답에 상세용 데이터를 싣지 않는다」(PRINCIPLES 응답 속도 원칙 ④).
*/
func TestSummarizeScoreChangeCarriesOnlyTheTopDriver(t *testing.T) {
	settings := model.DefaultScoreSettings()
	rows := []repository.ScoreLightRow{
		lightRow("2026-09-16", 48, 0.9, sec005930Sep16().Breakdown),
		lightRow("2026-09-15", 46, 1, sec005930Sep15().Breakdown),
	}
	sum := summarizeScoreChange(rows, settings.Weights, settings)

	if !sum.Available || sum.Delta == nil || *sum.Delta != 2 {
		t.Fatalf("46 → 48 = +2를 기대했다: %+v", sum)
	}
	if sum.FromDate != "2026-09-15" || sum.TradeDate != "2026-09-16" {
		t.Errorf("날짜가 어긋난다: %s → %s", sum.FromDate, sum.TradeDate)
	}
	if sum.TopDriver == nil {
		t.Fatal("가장 큰 원인이 비어 있다")
	}
	// 실측: 추세 +2.31이 가장 크다.
	if sum.TopDriver.Axis != model.AxisTechnical {
		t.Errorf("추세가 가장 큰 원인이어야 한다: %+v", sum.TopDriver)
	}
	if sum.TopDriver.Metrics != nil {
		t.Errorf("목록 행에 지표 표가 실렸다: %d건", len(sum.TopDriver.Metrics))
	}
}

// 창 안에 한 행뿐이면 비교 대상이 없는 것이고, 그것은 오류가 아니다.
func TestSummarizeScoreChangeWithSingleRow(t *testing.T) {
	settings := model.DefaultScoreSettings()
	sum := summarizeScoreChange(
		[]repository.ScoreLightRow{lightRow("2026-09-16", 48, 0.9, sec005930Sep16().Breakdown)},
		settings.Weights, settings,
	)
	if sum.Available || sum.Reason != "no_prior_row" {
		t.Errorf("직전 회차가 없다고 말해야 한다: %+v", sum)
	}
	if sum.Score == nil || *sum.Score != 48 {
		t.Errorf("최신 점수는 그대로 나가야 한다: %+v", sum.Score)
	}
}

// 인풋 블롭 없이도 축 귀인은 돌아야 한다 — 목록 경로가 일부러 안 읽는 값이다.
func TestAttributeChangeWorksWithoutInputsBlob(t *testing.T) {
	settings := model.DefaultScoreSettings()
	from := lightToView(lightRow("2026-09-15", 46, 1, sec005930Sep15().Breakdown))
	to := lightToView(lightRow("2026-09-16", 48, 0.9, sec005930Sep16().Breakdown))
	ch := attributeChange(model.ScoreChangePrevDay, &from, to, settings.Weights, settings)

	if !ch.Available {
		t.Fatalf("인풋이 없다고 비교를 못 할 이유는 없다: %s", ch.Reason)
	}
	var sum float64
	for _, d := range ch.Drivers {
		sum += d.Effect
		if len(d.Metrics) > 0 {
			t.Errorf("%s: 인풋이 없는데 지표가 나왔다", d.Label)
		}
	}
	if math.Abs(sum-ch.ExactDelta) > 0.05 {
		t.Errorf("드라이버 합 %.4f ≠ 실제 변화 %.4f", sum, ch.ExactDelta)
	}
}

// ---------------------------------------------------------------------------
// 가중치 바꿔 보기
// ---------------------------------------------------------------------------

/*
가중치 귀인은 순서에 기대지만 **합은 정확해야** 한다.

축 값과 달리 가중치는 분모에도 들어가 선형이 아니다. 각 항이 「그 순서에서의 몫」인
것은 응답이 밝히지만, 합이 실제 변화와 다르면 그건 설명이 아니라 추측이다.
*/
func TestAttributeWeightChangeDriversSumExactly(t *testing.T) {
	settings := model.DefaultScoreSettings()
	row := sec005930Sep16()
	to := model.ScoreWeights{Technical: 20, Quality: 25, Growth: 20, Valuation: 25, Sentiment: 10}

	out := attributeWeightChange(row, settings.Weights, to, settings)

	var sum float64
	for _, d := range out.Drivers {
		sum += d.Effect
		if d.Kind != model.DriverWeight {
			t.Errorf("가중치 드라이버여야 한다: %v", d.Kind)
		}
	}
	if math.Abs(sum-out.ExactDelta) > 0.05 {
		t.Errorf("드라이버 합 %.4f ≠ 실제 변화 %.4f", sum, out.ExactDelta)
	}
	if out.FromScore != 48 {
		t.Errorf("현재 가중치의 점수는 저장값 48과 같아야 한다: %d", out.FromScore)
	}
	// 추세(26.9)를 덜 보고 가치(23.2)를 더 보면 둘 다 낮은 축이라 점수는 거의 안 움직인다.
	// 중요한 것은 방향이 아니라 **합이 맞는다는 것**이므로 값 자체는 못박지 않는다.
	if len(out.Order) != len(model.ScoreAxes) {
		t.Errorf("계산 순서를 응답이 밝혀야 한다: %+v", out.Order)
	}
}

// 바꾼 축만 드라이버로 나온다 — 안 건드린 축이 0점짜리 줄로 늘어서면 화면이 읽히지 않는다.
func TestAttributeWeightChangeSkipsUntouchedAxes(t *testing.T) {
	settings := model.DefaultScoreSettings()
	to := settings.Weights
	to.Set(model.AxisValuation, 40)

	out := attributeWeightChange(sec005930Sep16(), settings.Weights, to, settings)
	if len(out.Drivers) != 1 || out.Drivers[0].Axis != model.AxisValuation {
		t.Fatalf("가치 축 하나만 나와야 한다: %+v", out.Drivers)
	}
	if out.Drivers[0].From == nil || *out.Drivers[0].From != 15 ||
		out.Drivers[0].To == nil || *out.Drivers[0].To != 40 {
		t.Errorf("전후 가중치를 적어야 한다: %+v", out.Drivers[0])
	}
}

// 데이터가 없는 축의 가중치는 아무리 올려도 점수를 못 바꾼다 — 그때는 드라이버가
// 아예 없어야 한다(0점짜리 줄을 만들지 않는다).
func TestAttributeWeightChangeIgnoresEmptyAxis(t *testing.T) {
	settings := model.DefaultScoreSettings()
	to := settings.Weights
	to.Set(model.AxisSentiment, 90) // 005930 09-16에는 심리·수급 축이 없다

	out := attributeWeightChange(sec005930Sep16(), settings.Weights, to, settings)
	if len(out.Drivers) != 0 {
		t.Errorf("없는 축의 가중치는 점수를 못 바꾼다: %+v", out.Drivers)
	}
	if out.Delta != 0 {
		t.Errorf("점수가 움직이면 안 된다: %d", out.Delta)
	}
}

// 사용자에게 나가는 문장은 ~입니다 / ~합니다 (2026-09-17 사용자 결정).
func TestUserFacingSentencesUsePoliteForm(t *testing.T) {
	settings := model.DefaultScoreSettings()
	from := sec005930Sep15()
	ch := attributeChange(model.ScoreChangePrevDay, &from, sec005930Sep16(), settings.Weights, settings)

	for _, d := range ch.Drivers {
		if strings.HasSuffix(d.Detail, "다") && !strings.HasSuffix(d.Detail, "니다") {
			t.Errorf("평어체 문장이 나갔다: %q", d.Detail)
		}
	}
	etf := model.StockScoreView{ScoreKind: "technical_only", Coverage: 0.3}
	note := explainProvenance(etf, model.ScoreComposite{UsedWeight: 30, TotalWeight: 100}).Note
	if note == "" || (strings.HasSuffix(note, "다") && !strings.HasSuffix(note, "니다")) {
		t.Errorf("평어체 안내문이 나갔다: %q", note)
	}
}
