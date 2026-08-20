package calendar

import (
	"testing"
	"time"
)

func kst(y int, m time.Month, d, h, min int) time.Time {
	loc, _ := time.LoadLocation("Asia/Seoul")
	if loc == nil {
		loc = time.FixedZone("KST", 9*3600)
	}
	return time.Date(y, m, d, h, min, 0, 0, loc)
}

func TestSessionPhaseKRX(t *testing.T) {
	// 2026-08-14 is a Friday.
	cases := []struct {
		at   time.Time
		want Phase
	}{
		{kst(2026, 8, 14, 7, 0), PhasePreOpen},
		{kst(2026, 8, 14, 8, 59), PhasePreOpen},
		{kst(2026, 8, 14, 9, 0), PhaseOpen},
		{kst(2026, 8, 14, 12, 30), PhaseOpen},
		{kst(2026, 8, 14, 15, 30), PhaseOpen},
		{kst(2026, 8, 14, 16, 0), PhasePostOpen},
		{kst(2026, 8, 14, 23, 0), PhasePostOpen},
		// Saturday.
		{kst(2026, 8, 15, 11, 0), PhaseClosed},
	}
	for _, c := range cases {
		if got := SessionPhase(ScheduleKRX, c.at); got != c.want {
			t.Errorf("SessionPhase(KRX, %s) = %q, want %q", c.at.Format(time.RFC3339), got, c.want)
		}
	}
}

func TestInRegularSessionExcludesPreOpenLead(t *testing.T) {
	// shouldPollStatic starts 5분 일찍 열어 주지만, 브리핑 주기는 종을 기준으로
	// 삼는다.
	if InRegularSession(ScheduleKRX, kst(2026, 8, 14, 8, 57)) {
		t.Error("pre-open lead should not count as regular session")
	}
	if !InRegularSession(ScheduleKRX, kst(2026, 8, 14, 9, 1)) {
		t.Error("09:01 KST should be in session")
	}
}

func TestSessionPhaseAlwaysOpenCalendar(t *testing.T) {
	if got := SessionPhase(ScheduleAlways, kst(2026, 8, 15, 3, 0)); got != PhaseOpen {
		t.Errorf("SessionPhase(always) = %q, want %q", got, PhaseOpen)
	}
}
