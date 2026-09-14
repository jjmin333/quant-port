package service

import (
	"math"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

func researchSeries(pairs ...any) closeSeries {
	points := make([]repository.DailyClosePoint, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		points = append(points, repository.DailyClosePoint{
			Date: pairs[i].(string), Close: pairs[i+1].(float64),
		})
	}
	return newCloseSeries(points)
}

// mustTime lives in stamp_performance_test.go — the two suites share it.

func ptr[T any](v T) *T { return &v }

func baseCall(t *testing.T) model.ResearchCall {
	t.Helper()
	return model.ResearchCall{
		Symbol:      "005930",
		PublishedAt: mustTime(t, "2026-01-05T01:00:00Z"),
		TradeDate:   "2026-01-05",
		BenchmarkID: "kospi",
		Direction:   model.DirectionBuy,
		BasePrice:   100,
		HorizonDays: 30,
		Conviction:  70,
		Status:      model.StatusOpen,
	}
}

// A close touching the target resolves the call the day it happens, not at
// expiry — otherwise a call that already worked keeps showing as "open".
func TestJudgeResearchCallTargetHitUsesFirstReachingClose(t *testing.T) {
	c := baseCall(t)
	c.TargetPrice = ptr(110.0)
	c.StopPrice = ptr(90.0)

	prices := researchSeries(
		"2026-01-05", 100.0,
		"2026-01-06", 105.0,
		"2026-01-07", 112.0, // target
		"2026-01-08", 130.0,
	)
	j := judgeResearchCall(c, mustTime(t, "2026-01-09T00:00:00Z"), prices, closeSeries{})
	if !j.Changed || j.Status != model.StatusTargetHit {
		t.Fatalf("want target_hit, got %+v", j)
	}
	if j.Date != "2026-01-07" {
		t.Fatalf("want the first reaching close 2026-01-07, got %q", j.Date)
	}
	if j.Return == nil || math.Abs(*j.Return-12) > 1e-9 {
		t.Fatalf("want +12%% return, got %v", j.Return)
	}
}

// The stop is checked with the same walk as the target, so whichever comes
// first in time wins — not whichever is listed first in the code.
func TestJudgeResearchCallStopHitWhenItComesFirst(t *testing.T) {
	c := baseCall(t)
	c.TargetPrice = ptr(110.0)
	c.StopPrice = ptr(90.0)

	prices := researchSeries(
		"2026-01-06", 88.0, // stop first
		"2026-01-07", 115.0,
	)
	j := judgeResearchCall(c, mustTime(t, "2026-01-09T00:00:00Z"), prices, closeSeries{})
	if j.Status != model.StatusStopHit || j.Date != "2026-01-06" {
		t.Fatalf("want stop_hit on 2026-01-06, got %+v", j)
	}
}

// Judgement reads closes only. An intraday spike is invisible here by design —
// this test pins that a bar whose close never reached the target does not
// resolve the call.
func TestJudgeResearchCallIgnoresUnreachedTargetUntilExpiry(t *testing.T) {
	c := baseCall(t)
	c.TargetPrice = ptr(110.0)

	prices := researchSeries("2026-01-06", 109.99)
	j := judgeResearchCall(c, mustTime(t, "2026-01-07T00:00:00Z"), prices, closeSeries{})
	if j.Changed {
		t.Fatalf("call must stay open below the target close, got %+v", j)
	}
}

// A bound reached *after* the horizon expired must not resolve the call as a
// target hit — the horizon is the promise, and a late backfill cannot extend it.
func TestJudgeResearchCallIgnoresBoundAfterDeadline(t *testing.T) {
	c := baseCall(t)
	c.TargetPrice = ptr(110.0)

	prices := researchSeries(
		"2026-01-20", 101.0,
		"2026-02-10", 130.0, // after the 30-day horizon
	)
	j := judgeResearchCall(c, mustTime(t, "2026-02-20T00:00:00Z"), prices, closeSeries{})
	if j.Status == model.StatusTargetHit {
		t.Fatalf("a post-deadline close must not count as a target hit: %+v", j)
	}
	if j.Status != model.StatusExpiredHit {
		t.Fatalf("want expired_hit (price rose over the horizon), got %+v", j)
	}
}

func TestJudgeResearchCallExpiredMiss(t *testing.T) {
	c := baseCall(t)
	prices := researchSeries("2026-02-04", 90.0)
	j := judgeResearchCall(c, mustTime(t, "2026-02-10T00:00:00Z"), prices, closeSeries{})
	if j.Status != model.StatusExpiredMiss {
		t.Fatalf("want expired_miss, got %+v", j)
	}
}

// An elapsed horizon with no close to judge against is "unknown", never a miss.
// Counting it as a miss would penalise the author for our missing data.
func TestJudgeResearchCallExpiredUnknownWithoutCloses(t *testing.T) {
	c := baseCall(t)
	j := judgeResearchCall(c, mustTime(t, "2026-03-01T00:00:00Z"), closeSeries{}, closeSeries{})
	if j.Status != model.StatusExpiredUnknown || !j.Changed {
		t.Fatalf("want expired_unknown, got %+v", j)
	}
	if j.Return != nil {
		t.Fatalf("unknown verdict must carry no return, got %v", *j.Return)
	}
}

func TestJudgeResearchCallAlphaSubtractsBenchmark(t *testing.T) {
	c := baseCall(t)
	prices := researchSeries("2026-02-04", 110.0)                     // +10%
	bench := researchSeries("2026-01-05", 100.0, "2026-02-04", 104.0) // +4%
	j := judgeResearchCall(c, mustTime(t, "2026-02-10T00:00:00Z"), prices, bench)
	if j.Alpha == nil || math.Abs(*j.Alpha-6) > 1e-9 {
		t.Fatalf("want alpha +6pp, got %v", j.Alpha)
	}
}

// A sell call's target sits below the base price and is reached from above.
func TestJudgeResearchCallSellDirectionBounds(t *testing.T) {
	c := baseCall(t)
	c.Direction = model.DirectionSell
	c.TargetPrice = ptr(90.0)
	c.StopPrice = ptr(110.0)

	prices := researchSeries("2026-01-08", 89.0)
	j := judgeResearchCall(c, mustTime(t, "2026-01-10T00:00:00Z"), prices, closeSeries{})
	if j.Status != model.StatusTargetHit {
		t.Fatalf("want target_hit for a sell call falling to target, got %+v", j)
	}
}

// A 관망 call has no directional bound, so nothing resolves early; it is judged
// at expiry by whether the price stayed inside the band.
func TestJudgeResearchCallHoldUsesBandAtExpiry(t *testing.T) {
	c := baseCall(t)
	c.Direction = model.DirectionHold

	inside := researchSeries("2026-02-04", 101.0)
	if got := judgeResearchCall(c, mustTime(t, "2026-02-10T00:00:00Z"), inside, closeSeries{}); got.Status != model.StatusExpiredHit {
		t.Fatalf("1%% drift is inside the band, want expired_hit, got %+v", got)
	}
	outside := researchSeries("2026-02-04", 120.0)
	if got := judgeResearchCall(c, mustTime(t, "2026-02-10T00:00:00Z"), outside, closeSeries{}); got.Status != model.StatusExpiredMiss {
		t.Fatalf("20%% drift is outside the band, want expired_miss, got %+v", got)
	}
}

func TestJudgeResearchCallSkipsAlreadyResolved(t *testing.T) {
	c := baseCall(t)
	c.Status = model.StatusExpiredHit
	if j := judgeResearchCall(c, mustTime(t, "2026-03-01T00:00:00Z"), researchSeries("2026-02-04", 90.0), closeSeries{}); j.Changed {
		t.Fatalf("a resolved call must never be re-judged, got %+v", j)
	}
}

// Retracted calls stay out of the hit rate but still drive the retract rate —
// that pairing is what stops a track record from being curated.
func TestSummarizeResearchRetractionsCountSeparately(t *testing.T) {
	calls := []model.ResearchCall{
		{Status: model.StatusExpiredHit, Conviction: 60, VerdictReturn: ptr(10.0), VerdictAlpha: ptr(4.0)},
		{Status: model.StatusExpiredMiss, Conviction: 60, VerdictReturn: ptr(-5.0), VerdictAlpha: ptr(-2.0)},
		{Status: model.StatusRetracted, Conviction: 90},
		{Status: model.StatusOpen, Conviction: 50},
	}
	s := summarizeResearch(calls)
	if s.Resolved != 2 || s.Hits != 1 {
		t.Fatalf("want 2 resolved / 1 hit, got %d/%d", s.Resolved, s.Hits)
	}
	if s.HitRate == nil || math.Abs(*s.HitRate-50) > 1e-9 {
		t.Fatalf("want 50%% hit rate, got %v", s.HitRate)
	}
	if s.Retracted != 1 || s.RetractRate == nil || math.Abs(*s.RetractRate-25) > 1e-9 {
		t.Fatalf("want 1 retracted / 25%%, got %d / %v", s.Retracted, s.RetractRate)
	}
	if s.Open != 1 {
		t.Fatalf("want 1 open, got %d", s.Open)
	}
	if s.AvgAlpha == nil || math.Abs(*s.AvgAlpha-1) > 1e-9 {
		t.Fatalf("want avg alpha +1pp, got %v", s.AvgAlpha)
	}
}

// expired_unknown is neither a hit nor a miss and must not enter the rate.
func TestSummarizeResearchExcludesUnknown(t *testing.T) {
	s := summarizeResearch([]model.ResearchCall{
		{Status: model.StatusExpiredUnknown, Conviction: 80},
		{Status: model.StatusExpiredHit, Conviction: 80},
	})
	if s.Resolved != 1 || s.Hits != 1 {
		t.Fatalf("want 1 resolved / 1 hit, got %d/%d", s.Resolved, s.Hits)
	}
}

// Overstated conviction has to cost something measurable, or the field is
// decoration. A 90%-confident author who is right half the time scores worse
// than a 50%-confident one with the same record.
func TestSummarizeResearchCalibrationPunishesOverconfidence(t *testing.T) {
	confident := summarizeResearch([]model.ResearchCall{
		{Status: model.StatusExpiredHit, Conviction: 90},
		{Status: model.StatusExpiredMiss, Conviction: 90},
	})
	humble := summarizeResearch([]model.ResearchCall{
		{Status: model.StatusExpiredHit, Conviction: 50},
		{Status: model.StatusExpiredMiss, Conviction: 50},
	})
	if confident.Calibration == nil || humble.Calibration == nil {
		t.Fatal("both need a calibration score")
	}
	if *confident.Calibration <= *humble.Calibration {
		t.Fatalf("overconfidence must score worse: %v vs %v", *confident.Calibration, *humble.Calibration)
	}
}

func TestSummarizeResearchContrarianSliceIsSeparate(t *testing.T) {
	s := summarizeResearch([]model.ResearchCall{
		{Status: model.StatusExpiredHit, Contrarian: true, Conviction: 50},
		{Status: model.StatusExpiredMiss, Contrarian: true, Conviction: 50},
		{Status: model.StatusExpiredHit, Conviction: 50},
	})
	if s.ContrarianResolved != 2 || s.ContrarianHits != 1 {
		t.Fatalf("want 2/1 contrarian, got %d/%d", s.ContrarianResolved, s.ContrarianHits)
	}
	if s.ContrarianHitRate == nil || math.Abs(*s.ContrarianHitRate-50) > 1e-9 {
		t.Fatalf("want 50%% contrarian hit rate, got %v", s.ContrarianHitRate)
	}
}

// ------------------------------------------------------------------ odds

// oddsSeries builds a deterministic series that rises by `step` percent per
// session, so the expected reachability is checkable by hand.
func oddsSeries(n int, start, step float64) []repository.DailyClosePoint {
	points := make([]repository.DailyClosePoint, 0, n)
	price := start
	day := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		points = append(points, repository.DailyClosePoint{
			Date: day.AddDate(0, 0, i).Format(seriesDateLayout), Close: price,
		})
		price *= 1 + step/100
	}
	return points
}

// Too little history must say "insufficient", not return a small probability —
// a 0% that means "we don't know" is the most misleading number we could show.
func TestResearchTargetOddsInsufficientHistory(t *testing.T) {
	odds := researchTargetOdds(oddsSeries(40, 100, 0.1), 100, 200, 365)
	if odds == nil || !odds.Insufficient {
		t.Fatalf("want insufficient, got %+v", odds)
	}
	if odds.Probability != nil {
		t.Fatalf("insufficient history must carry no probability, got %v", *odds.Probability)
	}
}

func TestResearchTargetOddsAlwaysReachedOnSteadyRise(t *testing.T) {
	// +1%/session over a 30-day (≈21 session) window compounds to ~+23%, so a
	// +10% target is reached in every window.
	odds := researchTargetOdds(oddsSeries(400, 100, 1.0), 100, 110, 30)
	if odds == nil || odds.Insufficient {
		t.Fatalf("want a usable estimate, got %+v", odds)
	}
	if odds.Probability == nil || *odds.Probability < 99.9 {
		t.Fatalf("want ~100%% probability, got %v", odds.Probability)
	}
	if odds.Windows != odds.Reached {
		t.Fatalf("every window should reach: %d/%d", odds.Reached, odds.Windows)
	}
}

func TestResearchTargetOddsNeverReachedOnFlatSeries(t *testing.T) {
	odds := researchTargetOdds(oddsSeries(400, 100, 0), 100, 200, 90)
	if odds == nil || odds.Insufficient {
		t.Fatalf("want a usable estimate, got %+v", odds)
	}
	if odds.Probability == nil || *odds.Probability != 0 {
		t.Fatalf("a flat series can never double, want 0%%, got %v", odds.Probability)
	}
	if odds.RequiredPct != 100 {
		t.Fatalf("want required +100%%, got %v", odds.RequiredPct)
	}
}

// A downside target (sell call) must be measured in the other direction.
func TestResearchTargetOddsDownsideDirection(t *testing.T) {
	odds := researchTargetOdds(oddsSeries(400, 100, -1.0), 100, 90, 30)
	if odds == nil || odds.Probability == nil || *odds.Probability < 99.9 {
		t.Fatalf("a steadily falling series should reach a -10%% target, got %+v", odds)
	}
}

// ------------------------------------------------------------------ EV

func TestResearchExpectedValueUsesStatedAndHistorical(t *testing.T) {
	c := baseCall(t)
	c.TargetPrice = ptr(120.0) // +20%
	c.StopPrice = ptr(90.0)    // -10%
	c.Conviction = 70

	ev := researchExpectedValue(c, ptr(40.0))
	if ev == nil || ev.Stated == nil || ev.Historical == nil {
		t.Fatalf("want both EVs, got %+v", ev)
	}
	// 0.7*20 + 0.3*(-10) = 11
	if math.Abs(*ev.Stated-11) > 1e-9 {
		t.Fatalf("want stated EV 11, got %v", *ev.Stated)
	}
	// 0.4*20 + 0.6*(-10) = 2
	if math.Abs(*ev.Historical-2) > 1e-9 {
		t.Fatalf("want historical EV 2, got %v", *ev.Historical)
	}
	if ev.RiskReward == nil || math.Abs(*ev.RiskReward-2) > 1e-9 {
		t.Fatalf("want risk/reward 2.0, got %v", ev.RiskReward)
	}
}

func TestResearchExpectedValueNilWithoutTarget(t *testing.T) {
	c := baseCall(t)
	if ev := researchExpectedValue(c, nil); ev != nil {
		t.Fatalf("no target means no expected value, got %+v", ev)
	}
}

// ------------------------------------------------------------------ bounds

func TestNormalizeBoundsRejectsContradictoryPairs(t *testing.T) {
	cases := []struct {
		name      string
		direction model.ResearchDirection
		target    *float64
		stop      *float64
		wantErr   bool
	}{
		{"buy ok", model.DirectionBuy, ptr(120.0), ptr(90.0), false},
		{"buy target below base", model.DirectionBuy, ptr(80.0), nil, true},
		{"buy stop above base", model.DirectionBuy, ptr(120.0), ptr(110.0), true},
		{"sell ok", model.DirectionSell, ptr(80.0), ptr(110.0), false},
		{"sell target above base", model.DirectionSell, ptr(120.0), nil, true},
		{"sell stop below base", model.DirectionSell, ptr(80.0), ptr(90.0), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := normalizeBounds(tc.direction, 100, tc.target, tc.stop)
			if tc.wantErr != (err != nil) {
				t.Fatalf("wantErr=%v, got %v", tc.wantErr, err)
			}
		})
	}
}

// A 관망 call stores no bounds at all, so nothing can auto-resolve it early.
func TestNormalizeBoundsDropsBoundsForHold(t *testing.T) {
	target, stop, err := normalizeBounds(model.DirectionHold, 100, ptr(120.0), ptr(90.0))
	if err != nil {
		t.Fatalf("hold must not error: %v", err)
	}
	if target != nil || stop != nil {
		t.Fatalf("hold must drop both bounds, got %v / %v", target, stop)
	}
}

func TestIsContrarianOnlyWhenScoreDisagrees(t *testing.T) {
	if !isContrarian(model.DirectionBuy, ptr(35)) {
		t.Fatal("buying a 35-point symbol is contrarian")
	}
	if isContrarian(model.DirectionBuy, ptr(75)) {
		t.Fatal("buying a 75-point symbol agrees with the score")
	}
	if !isContrarian(model.DirectionSell, ptr(75)) {
		t.Fatal("selling a 75-point symbol is contrarian")
	}
	if isContrarian(model.DirectionBuy, nil) {
		t.Fatal("no score means no contrarian claim")
	}
}

// The public floor must not fire on a symbol whose market cap we simply failed
// to read — reporting "too small" for an unmeasured symbol would be a lie.
func TestResearchEligibleAllowsUnknownMarketCap(t *testing.T) {
	if ok, _ := researchEligible(model.CurrencyKRW, 0); !ok {
		t.Fatal("unknown market cap must not block publishing")
	}
	if ok, _ := researchEligible(model.CurrencyKRW, 1_000_000_000); ok {
		t.Fatal("10억 시총은 공개 발행 기준 미만")
	}
	if ok, _ := researchEligible(model.CurrencyUSD, 100_000_000); !ok {
		t.Fatal("$100M is above the USD floor")
	}
}

func TestNormalizeResearchHorizonFallsBackToDefault(t *testing.T) {
	if got := normalizeResearchHorizon(45); got != defaultResearchHorizonDays {
		t.Fatalf("unsupported horizon must fall back, got %d", got)
	}
	if got := normalizeResearchHorizon(365); got != 365 {
		t.Fatalf("365 is supported, got %d", got)
	}
}

func TestResearchProgressClamps(t *testing.T) {
	from := mustTime(t, "2026-01-01T00:00:00Z")
	to := from.AddDate(0, 0, 10)
	if got := researchProgress(from, to, from.AddDate(0, 0, -1)); got != 0 {
		t.Fatalf("before start clamps to 0, got %v", got)
	}
	if got := researchProgress(from, to, to.AddDate(0, 0, 5)); got != 1 {
		t.Fatalf("after end clamps to 1, got %v", got)
	}
	if got := researchProgress(from, to, from.AddDate(0, 0, 5)); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("halfway is 0.5, got %v", got)
	}
}
