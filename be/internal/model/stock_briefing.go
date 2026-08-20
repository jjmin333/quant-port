package model

// MarketBriefing is the deterministic, rule-based read of a symbol that sits
// ahead of the AI 요약.
//
// It is produced entirely from figures the server already computed — indicators,
// statement ratios, the 종합 점수 breakdown — with no LLM involved. That makes it
// reproducible, auditable, free, and available for every symbol including the
// ones outside the LLM pilot. The AI section then adds interpretation on top
// rather than being the only thing the user gets.
type MarketBriefing struct {
	AsOf     string  `json:"asOf,omitempty"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency,omitempty"`

	// KeyPoints are the few things that most define this symbol right now,
	// ordered by how much they matter.
	KeyPoints []BriefingPoint `json:"keyPoints,omitempty"`

	Upside   *BriefingScenario `json:"upside,omitempty"`
	Downside *BriefingScenario `json:"downside,omitempty"`
}

// BriefingTone drives the colour of a point in the UI.
type BriefingTone string

const (
	TonePositive BriefingTone = "positive"
	ToneNegative BriefingTone = "negative"
	ToneNeutral  BriefingTone = "neutral"
)

// BriefingPoint is one investment point. Label is the short category, Text
// carries the actual figures so the claim is always checkable.
type BriefingPoint struct {
	Label string       `json:"label"`
	Text  string       `json:"text"`
	Tone  BriefingTone `json:"tone"`
}

// BriefingScenario is one direction's outlook plus the price levels that would
// confirm it.
type BriefingScenario struct {
	Summary string        `json:"summary"`
	Levels  []PriceLevel  `json:"levels,omitempty"`
}

// PriceLevel is a real, derived price — a moving average, a swing extreme, a
// 52-week bound — never a target invented for the narrative.
type PriceLevel struct {
	Label       string  `json:"label"`
	Price       float64 `json:"price"`
	DistancePct float64 `json:"distancePct"`
}
