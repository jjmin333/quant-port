package service

import (
	"testing"
	"time"
)

func TestNextNXTRefreshSkipsWeekendAndPastSlot(t *testing.T) {
	kst := time.FixedZone("KST", 9*3600)
	cases := []struct{ now, want string }{
		// 수요일 19:00 → 그날 20:10
		{"2026-10-21T19:00:00", "2026-10-21T20:10:00"},
		// 수요일 20:10 정각은 지난 것으로 본다 → 목요일
		{"2026-10-21T20:10:00", "2026-10-22T20:10:00"},
		// 금요일 21:00 → 다음 월요일(10-26)
		{"2026-10-23T21:00:00", "2026-10-26T20:10:00"},
	}
	for _, c := range cases {
		now, _ := time.ParseInLocation("2006-01-02T15:04:05", c.now, kst)
		want, _ := time.ParseInLocation("2006-01-02T15:04:05", c.want, kst)
		if got := nextNXTRefresh(now); !got.Equal(want) {
			t.Errorf("nextNXTRefresh(%s) = %s, want %s", c.now, got.In(kst), c.want)
		}
	}
}
