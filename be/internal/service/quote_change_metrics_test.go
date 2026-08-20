package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func TestPreviousMonthClose_lastJulyBar(t *testing.T) {
	t.Parallel()
	loc := kst
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, loc)
	bars := []model.Candle{
		{Time: "2026-07-28", Close: 190_000},
		{Time: "2026-07-31", Close: 195_000},
		{Time: "2026-08-01", Close: 200_000},
		{Time: "2026-08-02", Close: 259_000},
	}
	got, ok := previousMonthClose(bars, now, loc, false, 0)
	if !ok {
		t.Fatal("expected previous month close")
	}
	if got != 195_000 {
		t.Fatalf("previous month close = %.0f want 195000", got)
	}
}

func TestPreviousMonthClose_krxMonthEndStamp(t *testing.T) {
	t.Parallel()
	loc := kst
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, loc)
	bars := []model.Candle{
		{Time: "2026-07-30", Close: 190_000},
		{Time: "2026-08-01T00:00:00+09:00", Close: 198_000},
		{Time: "2026-08-02", Close: 259_000},
	}
	// KRX month-end stamp (Aug 1 00:00) is included when includeKRXSlop is true.
	got, ok := previousMonthClose(bars, now, loc, true, 0)
	if !ok {
		t.Fatal("expected previous month close")
	}
	if got != 198_000 {
		t.Fatalf("previous month close = %.0f want 198000", got)
	}
}

func TestPreviousMonthClose_walkBackWhenMonthEndEqualsCurrent(t *testing.T) {
	t.Parallel()
	loc := kst
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, loc)
	bars := []model.Candle{
		{Time: "2026-07-25", Close: 255_000},
		{Time: "2026-07-31", Close: 259_000},
		{Time: "2026-08-02", Close: 259_000},
	}
	got, ok := previousMonthClose(bars, now, loc, false, 259_000)
	if !ok {
		t.Fatal("expected previous month close")
	}
	if got != 255_000 {
		t.Fatalf("previous month close = %.0f want 255000", got)
	}
}

func TestIsSamePrice(t *testing.T) {
	t.Parallel()
	if !isSamePrice(259_000, 259_000) {
		t.Fatal("expected same KRW price")
	}
	if isSamePrice(259_000, 255_000) {
		t.Fatal("expected different prices")
	}
}

func TestPreviousWeekClose(t *testing.T) {
	t.Parallel()
	loc := kst
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, loc)
	bars := []model.Candle{
		{Time: "2026-07-18T00:00:00+09:00", Close: 100},
		{Time: "2026-07-25T00:00:00+09:00", Close: 110},
		{Time: "2026-07-28T00:00:00+09:00", Close: 115},
	}
	got, ok := previousWeekClose(bars, now, loc)
	if !ok {
		t.Fatal("expected previous week close")
	}
	if got != 110 {
		t.Fatalf("previous week close = %.0f want 110", got)
	}
}

func TestClose52WeeksAgo(t *testing.T) {
	t.Parallel()
	loc := kst
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, loc)
	cutoff := now.AddDate(-1, 0, 0)
	bars := []model.Candle{
		{Time: cutoff.Add(-48 * time.Hour).Format(time.RFC3339), Close: 80},
		{Time: cutoff.Add(-24 * time.Hour).Format(time.RFC3339), Close: 85},
		{Time: now.Add(-24 * time.Hour).Format(time.RFC3339), Close: 120},
	}
	got, ok := close52WeeksAgo(bars, now, loc)
	if !ok {
		t.Fatal("expected 52-week close")
	}
	if got != 85 {
		t.Fatalf("52-week close = %.0f want 85", got)
	}
}

func TestBarsCoverYearHistory(t *testing.T) {
	t.Parallel()
	now := time.Now()
	recent := []model.Candle{
		{Time: now.Add(-30 * 24 * time.Hour).Format(time.RFC3339), Close: 100},
		{Time: now.Add(-1 * 24 * time.Hour).Format(time.RFC3339), Close: 110},
	}
	if barsCoverYearHistory(recent) {
		t.Fatal("expected recent bars to fail year coverage")
	}

	old := make([]model.Candle, 0, 370)
	for i := 370; i >= 1; i-- {
		old = append(old, model.Candle{
			Time:  now.Add(-time.Duration(i) * 24 * time.Hour).Format(time.RFC3339),
			Close: 100,
		})
	}
	if !barsCoverYearHistory(old) {
		t.Fatal("expected year-long bars to pass coverage")
	}
}

func TestChangeRateFromBaseline(t *testing.T) {
	t.Parallel()
	rate, ok := changeRateFromBaseline(110, 100)
	if !ok || rate != 10 {
		t.Fatalf("rate = %.2f ok=%v want 10", rate, ok)
	}
	if _, ok := changeRateFromBaseline(110, 0); ok {
		t.Fatal("expected false for zero baseline")
	}
}
