package model

import "time"

// 실적 발표일의 세 가지 상태. 화면에서 절대 섞어 그리지 않는다 — 우리가 추론한
// 날짜를 회사가 공시한 날짜처럼 보이게 만드는 것이 이 기능의 유일한 거짓말이다.
const (
	// EarningsDateConfirmed — 이미 발표됐다(국내 FnGuide 잠정실적, 미국 실적 확정).
	EarningsDateConfirmed = "confirmed"
	// EarningsDateScheduled — 소스가 준 예정일(미국 Finnhub만).
	// 화면에는 **오늘·어제(KST)** 만 나간다. 그보다 오래된 예정은 Finnhub가
	// 실적치를 안 준 잔류라 DropStaleOverseasScheduled가 조회에서 뺀다.
	EarningsDateScheduled = "scheduled"
	// EarningsDateEstimated — 작년 동기 발표일로 우리가 역산한 추정일(국내만).
	// DB에 저장되지 않는다. 실제 발표가 들어오면 자동으로 사라진다.
	EarningsDateEstimated = "estimated"
)

// 발표 시각대 (미국만 — 국내 소스는 시각을 주지 않는다).
const (
	EarningsSessionBeforeOpen  = "bmo"
	EarningsSessionAfterClose  = "amc"
	EarningsSessionDuringHours = "dmh"
)

// 서프라이즈 헤드라인 지표. 시장마다 관행이 다르다 — 국내는 영업이익, 미국은 EPS로
// "어닝 서프라이즈"를 말한다.
const (
	EarningsMetricOpIncome = "opIncome"
	EarningsMetricEPS      = "eps"
	// 매출은 헤드라인 지표는 아니지만 별도 열로 보여준다 — 이익보다 회계 재량이
	// 적어, 이익과 매출이 반대로 움직인 발표를 드러낸다.
	EarningsMetricRevenue = "revenue"
)

// EarningsMetric is one financial line: what the street expected, what came out,
// and (국내만) the source's QoQ change.
type EarningsMetric struct {
	Estimate *float64 `json:"estimate,omitempty"`
	Actual   *float64 `json:"actual,omitempty"`
	// QoQ는 FnGuide가 주는 전분기 대비 증감률(%). YoY가 아님을 실측으로 확인했다.
	QoQ *float64 `json:"qoq,omitempty"`
}

// HasBoth reports whether a surprise can be computed at all.
func (m EarningsMetric) HasBoth() bool { return m.Estimate != nil && m.Actual != nil }

// EarningsSurprise is the estimate-vs-actual gap on the market's headline metric.
//
// Pct is nil whenever Estimate <= 0. 적자 예상에서 흑자가 나면 (actual-est)/|est|가
// −800% 같은 무의미한 수가 되므로, 그때는 퍼센트 대신 Label로 말한다.
type EarningsSurprise struct {
	Metric   string   `json:"metric"`
	Estimate float64  `json:"estimate"`
	Actual   float64  `json:"actual"`
	Pct      *float64 `json:"pct,omitempty"`
	// "흑자전환" | "적자전환" | "적자지속" | "" (Pct가 있으면 빈 문자열)
	Label string `json:"label,omitempty"`
}

// EarningsEvent is one row of the market-wide 실적 캘린더.
//
// Estimated rows (DateStatus == EarningsDateEstimated) are computed at read time
// from last year's confirmed announcement and carry EstimateBasis as evidence.
type EarningsEvent struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name,omitempty"`
	Market string `json:"market"`

	// "2026-08-03". 정정 공시가 와도 원 발표일을 유지한다.
	Date       string `json:"date"`
	DateStatus string `json:"dateStatus"`
	Session    string `json:"session,omitempty"`
	// 정정이 있었던 경우 그 날짜. 수치는 정정본, 날짜는 원 발표일이라는 사실을
	// 화면이 밝힐 수 있어야 한다.
	RevisedAt string `json:"revisedAt,omitempty"`

	FiscalYear   int    `json:"fiscalYear"`
	FiscalPeriod string `json:"fiscalPeriod"` // "1Q".."4Q" | "FY"
	PeriodEnd    string `json:"periodEnd,omitempty"`
	Basis        string `json:"basis,omitempty"` // "연결" | "개별"

	Currency string `json:"currency"`
	Source   string `json:"source"`

	Revenue   EarningsMetric `json:"revenue"`
	OpIncome  EarningsMetric `json:"opIncome"`
	NetIncome EarningsMetric `json:"netIncome"`
	EPS       EarningsMetric `json:"eps"`

	Surprise *EarningsSurprise `json:"surprise,omitempty"`
	// RevenueSurprise — 매출은 시장의 헤드라인 지표는 아니지만 **이익보다 조작이
	// 어렵다**. 이익이 기대를 넘었는데 매출이 밑돌면 비용 절감이지 성장이 아니라는
	// 뜻이고, 두 열을 나란히 두어야 그게 보인다. 국내 93%·해외 95%에서 계산된다.
	RevenueSurprise *EarningsSurprise `json:"revenueSurprise,omitempty"`
	Reaction        *EarningsReaction `json:"reaction,omitempty"`

	// MarketCap — 발행주식수 × 최근 종가(기준통화). 표에서 「무엇이 큰 발표인가」로
	// 줄을 세우는 유일한 기준이다. 실시간 시세가 아니라 **규모 구간**으로 쓴다 —
	// 화면의 현재가 열과 몇 %는 다를 수 있고, 정렬 목적에는 그 차이가 무의미하다.
	MarketCap *float64 `json:"marketCap,omitempty"`

	// Alternates — 같은 (종목·회계기간)의 **다른 기준** 발표(주로 개별).
	//
	// FnGuide는 연결과 개별을 별 행으로 주고, 컨센서스가 어긋나 붙어 한 회사가
	// 같은 날 +54.6%(연결)와 −60.8%(개별)를 동시에 말한다(실측 338건). 국내 실적
	// 보도의 기준인 **연결을 대표로 세우고 나머지를 여기 접어 둔다** — 지우지
	// 않는 이유는 지주회사처럼 개별이 의미 있는 경우가 있기 때문이다.
	Alternates []EarningsEvent `json:"alternates,omitempty"`

	// 추정 행에만 채운다: "작년 2Q는 8/3 발표". 근거 없는 추정은 그냥 오답이다.
	EstimateBasis string `json:"estimateBasis,omitempty"`
}

// EarningsReaction is how the stock moved on the first session that could price
// the announcement in.
//
// # 왜 이 값이 필요한가
//
// 서프라이즈는 「기대를 얼마나 넘겼나」까지만 답한다. 투자자가 곧바로 묻는 다음
// 질문은 「그래서 주가는?」이고, 둘이 어긋나는 경우가 이 화면에서 가장 값어치
// 있는 정보다 — 실측: 달바글로벌은 컨센서스를 +14.5% 넘겼는데 다음 거래일에
// **−13.4%**였다. 숫자만 보면 좋은 실적인데 시장은 다르게 읽었다는 뜻이고,
// 반응 열이 없으면 그 사실을 알 길이 없다.
//
// # 어느 세션을 세는가 (하루 잘못 읽지 않으려면)
//
//	장전(bmo)  발표가 개장 전 → **그날** 종가가 첫 반응. 기준은 전 거래일 종가.
//	장후(amc)  발표가 마감 후 → **다음 거래일** 종가가 첫 반응. 기준은 그날 종가.
//	국내·미공개 잠정실적은 통상 마감 후 공시 → 장후와 같게 본다.
type EarningsReaction struct {
	// 기준 종가와 그 날짜.
	BaseClose float64 `json:"baseClose"`
	BaseDate  string  `json:"baseDate"`
	// 반응 종가와 그 날짜.
	Close float64 `json:"close"`
	Date  string  `json:"date"`
	Pct   float64 `json:"pct"`
	// 어느 세션 기준으로 셌는지 — 화면이 「다음 거래일」/「발표 당일」을 정확히
	// 말할 수 있어야 한다.
	Basis string `json:"basis"` // "sameDay" | "nextDay"
}

// EarningsHitHistory summarizes how often a company beats its consensus.
//
// 한 번의 서프라이즈는 우연일 수 있다. 「5분기 중 4번 상회」는 습관이다 —
// 이 회사의 가이던스가 보수적인지 공격적인지를 말해 준다.
type EarningsHitHistory struct {
	Quarters int `json:"quarters"`
	Beats    int `json:"beats"`
	Misses   int `json:"misses"`
	// 평균과 중위를 함께 준다 — 한 분기의 +6232%가 평균을 끌어올려 실제
	// 분위와 어긋나는 일이 흔하다.
	AvgPct    *float64 `json:"avgPct,omitempty"`
	MedianPct *float64 `json:"medianPct,omitempty"`
}

// EarningsStats is the 상단 스코어보드 — 이 구간 시장 전체의 온도.
type EarningsStats struct {
	Total     int `json:"total"`
	Confirmed int `json:"confirmed"`
	Scheduled int `json:"scheduled"`
	Estimated int `json:"estimated"`
	// 서프라이즈를 계산할 수 있었던 건수(= Beats + Misses).
	Compared  int                `json:"compared"`
	Beats     int                `json:"beats"`
	Misses    int                `json:"misses"`
	BeatRate  *float64           `json:"beatRate,omitempty"`
	AvgPct    *float64           `json:"avgPct,omitempty"`
	MedianPct *float64           `json:"medianPct,omitempty"`
	Best      *EarningsStatsPick `json:"best,omitempty"`
	Worst     *EarningsStatsPick `json:"worst,omitempty"`
}

// EarningsStatsPick is the headline name behind a stat.
type EarningsStatsPick struct {
	Symbol string  `json:"symbol"`
	Name   string  `json:"name"`
	Pct    float64 `json:"pct"`
}

// EarningsEventUpsert is the write-side shape for one ingested announcement.
//
// EarningsEvent과 따로 두는 이유: 적재에는 필요하지만 응답에 실릴 이유가 없는
// 값(SourceDate·Revised)이 있고, 반대로 응답에만 있는 값(Name·Surprise·
// EstimateBasis)은 저장하지 않는다. 한 구조체로 겸하면 API가 내부 상태를 흘린다.
type EarningsEventUpsert struct {
	Symbol       string
	Market       string
	EventDate    time.Time
	DateStatus   string
	Session      string
	FiscalYear   int
	FiscalPeriod string
	PeriodEnd    *time.Time
	Basis        string

	RevenueEst   *float64
	RevenueAct   *float64
	OpIncomeEst  *float64
	OpIncomeAct  *float64
	NetIncomeEst *float64
	NetIncomeAct *float64
	EPSEst       *float64
	EPSAct       *float64

	RevenueQoQ   *float64
	OpIncomeQoQ  *float64
	NetIncomeQoQ *float64

	Currency string
	Source   string
	// 이 수치를 실어 온 회차의 날짜. 같은 (종목·기간)에 여러 회차가 오면
	// 더 큰 SourceDate만 수치를 덮는다.
	SourceDate time.Time
	// 정정 공시로 온 회차인가 (FnGuide `[정정]` 접두).
	Revised bool
}

// EarningsDaySummary is one cell of the month heatmap.
//
// 월 전체를 상세 행으로 내리면 2,000행이 된다. 히트맵은 건수만 알면 되므로
// 이 경량 표현을 따로 둔다 (AGENT_STATE 응답 속도 원칙 #4).
type EarningsDaySummary struct {
	Date      string `json:"date"`
	Confirmed int    `json:"confirmed"`
	Scheduled int    `json:"scheduled"`
	Estimated int    `json:"estimated"`
	// Compared/Beats — 그날 컨센서스와 견줄 수 있었던 발표와 그중 상회한 수.
	//
	// 달력 칸 색이 이 비율을 말한다. 건수만으로 칠하면 「많은 날」과 「좋은 날」이
	// 구별되지 않는데, 달력을 훑는 사람이 실제로 찾는 것은 후자다. 비교 불가한
	// 발표(컨센서스 없음·기준선 0 이하)는 세지 않으므로 Compared ≤ Confirmed다.
	Compared int `json:"compared"`
	Beats    int `json:"beats"`
}

// EarningsCalendarEvent is one earnings-related date shown in the 종목 실적 탭.
//
// Kind은 예전 이름을 유지한다 — 홈 「실적·공시」 카드가 "upcoming"/"filed"로
// 분기하고 있어, 이름을 바꾸면 그 카드가 조용히 빈다. DateStatus가 더 정확한
// 세 값을 말하고, Kind는 그 사영이다.
type EarningsCalendarEvent struct {
	Date   string `json:"date"`
	Label  string `json:"label"`
	Source string `json:"source"` // "finnhub" | "dart" | "fnguide"
	Kind   string `json:"kind"`   // "upcoming" | "filed"

	DateStatus string            `json:"dateStatus,omitempty"`
	Session    string            `json:"session,omitempty"`
	Surprise   *EarningsSurprise `json:"surprise,omitempty"`
	// 추정 행 근거. Kind == "upcoming" && DateStatus == "estimated"일 때만.
	EstimateBasis string `json:"estimateBasis,omitempty"`
}

// EarningsQuarter is one period from a 정기보고서 (DART reprt_code / EDGAR 10-Q·10-K).
//
// ⚠️ **이 표의 수치는 발표 잠정치와 같은 기준이 아니다.** DART 정기보고서는
// 1분기를 뺀 나머지가 **누적**이다 — 반기보고서는 상반기 합계, 3분기보고서는
// 9개월 합계다. 반면 FnGuide 잠정실적발표(`EarningsEvent`)는 **분기 단독**이다.
//
// 그래서 둘을 한 행으로 합치지 않는다. 합치면 「2Q 컨센서스(분기)」 옆에
// 「2Q 실적(반기 누적)」이 놓여 서프라이즈가 두 배 넘게 부풀고, 화면은 그것이
// 왜 틀렸는지 말할 방법이 없다. 발표 잠정치는 EarningsTabView.Announcements,
// 정기보고서는 이 표 — 두 섹션으로 나란히 둔다.
type EarningsQuarter struct {
	FiscalYear      int        `json:"fiscalYear"`
	Period          string     `json:"period"` // "1Q","반기","3Q","연간" (KR) or "Q1","Q2","Q3","FY" (US)
	ReportCode      string     `json:"reportCode,omitempty"`
	PeriodEnd       *time.Time `json:"periodEnd,omitempty"`
	Revenue         *int64     `json:"revenue,omitempty"`
	OperatingIncome *int64     `json:"operatingIncome,omitempty"`
	NetIncome       *int64     `json:"netIncome,omitempty"`
	EPSBasic        *float64   `json:"epsBasic,omitempty"`
}

// EarningsConsensus is the most recent period's estimate-vs-actual comparison.
//
// 예전에는 이 구조체가 "무료 소스에는 컨센서스가 없다"는 안내문만 실었다.
// 실제로는 있다 — Finnhub 응답에 epsEstimate/epsActual이 실려 왔는데 파서가
// 버리고 있었고, 국내는 FnGuide 잠정실적발표가 추정치와 발표치를 함께 준다.
type EarningsConsensus struct {
	Available bool              `json:"available"`
	Period    string            `json:"period,omitempty"` // "2026 2Q"
	Surprise  *EarningsSurprise `json:"surprise,omitempty"`
	// 없을 때만 이유를 적는다.
	Message string `json:"message,omitempty"`
}

// EarningsTabView is the full response for GET /stocks/{symbol}/earnings.
type EarningsTabView struct {
	Symbol    string                  `json:"symbol"`
	AssetType QuoteAssetType          `json:"assetType"`
	Calendar  []EarningsCalendarEvent `json:"calendar"`
	// Announcements — 발표 잠정치 + 컨센서스 대비 서프라이즈 (분기 단독 기준).
	// Quarterly와 기준이 달라 한 표로 합치지 않는다 (EarningsQuarter 주석 참고).
	Announcements []EarningsEvent   `json:"announcements"`
	Quarterly     []EarningsQuarter `json:"quarterly"`
	Consensus     EarningsConsensus `json:"consensus"`
	// HitHistory — 이 회사가 컨센서스를 습관적으로 넘기는지.
	HitHistory *EarningsHitHistory `json:"hitHistory,omitempty"`
}
