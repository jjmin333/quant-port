package model

import (
	"fmt"
	"time"
)

// EarningsSurpriseRow is one past quarter's consensus-vs-actual pair, taken from
// a source that knows the numbers but **not the announcement date**.
//
// EarningsEvent과 나란히 두지 않고 별도 타입인 이유는 Date 필드가 없다는 것 하나다.
// 같은 구조체에 발표일을 빈 문자열로 담으면, 언젠가 누군가 그것을 캘린더에 그린다.
type EarningsSurpriseRow struct {
	Symbol string
	// Name — stocks 조인으로 채운다(표에는 없다). EarningsEvent.Name과 같은 취급.
	Name   string
	Market string

	FiscalYear   int
	FiscalPeriod string // "1Q".."4Q" | "FY" — earnings_events와 같은 어휘
	// PeriodEnd — 회계기간 종료일. 발표일이 아니라 **정렬 축**이다.
	PeriodEnd time.Time

	EPSEst *float64
	EPSAct *float64

	Currency string
	Source   string
}

// PeriodKey identifies "this company's this reporting period" — the key both
// tables are merged on.
func (r EarningsSurpriseRow) PeriodKey() string {
	return EarningsPeriodMergeKey(r.Symbol, r.FiscalYear, r.FiscalPeriod)
}

// EarningsPeriodMergeKey is shared by earnings_events rows and surprise-history
// rows so the union in 「꾸준히 기대를 넘는 기업」 counts one quarter once.
//
// basis를 넣지 않는다 — 이 병합은 해외 전용이고 미국 소스에는 연결/개별이 없다.
// 국내를 넣게 되면 그때 basis를 키에 더해야 한다(연결과 개별은 다른 발표다).
func EarningsPeriodMergeKey(symbol string, year int, period string) string {
	return fmt.Sprintf("%s|%d|%s", symbol, year, period)
}

// FiscalPeriodFromQuarter normalizes Finnhub's numeric quarter to the string the
// `earnings_events` table already stores.
//
// **이 변환이 병합의 전부다.** `/calendar/earnings`는 quarter=3을 주고 적재기가
// "3Q"로 적는데(earnings_ingest.go overseasUpsert), `/stock/earnings`도 같은 3을
// 준다. 여기서 다른 문자열을 만들면 AAPL 3Q FY2026이 두 분기로 세어져 적중률이
// 조용히 절반이 된다.
func FiscalPeriodFromQuarter(q int) string {
	if q >= 1 && q <= 4 {
		return fmt.Sprintf("%dQ", q)
	}
	return "FY"
}

// AsEvent renders the row as an EarningsEvent so the hit-history computation
// stays in one place (ComputeHitHistory / ComputeSurprise / materialSurprise).
//
// Date는 일부러 비운다. 이 값이 화면의 캘린더로 흘러가면 안 되고, 적중 이력
// 계산은 날짜를 보지 않는다 — 순서는 호출자가 PeriodEnd로 이미 세운다.
func (r EarningsSurpriseRow) AsEvent() EarningsEvent {
	ev := EarningsEvent{
		Symbol:       r.Symbol,
		Name:         r.Name,
		Market:       r.Market,
		DateStatus:   EarningsDateConfirmed,
		FiscalYear:   r.FiscalYear,
		FiscalPeriod: r.FiscalPeriod,
		Currency:     r.Currency,
		Source:       r.Source,
		EPS:          EarningsMetric{Estimate: r.EPSEst, Actual: r.EPSAct},
	}
	if !r.PeriodEnd.IsZero() {
		ev.PeriodEnd = r.PeriodEnd.Format("2006-01-02")
	}
	ev.Surprise = ev.HeadlineSurprise()
	return ev
}
