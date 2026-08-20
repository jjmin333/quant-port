package model

import "time"

// Target modes for the scheduled AI 요약 batch.
const (
	// LLMTargetModeRadar selects each run's symbols from 마켓 레이더's 1일
	// 기준 top 100 (거래대금 + 거래량), minus ETF/ETN/ETP.
	LLMTargetModeRadar = "radar"
	// LLMTargetModeManual runs only PilotSymbols — the rollback path, and
	// what the feature did before the batch existed.
	LLMTargetModeManual = "manual"
)

// LLMSettings holds the admin-editable knobs for AI 요약 generation — see
// LLM 모델 관리 (/admin/llm).
//
// The schedule itself is deliberately NOT tunable here: each market runs once
// per trading day, finishing before that market opens (see
// StockSummaryService's batch runner). Off-schedule regeneration is a manual
// admin action instead of a tunable interval. The legacy
// llm_settings.refresh_interval_hours column is left in place (unread, it has
// a NOT NULL default) so no migration is required.
type LLMSettings struct {
	// PilotSymbols are always included in a run, ahead of the ranking, and
	// survive a budget cut that trims the tail. In manual mode they are the
	// entire target list.
	PilotSymbols []string `json:"pilotSymbols"`
	TargetMode   string   `json:"targetMode"`
	// DailyCap bounds one market run regardless of remaining quota — the
	// brake an admin can reach for without touching provider limits.
	DailyCap int `json:"dailyCap"`
	// QuotaReserve and TokenReserve are held back from the day's provider
	// allowance so the batch cannot starve news briefings and manual admin
	// refreshes for the rest of the day.
	QuotaReserve int       `json:"quotaReserve"`
	TokenReserve int64     `json:"tokenReserve"`
	UpdatedAt    time.Time `json:"updatedAt,omitempty"`
}

// TargetModeOrDefault normalizes an unset or unrecognized mode to radar.
func (s LLMSettings) TargetModeOrDefault() string {
	if s.TargetMode == LLMTargetModeManual {
		return LLMTargetModeManual
	}
	return LLMTargetModeRadar
}

func DefaultLLMSettings() LLMSettings {
	return LLMSettings{
		PilotSymbols: []string{"005930", "000660"},
		TargetMode:   LLMTargetModeRadar,
		DailyCap:     250,
		QuotaReserve: 80,
		TokenReserve: 120_000,
	}
}
