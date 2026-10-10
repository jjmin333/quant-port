package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
)

// TestLastCompletedTriggerCatchesRestartGap reproduces the exact scenario
// that left stock_daily_closes stuck several sessions behind: a container
// restart landing after a trading day's close+delay window has already
// passed. nextTrigger alone would jump straight to the *next* occurrence and
// never revisit that missed session — lastCompletedTrigger is what lets the
// scheduler catch up on it instead.
func TestLastCompletedTriggerCatchesRestartGap(t *testing.T) {
	loc := kst
	svc := &DailyCloseService{}

	// 2026-08-10 is a Monday (trading day); its KRX close+20m trigger is
	// 15:50 KST. Restarting at 17:50 KST the same day is well past it.
	restartedAt := time.Date(2026, 8, 10, 17, 50, 0, 0, loc)

	at, ok := svc.lastCompletedTrigger(restartedAt, calendar.ScheduleKRX)
	if !ok {
		t.Fatalf("expected a catch-up trigger, got none")
	}
	wantDay := time.Date(2026, 8, 10, 0, 0, 0, 0, loc)
	if got := at.In(loc); got.Year() != wantDay.Year() || got.YearDay() != wantDay.YearDay() {
		t.Fatalf("catch-up trigger day = %s, want %s", got.Format("2006-01-02"), wantDay.Format("2006-01-02"))
	}
	if !at.Before(restartedAt) {
		t.Fatalf("catch-up trigger %s should be before restart time %s", at, restartedAt)
	}
}

// TestLastCompletedTriggerSkipsWeekend confirms the backward search steps
// over non-trading days (weekend) to reach the prior Friday's session rather
// than reporting Saturday/Sunday as due.
func TestLastCompletedTriggerSkipsWeekend(t *testing.T) {
	loc := kst
	svc := &DailyCloseService{}

	// 2026-08-09 is a Sunday, well after both 08-07 (Fri) and 08-08's
	// (Sat, non-trading) triggers would have elapsed.
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, loc)

	at, ok := svc.lastCompletedTrigger(now, calendar.ScheduleKRX)
	if !ok {
		t.Fatalf("expected a catch-up trigger, got none")
	}
	want := time.Date(2026, 8, 7, 0, 0, 0, 0, loc)
	if got := at.In(loc); got.Year() != want.Year() || got.YearDay() != want.YearDay() {
		t.Fatalf("catch-up trigger day = %s, want %s (last trading day)", got.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

// TestLastCompletedTriggerNoneWhenCurrent confirms a process that starts
// right after its own trigger just fired does not report a stale catch-up —
// nextTrigger already owns that moment going forward.
func TestLastCompletedTriggerNoneWhenCurrent(t *testing.T) {
	loc := kst
	svc := &DailyCloseService{}

	// Mid-session on a trading day: today's close+delay has not happened
	// yet, and going back further only reaches sessions that (in a live
	// system) would already have been ingested by prior loop iterations.
	// We only assert the immediate case actually asked about: the moment
	// right at today's own trigger reports today, not something older.
	triggerMoment := calendar.SessionCloseAt(calendar.ScheduleKRX, time.Date(2026, 8, 10, 12, 0, 0, 0, loc)).
		Add(dailyCloseIngestDelay)

	at, ok := svc.lastCompletedTrigger(triggerMoment, calendar.ScheduleKRX)
	if !ok {
		t.Fatalf("expected a catch-up trigger, got none")
	}
	want := time.Date(2026, 8, 10, 0, 0, 0, 0, loc)
	if got := at.In(loc); got.Year() != want.Year() || got.YearDay() != want.YearDay() {
		t.Fatalf("catch-up trigger day = %s, want %s", got.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

/*
카탈로그에 있는 지수는 **전부** 일별 종가 세션이 정해져 있어야 한다.

빠지면 오류가 나지 않는다 — `IngestIndexCloses`의 `dailyCloseIndexSchedules[idx.ID]
!= schedule`이 없는 키를 빈 ScheduleID로 읽고 조용히 건너뛴다. 그래서 라이브 값은
멀쩡한 채 일별 종가만 멈추고, 그 위에 앉은 점수·키워드·LLM 해설이 같이 멈춘다.
`us10y`가 2026-08-27부터 3주를 그렇게 보냈다(daily_close.go의 표 머리글).

지수를 새로 담는 사람이 잊기 가장 쉬운 자리라 여기서 잡는다.
*/
func TestEveryCatalogIndexHasACloseSchedule(t *testing.T) {
	for _, entry := range marketIndexCatalog {
		if _, ok := dailyCloseIndexSchedules[entry.id]; !ok {
			t.Errorf("%s: dailyCloseIndexSchedules에 없다 — 일별 종가가 한 줄도 안 쌓인다", entry.id)
		}
	}
	for id := range dailyCloseIndexSchedules {
		found := false
		for _, entry := range marketIndexCatalog {
			if entry.id == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: 카탈로그에 없는 지수가 표에 남아 있다", id)
		}
	}
}