package model

/*
목표가 시나리오 — 하나의 숫자 대신 **약세·중립·강세 셋** (V2 요구 5).

# 왜 단일 목표가를 찍지 않나

실측(2026-09-18, 국내 250종목 × 6개 시점 × 90일): 90일 뒤 실제 가격은 현재가에서
**평균 22.9%** 떨어져 있었다. 그 오차를 아는 채로 한 숫자를 내놓으면 그 정밀도는
거짓이다. 요구 5가 Bull/Base/Bear를 말한 이유도 이것이라고 읽었다.

# 왜 증권사 목표가가 아니라 우리가 만드나

이 앱에는 증권사 목표가가 **들어오지 않는다.** 사용자 목표가는 2건, 스탬프는 3건이라
「과거 예측 오차」를 집계할 원본이 없다. 그래서 시나리오를 우리 데이터로 도출하고
**그 방법 자체를 백테스트**한다 — Phase 2에서 랭킹에 한 것과 같다.

# 왜 밸류에이션이 아니라 변동성인가 (2026-09-19 사용자 결정)

PER 기반을 먼저 재봤고 두 가지 이유로 접었다:

	· 앱의 PER은 **연간 순이익** 기준인데 TTM으로 다시 만들면 종목 화면과 어긋난다
	  (실측: 화승알앤에이 1.58 vs 앱 3.05).
	· 과거 PER은 `stock_scores`가 30거래일만 보관해 90일 지평을 맞춰 볼 수 없다.

가격 이력은 21개월치가 있어 **선행 편향 없이** 만들고 검증할 수 있다. 세 후보의 실측:

	방법                커버리지   90일 적중률   밴드 폭
	변동성 z=1.28        100%       72.3%        약 30%   ← 채택
	변동성 z=1.65        100%       82.6%        약 40%
	52주 고점·저점        100%       60.6%        54%

52주 범위는 **더 넓은데 덜 맞는다.** 종목별 편차도 극단적이라(삼성전자의 52주 저점은
현재가의 −73%) 약세 시나리오로 쓸 수 없었다.

# 점수로 방향을 기울이지 않는다

Phase 2 백테스트에서 종합 점수 상위 바구니가 KOSPI를 **8.89%p 밑돌았다.** 그 점수로
목표가를 위로 밀면 우리가 이미 틀렸다고 측정해 둔 것을 예측에 다시 넣는 일이 된다.
*/

// ScenarioKind is one of the three cases.
type ScenarioKind string

const (
	ScenarioBear ScenarioKind = "bear"
	ScenarioBase ScenarioKind = "base"
	ScenarioBull ScenarioKind = "bull"
)

var ScenarioLabels = map[ScenarioKind]string{
	ScenarioBear: "약세",
	ScenarioBase: "중립",
	ScenarioBull: "강세",
}

/*
시나리오 상수 — 사용자 결정(2026-09-19)으로 고정한 값들.

ScenarioZ 1.28은 정규분포의 80% 구간이다. 실측 적중률은 **72.3%** 로 이론값보다
낮은데, 실제 수익률의 꼬리가 로그정규보다 두껍기 때문이다. 화면은 이론값이 아니라
실측값을 적는다.

ScenarioHorizonDays 90은 백테스트를 그 길이로 했기 때문이다. 길이를 바꾸면 적중률을
그 길이로 다시 재야 한다 — 화면의 「72.3%」가 정확히 이 90일을 가리킨다.

ScenarioTradingDays 63은 90일(달력) 동안의 거래일 수다. 변동성은 거래일로 자란다.
*/
const (
	ScenarioZ             = 1.28
	ScenarioHorizonDays   = 90
	ScenarioTradingDays   = 63
	ScenarioVolWindowDays = 365
	// ScenarioMinReturns — 이만큼의 일간 수익률이 없으면 변동성을 말하지 않는다.
	// 60거래일(약 3개월)은 분기 하나를 덮는 최소한이다.
	ScenarioMinReturns = 60
)

// ScenarioPoint is one of the three prices.
type ScenarioPoint struct {
	Kind  ScenarioKind `json:"kind"`
	Label string       `json:"label"`
	Price float64      `json:"price"`
	// ChangePct는 현재가 대비 %. 중립은 언제나 0이다.
	ChangePct float64 `json:"changePct"`
	/*
		ImpliedPER — 이 가격이면 받게 되는 배수.

		밴드 숫자에 **뜻**을 주는 유일한 줄이다. 「407,579원」은 아무 말도 안 하지만
		「그 가격이면 PER 61배」는 판단할 수 있다. 앱이 이미 쓰는 PER을 비례로 늘린
		것이라 다른 화면과 어긋나지 않는다. PER이 없는 종목(국내 자격 기준 40%)에서는
		nil이고, 화면은 그 줄을 그리지 않는다.
	*/
	ImpliedPER *float64 `json:"impliedPer,omitempty"`
}

/*
ScenarioAccuracy — **이 방법의** 성적. 종목 하나의 성적이 아니다.

종목별로는 표본이 6건뿐이라 적중률이라 부를 수 없다. 화면이 그 구분을 잃으면
「이 종목은 72% 맞는다」로 읽히는데, 그것은 측정한 적 없는 문장이다.
*/
type ScenarioAccuracy struct {
	// HitRate — 90일 뒤 실제 가격이 약세~강세 안에 있었던 비율(%).
	HitRate float64 `json:"hitRate"`
	// BaseErrorPct — 중립(현재가) 대비 90일 뒤 실제 가격의 평균 절대 오차(%).
	BaseErrorPct float64 `json:"baseErrorPct"`
	// MissBelowPct / MissAbovePct — 벗어난 방향. 실측에서 아래가 1.8배 많았다.
	MissBelowPct float64 `json:"missBelowPct"`
	MissAbovePct float64 `json:"missAbovePct"`
	// Samples / Symbols / Dates — 표본을 숨기지 않는다.
	Samples int `json:"samples"`
	Symbols int `json:"symbols"`
	Dates   int `json:"dates"`
	// MeasuredAt / Market — 언제, 어느 시장에서 잰 값인가 (요구 8).
	MeasuredAt string `json:"measuredAt,omitempty"`
	Market     string `json:"market,omitempty"`
}

// PriceScenario is one symbol's answer.
type PriceScenario struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name,omitempty"`
	// AsOf는 **종가 기준일**이다. 실시간 시세가 아니라는 것을 화면이 말해야 한다.
	AsOf     string  `json:"asOf"`
	Close    float64 `json:"close"`
	Currency string  `json:"currency,omitempty"`
	// HorizonDays는 언제나 ScenarioHorizonDays다. 응답에 실어 화면이 제목을 짓는다.
	HorizonDays int `json:"horizonDays"`
	/*
		VolatilityPct — 이 종목의 **90일 변동성**(%). 밴드 폭이 곧 이 값이다.

		실측 평균은 23.1%인데 종목마다 크게 다르다: 카카오 24.2% · 삼성전자 36.3% ·
		SK하이닉스 44.4%. 모든 종목에 ±20%를 찍는 화면과 갈라지는 지점이다.
	*/
	VolatilityPct float64         `json:"volatilityPct"`
	Points        []ScenarioPoint `json:"points"`
	// CurrentPER은 내재 PER의 기준값. 없으면 nil이고 내재 PER도 전부 nil이다.
	CurrentPER *float64 `json:"currentPer,omitempty"`
	// Accuracy는 방법의 성적. 계산이 아직이면 nil이고 화면은 그 줄을 비운다.
	Accuracy *ScenarioAccuracy `json:"accuracy,omitempty"`
	/*
		Unavailable — 시나리오를 못 만든 이유.

		"not_enough_history" | "no_close". 빈 카드를 그리는 것과 「왜 없는지」를 적는
		것은 다른 답이다 (요구 8).
	*/
	Unavailable string `json:"unavailable,omitempty"`
}
