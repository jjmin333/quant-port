package model

import (
	"testing"
	"time"
)

func TestCollapseDomesticBasis_prefersConsolidated(t *testing.T) {
	individual := EarningsEvent{
		Symbol: "035420", Market: string(AssetDomestic), Date: "2026-08-07",
		FiscalYear: 2026, FiscalPeriod: "2Q", Basis: "개별",
		OpIncome: EarningsMetric{Actual: f(439e9)},
	}
	consolidated := EarningsEvent{
		Symbol: "035420", Market: string(AssetDomestic), Date: "2026-08-07",
		FiscalYear: 2026, FiscalPeriod: "2Q", Basis: "연결",
		OpIncome: EarningsMetric{Actual: f(520e9)},
	}
	us := EarningsEvent{
		Symbol: "AAPL", Market: string(AssetUS), Date: "2026-08-07",
		FiscalYear: 2026, FiscalPeriod: "3Q",
	}

	got := CollapseDomesticBasis([]EarningsEvent{individual, consolidated, us})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (연결 + US)", len(got))
	}
	if got[0].Basis != "연결" || got[0].OpIncome.Actual == nil || *got[0].OpIncome.Actual != 520e9 {
		t.Fatalf("first = %+v, want NAVER 연결", got[0])
	}
	if got[1].Symbol != "AAPL" {
		t.Fatalf("second = %+v, want AAPL", got[1])
	}
}

func TestCollapseDomesticBasis_keepsIndividualWhenAlone(t *testing.T) {
	only := EarningsEvent{
		Symbol: "004020", Market: string(AssetDomestic), Date: "2026-08-03",
		FiscalYear: 2026, FiscalPeriod: "2Q", Basis: "개별",
	}
	got := CollapseDomesticBasis([]EarningsEvent{only})
	if len(got) != 1 || got[0].Basis != "개별" {
		t.Fatalf("got = %+v, want lone 개별 row", got)
	}
}

func TestCollapseOverseasFiscalYearDup_targetKeepsRicherYear(t *testing.T) {
	wrongYear := EarningsEvent{
		Symbol: "TGT", Market: string(AssetUS), Date: "2026-08-19",
		DateStatus: EarningsDateConfirmed, Session: "bmo",
		FiscalYear: 2026, FiscalPeriod: "2Q",
		EPS:     EarningsMetric{Actual: f(2.46)},
		Revenue: EarningsMetric{Actual: f(26.54e9)},
	}
	realYear := EarningsEvent{
		Symbol: "TGT", Market: string(AssetUS), Date: "2026-08-19",
		DateStatus: EarningsDateConfirmed, Session: "bmo",
		FiscalYear: 2027, FiscalPeriod: "2Q",
		EPS:     EarningsMetric{Estimate: f(2.3095), Actual: f(2.46)},
		Revenue: EarningsMetric{Estimate: f(26.32e9), Actual: f(26.54e9)},
	}

	got := CollapseOverseasFiscalYearDup([]EarningsEvent{wrongYear, realYear})
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (same print, two fiscal years)", len(got))
	}
	if got[0].FiscalYear != 2027 {
		t.Fatalf("fiscalYear = %d, want 2027 (row with estimates)", got[0].FiscalYear)
	}
	if got[0].EPS.Estimate == nil || *got[0].EPS.Estimate != 2.3095 {
		t.Fatalf("eps estimate missing on collapsed TGT: %+v", got[0].EPS)
	}
	if got[0].Surprise == nil {
		t.Fatal("surprise should be recomputed from merged estimates+actuals")
	}
	if len(got[0].Alternates) != 0 {
		t.Fatalf("alternates = %d, want 0 (mis-year copy is not a second basis)", len(got[0].Alternates))
	}
}

func TestCollapseOverseasFiscalYearDup_keepsQuarterAndFY(t *testing.T) {
	q2 := EarningsEvent{
		Symbol: "FOO", Market: string(AssetUS), Date: "2026-08-19",
		FiscalYear: 2026, FiscalPeriod: "2Q",
	}
	fy := EarningsEvent{
		Symbol: "FOO", Market: string(AssetUS), Date: "2026-08-19",
		FiscalYear: 2026, FiscalPeriod: "FY",
	}
	got := CollapseOverseasFiscalYearDup([]EarningsEvent{q2, fy})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (2Q and FY are distinct prints)", len(got))
	}
}

func TestCollapseCalendarEvents_runsBothCollapses(t *testing.T) {
	individual := EarningsEvent{
		Symbol: "035420", Market: string(AssetDomestic), Date: "2026-08-07",
		FiscalYear: 2026, FiscalPeriod: "2Q", Basis: "개별",
	}
	consolidated := EarningsEvent{
		Symbol: "035420", Market: string(AssetDomestic), Date: "2026-08-07",
		FiscalYear: 2026, FiscalPeriod: "2Q", Basis: "연결",
	}
	tgtWrong := EarningsEvent{
		Symbol: "TGT", Market: string(AssetUS), Date: "2026-08-19",
		FiscalYear: 2026, FiscalPeriod: "2Q",
		EPS: EarningsMetric{Actual: f(2.46)},
	}
	tgtReal := EarningsEvent{
		Symbol: "TGT", Market: string(AssetUS), Date: "2026-08-19",
		FiscalYear: 2027, FiscalPeriod: "2Q",
		EPS: EarningsMetric{Estimate: f(2.31), Actual: f(2.46)},
	}

	got := CollapseCalendarEvents([]EarningsEvent{individual, consolidated, tgtWrong, tgtReal})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (NAVER 연결 + TGT 2027)", len(got))
	}
	if got[0].Symbol != "035420" || got[0].Basis != "연결" {
		t.Fatalf("first = %+v, want NAVER 연결", got[0])
	}
	if got[1].Symbol != "TGT" || got[1].FiscalYear != 2027 {
		t.Fatalf("second = %+v, want TGT 2027", got[1])
	}
}

func TestDropStaleOverseasScheduled_hidesPastKeepsToday(t *testing.T) {
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, KST)
	stale := EarningsEvent{
		Symbol: "AARD", Market: string(AssetUS), Date: "2026-08-25",
		DateStatus: EarningsDateScheduled, Session: EarningsSessionBeforeOpen,
		FiscalYear: 2026, FiscalPeriod: "2Q",
	}
	todayAMC := EarningsEvent{
		Symbol: "NVDA", Market: string(AssetUS), Date: "2026-08-26",
		DateStatus: EarningsDateScheduled, Session: EarningsSessionAfterClose,
		FiscalYear: 2027, FiscalPeriod: "2Q",
	}
	todayBMO := EarningsEvent{
		Symbol: "BAR", Market: string(AssetUS), Date: "2026-08-27",
		DateStatus: EarningsDateScheduled, Session: EarningsSessionBeforeOpen,
		FiscalYear: 2026, FiscalPeriod: "2Q",
	}
	confirmed := EarningsEvent{
		Symbol: "ADI", Market: string(AssetUS), Date: "2026-08-25",
		DateStatus: EarningsDateConfirmed, FiscalYear: 2026, FiscalPeriod: "3Q",
	}
	domestic := EarningsEvent{
		Symbol: "005930", Market: string(AssetDomestic), Date: "2026-08-25",
		DateStatus: EarningsDateScheduled, FiscalYear: 2026, FiscalPeriod: "2Q",
		Basis: "연결",
	}

	got := DropStaleOverseasScheduled(
		[]EarningsEvent{stale, todayAMC, todayBMO, confirmed, domestic}, now)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4 (drop stale bmo 8/25 only)", len(got))
	}
	for _, ev := range got {
		if ev.Symbol == "AARD" {
			t.Fatalf("stale 8/25 scheduled %s should be hidden", ev.Symbol)
		}
	}
}

func TestDropStaleOverseasScheduled_keepsFuture(t *testing.T) {
	now := time.Date(2026, 8, 21, 10, 0, 0, 0, KST)
	future := EarningsEvent{
		Symbol: "NVDA", Market: string(AssetUS), Date: "2026-11-17",
		DateStatus: EarningsDateScheduled, Session: "amc",
		FiscalYear: 2027, FiscalPeriod: "3Q",
	}
	got := DropStaleOverseasScheduled([]EarningsEvent{future}, now)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (upcoming scheduled stays)", len(got))
	}
}

func TestCollapseCalendarEvents_dropsStaleScheduled(t *testing.T) {
	// CollapseCalendarEvents uses time.Now(). A date two days before today
	// KST must disappear; yesterday must stay.
	kstToday := time.Now().In(KST)
	today := time.Date(kstToday.Year(), kstToday.Month(), kstToday.Day(), 0, 0, 0, 0, KST)
	staleDay := today.AddDate(0, 0, -2).Format("2006-01-02")
	yesterday := today.AddDate(0, 0, -1).Format("2006-01-02")

	stale := EarningsEvent{
		Symbol: "AARD", Market: string(AssetUS), Date: staleDay,
		DateStatus: EarningsDateScheduled, Session: EarningsSessionBeforeOpen,
		FiscalYear: 2026, FiscalPeriod: "2Q",
	}
	keep := EarningsEvent{
		Symbol: "FOO", Market: string(AssetUS), Date: yesterday,
		DateStatus: EarningsDateScheduled, Session: EarningsSessionAfterClose,
		FiscalYear: 2026, FiscalPeriod: "2Q",
	}
	got := CollapseCalendarEvents([]EarningsEvent{stale, keep})
	if len(got) != 1 || got[0].Symbol != "FOO" {
		t.Fatalf("got = %+v, want only yesterday amc scheduled (display today)", got)
	}
}
