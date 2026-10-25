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

type FinnhubClient struct {
	apiKey     string
	httpClient *http.Client
	budget     *apibudget.Manager
}

func NewFinnhubClient(apiKey string, httpClient *http.Client, budget *apibudget.Manager) *FinnhubClient {
	apiKey = strings.TrimSpace(apiKey)
	return &FinnhubClient{apiKey: apiKey, httpClient: withFinnhubToken(httpClient, apiKey), budget: budget}
}

// withFinnhubToken sends the key as the X-Finnhub-Token header instead of
// ?token=. 쿼리에 두면 타임아웃 때 *url.Error가 URL째 로그에 키를 남긴다
// (10-24 news general 타임아웃 실측). Finnhub 요청은 전부 이 클라이언트를 탄다.
func withFinnhubToken(hc *http.Client, key string) *http.Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	c := *hc
	base := c.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	c.Transport = finnhubAuth{base: base, key: key}
	return &c
}

type finnhubAuth struct {
	base http.RoundTripper
	key  string
}

func (t finnhubAuth) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("X-Finnhub-Token", t.key)
	return t.base.RoundTrip(r)
}

func (c *FinnhubClient) Enabled() bool {
	return c != nil && c.apiKey != ""
}

func (c *FinnhubClient) Quote(ctx context.Context, symbol string) (Quote, error) {
	out, err := c.QuoteBatch(ctx, []string{symbol})
	if err != nil {
		return Quote{}, err
	}
	q, ok := out[symbol]
	if !ok {
		return Quote{}, fmt.Errorf("finnhub quote %s: missing from batch", symbol)
	}
	return q, nil
}

// QuoteBatch fetches multiple symbols with one apibudget Check/Record.
func (c *FinnhubClient) QuoteBatch(ctx context.Context, symbols []string) (map[string]Quote, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("finnhub not configured")
	}
	if len(symbols) == 0 {
		return map[string]Quote{}, nil
	}
	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderFinnhub); err != nil {
			return nil, err
		}
	}

	out := make(map[string]Quote, len(symbols))
	for _, symbol := range symbols {
		q, err := c.quoteNoBudget(ctx, symbol)
		if err != nil {
			continue
		}
		out[symbol] = q
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("finnhub batch: all symbols failed")
	}
	if c.budget != nil {
		c.budget.Record(ctx, apibudget.ProviderFinnhub)
	}
	return out, nil
}

func (c *FinnhubClient) quoteNoBudget(ctx context.Context, symbol string) (Quote, error) {
	u := fmt.Sprintf(
		"https://finnhub.io/api/v1/quote?symbol=%s",
		url.QueryEscape(symbol),
	)
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
			c.budget.BlockFor(ctx, apibudget.ProviderFinnhub, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		return Quote{}, fmt.Errorf("finnhub quote %s: status 429", symbol)
	}
	if res.StatusCode != http.StatusOK {
		return Quote{}, fmt.Errorf("finnhub quote %s: status %d", symbol, res.StatusCode)
	}
	var payload struct {
		Current       float64 `json:"c"`
		Change        float64 `json:"d"`
		ChangePercent float64 `json:"dp"`
		Timestamp     int64   `json:"t"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return Quote{}, err
	}
	if payload.Current <= 0 {
		return Quote{}, fmt.Errorf("finnhub quote %s: empty price", symbol)
	}
	q := Quote{
		Value:      payload.Current,
		Change:     payload.Change,
		ChangeRate: payload.ChangePercent,
	}
	if payload.Timestamp > 0 {
		q.AsOf = time.Unix(payload.Timestamp, 0).UTC()
	}
	return q, nil
}
