package model

type PortfolioHistoryPoint struct {
	At    string  `json:"at"`
	Label string  `json:"label,omitempty"`
	Value float64 `json:"value"`
}

type PortfolioHistory struct {
	Range     string                  `json:"range"`
	Points    []PortfolioHistoryPoint `json:"points"`
	DataSince string                  `json:"dataSince,omitempty"`
}

type RiskAlert struct {
	Code    string             `json:"code"`
	Type    string             `json:"type"`
	Message string             `json:"message"`
	Metrics map[string]float64 `json:"metrics,omitempty"`
}

type BriefingEvent struct {
	Date   string `json:"date"`
	Symbol string `json:"symbol"`
	Event  string `json:"event"`
}

type MorningBriefing struct {
	Summary     string          `json:"summary"`
	Highlights  []string        `json:"highlights"`
	Earnings    []BriefingEvent `json:"earnings"`
	Disclosures []BriefingEvent `json:"disclosures"`
	Volatility  string          `json:"volatility"`
	GeneratedAt string          `json:"generatedAt"`
	Source      string          `json:"source"`
}

type DashboardInsights struct {
	Briefing    MorningBriefing `json:"briefing"`
	Risks       []RiskAlert     `json:"risks"`
	GeneratedAt string          `json:"generatedAt"`
}

type InsightsDiagnostics struct {
	LLMEnabled      bool     `json:"llmEnabled"`
	DartEnabled     bool     `json:"dartEnabled"`
	EarningsEnabled bool     `json:"earningsEnabled"`
	CheckedAt       string   `json:"checkedAt"`
	Hints           []string `json:"hints,omitempty"`
}
