package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func makeMinuteSeries(start string, count int) []model.Candle {
	t0 := parseQuoteCandleTime(start)
	out := make([]model.Candle, 0, count)
	for i := 0; i < count; i++ {
		ts := t0.Add(time.Duration(i) * time.Minute)
		out = append(out, model.Candle{
			Time:   formatCandleTime(ts, model.Interval1m),
			Open:   float64(100 + i),
			High:   float64(101 + i),
			Low:    float64(99 + i),
			Close:  float64(100 + i),
			Volume: int64(i + 1),
		})
	}
	return out
}

func TestPaginateAggregatedFromRaw_5m_continuousBuckets(t *testing.T) {
	// 25 consecutive 1m bars → 5 complete 5m buckets.
	stored := makeMinuteSeries("2026-03-25T09:00:00+09:00", 25)
	rawNext := "2026-03-25T09:00:00+09:00"

	page := paginateAggregatedFromRaw(stored, &rawNext, model.Interval5m, model.Interval1m, 3)
	if len(page.Candles) != 3 {
		t.Fatalf("len=%d want 3", len(page.Candles))
	}
	if page.Candles[0].Time != "2026-03-25T09:10:00+09:00" {
		t.Fatalf("first bucket=%q", page.Candles[0].Time)
	}
	if page.Candles[2].Time != "2026-03-25T09:20:00+09:00" {
		t.Fatalf("last bucket=%q", page.Candles[2].Time)
	}
	if page.NextBefore == nil || *page.NextBefore != "2026-03-25T09:00:00+09:00" {
		t.Fatalf("nextBefore=%v", page.NextBefore)
	}
}

func TestPaginateAggregatedFromRaw_noMoreHistory(t *testing.T) {
	stored := makeMinuteSeries("2026-03-25T09:00:00+09:00", 10)
	page := paginateAggregatedFromRaw(stored, nil, model.Interval5m, model.Interval1m, 5)
	if page.NextBefore != nil {
		t.Fatalf("expected nil nextBefore, got %v", page.NextBefore)
	}
}

func TestPaginateAggregatedFromRaw_1m_passthrough(t *testing.T) {
	stored := makeMinuteSeries("2026-03-25T09:00:00+09:00", 5)
	rawNext := "2026-03-25T09:00:00+09:00"
	page := paginateAggregatedFromRaw(stored, &rawNext, model.Interval1m, model.Interval1m, 3)
	if len(page.Candles) != 3 {
		t.Fatalf("len=%d want 3", len(page.Candles))
	}
	if page.Candles[0].Time != "2026-03-25T09:02:00+09:00" {
		t.Fatalf("first=%q", page.Candles[0].Time)
	}
}

func TestRawFetchLimitForInterval_includesBucketPadding(t *testing.T) {
	if got := rawFetchLimitForInterval(model.Interval5m, 10); got != 55 {
		t.Fatalf("5m limit=%d want 55", got)
	}
	if got := rawFetchLimitForInterval(model.Interval1w, 4); got != 35 {
		t.Fatalf("1w limit=%d want 35", got)
	}
}

func TestPaginateAggregatedFromRaw_multiPageContinuity(t *testing.T) {
	all := makeMinuteSeries("2026-03-25T09:00:00+09:00", 60)
	pageSize := 5

	firstRaw := all[len(all)-25:]
	firstNext := all[len(all)-26].Time
	page1 := paginateAggregatedFromRaw(firstRaw, &firstNext, model.Interval5m, model.Interval1m, pageSize)

	secondRaw := all[:len(all)-25]
	secondNext := secondRaw[0].Time
	page2 := paginateAggregatedFromRaw(secondRaw, &secondNext, model.Interval5m, model.Interval1m, pageSize)

	merged := append(page2.Candles, page1.Candles...)
	full := aggregateCandles(all, model.Interval5m)
	if len(full) < len(merged) {
		t.Fatalf("full agg too short: %d vs %d", len(full), len(merged))
	}
	tail := full[len(full)-len(merged):]
	for i := range merged {
		if merged[i].Time != tail[i].Time {
			t.Fatalf("gap at %d: merged=%q full=%q", i, merged[i].Time, tail[i].Time)
		}
	}
}
