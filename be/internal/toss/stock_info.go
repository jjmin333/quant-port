package toss

import (
	"context"
	"net/url"
	"strings"
)

// --- Stock Info API ---

func (c *Client) GetStocks(ctx context.Context, symbols []string) ([]StockInfo, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	q := url.Values{"symbols": {strings.Join(symbols, ",")}}
	var out Envelope[[]StockInfo]
	if err := c.get(ctx, "/api/v1/stocks", q, nil, &out); err != nil {
		return nil, err
	}
	return out.Result, nil
}

func (c *Client) GetStock(ctx context.Context, symbol string) (*StockInfo, error) {
	items, err := c.GetStocks(ctx, []string{symbol})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (c *Client) GetStockWarnings(ctx context.Context, symbol string) ([]StockWarning, error) {
	var out Envelope[[]StockWarning]
	if err := c.get(ctx, "/api/v1/stocks/"+url.PathEscape(symbol)+"/warnings", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Result, nil
}
