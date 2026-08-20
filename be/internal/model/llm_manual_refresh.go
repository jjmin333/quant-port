package model

// LLM manual refresh statuses, as reported to /admin/llm.
const (
	LLMRefreshPending = "pending"
	LLMRefreshRunning = "running"
	LLMRefreshDone    = "done"
	LLMRefreshError   = "error"
)

// LLMRefreshSymbolState is one symbol's progress inside a manual AI 요약
// regeneration run.
type LLMRefreshSymbolState struct {
	Symbol string `json:"symbol"`
	Status string `json:"status"` // pending | running | done | error
	// Source is the resulting summary's provenance ("llm" or "rule") — a
	// completed symbol can still read "rule" if the LLM is disabled or the
	// call failed and the rule-based fallback was used.
	Source      string `json:"source,omitempty"`
	GeneratedAt string `json:"generatedAt,omitempty"`
	Error       string `json:"error,omitempty"`
}

// LLMManualRefreshJob is the state of the most recent admin-triggered AI 요약
// regeneration. Only one runs at a time; the finished job stays readable so
// the admin page can show its result after the fact.
type LLMManualRefreshJob struct {
	ID         string                  `json:"id"`
	Running    bool                    `json:"running"`
	StartedAt  string                  `json:"startedAt"`
	FinishedAt string                  `json:"finishedAt,omitempty"`
	Symbols    []LLMRefreshSymbolState `json:"symbols"`
}
