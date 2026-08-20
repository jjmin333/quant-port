package model

// ComputeSurprise turns an estimate/actual pair into the displayable surprise.
//
// **서버에서 한 번만 계산한다.** 화면에서 다시 구하면 홈 카드·캘린더 목록·종목 탭이
// 같은 발표에 대해 다른 숫자를 말한다 (리포트의 `toTargetPct`와 같은 이유).
//
// 부호 처리가 이 함수의 존재 이유다. 순진하게 (actual-estimate)/|estimate|를 쓰면
// 적자를 예상했는데 흑자가 난 종목이 「+840% 서프라이즈」로 뜬다 — 방향은 맞지만
// 크기가 무의미하고, 목록을 서프라이즈 순으로 정렬하면 그런 행이 상단을 전부 차지한다.
// 그래서 기준치가 0 이하일 때는 퍼센트를 내리지 않고 말로 답한다.
func ComputeSurprise(metric string, m EarningsMetric) *EarningsSurprise {
	if !m.HasBoth() {
		return nil
	}
	est, act := *m.Estimate, *m.Actual
	out := &EarningsSurprise{Metric: metric, Estimate: est, Actual: act}

	switch {
	case est > 0 && act >= 0:
		pct := (act - est) / est * 100
		out.Pct = &pct
	case est > 0 && act < 0:
		// 퍼센트도 의미가 있고(흑자 예상 대비 얼마나 빗나갔나) 라벨도 필요하다.
		// 둘 중 하나만 주면 정보를 버리는 셈이라 함께 내린다.
		pct := (act - est) / est * 100
		out.Pct = &pct
		out.Label = "적자전환"
	case act > 0:
		// est <= 0 — 퍼센트를 만들 기준선이 없다.
		out.Label = "흑자전환"
	case est < 0 || act < 0:
		out.Label = "적자지속"
	}
	return out
}

// IsDivergentEarnings reports whether the surprise and the stock moved in
// opposite directions.
//
// 이 화면에서 가장 값어치 있는 신호다 — 숫자와 시장이 다르게 말한 경우.
// 문턱을 두는 이유: ±1% 서프라이즈에 ∓0.3% 반응은 엇갈린 게 아니라 잡음이다.
func IsDivergentEarnings(ev EarningsEvent) bool {
	if ev.Surprise == nil || ev.Surprise.Pct == nil || ev.Reaction == nil {
		return false
	}
	pct, reaction := *ev.Surprise.Pct, ev.Reaction.Pct
	if pct > -5 && pct < 5 {
		return false
	}
	if reaction > -2 && reaction < 2 {
		return false
	}
	return (pct > 0) != (reaction > 0)
}

// EarningsConsistency is one company's habit of beating (or missing) consensus.
type EarningsConsistency struct {
	Symbol  string             `json:"symbol"`
	Name    string             `json:"name"`
	Market  string             `json:"market"`
	History EarningsHitHistory `json:"history"`
}

// SurpriseMetricForMarket picks the line each market calls "어닝 서프라이즈".
//
// 국내는 영업이익, 미국은 EPS다. 한쪽 관행을 양쪽에 밀어붙이면 한 시장에서는
// 아무도 그 숫자로 이야기하지 않는다 — 국내 뉴스는 "영업이익 컨센서스 상회"라고
// 쓰고, 미국은 "EPS beat"이라고 쓴다.
// market은 "domestic" | "us" — QuoteAssetType과 같은 어휘를 쓴다. 시장 이름을
// 위한 상수를 새로 만들면 같은 값이 두 이름으로 돌아다닌다.
func SurpriseMetricForMarket(market string) string {
	if market == string(AssetUS) {
		return EarningsMetricEPS
	}
	return EarningsMetricOpIncome
}

// HeadlineSurprise computes the surprise on whichever metric the event's market
// leads with, falling back to the other when the leading one has no pair.
//
// 폴백이 있는 이유: 국내 소스는 EPS를 주지 않고, 미국 소스는 영업이익을 주지
// 않는다. 그래도 한 시장 안에서 두 종류가 섞이면 목록의 정렬 기준이 행마다
// 달라지므로, 폴백으로 만든 서프라이즈에도 Metric을 정확히 적어 둔다.
func (e *EarningsEvent) HeadlineSurprise() *EarningsSurprise {
	primary := SurpriseMetricForMarket(e.Market)
	if primary == EarningsMetricEPS {
		if s := ComputeSurprise(EarningsMetricEPS, e.EPS); s != nil {
			return s
		}
		return ComputeSurprise(EarningsMetricOpIncome, e.OpIncome)
	}
	if s := ComputeSurprise(EarningsMetricOpIncome, e.OpIncome); s != nil {
		return s
	}
	return ComputeSurprise(EarningsMetricEPS, e.EPS)
}
