package toss

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

// --- Market Indicators API ---

func (c *Client) GetMarketIndicatorPrices(ctx context.Context, symbols []string) ([]MarketIndicatorPriceResponse, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	q := url.Values{"symbols": {strings.Join(symbols, ",")}}
	var out Envelope[[]MarketIndicatorPriceResponse]
	if err := c.get(ctx, "/api/v1/market-indicators/prices", q, nil, &out); err != nil {
		return nil, err
	}
	return out.Result, nil
}

type GetMarketIndicatorCandlesParams struct {
	Symbol   string
	Interval CandleInterval
	Count    int
	Before   string
}

func (c *Client) GetMarketIndicatorCandles(ctx context.Context, p GetMarketIndicatorCandlesParams) (*MarketIndicatorCandlePageResponse, error) {
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
	var out Envelope[MarketIndicatorCandlePageResponse]
	path := "/api/v1/market-indicators/" + url.PathEscape(p.Symbol) + "/candles"
	if err := c.get(ctx, path, q, nil, &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}

type GetInvestorTradingParams struct {
	Symbol   string
	Interval InvestorTradingInterval
	Count    int
	Until    string
}

func (c *Client) GetInvestorTrading(ctx context.Context, p GetInvestorTradingParams) (*InvestorTradingResponse, error) {
	q := url.Values{"interval": {string(p.Interval)}}
	if p.Count > 0 {
		q.Set("count", strconv.Itoa(p.Count))
	}
	if p.Until != "" {
		q.Set("until", p.Until)
	}
	var out Envelope[InvestorTradingResponse]
	path := "/api/v1/market-indicators/" + url.PathEscape(p.Symbol) + "/investor-trading"
	if err := c.get(ctx, path, q, nil, &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}
