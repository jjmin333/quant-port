package service

import (
	"context"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
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
	if want := 10*llmBatchPacePerSymbol + batchStartMargin; small != want {
		t.Fatalf("batchWindow(10) = %s, want %s", small, want)
	}
	// 상한(350)만큼 계획해도 창이 그날 마감~기한 사이에 들어와야 한다. 넘치면
	// 매일 기한에서 잘린 꼬리가 이월된다.
	if got := batchWindow(model.DefaultLLMSettings().DailyCap); got > batchMaxWindow {
		t.Fatalf("batchWindow(DailyCap) = %s, exceeds the %s ceiling", got, batchMaxWindow)
	}
	// A cap large enough to overrun the ceiling must clamp, or the KR run
	// would start in the middle of the night.
	if got := batchWindow(10_000); got != batchMaxWindow {
		t.Fatalf("batchWindow(10000) = %s, want clamp at %s", got, batchMaxWindow)
	}
}

/*
국내 요약은 **그날 마감 뒤**에 돈다.

예전에는 다음 개장에서 거꾸로 세어 08:00에 돌렸는데, 이 배치가 읽는 값은 마지막으로
끝난 세션의 종가·지표뿐이라 마감 직후에 만들든 다음 날 아침에 만들든 내용이 같았다.
같은 값을 열여섯 시간 늦게 만들면서, 그동안 화면에는 **어제 종가 분석**이 떠 있었다.
*/
func TestNextBatchWindowRunsAfterKRClose(t *testing.T) {
	// Monday 2026-08-10, 12:00 KST — 장중. 다음은 그날 마감 뒤의 국내 창이다.
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, kstZone)

	window, ok := nextBatchWindow(now, 100, neverRan)
	if !ok {
		t.Fatal("expected a window")
	}
	if window.schedule.market != toss.MarketKR {
		t.Fatalf("market = %s, want KR", window.schedule.market)
	}

	deadline := window.deadline.In(kstZone)
	wantDeadline := time.Date(2026, 8, 10, 18, 30, 0, 0, kstZone)
	if !deadline.Equal(wantDeadline) {
		t.Fatalf("deadline = %s, want %s", deadline, wantDeadline)
	}
	// 요점: 그날 15:30 마감보다 뒤이고, 같은 날 안에서 끝난다.
	if deadline.Hour() < 16 {
		t.Fatalf("KR deadline %s must fall after the 15:30 close", deadline)
	}
	// 종가 적재(15:50~16:15)가 끝나기 전에는 시작하지 않는다.
	earliest := time.Date(2026, 8, 10, 16, 30, 0, 0, kstZone)
	if window.startAt.Before(earliest) {
		t.Fatalf("startAt = %s, must not precede %s (close ingest)", window.startAt.In(kstZone), earliest)
	}
	if !window.startAt.Before(window.deadline) {
		t.Fatalf("start %s must precede deadline %s", window.startAt, window.deadline)
	}
}

// 해외도 같다 — 16:00 ET 마감 뒤, 적재(≈17:37 ET)가 끝난 다음이다.
func TestNextBatchWindowRunsAfterUSClose(t *testing.T) {
	// Monday 2026-08-10, 19:00 KST. 그날 국내 창(마감 18:30)은 지났으므로 다음은
	// 뉴욕 월요일 장이 끝난 뒤의 해외 창이다.
	now := time.Date(2026, 8, 10, 19, 0, 0, 0, kstZone)

	window, ok := nextBatchWindow(now, 100, neverRan)
	if !ok {
		t.Fatal("expected a window")
	}
	if window.schedule.market != toss.MarketUS {
		t.Fatalf("market = %s, want US", window.schedule.market)
	}

	deadline := window.deadline.In(nyZone)
	wantDeadline := time.Date(2026, 8, 10, 20, 0, 0, 0, nyZone)
	if !deadline.Equal(wantDeadline) {
		t.Fatalf("deadline = %s, want %s", deadline, wantDeadline)
	}
	if deadline.Hour() < 16 {
		t.Fatalf("US deadline %s must fall after the 16:00 ET close", deadline)
	}
	earliest := time.Date(2026, 8, 10, 18, 0, 0, 0, nyZone)
	if window.startAt.In(nyZone).Before(earliest) {
		t.Fatalf("startAt = %s, must not precede %s (close ingest)", window.startAt.In(nyZone), earliest)
	}
}

/*
큰 DailyCap이 창을 종가 적재 앞으로 밀지 못한다.

기한에서 계획한 일감만큼 거꾸로 빼는 구조라, 상한(`batchMaxWindow`, 2시간)까지 당겨지면
국내는 16:30 이전이 된다. 그 시각에는 그날 종가가 아직 표에 없어 **어제 값으로 오늘
분석을 만든다** — `earliestOn`이 막는 것이 정확히 이 경우다.
*/
func TestNextBatchWindowNeverStartsBeforeCloseIngest(t *testing.T) {
	now := time.Date(2026, 8, 10, 0, 30, 0, 0, kstZone)

	window, ok := nextBatchWindow(now, 100_000, neverRan)
	if !ok {
		t.Fatal("expected a window")
	}
	if window.schedule.market != toss.MarketKR {
		t.Fatalf("market = %s, want KR", window.schedule.market)
	}
	earliest := time.Date(2026, 8, 10, 16, 30, 0, 0, kstZone)
	if window.startAt.In(kstZone).Before(earliest) {
		t.Fatalf("startAt = %s, want not before %s even with a huge cap",
			window.startAt.In(kstZone), earliest)
	}
}

func TestNextBatchWindowSkipsHolidays(t *testing.T) {
	// 2026-08-15 (광복절) is a KRX holiday and falls on a Saturday, so the
	// next KR run is Monday the 17th. 19:00 금요일 — 그날 창(마감 18:30)은 이미 지났다.
	now := time.Date(2026, 8, 14, 19, 0, 0, 0, kstZone)

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
	want := time.Date(2026, 8, 18, 18, 30, 0, 0, kstZone) // 08-17 = 광복절 대체공휴일
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
	// Process restarts at 18:20 KST, inside the KR window (계획 50건이면 18:15~18:30)
	// with no run recorded: it should start immediately rather than wait a day.
	now := time.Date(2026, 8, 10, 18, 20, 0, 0, kstZone)

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
