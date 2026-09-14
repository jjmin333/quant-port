package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestAggregateCandlesPeriod_monthUsesFirstOfMonth(t *testing.T) {
	bars := []model.Candle{
		{Time: "2026-07-02", Open: 1, High: 2, Low: 1, Close: 2, Volume: 10},
		{Time: "2026-07-15", Open: 2, High: 3, Low: 2, Close: 3, Volume: 20},
		{Time: "2026-07-28", Open: 3, High: 4, Low: 3, Close: 4, Volume: 30},
	}
	out := aggregateCandlesPeriod(bars, periodMonth)
	if len(out) != 1 {
		t.Fatalf("len=%d want 1 monthly bar", len(out))
	}
	if out[0].Time != "2026-07-01" {
		t.Fatalf("month time=%q want 2026-07-01", out[0].Time)
	}
	if out[0].Close != 4 || out[0].Volume != 60 {
		t.Fatalf("close=%v volume=%v", out[0].Close, out[0].Volume)
	}
}

func TestCandleFetchCount_scalesByInterval(t *testing.T) {
	if got := candleFetchCount(model.Interval5m); got != 600 {
		t.Fatalf("5m=%d want 600", got)
	}
	if got := candleFetchCount(model.Interval30m); got != maxIntradayRawBars {
		t.Fatalf("30m=%d want %d", got, maxIntradayRawBars)
	}
}

func TestCandlePageSize_returnsAggregatedBarCount(t *testing.T) {
	if got := candlePageSize(model.Interval5m); got != intradayDisplayBars {
		t.Fatalf("5m page=%d want %d", got, intradayDisplayBars)
	}
	if got := candlePageSize(model.Interval30m); got != intradayDisplayBars {
		t.Fatalf("30m page=%d want %d", got, intradayDisplayBars)
	}
}

func TestTrimCandlesTail(t *testing.T) {
	bars := []model.Candle{
		{Time: "a"}, {Time: "b"}, {Time: "c"}, {Time: "d"},
	}
	out := trimCandlesTail(bars, 2)
	if len(out) != 2 || out[0].Time != "c" || out[1].Time != "d" {
		t.Fatalf("trim=%v", out)
	}
}

func TestNeedsIntradayDepthRefresh_shallowSnapshot(t *testing.T) {
	snap := model.CandleSnapshot{
		Interval: model.Interval30m,
		Completed: []model.Candle{
			{Time: "2026-07-30T11:00:00+09:00"},
			{Time: "2026-07-31T18:00:00+09:00"},
		},
	}
	if !needsIntradayDepthRefresh(model.Interval30m, snap) {
		t.Fatal("expected shallow 30m snapshot to need refresh")
	}
}
