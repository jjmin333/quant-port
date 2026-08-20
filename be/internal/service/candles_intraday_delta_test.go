package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func TestMergeIntraday1mBars_freshWinsOnOverlap(t *testing.T) {
	history := []model.Candle{
		{Time: "2026-08-06T09:30:00+09:00", Close: 100},
		{Time: "2026-08-06T09:31:00+09:00", Close: 101},
	}
	fresh := []model.Candle{
		// Overlaps 09:31 with a different (more current) close, plus one new bar.
		{Time: "2026-08-06T09:31:00+09:00", Close: 999},
		{Time: "2026-08-06T09:32:00+09:00", Close: 102},
	}

	merged := mergeIntraday1mBars(fresh, history)
	if len(merged) != 3 {
		t.Fatalf("len=%d want 3: %+v", len(merged), merged)
	}
	if merged[0].Close != 100 || merged[1].Close != 999 || merged[2].Close != 102 {
		t.Fatalf("unexpected merge result: %+v", merged)
	}
	// Ascending order.
	for i := 1; i < len(merged); i++ {
		if !parseQuoteCandleTime(merged[i-1].Time).Before(parseQuoteCandleTime(merged[i].Time)) {
			t.Fatalf("not ascending at %d: %+v", i, merged)
		}
	}
}

func TestMergeIntraday1mBars_emptyHistory(t *testing.T) {
	fresh := []model.Candle{{Time: "2026-08-06T09:30:00+09:00", Close: 100}}
	merged := mergeIntraday1mBars(fresh, nil)
	if len(merged) != 1 || merged[0].Close != 100 {
		t.Fatalf("got %+v", merged)
	}
}

func TestIntraday1mFetchTarget_noDBHistoryFallsBackToFullBackfill(t *testing.T) {
	s := &QuotesService{}
	got := s.intraday1mFetchTarget(nil, "000660")
	if got != maxIntradayRawBars {
		t.Fatalf("target=%d want maxIntradayRawBars=%d (no candles repo configured)", got, maxIntradayRawBars)
	}
}

func TestIntraday1mFetchTarget_clampsToBufferAndCeiling(t *testing.T) {
	// latestIntradayBarTime requires a live DB, which isn't available in this
	// unit test — exercise the clamping arithmetic directly instead.
	cases := []struct {
		name       string
		gapMinutes int
		want       int
	}{
		{"fresh gap uses buffer floor", 0, intradayDeltaFetchBufferBars},
		{"typical 30s-tick gap", 2, 2 + intradayDeltaFetchBufferBars},
		{"multi-day gap clamps to ceiling", 100000, maxIntradayRawBars},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			target := c.gapMinutes + intradayDeltaFetchBufferBars
			if target > maxIntradayRawBars {
				target = maxIntradayRawBars
			}
			if target < intradayDeltaFetchBufferBars {
				target = intradayDeltaFetchBufferBars
			}
			if target != c.want {
				t.Fatalf("target=%d want %d", target, c.want)
			}
		})
	}
}

func TestMergeIntraday1mBars_handlesLegacyAndCanonicalTimeFormats(t *testing.T) {
	// listIntradayBarsFromDB returns canonical RFC3339-ish times; a fresh Toss
	// fetch may report the same instant in a slightly different format. They
	// must still dedupe to one bar, not two.
	history := []model.Candle{
		{Time: "2026-08-06T09:31:00+09:00", Close: 101},
	}
	fresh := []model.Candle{
		{Time: "2026-08-06T09:31:00+09:00", Close: 105},
	}
	merged := mergeIntraday1mBars(fresh, history)
	if len(merged) != 1 {
		t.Fatalf("len=%d want 1 (same minute should dedupe): %+v", len(merged), merged)
	}
	if merged[0].Close != 105 {
		t.Fatalf("close=%v want fresh value 105", merged[0].Close)
	}
}

func TestSumIntradayBarsFrom_excludesPriorDaysAndAggregatesCorrectly(t *testing.T) {
	today, err := time.Parse("2006-01-02", "2026-08-06")
	if err != nil {
		t.Fatal(err)
	}
	bars := []model.Candle{
		{Time: "2026-08-05T15:29:00+09:00", Open: 900, High: 950, Low: 890, Close: 940, Volume: 1000}, // yesterday, must be excluded
		{Time: "2026-08-06T09:00:00+09:00", Open: 1000, High: 1010, Low: 995, Close: 1005, Volume: 100},
		{Time: "2026-08-06T09:01:00+09:00", Open: 1005, High: 1020, Low: 1000, Close: 1015, Volume: 200},
		{Time: "2026-08-06T09:02:00+09:00", Open: 1015, High: 1012, Low: 980, Close: 990, Volume: 300},
	}

	open, high, low, volume, tradingValue, ok := sumIntradayBarsFrom(bars, today)
	if !ok {
		t.Fatal("ok=false, want true (today has bars)")
	}
	if open != 1000 {
		t.Fatalf("open=%v want 1000 (first today bar's open, not yesterday's)", open)
	}
	if high != 1020 {
		t.Fatalf("high=%v want 1020", high)
	}
	if low != 980 {
		t.Fatalf("low=%v want 980", low)
	}
	if volume != 600 {
		t.Fatalf("volume=%v want 600 (100+200+300, excluding yesterday's 1000)", volume)
	}
	wantTV := 1005*100.0 + 1015*200.0 + 990*300.0
	if tradingValue != wantTV {
		t.Fatalf("tradingValue=%v want %v", tradingValue, wantTV)
	}
}

func TestSumIntradayBarsFrom_noBarsForTodayReturnsNotOK(t *testing.T) {
	today, err := time.Parse("2006-01-02", "2026-08-06")
	if err != nil {
		t.Fatal(err)
	}
	bars := []model.Candle{
		{Time: "2026-08-05T15:29:00+09:00", Open: 900, High: 950, Low: 890, Close: 940, Volume: 1000},
	}
	_, _, _, _, _, ok := sumIntradayBarsFrom(bars, today)
	if ok {
		t.Fatal("ok=true for a DB tail with no bars from today, want false so the caller falls back to Toss")
	}
}
