package model

// LLMRequestLogEntry records one GenerateStockSummary call for admin
// auditing — what was actually sent to the LLM, and whether it succeeded.
// Stored newest-first, capped, in Redis (see StockSummaryService).
type LLMRequestLogEntry struct {
	Symbol    string `json:"symbol"`
	Name      string `json:"name"`
	Timestamp string `json:"timestamp"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	FactsJSON string `json:"factsJson"`
	// Manual marks calls triggered by an admin from /admin/llm rather than by
	// the daily 00:00 KST scheduler or an on-demand page view.
	Manual bool `json:"manual,omitempty"`
}
