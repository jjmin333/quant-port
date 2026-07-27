package toss

import (
	"context"
	"net/url"
)

// --- Account API ---

func (c *Client) GetAccounts(ctx context.Context) ([]Account, error) {
	var out Envelope[[]Account]
	if err := c.get(ctx, "/api/v1/accounts", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Result, nil
}

// --- Asset API ---

func (c *Client) GetHoldings(ctx context.Context, accountSeq int64, symbol string) (*HoldingsOverview, error) {
	q := url.Values{}
	if symbol != "" {
		q.Set("symbol", symbol)
	}
	var out Envelope[HoldingsOverview]
	if err := c.get(ctx, "/api/v1/holdings", q, WithAccount(accountSeq), &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}
