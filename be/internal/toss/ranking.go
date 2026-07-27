package toss

import (
	"context"
	"net/url"
	"strconv"
)

type GetRankingsParams struct {
	Type                      RankingType
	MarketCountry             MarketCountry
	Duration                  RankingDuration
	ExcludeInvestmentCaution  bool
	Count                     int
}

// --- Ranking API ---

func (c *Client) GetRankings(ctx context.Context, p GetRankingsParams) (*RankingResponse, error) {
	q := url.Values{
		"type":           {string(p.Type)},
		"marketCountry":  {string(p.MarketCountry)},
		"duration":       {string(p.Duration)},
		"excludeInvestmentCaution": {strconv.FormatBool(p.ExcludeInvestmentCaution)},
	}
	if p.Count > 0 {
		q.Set("count", strconv.Itoa(p.Count))
	}
	var out Envelope[RankingResponse]
	if err := c.get(ctx, "/api/v1/rankings", q, nil, &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}
