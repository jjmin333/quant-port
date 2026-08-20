package service

import (
	"context"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

func TestDetectKrSession_afterWindow(t *testing.T) {
	integrated := toss.IntegratedHour{
		RegularMarket: &toss.RegularMarketSession{
			StartTime: "2026-07-16T09:00:00+09:00",
			EndTime:   "2026-07-16T15:30:00+09:00",
		},
		AfterMarket: &toss.AfterMarketSession{
			StartTime: "2026-07-16T15:30:00+09:00",
			EndTime:   "2026-07-16T18:00:00+09:00",
		},
	}
	now := time.Date(2026, 7, 16, 16, 30, 0, 0, kst)
	if got := detectKrSession(integrated, now); got != "after" {
		t.Fatalf("session = %q want after", got)
	}
	closed := time.Date(2026, 7, 16, 19, 0, 0, 0, kst)
	if got := detectKrSession(integrated, closed); got != "closed" {
		t.Fatalf("session = %q want closed after after-market", got)
	}
}

func TestDetectMarketSession_domesticHolidayClosed(t *testing.T) {
	cals := marketCalendars{
		kr: &toss.KrMarketCalendarResponse{
			Today: toss.KrMarketDay{
				Date:       "2026-07-16",
				Integrated: nil,
			},
		},
	}
	now := time.Date(2026, 7, 16, 17, 0, 0, 0, kst)
	if got := detectMarketSession("domestic", cals, now); got != "closed" {
		t.Fatalf("session = %q want closed on holiday", got)
	}
}

func TestShouldLivePollSymbol_followsTossEnabled(t *testing.T) {
	s := &QuotesService{}
	if s.shouldLivePollSymbol(context.Background(), "005930") {
		t.Fatal("expected false when Toss disabled")
	}
}

func TestFilterLiveSessionSymbols(t *testing.T) {
	cases := []struct {
		session string
		keep    bool
	}{
		{"regular", true},
		{"pre", true},
		{"after", true},
		{"day", true},
		{"closed", false},
		{"", false},
	}
	for _, c := range cases {
		if got := quoteSessionHasLiveMicrostructure(c.session); got != c.keep {
			t.Errorf("session %q: live=%v want %v", c.session, got, c.keep)
		}
	}
}

func TestFilterLiveSessionSymbolsEmpty(t *testing.T) {
	s := &QuotesService{}
	if got := s.filterLiveSessionSymbols(context.Background(), nil); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}
