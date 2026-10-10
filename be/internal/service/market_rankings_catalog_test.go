package service

import (
	"testing"
	"time"
)

func TestPickRankingSymbolsToRegister_skipsKnownAndRecent(t *testing.T) {
	now := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	have := map[string]bool{"005930": true}
	got := pickRankingSymbolsToRegister([]string{"005930", "T1S0", "T1S0", "T2S0"}, have, now)
	if len(got) != 2 || got[0] != "T1S0" || got[1] != "T2S0" {
		t.Fatalf("first pass = %v", got)
	}
	if again := pickRankingSymbolsToRegister([]string{"T1S0"}, have, now.Add(5*time.Minute)); len(again) != 0 {
		t.Fatalf("asked again within retry window: %v", again)
	}
	if later := pickRankingSymbolsToRegister([]string{"T1S0"}, have, now.Add(rankingCatalogRetry+time.Minute)); len(later) != 1 {
		t.Fatalf("after retry window = %v", later)
	}
}
