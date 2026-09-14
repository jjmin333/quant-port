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
	Symbol    string         `json:"symbol"`
	AssetType QuoteAssetType `json:"assetType"`
	/*
		Overview는 개요 탭의 「AI 핵심 요약」 한 문단이다 — 아래 네 섹션 **전체를
		근거로** 쓴다.

		예전에는 이 필드가 없었고, 화면이 `valuation`을 그대로 첫 줄로 썼다. 그래서
		AI 분석 탭에는 기술적 분석·내부자 거래·뉴스 해석이 나란히 있는데 개요는
		밸류에이션 이야기만 했다 — 내부자가 대량 매도 중이어도 개요는 「PER이 동종
		대비 낮다」로 끝났다(2026-09-03 사용자 지적 §8).

		LLM이 켜져 있으면 같은 호출이 이 문단까지 함께 쓴다(**호출 수는 그대로**).
		꺼져 있거나 예전 캐시라면 서버가 네 섹션의 첫 문장을 엮어 만든다
		(`composeOverview`) — 어느 쪽이든 네 갈래를 다 거친다.
	*/
	Overview  string `json:"overview,omitempty"`
	Valuation string `json:"valuation"`
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
