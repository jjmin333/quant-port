package calendar

import (
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

// ShouldPoll uses Toss session calendars when available, otherwise embedded holiday JSON.
func ShouldPoll(
	schedule ScheduleID,
	now time.Time,
	kr *toss.KrMarketCalendarResponse,
	us *toss.UsMarketCalendarResponse,
) bool {
	switch schedule {
	case ScheduleAlways:
		return true
	case ScheduleWeekdays:
		return weekdaysKST(now)
	case ScheduleKRX:
		if kr != nil {
			return krRegularPollWindow(kr, now)
		}
		return Default().shouldPollStatic(schedule, now)
	case ScheduleUS:
		if us != nil {
			return usRegularPollWindow(us, now)
		}
		return Default().shouldPollStatic(schedule, now)
	default:
		return true
	}
}

func weekdaysKST(now time.Time) bool {
	seoul, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		seoul = time.FixedZone("KST", 9*3600)
	}
	switch now.In(seoul).Weekday() {
	case time.Saturday, time.Sunday:
		return false
	default:
		return true
	}
}

func krRegularPollWindow(kr *toss.KrMarketCalendarResponse, now time.Time) bool {
	if kr == nil || kr.Today.Integrated == nil || kr.Today.Integrated.RegularMarket == nil {
		return false
	}
	return inRegularPollWindow(
		now,
		kr.Today.Integrated.RegularMarket.StartTime,
		kr.Today.Integrated.RegularMarket.EndTime,
	)
}

func usRegularPollWindow(us *toss.UsMarketCalendarResponse, now time.Time) bool {
	if us == nil || us.Today.RegularMarket == nil {
		return false
	}
	return inRegularPollWindow(
		now,
		us.Today.RegularMarket.StartTime,
		us.Today.RegularMarket.EndTime,
	)
}

func inRegularPollWindow(now time.Time, startRaw, endRaw string) bool {
	start, ok1 := parseMarketTime(startRaw)
	end, ok2 := parseMarketTime(endRaw)
	if !ok1 || !ok2 {
		return false
	}
	preOpen := start.Add(-preOpenLead)
	return !now.Before(preOpen) && !now.After(end)
}

func parseMarketTime(raw string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
