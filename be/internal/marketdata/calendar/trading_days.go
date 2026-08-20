package calendar

import "time"

// IsTradingDay reports whether `day` is a regular trading day for the schedule
// (weekends and the embedded holiday list excluded). Always-on schedules
// (crypto, gold) treat every day as a trading day.
func IsTradingDay(schedule ScheduleID, day time.Time) bool {
	c := Default()
	s, ok := c.sessions[schedule]
	if !ok || s == nil || s.alwaysPoll {
		return true
	}
	return c.isTradingDay(s, day)
}

// TradingDaysBetween lists the trading days in [from, to] inclusive, oldest
// first, using the schedule's own timezone for day boundaries — a UTC instant
// can fall on a different calendar day in KST or ET.
func TradingDaysBetween(schedule ScheduleID, from, to time.Time) []time.Time {
	loc := LocationFor(schedule)
	start := startOfDay(from.In(loc))
	end := startOfDay(to.In(loc))
	if end.Before(start) {
		return nil
	}

	var out []time.Time
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		if IsTradingDay(schedule, day) {
			out = append(out, day)
		}
	}
	return out
}

// LocationFor returns the schedule's timezone, defaulting to KST for schedules
// without one of their own.
func LocationFor(schedule ScheduleID) *time.Location {
	c := Default()
	if s, ok := c.sessions[schedule]; ok && s != nil && s.loc != nil {
		return s.loc
	}
	if s, ok := c.sessions[ScheduleKRX]; ok && s != nil && s.loc != nil {
		return s.loc
	}
	return time.UTC
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
