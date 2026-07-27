package toss

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

// --- Market Data API (docs/toss-openapi.json → Market Data) ---

func (c *Client) GetOrderbook(ctx context.Context, symbol string) (*OrderbookResponse, error) {
	q := url.Values{"symbol": {symbol}}
	var out Envelope[OrderbookResponse]
	if err := c.get(ctx, "/api/v1/orderbook", q, nil, &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}

func (c *Client) GetPrices(ctx context.Context, symbols []string) ([]PriceResponse, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	q := url.Values{"symbols": {strings.Join(symbols, ",")}}
	var out Envelope[[]PriceResponse]
	if err := c.get(ctx, "/api/v1/prices", q, nil, &out); err != nil {
		return nil, err
	}
	return out.Result, nil
}

func (c *Client) GetPrice(ctx context.Context, symbol string) (*PriceResponse, error) {
	items, err := c.GetPrices(ctx, []string{symbol})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (c *Client) GetTrades(ctx context.Context, symbol string, count int) ([]TradeResponse, error) {
	q := url.Values{"symbol": {symbol}}
	if count > 0 {
		q.Set("count", strconv.Itoa(count))
	}
	var out Envelope[[]TradeResponse]
	if err := c.get(ctx, "/api/v1/trades", q, nil, &out); err != nil {
		return nil, err
	}
	return out.Result, nil
}

func (c *Client) GetPriceLimits(ctx context.Context, symbol string) (*PriceLimitResponse, error) {
	q := url.Values{"symbol": {symbol}}
	var out Envelope[PriceLimitResponse]
	if err := c.get(ctx, "/api/v1/price-limits", q, nil, &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}

type GetCandlesParams struct {
	Symbol   string
	Interval CandleInterval
	Count    int
	Before   string
	Adjusted *bool
}

func (c *Client) GetCandles(ctx context.Context, p GetCandlesParams) (*CandlePageResponse, error) {
	q := url.Values{
		"symbol":   {p.Symbol},
		"interval": {string(p.Interval)},
	}
	if p.Count > 0 {
		q.Set("count", strconv.Itoa(p.Count))
	}
	if p.Before != "" {
		q.Set("before", p.Before)
	}
	if p.Adjusted != nil {
		q.Set("adjusted", strconv.FormatBool(*p.Adjusted))
	}

	var out Envelope[CandlePageResponse]
	if err := c.get(ctx, "/api/v1/candles", q, nil, &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}
