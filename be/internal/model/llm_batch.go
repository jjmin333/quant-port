package model

// Scheduled AI 요약 batch — the daily run over 마켓 레이더's 1일 기준 top 100
// (see StockSummaryService's batch runner). These types are what the admin
// page reads: what was selected, what was dropped and why, and what one run
// actually spent.

// LLMBatchTarget is one symbol picked for a run.
type LLMBatchTarget struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
	// Rank is the best (lowest) position the symbol held across the ranking
	// slots it appeared in — the priority used when the budget can't cover
	// every target.
	Rank int `json:"rank"`
	// Pinned marks a symbol that came from the admin's always-include list
	// rather than from the ranking.
	Pinned bool `json:"pinned"`
}

// Reasons a ranked symbol is dropped before it ever costs a call.
const (
	LLMBatchExcludeDerivative = "derivative"   // ETF/ETN/ETP — not an individual company
	LLMBatchExcludeUnlisted   = "unlisted"     // no row in `stocks`
	LLMBatchExcludeSuspended  = "suspended"    // status is not NORMAL (거래정지·상폐)
	LLMBatchExcludeOverBudget = "over_budget"  // fit neither the call nor the token allowance
	LLMBatchExcludeDeadline   = "deadline"     // run hit its pre-open cutoff first
)

// LLMBatchSelection is the outcome of choosing targets, before any LLM call.
type LLMBatchSelection struct {
	Market string `json:"market"`
	// Considered counts distinct symbols seen across the ranking slots.
	Considered int `json:"considered"`
	// Excluded maps an LLMBatchExclude* reason to how many symbols it dropped.
	Excluded map[string]int    `json:"excluded"`
	Targets  []LLMBatchTarget  `json:"targets"`
	RankedAt string            `json:"rankedAt,omitempty"`
}

// Terminal states for one symbol inside a run.
const (
	LLMBatchSymbolDone    = "done"
	LLMBatchSymbolError   = "error"
	LLMBatchSymbolSkipped = "skipped"
)

// LLMBatchSymbolResult is what happened to one target.
type LLMBatchSymbolResult struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name,omitempty"`
	Status string `json:"status"`
	// Source is the summary the symbol ended up with — "llm" on success,
	// "rule" when the call failed and the rule-based text stood in.
	Source string `json:"source,omitempty"`
	Tokens int64  `json:"tokens,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Why a run stopped. Anything other than "completed" means targets were left
// on the table and moved to the carry-over queue.
const (
	LLMBatchStopCompleted = "completed"
	LLMBatchStopDeadline  = "deadline"
	LLMBatchStopQuota     = "quota"
	LLMBatchStopTokens    = "tokens"
	LLMBatchStopShutdown  = "shutdown"
)

// LLMBatchRun records one scheduled market run, kept for admin auditing.
type LLMBatchRun struct {
	Market string `json:"market"`
	// TradeDate is the session the run analysed, in that market's own
	// timezone — the KR run at 07:30 KST and the US run at 21:50 KST are
	// both preparing for the session that opens next.
	TradeDate  string `json:"tradeDate"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt,omitempty"`
	DeadlineAt string `json:"deadlineAt"`
	Running    bool   `json:"running"`

	Considered int            `json:"considered"`
	Excluded   map[string]int `json:"excluded,omitempty"`
	Planned    int            `json:"planned"`
	Succeeded  int            `json:"succeeded"`
	Failed     int            `json:"failed"`
	Skipped    int            `json:"skipped"`

	TokensUsed     int64 `json:"tokensUsed"`
	CallsRemaining int64 `json:"callsRemaining"`
	TokensRemaining int64 `json:"tokensRemaining"`

	StopReason string                 `json:"stopReason,omitempty"`
	Carryover  []string               `json:"carryover,omitempty"`
	Symbols    []LLMBatchSymbolResult `json:"symbols"`
}

// LLMBatchMarketStatus is one market's schedule and latest result.
type LLMBatchMarketStatus struct {
	Market string `json:"market"`
	// NextRunAt/NextDeadlineAt describe the upcoming window. Both are empty
	// when the market has no trading day inside the lookahead.
	NextRunAt      string        `json:"nextRunAt,omitempty"`
	NextDeadlineAt string        `json:"nextDeadlineAt,omitempty"`
	LastRun        *LLMBatchRun  `json:"lastRun,omitempty"`
	History        []LLMBatchRun `json:"history,omitempty"`
}

// LLMBatchStatus is the admin page's view of the scheduled batch: the current
// settings, what the provider budget still allows today, and each market's
// schedule and last result.
type LLMBatchStatus struct {
	TargetMode string `json:"targetMode"`
	DailyCap   int    `json:"dailyCap"`
	// CallsRemaining/TokensRemaining are today's unspent provider allowance
	// before the batch's own reserves are subtracted. -1 means unbounded.
	CallsRemaining   int64                  `json:"callsRemaining"`
	TokensRemaining  int64                  `json:"tokensRemaining"`
	AvgTokensPerCall int64                  `json:"avgTokensPerCall"`
	Markets          []LLMBatchMarketStatus `json:"markets"`
}
