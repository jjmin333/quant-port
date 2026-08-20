package model

import "time"

// NewsMarketBriefing is one generated read of a market's news at a moment in
// time — the 「AI 시장 브리핑」 card on the 국내주식/해외주식 뉴스 화면.
//
// Numbers and narrative are kept apart on purpose. Everything under Facts is
// counted from data we already hold, so it is right whether or not the LLM ran;
// Summary/Mood/Highlights/WatchPoints are the model's prose. When the LLM is
// disabled or fails, Source falls back to "rules" and the prose comes from the
// same rule-based assembly the front-end used to do inline — the card never
// goes blank.
type NewsMarketBriefing struct {
	Market      string    `json:"market"`
	GeneratedAt time.Time `json:"generatedAt"`
	Source      string    `json:"source"` // "llm" | "rules"

	// Summary is the full narrative. It is deliberately not truncated here —
	// the card clamps it visually and the 전체 보기 modal shows all of it, so
	// cutting it server-side would throw away what the modal exists to show.
	Summary string `json:"summary"`
	// Mood is 강세|중립|약세. It is the model's read, which can disagree with
	// the index move, so the UI always renders the real change rate beside it.
	Mood        string   `json:"mood,omitempty"`
	MoodReason  string   `json:"moodReason,omitempty"`
	Highlights  []string `json:"highlights,omitempty"`
	WatchPoints []string `json:"watchPoints,omitempty"`

	Facts NewsBriefingFacts `json:"facts"`
}

// NewsBriefingFacts is the counted, non-LLM half of a briefing. It is both the
// prompt input and what the card's four number tiles display, so the sentence
// and the tiles can never be reasoning from different figures.
type NewsBriefingFacts struct {
	Session       string                 `json:"session"` // 장중 | 장 마감 | 개장 전 | 휴장
	ArticleCount  int                    `json:"articleCount"`
	BreakingCount int                    `json:"breakingCount"`
	Indices       []NewsBriefingIndex    `json:"indices,omitempty"`
	TopMentioned  []NewsBriefingMention  `json:"topMentioned,omitempty"`
	Headlines     []NewsBriefingHeadline `json:"-"`
	// LatestPublishedAt is the newest article the briefing saw. The scheduler
	// compares it against the previous run to decide whether anything new has
	// happened at all — a quiet night must not spend an LLM call restating the
	// same articles.
	LatestPublishedAt time.Time `json:"-"`
}

type NewsBriefingIndex struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Value      float64 `json:"value"`
	ChangeRate float64 `json:"changeRate"`
}

type NewsBriefingMention struct {
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name"`
	Mentions   int     `json:"mentions"`
	ChangeRate float64 `json:"changeRate"`
	HasQuote   bool    `json:"hasQuote"`
}

type NewsBriefingHeadline struct {
	Title     string `json:"t"`
	At        string `json:"at"`
	Publisher string `json:"pub,omitempty"`
	Breaking  bool   `json:"breaking,omitempty"`
}

// NewsBriefingHistoryEntry is one past briefing compressed for the next
// prompt. The full Summary is not reused: ten full narratives would cost more
// input tokens than everything else in the prompt combined, and what the model
// needs from history is what was said and when, not how it was worded.
type NewsBriefingHistoryEntry struct {
	GeneratedAt time.Time `json:"generatedAt"`
	Mood        string    `json:"mood,omitempty"`
	Summary     string    `json:"summary"`
	WatchPoints []string  `json:"watchPoints,omitempty"`
}
