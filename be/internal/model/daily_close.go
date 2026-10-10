package model

// DailyCloseRow is one dated row of the 일별 종가 list: the confirmed close for
// that trading day, its move versus the previous close, and — when an analysis
// exists for that date — the score and AI summary.
//
// Score is a pointer and AISummary a plain string on purpose: a missing score
// must stay distinguishable from a score of 0 (the UI prints "-"), while a
// missing summary is simply empty text the UI can render as-is.
type DailyCloseRow struct {
	TradeDate      string   `json:"tradeDate"` // YYYY-MM-DD
	Weekday        string   `json:"weekday"`   // 월 · 화 · …
	Close          float64  `json:"close"`
	PrevClose      *float64 `json:"prevClose,omitempty"`
	ChangeAmount   *float64 `json:"changeAmount,omitempty"`
	ChangeRate     *float64 `json:"changeRate,omitempty"`
	Volume         int64    `json:"volume,omitempty"`
	Score          *int     `json:"score"`
	AISummary      string   `json:"aiSummary"`
	/*
		그날의 「핵심 요약」 한 문단 (2026-09-15 사용자 요청 1).

		`AISummary`는 LLM이 쓴 네 갈래 본문을 **이어붙인** 글이고, 이쪽은 같은 호출이
		그 넷을 전부 근거로 **다시 쓴** 요약이다. 종목 모달의 「간단」이 이 값을 쓴다 —
		예전에는 본문의 앞 두 문장을 잘라 썼는데, 자르는 것과 요약하는 것은 다른
		일이라 앞 두 문장이 하필 한 갈래만 말하면 「간단」도 한 갈래만 말했다.

		오늘 배치부터 쌓인다. 비어 있으면(과거 행·규칙 생성) 화면이 예전 방식으로
		되돌아가므로 이 칸이 없다고 요약이 사라지지는 않는다.
	*/
	AIOverview     string `json:"aiOverview,omitempty"`
	AnalysisSource string `json:"analysisSource,omitempty"` // llm | rule | ""
	/*
		지수 전용 — 그날의 LLM 해설과 국면 라벨.

		종목은 `AISummary` 하나에 LLM 요약이 들어가지만, 지수는 **규칙 문장과 해설이
		따로**다(규칙 문장은 언제나 있고 해설은 배치가 돌았을 때만 있다). 둘을 한 칸에
		합치면 어느 쪽을 보고 있는지 화면이 구분할 수 없으므로 칸을 나눠 둔다.
		종목 응답에서는 비어 온다.
	*/
	LLMNote   string `json:"llmNote,omitempty"`
	LLMStance string `json:"llmStance,omitempty"`
}

// DailyCloseSeries is the response for a stock or index daily-close list,
// newest row first.
type DailyCloseSeries struct {
	Symbol   string          `json:"symbol,omitempty"`
	IndexID  string          `json:"indexId,omitempty"`
	Name     string          `json:"name,omitempty"`
	Currency string          `json:"currency"`
	Rows     []DailyCloseRow `json:"rows"`
}

// DailyCloseAnalysis is the per-date score/summary written by the analysis
// pipeline. Score nil leaves the stored score untouched-as-empty.
type DailyCloseAnalysis struct {
	Symbol     string
	TradeDate  string
	Score      *int
	AISummary  string
	AIOverview string
	Source     string // llm | rule
}

var weekdayKo = [...]string{"일", "월", "화", "수", "목", "금", "토"}

// WeekdayKo maps time.Weekday's integer form to its single-letter Korean label.
func WeekdayKo(weekday int) string {
	if weekday < 0 || weekday >= len(weekdayKo) {
		return ""
	}
	return weekdayKo[weekday]
}
