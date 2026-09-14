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

/*
InRegularSessionWithLead는 정규장에 **개장 직전 구간까지 붙여서** 본다.

`InRegularSession`은 종을 기준으로만 답한다. 그건 시세 폴링에는 맞다 — 종이 울리기
전에는 값이 안 움직인다. 그런데 **뉴스는 종을 기다리지 않는다**: 국내 장 시작 전
한 시간(08:00~09:00 KST)에 전일 미국 증시 마감, 시간외 특징주, 개장 전 시황이
몰려 들어오고, 정작 브리핑은 그 시간대에 한 시간에 한 번만 돌아 09:00에 열어 본
화면이 07:00 기사로 만들어진 문장을 들고 있었다.

`lead`가 0 이하이면 `InRegularSession`과 같다. 휴장일에는 리드도 없다 — 열지 않는
날의 「개장 직전」은 없다.
*/
func InRegularSessionWithLead(schedule ScheduleID, now time.Time, lead time.Duration) bool {
	return Default().inRegularSessionWithLead(schedule, now, lead)
}

func (c *Calendar) inRegularSessionWithLead(schedule ScheduleID, now time.Time, lead time.Duration) bool {
	switch c.sessionPhase(schedule, now) {
	case PhaseOpen:
		return true
	case PhasePreOpen:
		if lead <= 0 {
			return false
		}
	default:
		// 휴장·장 마감에는 리드를 붙일 자리가 없다.
		return false
	}

	s, ok := c.sessions[schedule]
	if !ok || s == nil {
		return false
	}
	local := now.In(s.loc)
	open := time.Date(local.Year(), local.Month(), local.Day(), s.openHour, s.openMinute, 0, 0, s.loc)
	return !local.Before(open.Add(-lead))
}
