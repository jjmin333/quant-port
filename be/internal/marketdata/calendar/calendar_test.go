package calendar

import (
	"testing"
	"time"
)

func mustKST(t *testing.T, layout, value string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := time.ParseInLocation(layout, value, loc)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func mustNY(t *testing.T, layout, value string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := time.ParseInLocation(layout, value, loc)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestKRXClosedOnWeekend(t *testing.T) {
	sat := mustKST(t, "2006-01-02 15:04", "2026-07-25 10:00")
	if ShouldPoll(ScheduleKRX, sat, nil, nil) {
		t.Fatal("expected KRX closed on Saturday")
	}
}

func TestKRXPreOpenWindow(t *testing.T) {
	preOpen := mustKST(t, "2006-01-02 15:04", "2026-07-24 08:56")
	if !ShouldPoll(ScheduleKRX, preOpen, nil, nil) {
		t.Fatal("expected polling 4 minutes before KRX open")
	}
}

func TestKRXRegularSession(t *testing.T) {
	mid := mustKST(t, "2006-01-02 15:04", "2026-07-24 11:00")
	if !ShouldPoll(ScheduleKRX, mid, nil, nil) {
		t.Fatal("expected polling during KRX session")
	}
}

func TestKRXClosedAfterClose(t *testing.T) {
	after := mustKST(t, "2006-01-02 15:04", "2026-07-24 15:31")
	if ShouldPoll(ScheduleKRX, after, nil, nil) {
		t.Fatal("expected no polling after KRX close")
	}
}

func TestKRXHoliday(t *testing.T) {
	holiday := mustKST(t, "2006-01-02 15:04", "2026-01-01 10:00")
	if ShouldPoll(ScheduleKRX, holiday, nil, nil) {
		t.Fatal("expected KRX closed on holiday")
	}
}

func TestUSClosedOnWeekend(t *testing.T) {
	sat := mustNY(t, "2006-01-02 15:04", "2026-07-25 12:00")
	if ShouldPoll(ScheduleUS, sat, nil, nil) {
		t.Fatal("expected US equity closed on Saturday")
	}
}

func TestUSPreOpen(t *testing.T) {
	preOpen := mustNY(t, "2006-01-02 15:04", "2026-07-24 09:26")
	if !ShouldPoll(ScheduleUS, preOpen, nil, nil) {
		t.Fatal("expected polling before US cash open")
	}
}

func TestBTCAlwaysPolls(t *testing.T) {
	sat := mustKST(t, "2006-01-02 15:04", "2026-07-25 03:00")
	if !ShouldPoll(ScheduleAlways, sat, nil, nil) {
		t.Fatal("expected BTC schedule always active")
	}
}

// Regression: ScheduleAlways originally had no loc, so SessionCloseAt panicked
// with "missing Location in call to Time.In" the moment anything called it —
// which nothing did until worker-close-overseas started using session-close
// triggers for its indices. alwaysPoll must still resolve to a real close time
// (docs/DAILY_CLOSE_DESIGN.md §5.1: KST midnight), not a nil location.
func TestSessionCloseAtAlwaysDoesNotPanic(t *testing.T) {
	day := mustKST(t, "2006-01-02 15:04", "2026-07-25 00:00")
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("SessionCloseAt(ScheduleAlways) panicked: %v", r)
		}
	}()
	closeAt := SessionCloseAt(ScheduleAlways, day)
	if closeAt.IsZero() {
		t.Fatal("expected a non-zero close time for ScheduleAlways")
	}
}

func TestScheduleForIndex(t *testing.T) {
	if ScheduleForIndex("kospi") != ScheduleKRX {
		t.Fatal("kospi schedule")
	}
	if ScheduleForIndex("nasdaq") != ScheduleUS {
		t.Fatal("nasdaq schedule")
	}
	if ScheduleForIndex("btc") != ScheduleAlways {
		t.Fatal("btc schedule")
	}
}
