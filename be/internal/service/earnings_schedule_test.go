package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func atKST(year int, month time.Month, day, hour, min int) time.Time {
	return time.Date(year, month, day, hour, min, 0, 0, model.KST)
}

func TestIsDomesticEarningsSeason(t *testing.T) {
	in := []time.Month{1, 2, 4, 5, 7, 8, 10, 11}
	out := []time.Month{3, 6, 9, 12}
	for _, m := range in {
		if !IsDomesticEarningsSeason(m) {
			t.Errorf("month %d: want season", m)
		}
	}
	for _, m := range out {
		if IsDomesticEarningsSeason(m) {
			t.Errorf("month %d: want off-season", m)
		}
	}
}

func TestDomesticSlotsOn_SeasonEveryTwoHours(t *testing.T) {
	slots := DomesticSlotsOn(atKST(2026, 8, 19, 0, 0))
	if len(slots) != 6 {
		t.Fatalf("season slots = %d, want 6", len(slots))
	}
	want := []int{10, 12, 14, 16, 18, 20}
	for i, h := range want {
		if slots[i].Hour() != h || slots[i].Minute() != 0 {
			t.Errorf("slot %d = %s, want %02d:00", i, slots[i].Format("15:04"), h)
		}
	}
}

func TestDomesticSlotsOn_OffSeasonOnce(t *testing.T) {
	slots := DomesticSlotsOn(atKST(2026, 9, 3, 0, 0))
	if len(slots) != 1 || slots[0].Hour() != 18 || slots[0].Minute() != 30 {
		t.Fatalf("off-season slots = %v, want 18:30", slots)
	}
}

func TestNextDomesticSlot_SeasonMidDay(t *testing.T) {
	got := NextDomesticSlot(atKST(2026, 8, 19, 13, 10))
	want := atKST(2026, 8, 19, 14, 0)
	if !got.Equal(want) {
		t.Errorf("next = %s, want %s", got, want)
	}
}

func TestNextDomesticSlot_AfterLastGoesTomorrow(t *testing.T) {
	got := NextDomesticSlot(atKST(2026, 8, 19, 20, 0))
	// 20:00 정각은 이미 경과. 8월은 시즌이라 내일 10:00.
	want := atKST(2026, 8, 20, 10, 0)
	if !got.Equal(want) {
		t.Errorf("next = %s, want %s", got, want)
	}
}

func TestNextDomesticSlot_SeasonToOffSeason(t *testing.T) {
	got := NextDomesticSlot(atKST(2026, 8, 31, 20, 0))
	want := atKST(2026, 9, 1, 18, 30)
	if !got.Equal(want) {
		t.Errorf("Aug 31 after last → Sep 1 18:30, got %s", got)
	}
}

func TestLastElapsedDomesticSlot_RestartAfterEighteen(t *testing.T) {
	// 오늘 19:24 재시작 — 18:00은 지났고 20:00은 남음.
	got := LastElapsedDomesticSlot(atKST(2026, 8, 19, 19, 24))
	want := atKST(2026, 8, 19, 18, 0)
	if !got.Equal(want) {
		t.Errorf("elapsed = %s, want %s", got, want)
	}
}

func TestLastElapsedDomesticSlot_BeforeFirstLooksBack(t *testing.T) {
	got := LastElapsedDomesticSlot(atKST(2026, 8, 19, 9, 0))
	want := atKST(2026, 8, 18, 20, 0)
	if !got.Equal(want) {
		t.Errorf("elapsed = %s, want yesterday 20:00 got %s", got, want)
	}
}

func TestDomesticMonthsForSlot(t *testing.T) {
	if n := DomesticMonthsForSlot(atKST(2026, 8, 19, 14, 0)); n != 1 {
		t.Errorf("mid-day season months = %d, want 1", n)
	}
	if n := DomesticMonthsForSlot(atKST(2026, 8, 19, 20, 0)); n != 2 {
		t.Errorf("last season slot months = %d, want 2", n)
	}
	if n := DomesticMonthsForSlot(atKST(2026, 9, 3, 18, 30)); n != 2 {
		t.Errorf("off-season months = %d, want 2", n)
	}
}

func TestShouldCatchUp(t *testing.T) {
	elapsed := atKST(2026, 8, 19, 18, 0)
	if !ShouldCatchUp(time.Time{}, elapsed) {
		t.Error("never succeeded: want catch-up")
	}
	if !ShouldCatchUp(atKST(2026, 8, 18, 18, 30), elapsed) {
		t.Error("last success yesterday: want catch-up")
	}
	if ShouldCatchUp(atKST(2026, 8, 19, 18, 0).Add(30*time.Second), elapsed) {
		t.Error("already ran this slot: want skip")
	}
}

func TestOverseasSlots(t *testing.T) {
	now := atKST(2026, 8, 19, 11, 26)
	next := NextOverseasSlot(now)
	if !next.Equal(atKST(2026, 8, 20, 6, 0)) {
		t.Errorf("next overseas = %s, want tomorrow 06:00", next)
	}
	elapsed := LastElapsedOverseasSlot(now)
	if !elapsed.Equal(atKST(2026, 8, 19, 6, 0)) {
		t.Errorf("elapsed overseas = %s, want today 06:00", elapsed)
	}
	before := LastElapsedOverseasSlot(atKST(2026, 8, 19, 5, 0))
	if !before.Equal(atKST(2026, 8, 18, 6, 0)) {
		t.Errorf("before 06:00 elapsed = %s, want yesterday", before)
	}
}

func TestNextOverseasLiveSlot(t *testing.T) {
	got := NextOverseasLiveSlot(atKST(2026, 8, 19, 0, 0))
	if !got.Equal(atKST(2026, 8, 19, 0, 5)) {
		t.Errorf("on 00:00 next live = %s, want 00:05", got)
	}
	got = NextOverseasLiveSlot(atKST(2026, 8, 19, 12, 3))
	if !got.Equal(atKST(2026, 8, 19, 12, 5)) {
		t.Errorf("12:03 next live = %s, want 12:05", got)
	}
	got = NextOverseasLiveSlot(atKST(2026, 8, 19, 23, 57))
	if !got.Equal(atKST(2026, 8, 20, 0, 0)) {
		t.Errorf("23:57 next live = %s, want next midnight", got)
	}
}

func TestShouldSkipOverseasLive(t *testing.T) {
	if !ShouldSkipOverseasLive(atKST(2026, 8, 19, 6, 0), false) {
		t.Error("06:00 live: want skip even before full starts")
	}
	if !ShouldSkipOverseasLive(atKST(2026, 8, 19, 6, 9), false) {
		t.Error("06:09 live: want skip")
	}
	if ShouldSkipOverseasLive(atKST(2026, 8, 19, 6, 10), false) {
		t.Error("06:10 live: want run (flag covers leftover full ingest)")
	}
	if !ShouldSkipOverseasLive(atKST(2026, 8, 19, 14, 0), true) {
		t.Error("manual full at 14:00: want skip live")
	}
	if ShouldSkipOverseasLive(atKST(2026, 8, 19, 14, 0), false) {
		t.Error("14:00 idle: want live")
	}
}

func TestOverseasLiveWindow(t *testing.T) {
	from, to := OverseasLiveWindow(atKST(2026, 8, 20, 5, 5))
	if !from.Equal(atKST(2026, 8, 19, 0, 0)) || !to.Equal(atKST(2026, 8, 20, 0, 0)) {
		t.Errorf("live window = %s..%s, want Aug 19..20 00:00", from, to)
	}
}

func TestFinnhubCalendarChunkCount_Window(t *testing.T) {
	day := atKST(2026, 8, 20, 12, 0)
	got45 := FinnhubCalendarChunkCount(day.AddDate(0, 0, -45), day.AddDate(0, 0, 75))
	if got45 != 18 {
		t.Errorf("−45/+75 chunks = %d, want 18", got45)
	}
	got77 := FinnhubCalendarChunkCount(day.AddDate(0, 0, -49), day.AddDate(0, 0, 77))
	if got77 != 19 {
		t.Errorf("−49/+77 chunks = %d, want 19 (not the same as −45/+75)", got77)
	}
	got76 := FinnhubCalendarChunkCount(day.AddDate(0, 0, -OverseasFullBackDays), day.AddDate(0, 0, OverseasFullForwardDays))
	if got76 != 18 {
		t.Errorf("−49/+76 chunks = %d, want 18", got76)
	}
	from, to := OverseasLiveWindow(day)
	if n := FinnhubCalendarChunkCount(from, to); n != 1 {
		t.Errorf("live window chunks = %d, want 1", n)
	}
}
