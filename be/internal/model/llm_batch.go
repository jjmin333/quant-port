package model

// Scheduled AI 요약 batch — the daily run over 마켓 레이더's 1일 기준 top 100
// (see StockSummaryService's batch runner). These types are what the admin
// page reads: what was selected, what was dropped and why, and what one run
// actually spent.

/*
어느 층이 이 종목을 데려왔나.

목록은 **한 줄로 합쳐지지만 출처는 남긴다** — 관리자 화면이 「실시간 120 · AI 랭킹
180 · 보유관심 30」처럼 읽을 수 있어야, 어느 층이 예산을 다 먹는지 눈으로 본다.
순서 자체가 우선순위다(selectBatchTargets 주석의 표).
*/
const (
	LLMBatchSourcePinned   = "pinned"   // 관리자 항상 포함 목록
	LLMBatchSourceRadar    = "radar"    // 1순위 · 실시간 차트(마켓 레이더) 순위
	LLMBatchSourceScore    = "score"    // 2순위 · AI 추천 랭킹 회전
	LLMBatchSourceInterest = "interest" // 3순위 · 사용자 보유/관심 중 요약 없음·오래됨
)

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
	// Source is the LLMBatchSource* layer this symbol came from.
	Source string `json:"source,omitempty"`
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
	Excluded map[string]int   `json:"excluded"`
	Targets  []LLMBatchTarget `json:"targets"`
	RankedAt string           `json:"rankedAt,omitempty"`
	// BySource counts the selected targets per LLMBatchSource* layer, before
	// the budget cut.
	BySource map[string]int `json:"bySource,omitempty"`
}

// CountBySource tallies a target list by the layer that produced it.
func CountBySource(targets []LLMBatchTarget) map[string]int {
	out := map[string]int{}
	for _, target := range targets {
		source := target.Source
		if source == "" {
			source = LLMBatchSourceRadar
		}
		out[source]++
	}
	return out
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
	// BySource is the planned list's composition per LLMBatchSource* layer —
	// "레이더 106 · AI 랭킹 180 · 보유관심 30" as the admin page reads it.
	BySource map[string]int `json:"bySource,omitempty"`
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
	// CallsRemaining/TokensRemaining are this market's API-key bucket, not
	// a shared pool. When KR and US share a key the two cards show the same
	// numbers.
	CallsRemaining  int64 `json:"callsRemaining"`
	TokensRemaining int64 `json:"tokensRemaining"`
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
