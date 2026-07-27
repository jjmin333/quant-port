package toss

import (
	"context"
	"net/url"
)

type GetExchangeRateParams struct {
	BaseCurrency  Currency
	QuoteCurrency Currency
	DateTime      string
}

// --- Market Info API ---

func (c *Client) GetExchangeRate(ctx context.Context, p GetExchangeRateParams) (*ExchangeRateResponse, error) {
	q := url.Values{
		"baseCurrency":  {string(p.BaseCurrency)},
		"quoteCurrency": {string(p.QuoteCurrency)},
	}
	if p.DateTime != "" {
		q.Set("dateTime", p.DateTime)
	}
	var out Envelope[ExchangeRateResponse]
	if err := c.get(ctx, "/api/v1/exchange-rate", q, nil, &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}

func (c *Client) GetKrMarketCalendar(ctx context.Context, date string) (*KrMarketCalendarResponse, error) {
	q := url.Values{}
	if date != "" {
		q.Set("date", date)
	}
	var out Envelope[KrMarketCalendarResponse]
	if err := c.get(ctx, "/api/v1/market-calendar/KR", q, nil, &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}

func (c *Client) GetUsMarketCalendar(ctx context.Context, date string) (*UsMarketCalendarResponse, error) {
	q := url.Values{}
	if date != "" {
		q.Set("date", date)
	}
	var out Envelope[UsMarketCalendarResponse]
	if err := c.get(ctx, "/api/v1/market-calendar/US", q, nil, &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}
