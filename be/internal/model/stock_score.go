package model

import "time"

// ScoreAxis identifies one component of the 종합 점수. Kept as explicit
// constants (not free strings) so weights, breakdowns and the admin UI can
// never drift apart on a typo.
type ScoreAxis string

const (
	AxisTechnical ScoreAxis = "technical"
	AxisQuality   ScoreAxis = "quality"
	AxisGrowth    ScoreAxis = "growth"
	AxisValuation ScoreAxis = "valuation"
	AxisSentiment ScoreAxis = "sentiment"
)

// ScoreAxes is the canonical display order (weight editor and breakdown bars).
var ScoreAxes = []ScoreAxis{AxisTechnical, AxisQuality, AxisGrowth, AxisValuation, AxisSentiment}

// ScoreAxisLabels are the Korean labels shown in the admin UI and breakdown.
var ScoreAxisLabels = map[ScoreAxis]string{
	AxisTechnical: "추세",
	AxisQuality:   "수익성",
	AxisGrowth:    "성장",
	AxisValuation: "가치",
	AxisSentiment: "심리·수급",
}

// ScoreWeights is a full weight set. Values are relative — they are always
// renormalized over the axes that actually have data, so they need not sum to
// 100. The admin UI still presents them as percentages because that is what
// people reason about.
type ScoreWeights struct {
	Technical int `json:"technical"`
	Quality   int `json:"quality"`
	Growth    int `json:"growth"`
	Valuation int `json:"valuation"`
	Sentiment int `json:"sentiment"`
}

// DefaultScoreWeights matches migration 030's column defaults.
func DefaultScoreWeights() ScoreWeights {
	return ScoreWeights{Technical: 30, Quality: 25, Growth: 20, Valuation: 15, Sentiment: 10}
}

func (w ScoreWeights) Get(axis ScoreAxis) int {
	switch axis {
	case AxisTechnical:
		return w.Technical
	case AxisQuality:
		return w.Quality
	case AxisGrowth:
		return w.Growth
	case AxisValuation:
		return w.Valuation
	case AxisSentiment:
		return w.Sentiment
	}
	return 0
}

func (w *ScoreWeights) Set(axis ScoreAxis, v int) {
	switch axis {
	case AxisTechnical:
		w.Technical = v
	case AxisQuality:
		w.Quality = v
	case AxisGrowth:
		w.Growth = v
	case AxisValuation:
		w.Valuation = v
	case AxisSentiment:
		w.Sentiment = v
	}
}

func (w ScoreWeights) Total() int {
	return w.Technical + w.Quality + w.Growth + w.Valuation + w.Sentiment
}

// Clamped bounds every axis to 0..100 and guarantees a usable set: an all-zero
// input would make every score undefined, so it falls back to the defaults
// rather than silently disabling scoring.
func (w ScoreWeights) Clamped() ScoreWeights {
	out := ScoreWeights{}
	for _, axis := range ScoreAxes {
		v := w.Get(axis)
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
		out.Set(axis, v)
	}
	if out.Total() <= 0 {
		return DefaultScoreWeights()
	}
	return out
}

// ScoreSettings is the single-row global configuration.
type ScoreSettings struct {
	Weights ScoreWeights `json:"weights"`
	// CoverageFloor: below this share of weighted data the score is withheld.
	CoverageFloor float64 `json:"coverageFloor"`
	// ShrinkFull: coverage at or above this gets no shrink toward neutral.
	ShrinkFull float64   `json:"shrinkFull"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func DefaultScoreSettings() ScoreSettings {
	return ScoreSettings{
		Weights:       DefaultScoreWeights(),
		CoverageFloor: 0.20,
		ShrinkFull:    0.60,
	}
}

// SymbolScoreWeights is a per-symbol override of the global weights.
type SymbolScoreWeights struct {
	Symbol    string       `json:"symbol"`
	Name      string       `json:"name,omitempty"`
	Weights   ScoreWeights `json:"weights"`
	Note      string       `json:"note,omitempty"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

// ScoreBreakdown holds each axis's 0–100 score before weighting. nil means the
// axis had no usable data — never zero, which would read as a bad signal.
type ScoreBreakdown struct {
	Technical *float64 `json:"technical,omitempty"`
	Quality   *float64 `json:"quality,omitempty"`
	Growth    *float64 `json:"growth,omitempty"`
	Valuation *float64 `json:"valuation,omitempty"`
	Sentiment *float64 `json:"sentiment,omitempty"`
}

func (b ScoreBreakdown) Get(axis ScoreAxis) *float64 {
	switch axis {
	case AxisTechnical:
		return b.Technical
	case AxisQuality:
		return b.Quality
	case AxisGrowth:
		return b.Growth
	case AxisValuation:
		return b.Valuation
	case AxisSentiment:
		return b.Sentiment
	}
	return nil
}

func (b *ScoreBreakdown) Set(axis ScoreAxis, v *float64) {
	switch axis {
	case AxisTechnical:
		b.Technical = v
	case AxisQuality:
		b.Quality = v
	case AxisGrowth:
		b.Growth = v
	case AxisValuation:
		b.Valuation = v
	case AxisSentiment:
		b.Sentiment = v
	}
}

// StockScoreView is the API response for one symbol's latest score.
type StockScoreView struct {
	Symbol    string         `json:"symbol"`
	Name      string         `json:"name,omitempty"`
	TradeDate string         `json:"tradeDate"`
	Score     *int           `json:"score"`
	Grade     string         `json:"grade,omitempty"`
	ScoreKind string         `json:"scoreKind"`
	Coverage  float64        `json:"coverage"`
	PeerGroup string         `json:"peerGroup,omitempty"`
	// PeerPct is a 상위 N%: 1 is the best of the peer group, 100 the worst.
	// Read it through ReadPeerPct — see there for why 0 must never reach a
	// screen or an LLM prompt.
	PeerPct   *int           `json:"peerPct,omitempty"`
	Breakdown ScoreBreakdown `json:"breakdown"`
	Inputs    map[string]any `json:"inputs,omitempty"`
	// Weights actually used (global or per-symbol override), for transparency.
	Weights      ScoreWeights `json:"weights"`
	WeightSource string       `json:"weightSource"` // "global" | "symbol"
	ComputedAt   time.Time    `json:"computedAt"`
}

// ScoreGrade maps a final score to its display band. Bands are wide enough
// that a one-point move never changes the letter on its own (see hysteresis in
// the scoring service).
func ScoreGrade(score int) string {
	switch {
	case score >= 80:
		return "A+"
	case score >= 70:
		return "A"
	case score >= 60:
		return "B+"
	case score >= 50:
		return "B"
	case score >= 40:
		return "C+"
	case score >= 30:
		return "C"
	default:
		return "D"
	}
}

// StockScoreListItem is one row of the 국내/해외 score ranking list — the
// composite score plus just enough quote context (change rate, price) for a
// leaderboard row, without the full per-symbol breakdown.
type StockScoreListItem struct {
	Rank       int      `json:"rank"`
	Symbol     string   `json:"symbol"`
	Name       string   `json:"name"`
	AssetType  string   `json:"assetType"`
	Score      int      `json:"score"`
	Grade      string   `json:"grade,omitempty"`
	PeerPct    *int     `json:"peerPct,omitempty"`
	ChangeRate *float64 `json:"changeRate,omitempty"`
	ClosePrice *float64 `json:"closePrice,omitempty"`
	Currency   string   `json:"currency,omitempty"`
	TradeDate  string   `json:"tradeDate"`
	/*
		등락률·가격이 **무엇을 잰 값인지**. `"live"` 아니면 종가의 거래일(YYYY-MM-DD).

		랭킹은 하루 한 번 도는 배치가 만들지만 가격은 계속 움직인다. 장이 열려 있는
		동안(프리·정규·애프터)에는 실시간 시세로 덮어써 `"live"`가 되고, 애프터마켓까지
		끝나면 확정된 종가를 쓰고 그 거래일이 들어온다.

		**점수의 날(`TradeDate`)과 비교할 값이 아니다.** 점수의 날은 KST 날짜라 해외
		종목의 종가는 정상적으로 하루 뒤에 온다 — 그걸 「밀렸다」로 읽으면 해외 랭킹은
		언제나 밀린 것으로 보인다(실측: 08-31 점수에 08-28 종가, 전 종목 회색).
		이 필드는 그 판정을 대신하는 게 아니라 **출처를 그대로 말한다.**
	*/
	PriceAsOf string `json:"priceAsOf,omitempty"`

	/*
		RankValue는 **이 목록이 실제로 줄 세운 값**이다.

		AI 추천 랭킹은 종합 점수 말고도 다섯 축 각각으로 순위를 매길 수 있고(추세·수익성·
		성장·가치·심리·수급), 기간을 고르면 그 기간 안의 **최고점**으로 매긴다. 그때 행에
		적혀야 하는 숫자는 `Score`(최신일 종합)가 아니라 이 값이다 — 둘이 다른데 종합만
		보이면 목록이 뒤죽박죽으로 정렬된 것처럼 읽힌다.

		종합·최신일 조합에서는 `Score`와 같으므로 화면은 한 자리만 그리면 된다.
		점수가 없는 축이면 비어 있고, 그런 종목은 애초에 그 목록에 들어가지 않는다.
	*/
	RankValue *int `json:"rankValue,omitempty"`

	/*
		실시간 등락률의 기준가를 정할 때 쓰는 재료 — 화면에는 나가지 않는다.

		`ClosePrice`가 어느 세션 것인지(`PriceAsOf`)에 따라 「직전 종가」가 달라진다:
		저장된 마지막 종가가 아직 오늘 것이 아니면 그게 곧 직전 종가이고, 오늘 것이
		이미 들어와 있으면(미국 애프터마켓 중에는 그렇다) 그 행의 `prev_close`가
		직전 종가다. 서버가 이 둘 중 하나를 골라 쓴다.
	*/
	PrevClose *float64 `json:"-"`

	/*
		BasePrice — 등락률의 **기준가**. 화면이 스트림 가격으로 등락률을 낼 때 쓴다.

		위 `PrevClose`가 재료라면 이것은 **서버가 실제로 고른 답**이다(`scorePrevCloseHint`).
		둘을 나눈 이유는 화면이 그 고르는 규칙을 다시 구현하면 안 되기 때문이다 —
		「저장된 종가가 오늘 것인가」는 그 종목 시장의 표준시로 재야 하고, KST 자정을
		넘긴 미국 애프터마켓을 국내 날짜로 재면 한 세션씩 밀린다.

		이 값이 나가는 이유는 목록 가격이 SSE로 오기 때문이다(2026-09-04). 스트림은
		**가격만** 보낸다 — 기준가는 세션 안에서 안 변하니 목록이 뜰 때 한 번 받아 두면
		되고, 서버가 틱마다 종목별로 그걸 풀면 종목당 4~5 SELECT가 붙는다(실측: 이
		엔드포인트 20줄이 100 SELECT였다). 그 곱셈을 없애려고 푸시로 옮긴 것이라
		서버가 그 산수를 계속하면 앞뒤가 안 맞는다.

		비어 있으면 화면은 등락률을 **그리지 않는다**(0.00%로 때우지 않는다).
	*/
	BasePrice *float64 `json:"basePrice,omitempty"`
}

// ScoreDistribution is the batch's self-check (docs/STOCK_SCORE_DESIGN.md §7.1).
type ScoreDistribution struct {
	Count    int      `json:"count"`
	Mean     float64  `json:"mean"`
	StdDev   float64  `json:"stdDev"`
	Min      int      `json:"min"`
	Max      int      `json:"max"`
	AtFloor  int      `json:"atFloor"`  // score <= 1
	AtCeil   int      `json:"atCeil"`   // score >= 99
	Withheld int      `json:"withheld"` // coverage below floor
	Healthy  bool     `json:"healthy"`
	Warnings []string `json:"warnings,omitempty"`
}

// ScorePriceAsOfLive is the StockScoreListItem.PriceAsOf value meaning "this is
// the price right now", as opposed to a settled close's trade date.
const ScorePriceAsOfLive = "live"

// ReadPeerPct normalizes a stored peer rank on the way out.
//
// The rank is a 상위 N% and the score batch floors it at 1, but rows written
// before that floor existed carry 0 — the batch's percentile puts the single
// best symbol of a group at the 100th percentile, which inverts to 0. 「동종
// 상위 0%」 is not something anyone says, and on 09-08 twenty of the top 100 US
// scores read that way. Correcting on read means a stale row never reaches a
// screen or an LLM prompt saying it; the daily batch fixes the stored value.
func ReadPeerPct(v int) int {
	if v < 1 {
		return 1
	}
	if v > 100 {
		return 100
	}
	return v
}
