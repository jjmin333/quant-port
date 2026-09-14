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

/*
DailyCap·TokenReserve의 근거 (실측 2026-09-08, migrations/066).

무료 티어는 키마다 하루 500호출 · 1,000,000토큰이고, 안 쓴 몫은 이월되지 않는다.
한 호출이 실제로 청구한 토큰은 국내 2,258 · 해외 1,939(provider가 돌려준 값)이고,
뉴스 브리핑이 하루 국내 약 140K · 해외 약 80K를 따로 쓴다. 그래서 산술 천장은

	국내 (1,000,000 − 140,000) / 2,258 ≈ 381건
	해외 (1,000,000 −  80,000) / 1,939 ≈ 474건 (호출 기준으로는 475건)

이고, 여기에 20% 여유를 남긴 값이 350이다. 상한을 열어 두는 이유는 **실사용을
정하는 것이 상한이 아니라 후보 수**이기 때문이다 — 상한 250이던 시절에도 레이더
후보가 먼저 말라 실제로는 106건만 돌았다.

진짜 안전판은 이 숫자가 아니라 아래 두 예비분이다. TokenReserve를 200K로 올린 것은
배치가 끝난 뒤 자정까지 도는 브리핑과 관리자 수동 생성이 굶지 않게 하기 위해서다.
*/
func DefaultLLMSettings() LLMSettings {
	return LLMSettings{
		PilotSymbols: []string{"005930", "000660"},
		TargetMode:   LLMTargetModeRadar,
		DailyCap:     350,
		QuotaReserve: 80,
		TokenReserve: 200_000,
	}
}
