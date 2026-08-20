package service

import (
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/toss"
)

// When each market's AI 요약 batch runs.
//
// The run is scheduled backwards from the market's open rather than forwards
// from its close: what matters is that a user opening the app before the bell
// already sees an analysis of the last completed session. So each market gets
// a *deadline*, and the start time is derived from how long the planned work
// should take.
//
// KR closes 15:30 KST and opens 09:00 KST; the batch must be done by 08:00.
// The US regular session opens 22:30 KST under daylight saving and 23:30 KST
// otherwise — 22:30 is the earlier of the two, so using it as the deadline is
// correct year-round rather than only half the year.

type batchSchedule struct {
	market   toss.MarketCountry
	calendar calendar.ScheduleID
	// deadline is a wall-clock time in the market's own timezone: KST for
	// KRX, ET for the US session. Expressing it locally is what makes the
	// US window track daylight saving automatically.
	deadlineHour   int
	deadlineMinute int
}

var batchSchedules = []batchSchedule{
	// 09:00 KST open — done a full hour before.
	{market: toss.MarketKR, calendar: calendar.ScheduleKRX, deadlineHour: 8, deadlineMinute: 0},
	// 09:30 ET open — done an hour before, which lands at 22:30 KST in
	// summer and 23:30 KST in winter, both ahead of that day's bell.
	{market: toss.MarketUS, calendar: calendar.ScheduleUS, deadlineHour: 8, deadlineMinute: 30},
}

const (
	// batchStartMargin is slack between the planned finish and the deadline,
	// absorbing slow LLM responses and per-symbol data assembly.
	batchStartMargin = 10 * time.Minute
	// batchMaxWindow caps how early a run may begin, so a large DailyCap
	// cannot push the KR run back into the middle of the night where a
	// failure would go unnoticed until the open.
	batchMaxWindow = 2 * time.Hour
)

// batchWindow is how long before its deadline a run should start, given how
// many symbols it plans to process.
func batchWindow(planned int) time.Duration {
	if planned < 1 {
		planned = 1
	}
	window := time.Duration(planned)*llmRefreshSpacing + batchStartMargin
	if window > batchMaxWindow {
		return batchMaxWindow
	}
	return window
}

// deadlineOn returns the schedule's cutoff for the market-local day
// containing `day`.
func (b batchSchedule) deadlineOn(day time.Time) time.Time {
	loc := calendar.LocationFor(b.calendar)
	local := day.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), b.deadlineHour, b.deadlineMinute, 0, 0, loc)
}

// tradeDateOn is the market-local calendar date a run belongs to — the key
// under which its record is stored, and the session it is preparing for.
func (b batchSchedule) tradeDateOn(day time.Time) string {
	return day.In(calendar.LocationFor(b.calendar)).Format("2006-01-02")
}

// isTradingDay reports whether the market trades on the day containing `at`.
// Holidays and weekends are read from the embedded exchange calendars, so a
// closed session simply produces no run — nothing is spent on a day whose
// prices will not have moved.
func (b batchSchedule) isTradingDay(at time.Time) bool {
	return calendar.IsTradingDay(b.calendar, at)
}

// batchRun is one scheduled occurrence: when to start, when to be done, and
// which market it covers.
type batchRunWindow struct {
	schedule batchSchedule
	startAt  time.Time
	deadline time.Time
	// asOf pins the run to a specific calendar day instead of deriving it
	// from wall-clock time at execution. Zero means "use time.Now()", which
	// is what the scheduled path wants (see runMarketBatch's midnight-crossed
	// safety check). A manual trigger fired on a closed session sets this to
	// the last completed close so the run still has a trading day to attach
	// its results to.
	asOf time.Time
}

// nextBatchWindow returns the earliest window to act on, scanning today and
// the following days until a trading day is found for some market. A window
// whose start has already passed but whose deadline has not is returned with
// startAt == now: that is the restart-catch-up case, where the process came
// up mid-window and the day's run has not happened yet.
//
// alreadyRan reports whether a market's run for a given trade date is already
// recorded, so a restart cannot spend the quota twice.
func nextBatchWindow(
	now time.Time,
	planned int,
	alreadyRan func(market toss.MarketCountry, tradeDate string) bool,
) (batchRunWindow, bool) {
	window := batchWindow(planned)

	var best batchRunWindow
	found := false
	// Seven days is enough to clear the longest exchange holiday run
	// (Korea's 추석/설 stretches) without looping unbounded.
	for dayOffset := 0; dayOffset <= 7 && !found; dayOffset++ {
		day := now.AddDate(0, 0, dayOffset)
		for _, sched := range batchSchedules {
			if !sched.isTradingDay(day) {
				continue
			}
			deadline := sched.deadlineOn(day)
			if !deadline.After(now) {
				continue
			}
			if alreadyRan != nil && alreadyRan(sched.market, sched.tradeDateOn(day)) {
				continue
			}
			startAt := deadline.Add(-window)
			if startAt.Before(now) {
				startAt = now
			}
			candidate := batchRunWindow{schedule: sched, startAt: startAt, deadline: deadline}
			if !found || candidate.startAt.Before(best.startAt) {
				best = candidate
				found = true
			}
		}
	}
	return best, found
}
