package service

import "testing"

func fptr(v float64) *float64 { return &v }

func TestMergeFinnhubCalendar_targetPrefersYearWithEstimates(t *testing.T) {
	wrong := FinnhubEarningsRow{
		Symbol: "TGT", Date: "2026-08-19", Hour: "bmo", Quarter: 2, Year: 2026,
		EPSActual: fptr(2.46), RevenueActual: fptr(26.54e9),
	}
	real := FinnhubEarningsRow{
		Symbol: "TGT", Date: "2026-08-19", Hour: "bmo", Quarter: 2, Year: 2027,
		EPSEstimate: fptr(2.3095), EPSActual: fptr(2.46),
		RevenueEstimate: fptr(26.32e9), RevenueActual: fptr(26.54e9),
	}

	got := mergeFinnhubCalendar([]FinnhubEarningsRow{wrong, real})
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Year != 2027 {
		t.Fatalf("year = %d, want 2027", got[0].Year)
	}
	if got[0].EPSEstimate == nil || *got[0].EPSEstimate != 2.3095 {
		t.Fatalf("epsEstimate = %v, want 2.3095", got[0].EPSEstimate)
	}
}

func TestMergeFinnhubCalendar_keepsQuarterAndFY(t *testing.T) {
	q2 := FinnhubEarningsRow{Symbol: "FOO", Date: "2026-08-19", Quarter: 2, Year: 2026}
	fy := FinnhubEarningsRow{Symbol: "FOO", Date: "2026-08-19", Quarter: 0, Year: 2026}

	got := mergeFinnhubCalendar([]FinnhubEarningsRow{q2, fy})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (quarter vs FY)", len(got))
	}
}

func TestMergeFinnhubCalendar_fillsMissingHourAndActuals(t *testing.T) {
	a := FinnhubEarningsRow{
		Symbol: "BAR", Date: "2026-08-19", Quarter: 1, Year: 2026,
		EPSEstimate: fptr(1.0),
	}
	b := FinnhubEarningsRow{
		Symbol: "BAR", Date: "2026-08-19", Hour: "amc", Quarter: 1, Year: 2026,
		EPSActual: fptr(1.1),
	}

	got := mergeFinnhubCalendar([]FinnhubEarningsRow{a, b})
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Hour != "amc" {
		t.Fatalf("hour = %q, want amc", got[0].Hour)
	}
	if got[0].EPSEstimate == nil || got[0].EPSActual == nil {
		t.Fatalf("expected estimate and actual merged: %+v", got[0])
	}
}
