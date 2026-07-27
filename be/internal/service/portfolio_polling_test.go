package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

func TestComputePollingPolicy_activeDuringKrRegular(t *testing.T) {
	cals := marketCalendars{
		kr: &toss.KrMarketCalendarResponse{
			Today: toss.KrMarketDay{
				Integrated: &toss.IntegratedHour{
					RegularMarket: &toss.RegularMarketSession{
						StartTime: "2026-07-16T09:00:00+09:00",
						EndTime:   "2026-07-16T15:30:00+09:00",
					},
				},
			},
		},
	}
	now := time.Date(2026, 7, 16, 11, 0, 0, 0, kst)
	holdings := []model.UnifiedHolding{{AssetType: "domestic", Currency: "KRW"}}

	policy := computePollingPolicy(holdings, cals, now)
	if policy.Mode != "active" {
		t.Fatalf("mode = %q want active", policy.Mode)
	}
}

func TestComputePollingPolicy_idleAfterAllSessions(t *testing.T) {
	cals := marketCalendars{
		kr: &toss.KrMarketCalendarResponse{
			Today: toss.KrMarketDay{
				Integrated: &toss.IntegratedHour{
					PreMarket: &toss.PreMarketSession{
						StartTime: "2026-07-17T08:30:00+09:00",
						EndTime:   "2026-07-17T09:00:00+09:00",
					},
					RegularMarket: &toss.RegularMarketSession{
						StartTime: "2026-07-17T09:00:00+09:00",
						EndTime:   "2026-07-17T15:30:00+09:00",
					},
					AfterMarket: &toss.AfterMarketSession{
						StartTime: "2026-07-16T15:40:00+09:00",
						EndTime:   "2026-07-16T18:00:00+09:00",
					},
				},
			},
			NextBusinessDay: toss.KrMarketDay{
				Integrated: &toss.IntegratedHour{
					PreMarket: &toss.PreMarketSession{
						StartTime: "2026-07-17T08:30:00+09:00",
						EndTime:   "2026-07-17T09:00:00+09:00",
					},
				},
			},
		},
		us: &toss.UsMarketCalendarResponse{
			Today: toss.UsMarketDay{
				AfterMarket: &toss.UsAfterMarketSession{
					StartTime: "2026-07-16T05:00:00+09:00",
					EndTime:   "2026-07-16T07:00:00+09:00",
				},
			},
			NextBusinessDay: toss.UsMarketDay{
				PreMarket: &toss.UsPreMarketSession{
					StartTime: "2026-07-16T22:00:00+09:00",
					EndTime:   "2026-07-16T22:30:00+09:00",
				},
			},
		},
	}
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, kst)
	holdings := []model.UnifiedHolding{
		{AssetType: "domestic", Currency: "KRW"},
		{AssetType: "us", Currency: "USD"},
	}

	policy := computePollingPolicy(holdings, cals, now)
	if policy.Mode != "idle" {
		t.Fatalf("mode = %q want idle", policy.Mode)
	}
	if policy.FullRefreshIntervalSec != model.PortfolioIdleFullRefreshIntervalSec {
		t.Fatalf("full interval = %d want %d", policy.FullRefreshIntervalSec, model.PortfolioIdleFullRefreshIntervalSec)
	}
	if policy.IdleUntil.IsZero() {
		t.Fatal("expected idle until to be set")
	}
}

func TestEarliestAfter(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, kst)
	times := []time.Time{
		time.Date(2026, 7, 16, 22, 0, 0, 0, kst),
		time.Date(2026, 7, 16, 9, 0, 0, 0, kst),
		time.Date(2026, 7, 16, 7, 0, 0, 0, kst),
	}
	got := earliestAfter(times, now)
	want := time.Date(2026, 7, 16, 9, 0, 0, 0, kst)
	if !got.Equal(want) {
		t.Fatalf("earliest = %v want %v", got, want)
	}
}
