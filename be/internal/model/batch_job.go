package model

import "time"

// BatchJobID identifies one manageable batch. Constants rather than free
// strings so a route, a run row and the UI can never drift apart.
type BatchJobID string

const (
	// BatchDailyCloseDomestic re-projects KRX stock (+ KRW/USD) daily closes
	// from candle_bars. Runs on its own KRX-close trigger, in its own worker
	// process, independent of the overseas job below.
	BatchDailyCloseDomestic BatchJobID = "daily_close_domestic"
	// BatchDailyCloseOverseas re-projects US-listed stock/ETF (+ US indices,
	// gold, BTC) daily closes from candle_bars. Runs on its own US-close
	// trigger, independent of the domestic job.
	BatchDailyCloseOverseas BatchJobID = "daily_close_overseas"
	// BatchDailyCloseFetchStale fetches sessions missing from symbols that
	// already have some history.
	BatchDailyCloseFetchStale BatchJobID = "daily_close_fetch_stale"
	// BatchDailyCloseFetchMissing fetches symbols with no stored close at all.
	BatchDailyCloseFetchMissing BatchJobID = "daily_close_fetch_missing"
	// BatchDailyCloseFetchHistory pages Toss daily candles backward until
	// `since` for symbols whose stored history starts too late (earnings reaction gap).
	BatchDailyCloseFetchHistory BatchJobID = "daily_close_fetch_history"
	// BatchStockScores recomputes the 종합 점수 for every symbol and index.
	BatchStockScores BatchJobID = "stock_scores"
	// BatchCandleRollup folds aged intraday bars into coarser tiers
	// (1m→5m→10m→30m→60m→만료). 되돌릴 수 없다 — 접힌 1분봉은 삭제된다.
	BatchCandleRollup BatchJobID = "candle_rollup"
	// BatchDataRetention 만료 데이터 정리 (리프레시 토큰 등).
	BatchDataRetention BatchJobID = "data_retention"
	// BatchIndexCloses backfills index close history from Toss/FRED.
	BatchIndexCloses BatchJobID = "index_closes"
	// BatchLLMSummaryDomestic runs the scheduled AI 요약 batch for the KR
	// market immediately, spending LLM quota. Normally fires once per trading
	// day before the KRX open (see stock_summary_schedule.go).
	BatchLLMSummaryDomestic BatchJobID = "llm_summary_domestic"
	// BatchLLMSummaryOverseas is the US-market counterpart.
	BatchLLMSummaryOverseas BatchJobID = "llm_summary_overseas"
	// BatchNewsBriefingDomestic regenerates the 국내 뉴스 「AI 시장 브리핑」 now,
	// spending LLM quota. Normally fires on its own schedule (30분 장중 /
	// 1시간 그 외, see news_briefing_schedule.go) and skips slots with no new
	// articles; a manual run generates regardless.
	BatchNewsBriefingDomestic BatchJobID = "news_briefing_domestic"
	// BatchNewsBriefingOverseas is the 해외 뉴스 counterpart.
	BatchNewsBriefingOverseas BatchJobID = "news_briefing_overseas"
	/*
		BatchIndexNotes writes each tracked index's LLM 해설.

		점수 배치(`stock_scores`)의 00:25 슬롯에 얹혀 돌지만 **잡 이름은 따로 둔다.**
		LLM을 쓰는 유일한 작업인데 이름이 없으면 관리자 배치 화면에 안 보이고, 프롬프트를
		고친 뒤 다시 돌리려면 전 종목 재계산까지 끌고 와야 한다 — 종목 AI 요약이 국내·해외로
		나뉘어 각자 이름을 가진 것과 같은 이유다.
	*/
	BatchIndexNotes BatchJobID = "index_notes"
	// BatchResearchVerdict judges every open 리서치 콜 against stored closes.
	// It must run after the daily-close workers have landed the session —
	// judging earlier would resolve calls against the previous day.
	BatchResearchVerdict BatchJobID = "research_verdict"
	// BatchEarningsDomestic ingests 국내 잠정실적발표 from FnGuide.
	// 시즌에는 당월을 2시간마다 다시 받고, 하루의 마지막 슬롯(그리고 비시즌
	// 18:30)만 전월을 겹친다 — 정정이 다음 달에 올 수 있어서다.
	BatchEarningsDomestic BatchJobID = "earnings_domestic"
	// BatchEarningsOverseas ingests the US earnings calendar from Finnhub.
	BatchEarningsOverseas BatchJobID = "earnings_overseas"
	// BatchInsiderDomestic ingests 국내 임원ㆍ주요주주 소유보고 from DART.
	// 대상은 등급제로 고른다 — 관심·보유 종목이 먼저, 나머지는 마지막으로
	// 받은 지 오래된 것부터(insider_ingest.go).
	BatchInsiderDomestic BatchJobID = "insider_domestic"
	// BatchInsiderOverseas ingests 미국 Form 4 내부자 거래 from SEC EDGAR.
	BatchInsiderOverseas BatchJobID = "insider_overseas"
	// BatchFinancialsDomestic ingests 국내 분기 재무 from DART — 재무제표 상세
	// 모달과 실적 탭의 「분기 실적」이 같은 표를 읽는다.
	BatchFinancialsDomestic BatchJobID = "financials_domestic"
	// BatchFinancialsOverseas is the SEC EDGAR counterpart.
	BatchFinancialsOverseas BatchJobID = "financials_overseas"
	// BatchDisclosures sweeps one day of DART filings for the whole market.
	// 종목별이 아니라 날짜별이라 종목 수와 무관하게 하루 몇 호출이면 끝난다.
	BatchDisclosures BatchJobID = "disclosures_daily"
	// BatchWarnings refreshes 종목 지정 상태(투자주의·거래정지 등) for every
	// listed symbol. 등급제 로테이션이 아니라 **전 종목 매일**이다 — 하루 단위로
	// 붙고 풀리는 데이터라 주기를 데이터에 맞춘다.
	BatchWarnings BatchJobID = "warnings_sync"
	/*
		BatchTossUniverse — 「토스가 어떤 종목의 거래를 지원하는가」를 확정한다.

		하루 7호출(/stocks/all × 마켓 7)로 그 집합을 받고, 우리 카탈로그에서 그
		집합에 없는 NORMAL 종목을 화면·수집에서 숨긴다. 예전에는 그 사실을 종목마다
		404를 한 번씩 맞아 가며 배웠다 — 실측으로 경고 스윕 호출의 19%(3,388건)와
		해외 종가 구멍 메우기의 회차당 3,250건이 그 404였다.

		같은 응답의 한국어 종목명으로 뉴스 검색 별칭도 갱신한다(알파벳 A · 알파벳 C).
	*/
	BatchTossUniverse BatchJobID = "toss_universe"
	/*
		BatchWatchdog는 다른 잡들이 제때 돌았는지 판정한다.

		자기도 예약 잡이라 원장에 남고, 그래서 **자기 침묵도 판정 대상**이 된다 —
		워치독이 죽으면 마지막 보고서의 나이가 그것을 말한다. 별도 컨테이너로 떼지
		않은 이유가 여기 있다: 감시자를 하나 더 만들면 그 감시자를 감시할 것이 또
		필요해진다. 대신 워치독은 가장 가벼운 워커에 얹고, API가 그 보고서의 나이를
		읽어 워치독의 죽음을 본다.
	*/
	BatchWatchdog BatchJobID = "batch_watchdog"
)

// BatchJobCost flags whether a run consumes an external quota, so the console
// can warn before starting a rate-limited job.
type BatchJobCost string

const (
	// CostFree reads only data already in MySQL.
	CostFree BatchJobCost = "free"
	// CostRateLimited calls an upstream API and shares its budget with the
	// live app.
	CostRateLimited BatchJobCost = "rate_limited"
)

// BatchJobCategory groups jobs in the admin console.
type BatchJobCategory string

const (
	// CategoryIngest — 외부 소스에서 받아 표에 담는 잡. 조회 경로가 이 표들만
	// 읽으므로, 여기가 멈추면 화면이 낡는다.
	CategoryIngest BatchJobCategory = "ingest"
	// CategoryDerive — 이미 가진 데이터로 계산하는 잡. 외부 한도와 무관하다.
	CategoryDerive BatchJobCategory = "derive"
	// CategoryLLM — LLM 지출이 발생하는 잡.
	CategoryLLM BatchJobCategory = "llm"
)

// BatchJobSpec describes one job for the admin console.
type BatchJobSpec struct {
	ID      BatchJobID `json:"id"`
	Label   string     `json:"label"`
	Purpose string     `json:"purpose"`
	// Category groups the console into sections. 잡이 16개를 넘어가면서 한
	// 목록으로는 "지금 뭘 보고 있는지"가 사라졌다.
	Category BatchJobCategory `json:"category"`
	Cost     BatchJobCost     `json:"cost"`
	Schedule string           `json:"schedule"`
	// AcceptsSince / AcceptsLimit tell the UI which controls to render.
	AcceptsSince bool `json:"acceptsSince"`
	AcceptsLimit bool `json:"acceptsLimit"`
	// Note surfaces an operational caveat.
	Note string `json:"note,omitempty"`
}

// BatchJobRun is one recorded execution.
type BatchJobRun struct {
	ID          int64          `json:"id"`
	JobID       BatchJobID     `json:"jobId"`
	Status      string         `json:"status"` // queued | running | done | error
	TriggerKind string         `json:"triggerKind"`
	Params      map[string]any `json:"params,omitempty"`
	StartedAt   time.Time      `json:"startedAt"`
	FinishedAt  *time.Time     `json:"finishedAt,omitempty"`
	Message     string         `json:"message,omitempty"`
	Result      map[string]any `json:"result,omitempty"`
}

/*
잡의 건강 상태 — 「돌았다」의 기록이 아니라 「돌았어야 했다」의 판정이다.

원장(`batch_job_runs`)은 실행된 것만 안다. 실행되지 **않은** 것은 아무 흔적도 남기지
않으므로, 기대 시각을 따로 계산하지 않으면 「휴장이라 안 돎」과 「죽어서 안 돎」이
화면에서 똑같이 보인다 — 실제로 2026-09-08에 사람이 그 둘을 헷갈렸다.
*/
const (
	// BatchHealthOK — 마지막 성공이 기대 시각 이후다.
	BatchHealthOK = "ok"
	// BatchHealthPending — 기대 시각은 지났지만 아직 유예 시간 안이다.
	BatchHealthPending = "pending"
	// BatchHealthLate — 유예를 넘겼는데 성공 기록이 없다.
	BatchHealthLate = "late"
	// BatchHealthFailed — 기대 시각 이후에 돌긴 했는데 실패로 끝났다.
	BatchHealthFailed = "failed"
	// BatchHealthStuck — running 상태로 최대 실행시간을 넘겼다.
	BatchHealthStuck = "stuck"
	// BatchHealthUnmanaged — 예약이 없는 잡(수동 전용·부모 잡에 포함). 판정하지 않는다.
	BatchHealthUnmanaged = "unmanaged"
)

// BatchHealthNeedsAttention reports whether a health value should reach a human.
func BatchHealthNeedsAttention(health string) bool {
	switch health {
	case BatchHealthLate, BatchHealthFailed, BatchHealthStuck:
		return true
	}
	return false
}

// BatchJobHealth is one job's judged state at a moment.
type BatchJobHealth struct {
	JobID  BatchJobID `json:"jobId"`
	Label  string     `json:"label"`
	Health string     `json:"health"`
	// ExpectedAt is the most recent moment this job should already have run.
	// Empty for unmanaged jobs.
	ExpectedAt *time.Time `json:"expectedAt,omitempty"`
	// LastSuccessAt / LastRunAt come straight from the ledger.
	LastSuccessAt *time.Time `json:"lastSuccessAt,omitempty"`
	LastRunAt     *time.Time `json:"lastRunAt,omitempty"`
	LastStatus    string     `json:"lastStatus,omitempty"`
	Message       string     `json:"message,omitempty"`
	// LateBy is how far past the grace period this job is (0 unless late).
	LateBySeconds int64 `json:"lateBySeconds,omitempty"`
}

/*
BatchHealthReport — 한 판정 회차 전체.

`ComputedAt`이 이 보고서의 **유일한 생명 신호**다. 워치독이 죽으면 판정이 조용히
멈추는데, 그 침묵은 보고서 안에는 못 적는다 — 읽는 쪽이 나이를 봐야 안다.
*/
type BatchHealthReport struct {
	ComputedAt time.Time        `json:"computedAt"`
	Jobs       []BatchJobHealth `json:"jobs"`
	// Unhealthy counts jobs in late/failed/stuck.
	Unhealthy int `json:"unhealthy"`
	// StaleAfterSeconds is how old ComputedAt may get before the report itself
	// should be treated as unreliable.
	StaleAfterSeconds int64 `json:"staleAfterSeconds"`
}

/*
BatchHealthView — 화면이 배너 한 줄을 그리는 데 필요한 전부.

`Report`는 **읽는 순간 계산한** 판정이고, `WatchdogAgeSeconds`는 저장된 마지막
판정의 나이다. 둘을 함께 주는 이유: 전자는 잡들의 상태를, 후자는 **감시 자체의
상태**를 말한다. 워치독이 죽어도 화면은 계속 옳은 판정을 보여주되, 「감시가 멈췄다」는
사실을 함께 띄울 수 있어야 한다.
*/
type BatchHealthView struct {
	Report BatchHealthReport `json:"report"`
	// WatchdogAgeSeconds is nil when the watchdog has never recorded a pass.
	WatchdogAgeSeconds *int64 `json:"watchdogAgeSeconds,omitempty"`
	// WatchdogStale is true when that age exceeds the report's staleAfter, or
	// when no pass was ever recorded.
	WatchdogStale bool `json:"watchdogStale"`
}

// BatchJobStatus is a job plus its most recent run and current coverage.
type BatchJobStatus struct {
	Spec     BatchJobSpec   `json:"spec"`
	LastRun  *BatchJobRun   `json:"lastRun,omitempty"`
	Running  bool           `json:"running"`
	Coverage map[string]any `json:"coverage,omitempty"`
	// Health is this job's judged state — see BatchHealth* above.
	Health string `json:"health,omitempty"`
	// ExpectedAt is the most recent moment the job should already have run.
	ExpectedAt *time.Time `json:"expectedAt,omitempty"`
}
