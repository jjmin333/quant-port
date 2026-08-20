package service

import (
	"context"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/toss"
)

var nyZone = func() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.FixedZone("EST", -5*3600)
	}
	return loc
}()

// neverRan is the "no run recorded yet" predicate for scheduling tests.
func neverRan(toss.MarketCountry, string) bool { return false }

func TestBatchWindowScalesWithPlannedWork(t *testing.T) {
	small := batchWindow(10)
	if want := 10*llmRefreshSpacing + batchStartMargin; small != want {
		t.Fatalf("batchWindow(10) = %s, want %s", small, want)
	}
	// A cap large enough to overrun the ceiling must clamp, or the KR run
	// would start in the middle of the night.
	if got := batchWindow(10_000); got != batchMaxWindow {
		t.Fatalf("batchWindow(10000) = %s, want clamp at %s", got, batchMaxWindow)
	}
}

func TestNextBatchWindowFinishesBeforeMarketOpen(t *testing.T) {
	// Monday 2026-08-10, 22:00 KST — after that day's KR deadline, and after
	// the US window for the NY day that was still in progress.
	now := time.Date(2026, 8, 10, 22, 0, 0, 0, kstZone)

	window, ok := nextBatchWindow(now, 100, neverRan)
	if !ok {
		t.Fatal("expected a window")
	}
	if window.schedule.market != toss.MarketKR {
		t.Fatalf("market = %s, want KR", window.schedule.market)
	}

	deadline := window.deadline.In(kstZone)
	wantDeadline := time.Date(2026, 8, 11, 8, 0, 0, 0, kstZone)
	if !deadline.Equal(wantDeadline) {
		t.Fatalf("deadline = %s, want %s", deadline, wantDeadline)
	}
	// The whole point: finished before the 09:00 KST bell.
	if deadline.Hour() >= 9 {
		t.Fatalf("KR deadline %s is not before the 09:00 open", deadline)
	}
	if !window.startAt.Before(window.deadline) {
		t.Fatalf("start %s must precede deadline %s", window.startAt, window.deadline)
	}
	if got := window.deadline.Sub(window.startAt); got != batchWindow(100) {
		t.Fatalf("window length = %s, want %s", got, batchWindow(100))
	}
}

func TestNextBatchWindowUSFinishesBeforeUSOpen(t *testing.T) {
	// Monday 2026-08-10, 10:00 KST. The KR deadline has passed, so the next
	// run is the US one preparing for that night's session — which in NY is
	// still Monday morning.
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, kstZone)

	window, ok := nextBatchWindow(now, 100, neverRan)
	if !ok {
		t.Fatal("expected a window")
	}
	if window.schedule.market != toss.MarketUS {
		t.Fatalf("market = %s, want US", window.schedule.market)
	}

	deadline := window.deadline.In(nyZone)
	wantDeadline := time.Date(2026, 8, 10, 8, 30, 0, 0, nyZone)
	if !deadline.Equal(wantDeadline) {
		t.Fatalf("deadline = %s, want %s", deadline, wantDeadline)
	}
	// 08:30 ET is an hour before the 09:30 ET open in either DST state,
	// which lands at or before 22:30 KST year-round.
	if kst := window.deadline.In(kstZone); kst.Hour() > 22 || (kst.Hour() == 22 && kst.Minute() > 30) {
		t.Fatalf("US deadline %s is later than 22:30 KST", kst)
	}
}

func TestNextBatchWindowSkipsHolidays(t *testing.T) {
	// 2026-08-15 (광복절) is a KRX holiday and falls on a Saturday, so the
	// next KR run is Monday the 17th.
	now := time.Date(2026, 8, 14, 9, 0, 0, 0, kstZone)

	var krWindow batchRunWindow
	found := false
	// Walk forward, skipping US windows, until the next KR run turns up.
	for cursor := now; cursor.Before(now.AddDate(0, 0, 6)); {
		window, ok := nextBatchWindow(cursor, 50, neverRan)
		if !ok {
			break
		}
		if window.schedule.market == toss.MarketKR {
			krWindow = window
			found = true
			break
		}
		cursor = window.deadline.Add(time.Minute)
	}
	if !found {
		t.Fatal("expected a KR window")
	}
	got := krWindow.deadline.In(kstZone)
	want := time.Date(2026, 8, 17, 8, 0, 0, 0, kstZone)
	if !got.Equal(want) {
		t.Fatalf("KR deadline = %s, want %s (weekend + 광복절 skipped)", got, want)
	}
	if !calendar.IsTradingDay(calendar.ScheduleKRX, got) {
		t.Fatalf("%s should be a KRX trading day", got)
	}
}

func TestNextBatchWindowSkipsAlreadyRecordedRun(t *testing.T) {
	// 06:00 KST Monday: today's KR window is still ahead...
	now := time.Date(2026, 8, 10, 6, 0, 0, 0, kstZone)

	window, ok := nextBatchWindow(now, 50, neverRan)
	if !ok || window.schedule.market != toss.MarketKR {
		t.Fatalf("expected today's KR window, got %+v (ok=%v)", window, ok)
	}

	// ...but once it is recorded, a restart must not spend the quota again.
	ranKRToday := func(market toss.MarketCountry, tradeDate string) bool {
		return market == toss.MarketKR && tradeDate == "2026-08-10"
	}
	next, ok := nextBatchWindow(now, 50, ranKRToday)
	if !ok {
		t.Fatal("expected a following window")
	}
	if next.schedule.market == toss.MarketKR && next.schedule.tradeDateOn(next.deadline) == "2026-08-10" {
		t.Fatal("already-recorded KR run was scheduled again")
	}
}

func TestNextBatchWindowCatchesUpMidWindow(t *testing.T) {
	// Process restarts at 07:50 KST, inside the KR window (07:00–08:00 for
	// a 50-symbol plan) with no run recorded: it should start immediately
	// rather than wait a day.
	now := time.Date(2026, 8, 10, 7, 50, 0, 0, kstZone)

	window, ok := nextBatchWindow(now, 50, neverRan)
	if !ok {
		t.Fatal("expected a window")
	}
	if window.schedule.market != toss.MarketKR {
		t.Fatalf("market = %s, want KR", window.schedule.market)
	}
	if !window.startAt.Equal(now) {
		t.Fatalf("startAt = %s, want immediate start at %s", window.startAt, now)
	}
}

func TestScheduleForLooksUpKnownMarkets(t *testing.T) {
	if _, ok := scheduleFor(toss.MarketKR); !ok {
		t.Fatal("expected a KR schedule")
	}
	if _, ok := scheduleFor(toss.MarketUS); !ok {
		t.Fatal("expected a US schedule")
	}
	if _, ok := scheduleFor(toss.MarketCountry("XX")); ok {
		t.Fatal("an unknown market must not resolve to a schedule")
	}
}

// TestBeginEndMarketRunGuardsConcurrency covers the guard RunBatchNow (the
// 배치 관리 manual trigger) relies on to avoid racing the scheduled batch for
// the same market on the same Redis run/history keys and budget check.
func TestBeginEndMarketRunGuardsConcurrency(t *testing.T) {
	s := &StockSummaryService{}
	if !s.beginMarketRun(toss.MarketKR) {
		t.Fatal("first claim for KR should succeed")
	}
	if s.beginMarketRun(toss.MarketKR) {
		t.Fatal("a second concurrent claim for KR must be rejected")
	}
	if !s.beginMarketRun(toss.MarketUS) {
		t.Fatal("a different market must not be blocked by KR's claim")
	}
	s.endMarketRun(toss.MarketKR)
	if !s.beginMarketRun(toss.MarketKR) {
		t.Fatal("claim should succeed again once the run has ended")
	}
}

// TestRunMarketBatchManualUsesLastCloseOnHoliday covers the 종가 기준 fallback:
// a manual trigger (window.asOf set, as RunBatchNow does when today is
// closed) must not hit the "휴장" guard that blocks the scheduled path. If the
// guard were still applied, this would fail with the holiday error instead of
// reaching (and failing at) the next check.
func TestRunMarketBatchManualUsesLastCloseOnHoliday(t *testing.T) {
	sched, ok := scheduleFor(toss.MarketKR)
	if !ok {
		t.Fatal("expected a KR schedule")
	}
	// 2026-08-15 (광복절) is a KRX holiday and falls on a Saturday.
	holiday := time.Date(2026, 8, 15, 10, 0, 0, 0, kstZone)
	if sched.isTradingDay(holiday) {
		t.Fatal("2026-08-15 should not be a KRX trading day (weekend + 광복절)")
	}

	window := batchRunWindow{
		schedule: sched,
		startAt:  holiday,
		deadline: holiday.Add(manualBatchWindow),
		asOf:     calendar.LastSessionCloseAt(sched.calendar, holiday),
	}
	if wantDate := "2026-08-14"; sched.tradeDateOn(window.asOf) != wantDate {
		t.Fatalf("asOf resolved to %s, want last trading day %s", sched.tradeDateOn(window.asOf), wantDate)
	}

	s := &StockSummaryService{}
	_, err := s.runMarketBatch(context.Background(), window)
	if err == nil {
		t.Fatal("expected an error (LLM disabled), but the holiday guard should not be the cause")
	}
	if got, want := err.Error(), "LLM이 비활성화되어 있습니다"; got != want {
		t.Fatalf("err = %q, want %q — the holiday guard likely fired instead of being bypassed", got, want)
	}
}

func TestRunBatchNowRejectsUnknownMarket(t *testing.T) {
	s := &StockSummaryService{}
	if _, _, err := s.RunBatchNow(context.Background(), toss.MarketCountry("XX")); err == nil {
		t.Fatal("expected an error for an unknown market")
	}
}

func TestNormalizeManualSymbols(t *testing.T) {
	got := normalizeManualSymbols([]string{" 005930 ", "005930", "aapl", "", "  "})
	want := []string{"005930", "AAPL"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	many := make([]string, 0, maxManualRefreshSymbols+5)
	for i := 0; i < maxManualRefreshSymbols+5; i++ {
		many = append(many, string(rune('A'+i))+"SYM")
	}
	if capped := normalizeManualSymbols(many); len(capped) != maxManualRefreshSymbols {
		t.Fatalf("expected cap at %d, got %d", maxManualRefreshSymbols, len(capped))
	}
}
