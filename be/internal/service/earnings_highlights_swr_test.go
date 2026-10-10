package service

import (
	"testing"
	"time"
)

// 「신선한 시간」은 30분과 날짜 둘 다 걸린다 — 자정을 넘기면 창이 바뀌므로 30분이
// 안 지났어도 다시 만들어야 한다. 낡은 값을 주는 것은 호출자가 정한다.
func TestEarningsHighlightsEntryFreshness(t *testing.T) {
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		entry earningsHighlightsEntry
		want  bool
	}{
		{"just built", earningsHighlightsEntry{Day: "2026-09-25", BuiltAt: now.Add(-time.Minute)}, true},
		{"past ttl", earningsHighlightsEntry{Day: "2026-09-25", BuiltAt: now.Add(-earningsHighlightsTTL - time.Second)}, false},
		{"yesterday", earningsHighlightsEntry{Day: "2026-09-24", BuiltAt: now.Add(-time.Minute)}, false},
		{"zero", earningsHighlightsEntry{}, false},
	}
	for _, c := range cases {
		if got := c.entry.isFresh("2026-09-25", now); got != c.want {
			t.Errorf("%s: isFresh=%v want %v", c.name, got, c.want)
		}
	}
	if earningsHighlightsKeep <= earningsHighlightsTTL {
		t.Fatal("사는 시간이 신선한 시간보다 길어야 낡은 값을 줄 수 있다")
	}
}
