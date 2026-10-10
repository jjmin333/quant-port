package model

/*
포트폴리오 진단 — 「샀을 때의 이유가 지금도 유효한가」 (V2 요구 6).

# 이 기능이 **할 수 없는** 것부터

가장 자연스러운 질문은 「살 때 점수가 몇이었고 지금은 몇인가」다. 그런데 이 앱에서는
대부분의 보유 종목에 그 답이 **없다**:

	보유 매수    2026-03-03부터 (실측: 48건 중 대부분이 이 날짜)
	점수 보관    2026-08-07부터 (30거래일)

`stock_scores`는 행당 1.5KB × 17,906종목/일이라 연 7.5GB로 자라고, 그래서 30거래일만
남긴다(`repository.ScoreRetentionDates`). 다섯 달 전에 산 종목의 그날 점수는 **지워졌고
되살릴 수 없다.**

그 사실을 빈칸으로 두면 화면이 「점수가 안 변했다」로 읽힌다. 그래서 이 타입들은
`ScoreAtPurchase`를 **포인터로** 들고, 없으면 왜 없는지(`ScoreAtPurchaseReason`)를
함께 낸다 (요구 8).

# 그래서 무엇으로 판단하나

보관 기간보다 **깊은 기록이 있는 신호**들이다:

	점수 추세    보관 창 안에서 어디서 어디로 왔나 (2026-08-07~)
	실적        보유 기간 중 발표된 실적의 서프라이즈 (2025-07~)
	내부자      보유 기간 중 순매수·순매도 (2026-07~)

# 수익률은 신호가 아니다

「올랐으니 이유가 강화됐다」는 동어반복이다 — 판단의 근거가 아니라 판단의 **결과**다.
수익률은 화면에 나가지만 판정에는 들어가지 않는다. 이 구분이 없으면 이 기능은
「오른 종목은 좋은 종목」이라고 말하는 값비싼 장치가 된다.
*/

// DiagnosisVerdict is the four-way answer.
type DiagnosisVerdict string

const (
	// DiagnosisStronger — 신호들이 대체로 좋아졌다.
	DiagnosisStronger DiagnosisVerdict = "stronger"
	// DiagnosisSteady — 뚜렷한 방향이 없다.
	DiagnosisSteady DiagnosisVerdict = "steady"
	// DiagnosisWeaker — 신호들이 대체로 나빠졌다.
	DiagnosisWeaker DiagnosisVerdict = "weaker"
	/*
		DiagnosisUnknown — **판단 보류.**

		네 번째가 있어야 나머지 셋이 정직해진다. 신호가 하나뿐인데 「유지」라고 말하면
		그것은 판단이 아니라 기본값이다.
	*/
	DiagnosisUnknown DiagnosisVerdict = "unknown"
)

var DiagnosisVerdictLabels = map[DiagnosisVerdict]string{
	DiagnosisStronger: "강화",
	DiagnosisSteady:   "유지",
	DiagnosisWeaker:   "약화",
	DiagnosisUnknown:  "판단 보류",
}

// DiagnosisSignalID identifies one input to the verdict.
type DiagnosisSignalID string

const (
	SignalScoreTrend DiagnosisSignalID = "score_trend"
	SignalEarnings   DiagnosisSignalID = "earnings"
	SignalInsider    DiagnosisSignalID = "insider"
)

var DiagnosisSignalLabels = map[DiagnosisSignalID]string{
	SignalScoreTrend: "AI 점수 추세",
	SignalEarnings:   "실적",
	SignalInsider:    "내부자 거래",
}

/*
DiagnosisSignal — 판정에 쓰인(또는 못 쓰인) 신호 하나.

`Available`이 false면 `Direction`은 뜻이 없다. **못 쓴 신호도 목록에 남긴다** — 무엇을
보고 판단했는지보다 무엇을 **못 보고** 판단했는지가 더 중요할 때가 있다.
*/
type DiagnosisSignal struct {
	ID        DiagnosisSignalID `json:"id"`
	Label     string            `json:"label"`
	Available bool              `json:"available"`
	// Reason은 못 쓴 이유: "no_data" | "out_of_retention" | "not_collected_yet".
	Reason string `json:"reason,omitempty"`
	// Direction: -1 악화 · 0 중립 · +1 개선. Available일 때만 읽는다.
	Direction int `json:"direction"`
	// Detail은 숫자로 지은 한 문장 — 「59 → 46 (−13)」.
	Detail string `json:"detail,omitempty"`
	/*
		Strong — 이 신호 **하나만으로도** 방향을 말할 만큼 컸는가 (문턱의 두 배).

		「신호가 둘 미만이면 보류」라는 규칙은 약한 증거를 막으려는 것인데, 실측에서
		그 규칙이 **가장 강한 증거를 묻었다** — 카카오는 매수 시점 75점이 29점으로
		46점 떨어졌는데 실적·내부자 기록이 없어 「판단 보류」가 됐다. 약한 것은 증거의
		**다양성**이지 강도가 아니다.
	*/
	Strong bool `json:"strong,omitempty"`
}

// DiagnosisScore is the score lane of one holding.
type DiagnosisScore struct {
	Current *int   `json:"current"`
	Grade   string `json:"grade,omitempty"`
	AsOf    string `json:"asOf,omitempty"`
	// FromDate / FromValue는 **보관 창의 첫 회차**다 — 매수 시점이 아니다.
	FromDate  string `json:"fromDate,omitempty"`
	FromValue *int   `json:"fromValue,omitempty"`
	Delta     *int   `json:"delta,omitempty"`
	// Driver는 Phase 1 귀인이 만든 최근 한 문장.
	Driver string `json:"driver,omitempty"`
	// Thesis — 사용자가 「살 때 본 축」을 적었으면 그 축들의 평균 변화. 있으면 점수 신호가
	// 종합 대신 이것을 쓴다(2026-09-24 사용자 결정 — 매수 이유 태그).
	Thesis *DiagnosisThesis `json:"thesis,omitempty"`
}

// DiagnosisThesis is the average of the user's thesis axes, then and now.
type DiagnosisThesis struct {
	Axes      []string `json:"axes"`
	Label     string   `json:"label"`
	FromDate  string   `json:"fromDate,omitempty"`
	FromValue *int     `json:"fromValue,omitempty"`
	Current   *int     `json:"current,omitempty"`
	Delta     *int     `json:"delta,omitempty"`
}

// DiagnosisEarnings is the most recent earnings inside the holding period.
type DiagnosisEarnings struct {
	Count  int    `json:"count"`
	Period string `json:"period,omitempty"`
	Date   string `json:"date,omitempty"`
	// Surprise는 추정치가 있을 때만 — 없으면 「예상대로였다」가 아니라 「비교할 예상이 없었다」.
	Surprise *float64 `json:"surprise,omitempty"`
}

// DiagnosisInsider aggregates insider trades inside the holding period.
type DiagnosisInsider struct {
	Count      int     `json:"count"`
	BuyShares  float64 `json:"buyShares"`
	SellShares float64 `json:"sellShares"`
	NetShares  float64 `json:"netShares"`
}

// HoldingDiagnosis is one held symbol's answer.
type HoldingDiagnosis struct {
	Symbol   string   `json:"symbol"`
	Name     string   `json:"name,omitempty"`
	Quantity float64  `json:"quantity"`
	AvgPrice *float64 `json:"avgPrice,omitempty"`
	Currency string   `json:"currency,omitempty"`
	// LastClose는 **종가**다. 실시간 시세가 아니라는 것을 화면이 말해야 한다.
	LastClose     *float64 `json:"lastClose,omitempty"`
	LastCloseDate string   `json:"lastCloseDate,omitempty"`
	// ReturnPct는 화면에 나가지만 **판정에는 쓰이지 않는다** — 머리글 참조.
	ReturnPct *float64 `json:"returnPct,omitempty"`

	FirstBought string `json:"firstBought,omitempty"`
	HoldingDays int    `json:"holdingDays,omitempty"`

	Verdict DiagnosisVerdict  `json:"verdict"`
	Signals []DiagnosisSignal `json:"signals"`
	// SignalCount는 **실제로 쓴** 신호 수. 화면이 「근거 1개」를 적을 수 있어야
	// 사용자가 판정의 무게를 스스로 정한다.
	SignalCount int `json:"signalCount"`

	Score    *DiagnosisScore    `json:"score,omitempty"`
	Earnings *DiagnosisEarnings `json:"earnings,omitempty"`
	Insider  *DiagnosisInsider  `json:"insider,omitempty"`

	/*
		ScoreAtPurchase — 살 때의 점수. **거의 언제나 nil이다.**

		점수는 30거래일만 보관하고 대부분의 매수는 그보다 오래됐다. nil을 빈칸으로
		그리면 「안 변했다」로 읽히므로, 왜 없는지를 함께 낸다.
	*/
	ScoreAtPurchase       *int   `json:"scoreAtPurchase,omitempty"`
	ScoreAtPurchaseReason string `json:"scoreAtPurchaseReason,omitempty"`
}

// HoldingsDiagnosisSummary counts the verdicts.
type HoldingsDiagnosisSummary struct {
	Stronger int `json:"stronger"`
	Steady   int `json:"steady"`
	Weaker   int `json:"weaker"`
	Unknown  int `json:"unknown"`
}

// HoldingsDiagnosis is the whole payload.
type HoldingsDiagnosis struct {
	Items   []HoldingDiagnosis       `json:"items"`
	Summary HoldingsDiagnosisSummary `json:"summary"`
	/*
		ScoreRetentionFrom — 점수가 **언제부터** 남아 있는지.

		화면이 「매수 시점과 비교할 수 없습니다」를 말할 때 그 근거가 이 날짜다. 없는
		것을 없다고만 하면 사용자는 고장으로 읽는다.
	*/
	ScoreRetentionFrom string `json:"scoreRetentionFrom,omitempty"`
	// Note는 한 줄 안내 — 보관 한계를 사람 문장으로.
	Note string `json:"note,omitempty"`
}

/*
진단 문턱 — 신호 하나가 「움직였다」고 말하려면 얼마나 움직여야 하나.

점수 5점: 등급 밴드가 10점 폭이므로(A 70~79 등) 그 절반이다. 매일 1~2점씩 흔들리는
값이라 그보다 낮게 잡으면 모든 종목이 매일 방향을 바꾼다.

서프라이즈 5%: 컨센서스 대비 이 정도는 돼야 「빗나갔다」고 부른다.

최소 신호 2개: 하나로 판정하면 그것은 판단이 아니라 그 신호의 별명이다. **다만 그 하나가
문턱의 두 배를 넘으면 판정한다** — 실측에서 이 규칙이 매수 시점 대비 −46점이라는 가장
강한 증거를 「판단 보류」로 묻었다. 약한 것은 증거의 다양성이지 강도가 아니다.
*/
const (
	DiagnosisScoreDelta       = 5
	DiagnosisSurprisePct      = 5.0
	DiagnosisMinSignals       = 2
	DiagnosisVerdictThreshold = 2
	/*
		DiagnosisInsiderVolumePct — 내부자 순매매가 **하루 거래량의 1%**는 돼야 방향으로 센다.

		순매매에는 크기 개념이 없었다. 애플은 3건·4,316주 순매도가 다른 종목의 수백만 주와
		똑같이 −1로 세어졌는데, 그것은 하루 거래량(4,084만 주)의 **0.011%**다. 실측(2026-09-18)
		네 종목의 하루 거래량 대비 순매매:

			애플        0.011%      삼성전자    0.077%
			SK하이닉스   0.27%       엔비디아     4.7%

		1%는 이 네 점에 맞춘 값이 아니라 그 위아래 어디에 그어도 같은 편을 가르는 간격이다.
		하루치 거래량조차 1% 못 채우는 움직임을 「내부자가 팔고 있다」고 읽을 근거가 없다.
		모자라면 신호를 **버리는 게 아니라 방향을 0으로** 둔다 — 재 보고 방향이 없었다는
		것과 재지 못했다는 것은 다른 말이다.
	*/
	DiagnosisInsiderVolumePct = 0.01
	/*
		DiagnosisStrongWeight — 강한 신호는 두 몫을 센다.

		이게 없으면 **아무 말도 안 하는 증거가 판정을 뒤집는다.** 실측에서 카카오는 강한
		−46 하나로 「약화」였는데, 삼성전자는 강한 −13 옆에 방향 0짜리 신호가 하나 붙자
		합이 문턱(2)에 못 미쳐 「유지」가 됐다. 침묵하는 신호를 더했다고 답이 바뀌면
		그것은 판정이 아니라 신호 개수 세기다.
	*/
	DiagnosisStrongWeight = 2
)
