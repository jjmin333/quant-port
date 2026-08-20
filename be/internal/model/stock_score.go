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
}

// ScoreDistribution is the batch's self-check (docs/STOCK_SCORE_DESIGN.md §7.1).
type ScoreDistribution struct {
	Count    int     `json:"count"`
	Mean     float64 `json:"mean"`
	StdDev   float64 `json:"stdDev"`
	Min      int     `json:"min"`
	Max      int     `json:"max"`
	AtFloor  int     `json:"atFloor"`  // score <= 1
	AtCeil   int     `json:"atCeil"`   // score >= 99
	Withheld int     `json:"withheld"` // coverage below floor
	Healthy  bool    `json:"healthy"`
	Warnings []string `json:"warnings,omitempty"`
}
