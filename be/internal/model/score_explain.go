package model

import "time"

/*
점수 설명 — 「왜 이 점수인가」와 「왜 어제와 다른가」에 **저장된 값만으로** 답한다.

# 왜 새 계산이 아니라 설명인가

종합 점수는 하루 한 번 배치가 만들고, 그때 쓴 재료를 `stock_scores.inputs`에 통째로
남긴다(지표 원값 · 피어 백분위 · 어느 그룹에서 순위를 매겼는지 · 도메인 밖이라 버린 값 ·
기술적 지표 전부). 이 파일의 타입들은 **그 저장분을 읽어 배열할 뿐** 아무것도 다시
계산하지 않는다 — LLM도 부르지 않고 외부 API도 치지 않는다.

# 귀인이 근사가 아닌 이유

종합 점수는 `raw = Σ(가중치×축점수) / Σ(쓰인 가중치)` 뒤에 커버리지만큼 중립(50)으로
당기는 축소(`shrink`)를 한 값이다. 가중치·축 유무·축소율을 고정하면 raw는 축 점수에
대해 **선형**이라, 두 날짜 사이의 점수 차이를 다음 셋으로 **잔차 없이** 가른다.

 1. 축 점수가 움직였다 (축마다 정확히 `shrink × 가중치비중 × Δ축점수`)
 2. 있던 축이 사라졌거나 없던 축이 생겼다 (분모가 바뀐다)
 3. 커버리지가 바뀌어 축소율이 달라졌다

남는 잔차는 **정수 반올림뿐**이고, 그 값도 숨기지 않고 `roundingResidual`로 적는다.
「대충 추세 때문인 것 같다」와 「추세가 −2.3점, 심리축 소멸이 −0.4점」은 다른 제품이다.
*/

// ScoreExplain is the full payload of GET /stocks/{symbol}/score/explain.
type ScoreExplain struct {
	Symbol    string `json:"symbol"`
	Name      string `json:"name,omitempty"`
	TradeDate string `json:"tradeDate"`
	// Score is nil when coverage fell below the floor and the batch withheld
	// the number. That is a state the screen must render as 「보류」, not 0.
	Score     *int      `json:"score"`
	Grade     string    `json:"grade,omitempty"`
	ScoreKind string    `json:"scoreKind"`
	Coverage  float64   `json:"coverage"`
	PeerGroup string    `json:"peerGroup,omitempty"`
	PeerPct   *int      `json:"peerPct,omitempty"`
	AssetType string    `json:"assetType,omitempty"`
	Close     *float64  `json:"close,omitempty"`
	MarketCap *float64  `json:"marketCap,omitempty"`
	BarCount  int       `json:"barCount,omitempty"`
	ComputedAt time.Time `json:"computedAt"`

	Weights      ScoreWeights `json:"weights"`
	WeightSource string       `json:"weightSource"`

	Composite ScoreComposite        `json:"composite"`
	Axes      []ScoreAxisExplain    `json:"axes"`
	Rejected  []ScoreRejectedMetric `json:"rejected,omitempty"`
	Changes   []ScoreChange         `json:"changes"`
	History   []ScoreHistoryPoint   `json:"history"`

	// Provenance answers 요구 8 for this one number: 언제 기준인지, 어디서 왔는지,
	// 추정인지 실제인지. Carried on the payload itself so a screen can never
	// show the score without being able to show its stamp.
	Provenance ScoreProvenance `json:"provenance"`
}

// ScoreComposite exposes the arithmetic behind the single number, so a reader
// can check it rather than take it on faith.
type ScoreComposite struct {
	// Raw is the weighted average before the coverage shrink.
	Raw float64 `json:"raw"`
	// Final is after the shrink, before rounding to the displayed integer.
	Final float64 `json:"final"`
	// ShrinkFactor is 1 when coverage is complete enough that the score is
	// reported as computed; below that the score is pulled toward 50.
	ShrinkFactor float64 `json:"shrinkFactor"`
	// UsedWeight / TotalWeight — the axes that actually had data versus all of
	// them. 「90/100」 is the honest way to say 심리·수급 축이 비어 있다.
	UsedWeight  int `json:"usedWeight"`
	TotalWeight int `json:"totalWeight"`
	// CoverageFloor / ShrinkFull are the settings in force, so the same screen
	// can say why a score was withheld or damped.
	CoverageFloor float64 `json:"coverageFloor"`
	ShrinkFull    float64 `json:"shrinkFull"`
	// Withheld: coverage below the floor — the batch stored no score at all.
	Withheld bool `json:"withheld"`
}

// ScoreAxisExplain is one of the five axes with its contribution made explicit.
type ScoreAxisExplain struct {
	Axis  ScoreAxis `json:"axis"`
	Label string    `json:"label"`
	// Score is nil when the axis had no usable data — never 0, which reads as
	// 「최하위」.
	Score  *float64 `json:"score"`
	Weight int      `json:"weight"`
	// WeightShare is the weight renormalized over the axes that actually had
	// data (%). This is the number that decides how much the axis moved the
	// composite — the raw weight overstates it whenever an axis is missing.
	WeightShare float64 `json:"weightShare"`
	// Contribution is this axis's share of the final score, in points. The
	// contributions plus the shrink adjustment reconstruct Final exactly.
	Contribution *float64 `json:"contribution,omitempty"`
	Available    bool     `json:"available"`
	// MissingReason is filled only when the axis is absent: "no_data" or, for
	// ETFs on the statement axes, "not_applicable".
	MissingReason string               `json:"missingReason,omitempty"`
	Metrics       []ScoreMetricExplain `json:"metrics,omitempty"`
}

// ScoreMetricExplain is one raw figure and the percentile it became.
type ScoreMetricExplain struct {
	Key   string    `json:"key"`
	Label string    `json:"label"`
	Axis  ScoreAxis `json:"axis"`
	// Unit: "%" | "x" | "score". The screen needs it to format 29.9 as 29.9%
	// and 2.33 as 2.33배.
	Unit string `json:"unit"`
	// HigherIsBetter is about the RAW figure, not the percentile. 부채비율은
	// 낮을수록 좋고, 그래서 29.9%가 백분위 87.5로 뒤집혀 들어간다 — 그 뒤집힘을
	// 화면이 설명할 수 있어야 「낮은데 왜 점수가 높지」가 안 생긴다.
	HigherIsBetter bool     `json:"higherIsBetter"`
	Raw            *float64 `json:"raw,omitempty"`
	Percentile     *float64 `json:"percentile,omitempty"`
	// RankedIn is the peer level the percentile was actually measured in. A
	// surprising percentile is almost always explained by this field.
	RankedIn string `json:"rankedIn,omitempty"`
}

// ScoreRejectedMetric is a figure the batch saw and refused to use.
//
// 버린 값을 숨기면 「이 회사는 수익성 점수가 왜 없나」에 답할 수 없다. 도메인 밖
// 값은 대개 분모가 0에 가깝거나 단위가 섞인 깨진 입력이고, 그 사실 자체가 정보다.
type ScoreRejectedMetric struct {
	Key      string    `json:"key"`
	Label    string    `json:"label"`
	Axis     ScoreAxis `json:"axis"`
	Unit     string    `json:"unit"`
	Value    float64   `json:"value"`
	DomainLo float64   `json:"domainLo"`
	DomainHi float64   `json:"domainHi"`
	Reason   string    `json:"reason"` // "out_of_domain"
}

// ScoreChangeWindow identifies one comparison window.
type ScoreChangeWindow string

const (
	ScoreChangePrevDay  ScoreChangeWindow = "1d"
	ScoreChangePrevWeek ScoreChangeWindow = "1w"
)

var ScoreChangeWindowLabels = map[ScoreChangeWindow]string{
	ScoreChangePrevDay:  "전일 대비",
	ScoreChangePrevWeek: "전주 대비",
}

// ScoreChange is one window's decomposition.
type ScoreChange struct {
	Window   ScoreChangeWindow `json:"window"`
	Label    string            `json:"label"`
	FromDate string            `json:"fromDate,omitempty"`
	ToDate   string            `json:"toDate"`
	// Available is false when there is no comparable row — the symbol is newly
	// scored, or the batch did not run. 「변화 없음」과 「비교할 것이 없음」은
	// 다른 답이라 0으로 때우지 않는다.
	Available bool `json:"available"`
	// Reason explains an unavailable window: "no_prior_row" | "score_withheld".
	Reason string `json:"reason,omitempty"`

	FromScore *int `json:"fromScore,omitempty"`
	ToScore   *int `json:"toScore,omitempty"`
	// Delta is the displayed integer difference.
	Delta *int `json:"delta,omitempty"`
	// ExactDelta is the difference before rounding, which is what the drivers
	// actually sum to.
	ExactDelta float64             `json:"exactDelta"`
	Drivers    []ScoreChangeDriver `json:"drivers"`
	// RoundingResidual is Delta − ExactDelta. It is the ONLY unexplained part,
	// and it is reported rather than quietly folded into a driver.
	RoundingResidual float64 `json:"roundingResidual"`
}

// ScoreChangeDriverKind distinguishes the three ways a score can move.
type ScoreChangeDriverKind string

const (
	// DriverAxis — 축 점수 자체가 움직였다.
	DriverAxis ScoreChangeDriverKind = "axis"
	// DriverAvailability — 축이 생기거나 사라져 분모가 바뀌었다. 점수가 오른
	// 것처럼 보이는 날의 진짜 원인이 자주 이것이다.
	DriverAvailability ScoreChangeDriverKind = "availability"
	// DriverShrink — 커버리지가 바뀌어 중립으로 당기는 정도가 달라졌다.
	DriverShrink ScoreChangeDriverKind = "shrink"
	// DriverWeight — 가중치 설정이 달라졌다. 관리자 화면의 「바꿔 보기」에서만 나온다.
	DriverWeight ScoreChangeDriverKind = "weight"
)

// ScoreChangeDriver is one attributed cause, in points of the final score.
type ScoreChangeDriver struct {
	Kind  ScoreChangeDriverKind `json:"kind"`
	Axis  ScoreAxis             `json:"axis,omitempty"`
	Label string                `json:"label"`
	// Detail is a short factual sentence built from the numbers — not an LLM
	// sentence. 「추세 71.2 → 26.9」.
	Detail string `json:"detail"`
	// Effect is this cause's contribution to the score change, in points.
	Effect float64            `json:"effect"`
	From   *float64           `json:"from,omitempty"`
	To     *float64           `json:"to,omitempty"`
	Metrics []ScoreMetricDelta `json:"metrics,omitempty"`
}

// ScoreMetricDelta is one metric's movement inside an axis driver.
type ScoreMetricDelta struct {
	Key            string    `json:"key"`
	Label          string    `json:"label"`
	Axis           ScoreAxis `json:"axis,omitempty"`
	Unit           string    `json:"unit"`
	HigherIsBetter bool      `json:"higherIsBetter"`
	FromRaw        *float64  `json:"fromRaw,omitempty"`
	ToRaw          *float64  `json:"toRaw,omitempty"`
	FromPercentile *float64  `json:"fromPercentile,omitempty"`
	ToPercentile   *float64  `json:"toPercentile,omitempty"`
	// PercentileDelta is what actually moved the axis, since the axis score is
	// the mean of its metrics' percentiles.
	PercentileDelta *float64 `json:"percentileDelta,omitempty"`
}

// ScoreHistoryPoint is one dated point for the trend line. Deliberately thinner
// than StockScoreView — the sparkline needs the score and the axes, not the
// whole inputs blob, and this payload already carries one full day of those.
type ScoreHistoryPoint struct {
	TradeDate string         `json:"tradeDate"`
	Score     *int           `json:"score"`
	Coverage  float64        `json:"coverage"`
	Breakdown ScoreBreakdown `json:"breakdown"`
}

// ScoreDataBasis says whether a figure is measured or estimated. 요구 8.
type ScoreDataBasis string

const (
	// BasisActual — 확정된 실제값으로 계산했다.
	BasisActual ScoreDataBasis = "actual"
	// BasisPartial — 일부 축이 비어 커버리지가 100%가 아니다. 점수는 그만큼
	// 중립으로 당겨져 있다.
	BasisPartial ScoreDataBasis = "partial"
)

// ScoreProvenance is the 기준시각·출처·추정 여부 stamp for the score.
type ScoreProvenance struct {
	// AsOf is the trade date the inputs belong to (KST calendar date).
	AsOf string `json:"asOf"`
	// ComputedAt is when the batch produced the row.
	ComputedAt time.Time      `json:"computedAt"`
	Basis      ScoreDataBasis `json:"basis"`
	// Sources lists where the inputs came from, so the screen does not have to
	// hardcode a list that drifts from the batch.
	Sources []string `json:"sources"`
	// Note is a one-line caveat when there is one (e.g. ETF technical-only).
	Note string `json:"note,omitempty"`
}

/*
ScoreChangeSummary — 목록 한 행에 실리는 **한 줄짜리** 점수 변화.

`ScoreExplain`과 같은 답을 하지만 크기가 다르다. 설명 패널은 한 종목을 펼쳤을 때
한 번 부르는 15KB짜리이고, 이것은 **행마다** 필요한 60B짜리다. 목록에 큰 쪽을
실으면 20종목에 300KB와 스무 번의 조회가 된다(PRINCIPLES 「응답 속도 원칙」 ①·④).

그래서 지표 표도, 기술적 커브도, 이력도 싣지 않는다 — 변화량과 **가장 큰 원인 하나**뿐이다.
*/
type ScoreChangeSummary struct {
	Symbol    string `json:"symbol"`
	TradeDate string `json:"tradeDate"`
	Score     *int   `json:"score"`
	// Available false → 비교할 직전 회차가 없거나 한쪽 점수가 보류였다.
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	FromDate  string `json:"fromDate,omitempty"`
	FromScore *int   `json:"fromScore,omitempty"`
	Delta     *int   `json:"delta,omitempty"`
	// TopDriver is the single largest cause. nil when nothing moved.
	TopDriver *ScoreChangeDriver `json:"topDriver,omitempty"`
}

/*
ScoreWeightChange — 가중치를 바꾸면 이 종목의 점수가 어떻게 되는가 (관리자 전용).

# 왜 축마다의 효과가 순서에 기대는가

종합 점수는 `Σ(w·v) / Σw`라 **가중치에 대해서는 선형이 아니다**(분모에도 들어간다).
축 값 변화와 달리 「각 축의 몫」이 서로 독립이 아니므로, 여기서는 축을 정해진 순서로
하나씩 바꾸며 그때마다의 증분을 적는다. 합은 정확하지만 **각 항은 그 순서에서의
몫**이고, 응답이 그 순서를 숨기지 않는다(`Order`).
*/
type ScoreWeightChange struct {
	Symbol      string       `json:"symbol"`
	TradeDate   string       `json:"tradeDate"`
	FromWeights ScoreWeights `json:"fromWeights"`
	ToWeights   ScoreWeights `json:"toWeights"`
	FromScore   int          `json:"fromScore"`
	ToScore     int          `json:"toScore"`
	Delta       int          `json:"delta"`
	ExactDelta  float64      `json:"exactDelta"`
	// Order is the axis order the cascade used, so a reader can reproduce it.
	Order            []ScoreAxis         `json:"order"`
	Drivers          []ScoreChangeDriver `json:"drivers"`
	RoundingResidual float64             `json:"roundingResidual"`
}
