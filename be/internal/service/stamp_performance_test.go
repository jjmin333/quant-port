package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

func pt(date string, close float64) repository.DailyClosePoint {
	return repository.DailyClosePoint{Date: date, Close: close}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return parsed
}

func TestCloseSeriesOnOrBefore(t *testing.T) {
	s := newCloseSeries([]repository.DailyClosePoint{
		pt("2026-08-07", 100), pt("2026-08-03", 90), pt("2026-08-05", 95),
	})
	got, ok := s.onOrBefore("2026-08-06")
	if !ok || got.Date != "2026-08-05" || got.Close != 95 {
		t.Fatalf("onOrBefore(08-06) = %+v ok=%v, want 08-05/95", got, ok)
	}
	if _, ok := s.onOrBefore("2026-08-01"); ok {
		t.Fatal("expected no point before the series start")
	}
	latest, ok := s.latest()
	if !ok || latest.Date != "2026-08-07" {
		t.Fatalf("latest = %+v, want 08-07", latest)
	}
}

// A horizon whose only available close is the stamp's own day has not elapsed —
// reporting 0% there would look like "flat" instead of "not yet".
func TestBuildStampReturnNilBeforeHorizonElapses(t *testing.T) {
	prices := newCloseSeries([]repository.DailyClosePoint{pt("2026-08-07", 100)})
	base := mustTime(t, "2026-08-07T05:00:00Z")

	got := buildStampReturn("1M", "2026-08-07", 100, horizonTargetDate(base, "1M"), prices, "", closeSeries{})
	if got.Return != nil {
		t.Fatalf("Return = %v, want nil (horizon not elapsed)", *got.Return)
	}
	if got.AsOfDate != "" {
		t.Fatalf("AsOfDate = %q, want empty", got.AsOfDate)
	}
}

// Friday stamp: the 1D target lands on Saturday, so "last close on or before
// target" is the stamp's own day. The next trading close must fill the bucket.
func TestBuildStampReturnBridgesWeekendGap(t *testing.T) {
	prices := newCloseSeries([]repository.DailyClosePoint{
		pt("2026-08-07", 100), pt("2026-08-10", 103),
	})
	friday := mustTime(t, "2026-08-07T05:00:00Z")

	got := buildStampReturn("1D", "2026-08-07", 100, horizonTargetDate(friday, "1D"), prices, "", closeSeries{})
	if got.AsOfDate != "2026-08-10" {
		t.Fatalf("AsOfDate = %q, want 2026-08-10 (next trading close)", got.AsOfDate)
	}
	if got.Return == nil || *got.Return != 3 {
		t.Fatalf("Return = %v, want 3", got.Return)
	}
}

// The gap bridge must not let a long-dormant symbol fill a short bucket with a
// close months later.
func TestBuildStampReturnRejectsCloseBeyondGapWindow(t *testing.T) {
	prices := newCloseSeries([]repository.DailyClosePoint{
		pt("2026-08-07", 100), pt("2026-11-20", 160),
	})
	base := mustTime(t, "2026-08-07T05:00:00Z")

	got := buildStampReturn("1D", "2026-08-07", 100, horizonTargetDate(base, "1D"), prices, "", closeSeries{})
	if got.Return != nil {
		t.Fatalf("Return = %v, want nil (close is far outside the 1D window)", *got.Return)
	}
}

func TestBuildStampReturnUsesLastCloseAtOrBeforeTarget(t *testing.T) {
	prices := newCloseSeries([]repository.DailyClosePoint{
		pt("2026-08-07", 100), pt("2026-08-12", 110), pt("2026-08-17", 120),
	})
	base := mustTime(t, "2026-08-07T05:00:00Z") // 1W target = 08-14, no bar → 08-12

	got := buildStampReturn("1W", "2026-08-07", 100, horizonTargetDate(base, "1W"), prices, "", closeSeries{})
	if got.AsOfDate != "2026-08-12" {
		t.Fatalf("AsOfDate = %q, want 2026-08-12", got.AsOfDate)
	}
	if got.Return == nil || *got.Return != 10 {
		t.Fatalf("Return = %v, want 10", got.Return)
	}
	if got.Alpha != nil {
		t.Fatal("Alpha must stay nil without a benchmark series")
	}
}

func TestBuildStampReturnAlpha(t *testing.T) {
	prices := newCloseSeries([]repository.DailyClosePoint{
		pt("2026-08-07", 100), pt("2026-08-14", 92),
	})
	bench := newCloseSeries([]repository.DailyClosePoint{
		pt("2026-08-07", 2000), pt("2026-08-14", 1780), // -11%
	})
	base := mustTime(t, "2026-08-07T05:00:00Z")

	got := buildStampReturn("1W", "2026-08-07", 100, horizonTargetDate(base, "1W"), prices, "kospi", bench)
	if got.Return == nil || *got.Return != -8 {
		t.Fatalf("Return = %v, want -8", got.Return)
	}
	if got.BenchmarkRet == nil || *got.BenchmarkRet != -11 {
		t.Fatalf("BenchmarkRet = %v, want -11", got.BenchmarkRet)
	}
	// Down 8% while the market fell 11% is a +3%p alpha — the number the stamp
	// exists to surface.
	if got.Alpha == nil || *got.Alpha != 3 {
		t.Fatalf("Alpha = %v, want 3", got.Alpha)
	}
}

func TestStampVerdict(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	cases := []struct {
		name   string
		stance model.StampStance
		ret    *float64
		want   model.StampVerdict
	}{
		{"bullish up", model.StanceBullish, f(4.2), model.VerdictHit},
		{"bullish down", model.StanceBullish, f(-0.1), model.VerdictMiss},
		{"bearish down", model.StanceBearish, f(-6), model.VerdictHit},
		{"bearish up", model.StanceBearish, f(2), model.VerdictMiss},
		{"neutral inside band", model.StanceNeutral, f(-2.9), model.VerdictHit},
		{"neutral outside band", model.StanceNeutral, f(3.1), model.VerdictMiss},
		{"no data", model.StanceBullish, nil, model.VerdictUnknown},
	}
	for _, tc := range cases {
		if got := stampVerdict(tc.stance, tc.ret); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func ex(date string, high, low float64) repository.DailyExtreme {
	return repository.DailyExtreme{Date: date, High: high, Low: low}
}

func TestBuildStampViewPendingUntilDue(t *testing.T) {
	stamp := model.Stamp{
		Symbol: "005930", StampedAt: mustTime(t, "2026-08-07T05:00:00Z"),
		TradeDate: "2026-08-07", Price: 100, Stance: model.StanceBullish,
		TargetPrice: 130, DueAt: mustTime(t, "2026-09-07T06:30:00Z"),
	}
	prices := newCloseSeries([]repository.DailyClosePoint{pt("2026-08-07", 100), pt("2026-08-10", 104)})

	view := buildStampView(stamp, mustTime(t, "2026-08-10T05:00:00Z"), 0, prices, closeSeries{}, nil)
	if view.Verdict != model.VerdictPending {
		t.Fatalf("Verdict = %q, want pending", view.Verdict)
	}
	if view.Due {
		t.Fatal("Due should be false before the deadline")
	}
	// 1D is already resolvable even while the verdict is pending.
	if view.Returns[0].Return == nil {
		t.Fatal("1D return should resolve from the 08-10 close")
	}
	if view.Current == nil || view.Current.Source != "close" || view.Current.Price != 104 {
		t.Fatalf("Current = %+v, want the latest close", view.Current)
	}
	// 100 → 130 target, price at 104: 4 of the 30-point move = 13.3%.
	if view.TargetProgress == nil || *view.TargetProgress < 13.3 || *view.TargetProgress > 13.4 {
		t.Fatalf("TargetProgress = %v, want ~13.33", view.TargetProgress)
	}
	if view.ExpectedReturn == nil || *view.ExpectedReturn != 30 {
		t.Fatalf("ExpectedReturn = %v, want 30", view.ExpectedReturn)
	}
}

func TestBuildStampViewJudgesAfterDeadline(t *testing.T) {
	stamp := model.Stamp{
		Symbol: "005930", StampedAt: mustTime(t, "2026-06-01T05:00:00Z"),
		TradeDate: "2026-06-01", Price: 100, Stance: model.StanceBullish,
		TargetPrice: 120, DueAt: mustTime(t, "2026-07-01T06:30:00Z"),
	}
	prices := newCloseSeries([]repository.DailyClosePoint{pt("2026-06-01", 100), pt("2026-07-01", 112)})

	view := buildStampView(stamp, mustTime(t, "2026-08-01T05:00:00Z"), 118, prices, closeSeries{}, nil)
	if !view.Due {
		t.Fatal("Due should be true once the deadline passed")
	}
	if view.Verdict != model.VerdictHit {
		t.Fatalf("Verdict = %q, want hit", view.Verdict)
	}
	if view.Horizon == nil || view.Horizon.Return == nil || *view.Horizon.Return != 12 {
		t.Fatalf("Horizon return = %+v, want 12", view.Horizon)
	}
	// Direction decides the verdict; the target error is reported alongside it.
	// Settled at 112 against a 120 target = 6.667% off.
	if view.TargetAccuracy == nil || *view.TargetAccuracy < 6.6 || *view.TargetAccuracy > 6.7 {
		t.Fatalf("TargetAccuracy = %v, want ~6.67", view.TargetAccuracy)
	}
	// A live quote wins over the stored close for the 현재 column.
	if view.Current == nil || view.Current.Source != "quote" || view.Current.Price != 118 {
		t.Fatalf("Current = %+v, want the live quote", view.Current)
	}
}

// The whole point of judging on intraday range: a target that was touched and
// given back is a target that was reached.
func TestBuildStampViewEarlyHitClosesTheStamp(t *testing.T) {
	stamp := model.Stamp{
		Symbol: "005930", StampedAt: mustTime(t, "2026-06-01T05:00:00Z"),
		TradeDate: "2026-06-01", Price: 100, Stance: model.StanceBullish,
		TargetPrice: 120, DueAt: mustTime(t, "2026-09-01T06:30:00Z"),
	}
	prices := newCloseSeries([]repository.DailyClosePoint{pt("2026-06-01", 100), pt("2026-06-20", 98)})
	extremes := []repository.DailyExtreme{
		ex("2026-06-01", 101, 99),
		ex("2026-06-10", 121, 110), // touched 120 and closed below it later
		ex("2026-06-20", 105, 96),
	}

	view := buildStampView(stamp, mustTime(t, "2026-07-01T05:00:00Z"), 98, prices, closeSeries{}, extremes)
	if view.Verdict != model.VerdictHit {
		t.Fatalf("Verdict = %q, want hit — the high reached the target", view.Verdict)
	}
	if !view.Due {
		t.Fatal("an early hit settles the stamp, so it should be due for opening")
	}
	if view.TargetHit == nil || view.TargetHit.Date != "2026-06-10" {
		t.Fatalf("TargetHit = %+v, want 2026-06-10", view.TargetHit)
	}
	if view.TargetAccuracy == nil || *view.TargetAccuracy != 0 {
		t.Fatalf("TargetAccuracy = %v, want 0 for a reached target", view.TargetAccuracy)
	}
	if view.PeakReturn == nil || *view.PeakReturn != 21 {
		t.Fatalf("PeakReturn = %v, want 21", view.PeakReturn)
	}
	if view.TroughReturn == nil || *view.TroughReturn != -4 {
		t.Fatalf("TroughReturn = %v, want -4", view.TroughReturn)
	}
}

// The stamp day's own high may predate the stamp, so it must not credit a hit.
func TestFindTargetHitIgnoresTheStampDay(t *testing.T) {
	stamp := model.Stamp{
		StampedAt: mustTime(t, "2026-06-01T05:00:00Z"), TradeDate: "2026-06-01",
		Price: 100, Stance: model.StanceBullish, TargetPrice: 120,
		DueAt: mustTime(t, "2026-09-01T06:30:00Z"),
	}
	extremes := []repository.DailyExtreme{ex("2026-06-01", 130, 99)}
	if hit := findTargetHit(stamp, extremes, mustTime(t, "2026-07-01T05:00:00Z")); hit != nil {
		t.Fatalf("findTargetHit = %+v, want nil on the stamp day itself", hit)
	}
}

// 관망 has no direction to reach, so it is only answerable at the deadline.
func TestFindTargetHitSkipsNeutral(t *testing.T) {
	stamp := model.Stamp{
		StampedAt: mustTime(t, "2026-06-01T05:00:00Z"), TradeDate: "2026-06-01",
		Price: 100, Stance: model.StanceNeutral, TargetPrice: 101,
		DueAt: mustTime(t, "2026-09-01T06:30:00Z"),
	}
	extremes := []repository.DailyExtreme{ex("2026-06-05", 130, 70)}
	if hit := findTargetHit(stamp, extremes, mustTime(t, "2026-07-01T05:00:00Z")); hit != nil {
		t.Fatalf("findTargetHit = %+v, want nil for 관망", hit)
	}
}

func TestFindTargetHitBearish(t *testing.T) {
	stamp := model.Stamp{
		StampedAt: mustTime(t, "2026-06-01T05:00:00Z"), TradeDate: "2026-06-01",
		Price: 100, Stance: model.StanceBearish, TargetPrice: 85,
		DueAt: mustTime(t, "2026-09-01T06:30:00Z"),
	}
	extremes := []repository.DailyExtreme{
		ex("2026-06-05", 99, 86), // one won above the target
		ex("2026-06-08", 92, 84),
	}
	hit := findTargetHit(stamp, extremes, mustTime(t, "2026-07-01T05:00:00Z"))
	if hit == nil || hit.Date != "2026-06-08" || hit.Price != 84 {
		t.Fatalf("findTargetHit = %+v, want 2026-06-08/84", hit)
	}
}

// A hit found after the deadline belongs to the next stamp, not this one.
func TestFindTargetHitStopsAtTheDeadline(t *testing.T) {
	stamp := model.Stamp{
		StampedAt: mustTime(t, "2026-06-01T05:00:00Z"), TradeDate: "2026-06-01",
		Price: 100, Stance: model.StanceBullish, TargetPrice: 120,
		DueAt: mustTime(t, "2026-06-15T06:30:00Z"),
	}
	extremes := []repository.DailyExtreme{ex("2026-06-20", 125, 118)}
	if hit := findTargetHit(stamp, extremes, mustTime(t, "2026-07-01T05:00:00Z")); hit != nil {
		t.Fatalf("findTargetHit = %+v, want nil past the deadline", hit)
	}
}

func TestStanceForTarget(t *testing.T) {
	cases := []struct {
		price, target float64
		want          model.StampStance
	}{
		{100, 130, model.StanceBullish},
		{100, 103.1, model.StanceBullish},
		{100, 103, model.StanceNeutral}, // exactly the band edge stays 관망
		{100, 100, model.StanceNeutral},
		{100, 97, model.StanceNeutral},
		{100, 96.9, model.StanceBearish},
		{100, 70, model.StanceBearish},
		{0, 100, model.StanceNeutral}, // no captured price to compare against
	}
	for _, tc := range cases {
		if got := model.StanceForTarget(tc.price, tc.target); got != tc.want {
			t.Errorf("StanceForTarget(%v, %v) = %q, want %q", tc.price, tc.target, got, tc.want)
		}
	}
}

func TestSummarizeStampsCountsOnlyJudged(t *testing.T) {
	base := mustTime(t, "2026-06-01T05:00:00Z")
	due := mustTime(t, "2026-07-01T06:30:00Z")
	prices := newCloseSeries([]repository.DailyClosePoint{pt("2026-06-01", 100), pt("2026-07-01", 112)})
	now := mustTime(t, "2026-08-01T05:00:00Z")

	hit := buildStampView(model.Stamp{
		Symbol: "A", StampedAt: base, TradeDate: "2026-06-01", Price: 100,
		Stance: model.StanceBullish, TargetPrice: 112, DueAt: due,
	}, now, 0, prices, closeSeries{}, nil)
	miss := buildStampView(model.Stamp{
		Symbol: "B", StampedAt: base, TradeDate: "2026-06-01", Price: 100,
		Stance: model.StanceBearish, TargetPrice: 80, DueAt: due,
	}, now, 0, prices, closeSeries{}, nil)
	pending := buildStampView(model.Stamp{
		Symbol: "C", StampedAt: now, TradeDate: "2026-08-01", Price: 100,
		Stance: model.StanceBullish, TargetPrice: 120,
		DueAt: mustTime(t, "2026-09-01T06:30:00Z"),
	}, now, 0, closeSeries{}, closeSeries{}, nil)

	got := summarizeStamps([]model.StampView{hit, miss, pending})
	if got.Total != 3 || got.Symbols != 3 {
		t.Fatalf("Total/Symbols = %d/%d, want 3/3", got.Total, got.Symbols)
	}
	if got.Judged != 2 || got.Hits != 1 {
		t.Fatalf("Judged/Hits = %d/%d, want 2/1", got.Judged, got.Hits)
	}
	if got.HitRate == nil || *got.HitRate != 50 {
		t.Fatalf("HitRate = %v, want 50 (pending must not dilute it)", got.HitRate)
	}
	if got.DueCount != 2 {
		t.Fatalf("DueCount = %d, want 2", got.DueCount)
	}
	// A settled at exactly its target (0% error), B settled 40% away from 80.
	if got.AvgTargetError == nil || *got.AvgTargetError != 20 {
		t.Fatalf("AvgTargetError = %v, want 20", got.AvgTargetError)
	}
	if got.WithinBand != 1 {
		t.Fatalf("WithinBand = %d, want 1", got.WithinBand)
	}
}
