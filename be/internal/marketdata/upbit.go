package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

type UpbitClient struct {
	accessKey  string
	secretKey  string
	httpClient *http.Client
	budget     *apibudget.Manager
}

func NewUpbitClient(accessKey, secretKey string, httpClient *http.Client, budget *apibudget.Manager) *UpbitClient {
	return &UpbitClient{
		accessKey:  strings.TrimSpace(accessKey),
		secretKey:  strings.TrimSpace(secretKey),
		httpClient: httpClient,
		budget:     budget,
	}
}

func (c *UpbitClient) Enabled() bool {
	return c != nil && c.httpClient != nil
}

// Ticker uses the public Quotation API (no auth required).
func (c *UpbitClient) Ticker(ctx context.Context, market string) (Quote, error) {
	if !c.Enabled() {
		return Quote{}, fmt.Errorf("upbit client not configured")
	}
	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderUpbit); err != nil {
			return Quote{}, err
		}
	}
	u := "https://api.upbit.com/v1/ticker?markets=" + url.QueryEscape(market)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Quote{}, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return Quote{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		if c.budget != nil {
			c.budget.BlockFor(ctx, apibudget.ProviderUpbit, time.Minute)
		}
		return Quote{}, fmt.Errorf("upbit ticker %s: status 429", market)
	}
	if res.StatusCode != http.StatusOK {
		return Quote{}, fmt.Errorf("upbit ticker %s: status %d", market, res.StatusCode)
	}
	var rows []upbitTickerRow
	if err := json.NewDecoder(res.Body).Decode(&rows); err != nil {
		return Quote{}, err
	}
	if len(rows) == 0 || rows[0].TradePrice <= 0 {
		return Quote{}, fmt.Errorf("upbit ticker %s: empty", market)
	}
	if c.budget != nil {
		c.budget.Record(ctx, apibudget.ProviderUpbit)
	}
	row := rows[0]
	return upbitQuoteFromRow(row), nil
}

// DailyCloses returns daily close prices oldest-first (for sparklines).
func (c *UpbitClient) DailyCloses(ctx context.Context, market string, count int) ([]float64, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("upbit client not configured")
	}
	if count <= 0 {
		count = 20
	}
	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderUpbit); err != nil {
			return nil, err
		}
	}
	u := fmt.Sprintf(
		"https://api.upbit.com/v1/candles/days?market=%s&count=%d",
		url.QueryEscape(market),
		count,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		if c.budget != nil {
			c.budget.BlockFor(ctx, apibudget.ProviderUpbit, time.Minute)
		}
		return nil, fmt.Errorf("upbit candles %s: status 429", market)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upbit candles %s: status %d", market, res.StatusCode)
	}
	var rows []struct {
		TradePrice float64 `json:"trade_price"`
	}
	if err := json.NewDecoder(res.Body).Decode(&rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("upbit candles %s: empty", market)
	}
	if c.budget != nil {
		c.budget.Record(ctx, apibudget.ProviderUpbit)
	}
	closes := make([]float64, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].TradePrice > 0 {
			closes = append(closes, rows[i].TradePrice)
		}
	}
	if len(closes) == 0 {
		return nil, fmt.Errorf("upbit candles %s: no closes", market)
	}
	return closes, nil
}

// TickerWithSparkline fetches ticker + daily closes with one apibudget Check/Record.
func (c *UpbitClient) TickerWithSparkline(ctx context.Context, market string, sparkCount int) (Quote, error) {
	if !c.Enabled() {
		return Quote{}, fmt.Errorf("upbit client not configured")
	}
	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderUpbit); err != nil {
			return Quote{}, err
		}
	}
	q, err := c.tickerNoBudget(ctx, market)
	if err != nil {
		return Quote{}, err
	}
	closes, err := c.dailyClosesNoBudget(ctx, market, sparkCount)
	if err == nil {
		q.Sparkline = closes
	}
	if c.budget != nil {
		c.budget.Record(ctx, apibudget.ProviderUpbit)
	}
	return q, nil
}

func (c *UpbitClient) tickerNoBudget(ctx context.Context, market string) (Quote, error) {
	u := "https://api.upbit.com/v1/ticker?markets=" + url.QueryEscape(market)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Quote{}, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return Quote{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		return Quote{}, fmt.Errorf("upbit ticker %s: status 429", market)
	}
	if res.StatusCode != http.StatusOK {
		return Quote{}, fmt.Errorf("upbit ticker %s: status %d", market, res.StatusCode)
	}
	var rows []upbitTickerRow
	if err := json.NewDecoder(res.Body).Decode(&rows); err != nil {
		return Quote{}, err
	}
	if len(rows) == 0 || rows[0].TradePrice <= 0 {
		return Quote{}, fmt.Errorf("upbit ticker %s: empty", market)
	}
	row := rows[0]
	return upbitQuoteFromRow(row), nil
}

func (c *UpbitClient) dailyClosesNoBudget(ctx context.Context, market string, count int) ([]float64, error) {
	if count <= 0 {
		count = 20
	}
	u := fmt.Sprintf(
		"https://api.upbit.com/v1/candles/days?market=%s&count=%d",
		url.QueryEscape(market),
		count,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upbit candles %s: status %d", market, res.StatusCode)
	}
	var rows []struct {
		TradePrice float64 `json:"trade_price"`
	}
	if err := json.NewDecoder(res.Body).Decode(&rows); err != nil {
		return nil, err
	}
	closes := make([]float64, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].TradePrice > 0 {
			closes = append(closes, rows[i].TradePrice)
		}
	}
	if len(closes) == 0 {
		return nil, fmt.Errorf("upbit candles %s: no closes", market)
	}
	return closes, nil
}

type upbitTickerRow struct {
	TradePrice        float64 `json:"trade_price"`
	SignedChangePrice float64 `json:"signed_change_price"`
	SignedChangeRate  float64 `json:"signed_change_rate"`
	TradeTimestamp    int64   `json:"trade_timestamp"`
}

func upbitQuoteFromRow(row upbitTickerRow) Quote {
	q := Quote{
		Value:      row.TradePrice,
		Change:     row.SignedChangePrice,
		ChangeRate: row.SignedChangeRate * 100,
	}
	if row.TradeTimestamp > 0 {
		q.AsOf = time.UnixMilli(row.TradeTimestamp).UTC()
	}
	return q
}
