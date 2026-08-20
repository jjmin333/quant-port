package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/toss"
)

func TestRankingsCacheKey(t *testing.T) {
	t.Parallel()

	key := rankingsCacheKey(toss.GetRankingsParams{
		Type:                     toss.RankingMarketTradingAmount,
		MarketCountry:            toss.MarketKR,
		Duration:                 toss.RankingRealtime,
		Count:                    20,
		ExcludeInvestmentCaution: true,
	})
	want := "market:rankings:v2:KR:MARKET_TRADING_AMOUNT:realtime:true"
	if key != want {
		t.Fatalf("key = %q, want %q", key, want)
	}
}
