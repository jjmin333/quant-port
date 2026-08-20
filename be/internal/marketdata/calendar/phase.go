package calendar

import "time"

// Phase is where a market is in its trading day, as a label meant for display
// and for prompt context rather than for trading decisions.
type Phase string

const (
	PhaseClosed   Phase = "휴장"   // weekend or holiday
	PhasePreOpen  Phase = "개장 전" // trading day, before the bell
	PhaseOpen     Phase = "장중"
	PhasePostOpen Phase = "장 마감" // trading day, after the close
)

// SessionPhase reports the market's phase at now. Calendars with no session of
// their own (crypto, "always") are always PhaseOpen — they never close.
func SessionPhase(schedule ScheduleID, now time.Time) Phase {
	return Default().sessionPhase(schedule, now)
}

// InRegularSession reports whether the regular session is running right now.
// The pre-open lead that shouldPollStatic allows is deliberately *not* counted
// here: a caller pacing work by session ("장중에는 자주, 그 밖에는 드물게") wants
// the bell, not the warm-up.
func InRegularSession(schedule ScheduleID, now time.Time) bool {
	return SessionPhase(schedule, now) == PhaseOpen
}

func (c *Calendar) sessionPhase(schedule ScheduleID, now time.Time) Phase {
	if c == nil {
		return PhaseOpen
	}
	s, ok := c.sessions[schedule]
	if !ok || s == nil || s.alwaysPoll {
		return PhaseOpen
	}

	local := now.In(s.loc)
	if s.weekdaysOnly {
		switch local.Weekday() {
		case time.Saturday, time.Sunday:
			return PhaseClosed
		}
	}
	if len(s.holidays) > 0 {
		if _, closed := s.holidays[local.Format("2006-01-02")]; closed {
			return PhaseClosed
		}
	}

	open := time.Date(local.Year(), local.Month(), local.Day(), s.openHour, s.openMinute, 0, 0, s.loc)
	closeAt := time.Date(local.Year(), local.Month(), local.Day(), s.closeHour, s.closeMinute, 59, 0, s.loc)

	switch {
	case local.Before(open):
		return PhasePreOpen
	case local.After(closeAt):
		return PhasePostOpen
	default:
		return PhaseOpen
	}
}
