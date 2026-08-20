package model

// NewsSentimentSummary aggregates per-article sentiment labels. Counts are
// always computed from real article ids server-side — never reported
// directly by the LLM — so Total always matches the number of articles fed
// into the prompt.
type NewsSentimentSummary struct {
	Positive     int     `json:"positive"`
	Negative     int     `json:"negative"`
	Neutral      int     `json:"neutral"`
	Total        int     `json:"total"`
	AverageScore float64 `json:"averageScore"` // -100..100
}

// StockSummaryView is the full response for GET /stocks/{symbol}/summary —
// one ~3-sentence text block per tab category, shared across every viewer of
// the symbol (see StockSummaryService for the caching/refresh design).
type StockSummaryView struct {
	Symbol        string                `json:"symbol"`
	AssetType     QuoteAssetType        `json:"assetType"`
	Valuation     string                `json:"valuation"`
	Technical     string                `json:"technical"`
	Insider       string                `json:"insider"`
	News          string                `json:"news"`
	NewsSentiment *NewsSentimentSummary `json:"newsSentiment,omitempty"`
	Keywords      []string              `json:"keywords,omitempty"`
	// Briefing is the rule-based 시장 분석 shown ahead of the AI prose. Always
	// server-derived, so it is present even when the LLM is off or the symbol
	// was not in the day's batch.
	Briefing *MarketBriefing `json:"briefing,omitempty"`
	Source   string          `json:"source"` // "llm" | "rule"
	// GeneratedAt is when this view's numbers were computed.
	GeneratedAt string `json:"generatedAt"`
	// AnalyzedAt is when the AI 요약 text was actually written. It lags
	// GeneratedAt whenever a later rebuild refreshed the figures and carried
	// the previous run's prose forward, which is the normal case between two
	// scheduled batches. Empty means the two are the same.
	AnalyzedAt string `json:"analyzedAt,omitempty"`
}

// StockSummaryPilotStatus is one pilot symbol's AI 요약 cache status, for
// admin monitoring (GET /api/v1/admin/stock-summary). Read-only — never
// triggers generation.
type StockSummaryPilotStatus struct {
	Symbol      string `json:"symbol"`
	Cached      bool   `json:"cached"`
	Source      string `json:"source,omitempty"`
	GeneratedAt string `json:"generatedAt,omitempty"`
}
