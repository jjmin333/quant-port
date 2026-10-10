package calendar

import "time"

// SessionCloseAt returns the regular session close for the given calendar day (UTC).
func SessionCloseAt(schedule ScheduleID, day time.Time) time.Time {
	return Default().sessionCloseAt(schedule, day)
}

// LastSessionCloseAt returns the most recent completed session close at or before now (UTC).
func LastSessionCloseAt(schedule ScheduleID, now time.Time) time.Time {
	return Default().lastSessionCloseAt(schedule, now)
}

// SessionOpenAt returns the regular session open for the given calendar day
// (UTC), whether or not the exchange trades that day.
func SessionOpenAt(schedule ScheduleID, day time.Time) time.Time {
	return Default().sessionOpenAt(schedule, day)
}

func (c *Calendar) sessionOpenAt(schedule ScheduleID, day time.Time) time.Time {
	s, ok := c.sessions[schedule]
	if !ok || s == nil {
		return day.UTC()
	}
	local := day.In(s.loc)
	return time.Date(
		local.Year(), local.Month(), local.Day(),
		s.openHour, s.openMinute, 0, 0, s.loc,
	).UTC()
}

func (c *Calendar) sessionCloseAt(schedule ScheduleID, day time.Time) time.Time {
	s, ok := c.sessions[schedule]
	if !ok || s == nil {
		return day.UTC()
	}
	local := day.In(s.loc)
	closeLocal := time.Date(
		local.Year(), local.Month(), local.Day(),
		s.closeHour, s.closeMinute, 0, 0, s.loc,
	)
	return closeLocal.UTC()
}

func (c *Calendar) lastSessionCloseAt(schedule ScheduleID, now time.Time) time.Time {
	s, ok := c.sessions[schedule]
	if !ok || s == nil || s.alwaysPoll {
		return now.UTC()
	}
	local := now.In(s.loc)
	for i := 0; i < 14; i++ {
		day := local.AddDate(0, 0, -i)
		if !c.isTradingDay(s, day) {
			continue
		}
		closeAt := c.sessionCloseAt(schedule, day)
		if i == 0 && local.Before(closeAt.In(s.loc)) {
			continue
		}
		return closeAt
	}
	return c.sessionCloseAt(schedule, local)
}

func (c *Calendar) isTradingDay(s *session, day time.Time) bool {
	local := day.In(s.loc)
	if s.weekdaysOnly {
		switch local.Weekday() {
		case time.Saturday, time.Sunday:
			return false
		}
	}
	if len(s.holidays) > 0 {
		if _, closed := s.holidays[local.Format("2006-01-02")]; closed {
			return false
		}
	}
	return true
}
