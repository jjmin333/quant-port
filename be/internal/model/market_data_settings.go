package model

import "time"

type MarketDataSettings struct {
	RankingsPollRealtimeMinutes int       `json:"rankingsPollRealtimeMinutes"`
	RankingsPollShortMinutes    int       `json:"rankingsPollShortMinutes"`
	RankingsPollLongMinutes     int       `json:"rankingsPollLongMinutes"`
	QuoteOverlayPollSeconds     int       `json:"quoteOverlayPollSeconds"`
	RankingsClientPollSeconds   int       `json:"rankingsClientPollSeconds"`
	SharesPollHours             int       `json:"sharesPollHours"`
	FundamentalsPollHours       int       `json:"fundamentalsPollHours"`
	UpdatedAt                   time.Time `json:"updatedAt"`
}

func DefaultMarketDataSettings() MarketDataSettings {
	return MarketDataSettings{
		RankingsPollRealtimeMinutes: 5,
		RankingsPollShortMinutes:    15,
		RankingsPollLongMinutes:     60,
		QuoteOverlayPollSeconds:     2,
		RankingsClientPollSeconds:   60,
		SharesPollHours:             24,
		FundamentalsPollHours:       24,
	}
}
