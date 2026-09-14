package service

import (
	"context"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

func TestRankingsPollIntervalFor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		duration toss.RankingDuration
		want     time.Duration
	}{
		{toss.RankingRealtime, 5 * time.Minute},
		{toss.Ranking1d, 15 * time.Minute},
		{toss.Ranking1w, 15 * time.Minute},
		{toss.Ranking1mo, time.Hour},
		{toss.Ranking1y, time.Hour},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.duration), func(t *testing.T) {
			t.Parallel()
			if got := defaultRankingsPollIntervalFor(tc.duration); got != tc.want {
				t.Fatalf("interval = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAlignedRankingBucket(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		now      time.Time
		interval time.Duration
		want     time.Time
	}{
		{
			name:     "floors to 5m mark",
			now:      time.Date(2026, 8, 6, 14, 7, 23, 0, time.UTC),
			interval: 5 * time.Minute,
			want:     time.Date(2026, 8, 6, 14, 5, 0, 0, time.UTC),
		},
		{
			name:     "already on the mark stays put",
			now:      time.Date(2026, 8, 6, 14, 5, 0, 0, time.UTC),
			interval: 5 * time.Minute,
			want:     time.Date(2026, 8, 6, 14, 5, 0, 0, time.UTC),
		},
		{
			name: "KST wall clock also lands on a clean 5m mark",
			// 23:07 KST = 14:07 UTC -> floors to 14:05 UTC = 23:05 KST.
			now:      time.Date(2026, 8, 6, 23, 7, 0, 0, time.FixedZone("KST", 9*3600)),
			interval: 5 * time.Minute,
			want:     time.Date(2026, 8, 6, 14, 5, 0, 0, time.UTC),
		},
		{
			name:     "1h interval floors to the hour",
			now:      time.Date(2026, 8, 6, 14, 47, 0, 0, time.UTC),
			interval: time.Hour,
			want:     time.Date(2026, 8, 6, 14, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := alignedRankingBucket(tc.now, tc.interval)
			if !got.Equal(tc.want) {
				t.Fatalf("bucket = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDueRankingsPollParams_bucketBoundary(t *testing.T) {
	t.Parallel()

	svc := &MarketService{}
	target := toss.GetRankingsParams{
		Type:          toss.RankingMarketTradingAmount,
		MarketCountry: toss.MarketKR,
		Duration:      toss.RankingRealtime,
	}
	targetKey := rankingsPollSlotKey(target)

	isDue := func(now time.Time, lastFetched map[string]time.Time) bool {
		for _, p := range svc.dueRankingsPollParams(context.Background(), now, lastFetched) {
			if rankingsPollSlotKey(p) == targetKey {
				return true
			}
		}
		return false
	}

	// Fetched at 14:02 -> recorded bucket is 14:00 (via pollRankingsBatch's
	// own bookkeeping, reproduced here directly).
	lastFetched := map[string]time.Time{
		targetKey: alignedRankingBucket(time.Date(2026, 8, 6, 14, 2, 0, 0, time.UTC), 5*time.Minute),
	}

	stillWithinBucket := time.Date(2026, 8, 6, 14, 4, 30, 0, time.UTC)
	if isDue(stillWithinBucket, lastFetched) {
		t.Fatal("due=true within the same 5m bucket, want false")
	}

	crossedBoundary := time.Date(2026, 8, 6, 14, 5, 10, 0, time.UTC)
	if !isDue(crossedBoundary, lastFetched) {
		t.Fatal("due=false after crossing the 5m boundary, want true")
	}
}

func TestIsSupportedRankingPollParams(t *testing.T) {
	t.Parallel()

	ok := isSupportedRankingPollParams(toss.GetRankingsParams{
		Type:          toss.RankingMarketTradingAmount,
		MarketCountry: toss.MarketKR,
		Duration:      toss.Ranking1mo,
	})
	if !ok {
		t.Fatal("expected 1mo duration to be supported")
	}

	bad := isSupportedRankingPollParams(toss.GetRankingsParams{
		Type:          toss.RankingTopGainers,
		MarketCountry: toss.MarketKR,
		Duration:      toss.Ranking1d,
	})
	if bad {
		t.Fatal("expected TOP_GAINERS to be unsupported")
	}
}

func TestAllRankingsPollParamsCount(t *testing.T) {
	t.Parallel()

	svc := &MarketService{}
	got := len(svc.allRankingsPollParams())
	want := len(rankingsPollMarkets) * len(rankingsPollTypes) * len(rankingsPollDurations)
	if got != want {
		t.Fatalf("slots = %d, want %d", got, want)
	}
}
