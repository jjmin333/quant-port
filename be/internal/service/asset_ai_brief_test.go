package service

import "testing"

func TestBriefMood(t *testing.T) {
	for _, c := range []struct {
		w4   float64
		want string
	}{{0.31, "상승"}, {0.05, "상승"}, {0.04, "보합"}, {-0.04, "보합"}, {-0.2, "하락"}} {
		if got := BriefMoodRE(map[string]any{"w4Pct": c.w4}); got != c.want {
			t.Errorf("re w4=%v: got %s want %s", c.w4, got, c.want)
		}
	}
	rows := [][]any{{"한국은행 기준금리", 3.0, 3.0, 3.0}, {"국고채 3년", 3.94, 4.01, 3.70}}
	if got := BriefMoodRates(rows); got != "상승" {
		t.Errorf("rates: got %s want 상승", got)
	}
	rows[1] = []any{"국고채 3년", 3.94, 4.01, 3.90}
	if got := BriefMoodRates(rows); got != "보합" {
		t.Errorf("rates small move: got %s want 보합", got)
	}
	if got := BriefMoodRates(nil); got != "보합" {
		t.Errorf("rates empty: got %s", got)
	}
}
