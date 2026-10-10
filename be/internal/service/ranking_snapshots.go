package service

import (
	"context"
	"time"
)

const rankingSnapshotMinInterval = time.Minute

type RankingHistoryPointResponse struct {
	RankedAt   string   `json:"rankedAt"`
	Rank       int      `json:"rank"`
	ChangeRate *float64 `json:"changeRate,omitempty"`
}

func (s *MarketService) RankingSymbolHistory(
	ctx context.Context,
	symbol, marketCountry, rankingType, duration string,
	hours int,
) ([]RankingHistoryPointResponse, error) {
	if s.rankingSnapshots == nil {
		return []RankingHistoryPointResponse{}, nil
	}
	if hours <= 0 {
		hours = 24
	}
	if hours > 168 {
		hours = 168
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	rows, err := s.rankingSnapshots.SymbolHistory(ctx, symbol, marketCountry, rankingType, duration, since, 500)
	if err != nil {
		return nil, err
	}
	out := make([]RankingHistoryPointResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, RankingHistoryPointResponse{
			RankedAt:   row.RankedAt.UTC().Format(time.RFC3339),
			Rank:       row.Rank,
			ChangeRate: row.ChangeRate,
		})
	}
	return out, nil
}
