package model

import (
	"testing"
	"time"
)

func TestOverseasDisplayDate_amcAndDmhShift(t *testing.T) {
	if got := OverseasDisplayDate("2026-08-26", EarningsSessionAfterClose); got != "2026-08-27" {
		t.Fatalf("amc = %q, want 2026-08-27", got)
	}
	if got := OverseasDisplayDate("2026-08-26", EarningsSessionDuringHours); got != "2026-08-27" {
		t.Fatalf("dmh = %q, want 2026-08-27", got)
	}
}

func TestOverseasDisplayDate_bmoUnchanged(t *testing.T) {
	if got := OverseasDisplayDate("2026-08-27", EarningsSessionBeforeOpen); got != "2026-08-27" {
		t.Fatalf("bmo = %q, want 2026-08-27", got)
	}
}

func TestApplyOverseasDisplayDates_onlyUS(t *testing.T) {
	events := []EarningsEvent{
		{Symbol: "NVDA", Market: string(AssetUS), Date: "2026-08-26", Session: EarningsSessionAfterClose},
		{Symbol: "005930", Market: string(AssetDomestic), Date: "2026-08-14"},
	}
	ApplyOverseasDisplayDates(events)
	if events[0].Date != "2026-08-27" {
		t.Fatalf("US date = %q, want 2026-08-27", events[0].Date)
	}
	if events[1].Date != "2026-08-14" {
		t.Fatalf("KR date = %q, want unchanged 2026-08-14", events[1].Date)
	}
}

func TestFilterEventsByCalendarDate(t *testing.T) {
	from := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	to := from
	rows := []EarningsEvent{
		{Symbol: "NVDA", Date: "2026-08-27"},
		{Symbol: "CRM", Date: "2026-08-26"},
	}
	got := FilterEventsByCalendarDate(rows, from, to)
	if len(got) != 1 || got[0].Symbol != "NVDA" {
		t.Fatalf("got = %+v, want only NVDA on 8/27", got)
	}
}
