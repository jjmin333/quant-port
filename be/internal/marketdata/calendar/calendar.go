package calendar

import (
	"embed"
	"encoding/json"
	"sync"
	"time"
)

//go:embed holidays_krx.json holidays_us.json
var holidayFS embed.FS

const preOpenLead = 5 * time.Minute

// ScheduleID identifies a polling calendar for market indices.
type ScheduleID string

const (
	ScheduleKRX      ScheduleID = "krx"
	ScheduleUS       ScheduleID = "us_equity"
	ScheduleWeekdays ScheduleID = "weekdays_kst"
	ScheduleAlways   ScheduleID = "always"
)

type session struct {
	loc            *time.Location
	openHour       int
	openMinute     int
	closeHour      int
	closeMinute    int
	holidays       map[string]struct{}
	weekdaysOnly   bool
	alwaysPoll     bool
}

// Calendar decides whether an index provider should be polled at a given time.
type Calendar struct {
	sessions map[ScheduleID]*session
}

var (
	defaultCal     *Calendar
	defaultCalOnce sync.Once
)

func Default() *Calendar {
	defaultCalOnce.Do(func() {
		defaultCal = mustNewCalendar()
	})
	return defaultCal
}

func mustNewCalendar() *Calendar {
	seoul, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		seoul = time.FixedZone("KST", 9*3600)
	}
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		ny = time.FixedZone("EST", -5*3600)
	}

	krxHolidays := mustLoadHolidays("holidays_krx.json")
	usHolidays := mustLoadHolidays("holidays_us.json")

	return &Calendar{
		sessions: map[ScheduleID]*session{
			ScheduleKRX: {
				loc:          seoul,
				openHour:     9,
				openMinute:   0,
				closeHour:    15,
				closeMinute:  30,
				holidays:     krxHolidays,
				weekdaysOnly: true,
			},
			ScheduleUS: {
				loc:          ny,
				openHour:     9,
				openMinute:   30,
				closeHour:    16,
				closeMinute:  0,
				holidays:     usHolidays,
				weekdaysOnly: true,
			},
			ScheduleWeekdays: {
				loc:          seoul,
				openHour:     0,
				openMinute:   0,
				closeHour:    23,
				closeMinute:  59,
				weekdaysOnly: true,
			},
			ScheduleAlways: {
				alwaysPoll: true,
			},
		},
	}
}

func mustLoadHolidays(name string) map[string]struct{} {
	raw, err := holidayFS.ReadFile(name)
	if err != nil {
		return map[string]struct{}{}
	}
	var payload struct {
		Holidays []string `json:"holidays"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return map[string]struct{}{}
	}
	out := make(map[string]struct{}, len(payload.Holidays))
	for _, d := range payload.Holidays {
		out[d] = struct{}{}
	}
	return out
}

// shouldPollStatic reports whether to poll using embedded holiday JSON (fallback).
func (c *Calendar) shouldPollStatic(schedule ScheduleID, now time.Time) bool {
	if c == nil {
		return true
	}
	s, ok := c.sessions[schedule]
	if !ok || s == nil {
		return true
	}
	if s.alwaysPoll {
		return true
	}
	return s.inSession(now)
}

func (s *session) inSession(now time.Time) bool {
	local := now.In(s.loc)
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

	open := time.Date(local.Year(), local.Month(), local.Day(), s.openHour, s.openMinute, 0, 0, s.loc)
	close := time.Date(local.Year(), local.Month(), local.Day(), s.closeHour, s.closeMinute, 59, 0, s.loc)
	preOpen := open.Add(-preOpenLead)

	return !local.Before(preOpen) && !local.After(close)
}

// ScheduleForIndex maps a market index id to its polling schedule.
func ScheduleForIndex(indexID string) ScheduleID {
	switch indexID {
	case "kospi", "kosdaq":
		return ScheduleKRX
	case "usd-krw":
		return ScheduleWeekdays
	case "btc":
		return ScheduleAlways
	case "nasdaq", "nasdaq100", "nq100", "sp500", "dow", "vix", "sox", "gold":
		return ScheduleUS
	default:
		return ScheduleAlways
	}
}
