package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

const rankingSnapshotMinInterval = time.Minute

type RankingHistoryPointResponse struct {
	RankedAt   string   `json:"rankedAt"`
	Rank       int      `json:"rank"`
	ChangeRate *float64 `json:"changeRate,omitempty"`
}

func parseRankingChangeRate(raw *string) *float64 {
	if raw == nil {
		return nil
	}
	s := strings.TrimSpace(*raw)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func parseRankingRankedAt(raw *string) time.Time {
	if raw == nil {
		return time.Now().UTC()
	}
	s := strings.TrimSpace(*raw)
	if s == "" {
		return time.Now().UTC()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC()
	}
	return time.Now().UTC()
}

func (s *MarketService) RecordRankingSnapshot(ctx context.Context, p toss.GetRankingsParams, resp *toss.RankingResponse) {
	if s.rankingSnapshots == nil || resp == nil || len(resp.Rankings) == 0 {
		return
	}

	market := string(p.MarketCountry)
	typ := string(p.Type)
	dur := string(p.Duration)

	last, err := s.rankingSnapshots.LastRankedAt(ctx, market, typ, dur)
	if err != nil {
		return
	}
	rankedAt := parseRankingRankedAt(resp.RankedAt)
	if last != nil && rankedAt.Sub(*last) < rankingSnapshotMinInterval {
		return
	}

	entries := make([]repository.RankingSnapshotEntry, 0, len(resp.Rankings))
	for _, item := range resp.Rankings {
		entries = append(entries, repository.RankingSnapshotEntry{
			MarketCountry: market,
			RankingType:   typ,
			Duration:      dur,
			RankedAt:      rankedAt,
			Symbol:        item.Symbol,
			Rank:          item.Rank,
			ChangeRate:    parseRankingChangeRate(item.Price.ChangeRate),
		})
	}

	bg, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = s.rankingSnapshots.InsertBatch(bg, entries)
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
