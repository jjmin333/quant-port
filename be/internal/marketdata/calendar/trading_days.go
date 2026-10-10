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
