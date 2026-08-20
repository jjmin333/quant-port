package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// The Toss daily tail arrives newest-first, so the in-progress bar sits at
// index 0 rather than at the end.
func TestSplitFormingCandle_dailyDescendingPage(t *testing.T) {
	today := time.Now().In(kst).Format("2006-01-02")
	yesterday := time.Now().In(kst).AddDate(0, 0, -1).Format("2006-01-02")
	older := time.Now().In(kst).AddDate(0, 0, -2).Format("2006-01-02")

	bars := []model.Candle{
		{Time: today, Open: 1168000, High: 1614000, Low: 1168000, Close: 1507000},
		{Time: yesterday, Open: 1652000, High: 1701000, Low: 1598000, Close: 1616000},
		{Time: older, Open: 1580000, High: 1630000, Low: 1483000, Close: 1609000},
	}

	completed, forming := splitFormingCandle(bars, model.Interval1d)
	if forming == nil {
		t.Fatal("forming=nil, want today's in-progress bar")
	}
	if forming.Time != today {
		t.Fatalf("forming time=%q want %q", forming.Time, today)
	}
	if len(completed) != 2 {
		t.Fatalf("completed len=%d want 2", len(completed))
	}
	for _, c := range completed {
		if c.Time == today {
			t.Fatalf("today's bar leaked into completed history")
		}
	}
}

func TestSplitFormingCandle_intradayAscendingPage(t *testing.T) {
	now := time.Now().In(kst)
	bars := []model.Candle{
		{Time: model.FormatCandleTime(now.Add(-2*time.Minute), model.Interval1m), Close: 100},
		{Time: model.FormatCandleTime(now.Add(-time.Minute), model.Interval1m), Close: 101},
		{Time: model.FormatCandleTime(now, model.Interval1m), Close: 102},
	}

	completed, forming := splitFormingCandle(bars, model.Interval1m)
	if forming == nil || forming.Close != 102 {
		t.Fatalf("forming=%+v want the current-minute bar", forming)
	}
	if len(completed) != 2 {
		t.Fatalf("completed len=%d want 2", len(completed))
	}
}

// A page whose newest bar predates the current bucket has nothing forming.
func TestSplitFormingCandle_noBarInCurrentBucket(t *testing.T) {
	bars := []model.Candle{
		{Time: "2025-10-17", Close: 454500},
		{Time: "2025-10-16", Close: 455000},
	}

	completed, forming := splitFormingCandle(bars, model.Interval1d)
	if forming != nil {
		t.Fatalf("forming=%+v want nil", forming)
	}
	if len(completed) != 2 {
		t.Fatalf("completed len=%d want 2", len(completed))
	}
}

func TestLastCompletedTime_ignoresPageOrder(t *testing.T) {
	descending := []model.Candle{{Time: "2026-08-05"}, {Time: "2026-08-04"}, {Time: "2026-08-03"}}
	if got := lastCompletedTime(descending); got != "2026-08-05" {
		t.Fatalf("descending=%q want 2026-08-05", got)
	}
	ascending := []model.Candle{{Time: "2026-08-03"}, {Time: "2026-08-04"}, {Time: "2026-08-05"}}
	if got := lastCompletedTime(ascending); got != "2026-08-05" {
		t.Fatalf("ascending=%q want 2026-08-05", got)
	}
	if got := lastCompletedTime(nil); got != "" {
		t.Fatalf("empty=%q want \"\"", got)
	}
}
