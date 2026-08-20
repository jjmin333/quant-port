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

// BatchJobStatus is a job plus its most recent run and current coverage.
type BatchJobStatus struct {
	Spec     BatchJobSpec   `json:"spec"`
	LastRun  *BatchJobRun   `json:"lastRun,omitempty"`
	Running  bool           `json:"running"`
	Coverage map[string]any `json:"coverage,omitempty"`
}
