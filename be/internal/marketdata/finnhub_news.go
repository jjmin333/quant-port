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

type FinnhubNewsClient struct {
	apiKey     string
	httpClient *http.Client
	budget     *apibudget.Manager
}

func NewFinnhubNewsClient(apiKey string, httpClient *http.Client, budget *apibudget.Manager) *FinnhubNewsClient {
	return &FinnhubNewsClient{
		apiKey:     strings.TrimSpace(apiKey),
		httpClient: httpClient,
		budget:     budget,
	}
}

func (c *FinnhubNewsClient) Enabled() bool {
	return c != nil && c.apiKey != ""
}

// MarketNews fetches market-wide news (category: general, forex, crypto, merger).
func (c *FinnhubNewsClient) MarketNews(ctx context.Context, category string) ([]RawNewsItem, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("finnhub news not configured")
	}
	category = strings.TrimSpace(category)
	if category == "" {
		category = "general"
	}
	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderFinnhubNews); err != nil {
			return nil, err
		}
	}

	u := fmt.Sprintf(
		"https://finnhub.io/api/v1/news?category=%s&token=%s",
		url.QueryEscape(category),
		url.QueryEscape(c.apiKey),
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
			c.budget.BlockFor(ctx, apibudget.ProviderFinnhubNews, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		return nil, fmt.Errorf("finnhub news rate limited")
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub news status %d", res.StatusCode)
	}

	var payload []finnhubNewsArticle
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if c.budget != nil {
		c.budget.Record(ctx, apibudget.ProviderFinnhubNews)
	}
	return mapFinnhubNewsArticles(payload), nil
}

// CompanyNews fetches news for a US symbol between from/to (YYYY-MM-DD).
func (c *FinnhubNewsClient) CompanyNews(ctx context.Context, symbol, from, to string) ([]RawNewsItem, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("finnhub news not configured")
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return nil, fmt.Errorf("finnhub company news: empty symbol")
	}
	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderFinnhubNews); err != nil {
			return nil, err
		}
	}

	u := fmt.Sprintf(
		"https://finnhub.io/api/v1/company-news?symbol=%s&from=%s&to=%s&token=%s",
		url.QueryEscape(symbol),
		url.QueryEscape(from),
		url.QueryEscape(to),
		url.QueryEscape(c.apiKey),
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
			c.budget.BlockFor(ctx, apibudget.ProviderFinnhubNews, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		return nil, fmt.Errorf("finnhub company news rate limited")
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub company news status %d", res.StatusCode)
	}

	var payload []finnhubNewsArticle
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if c.budget != nil {
		c.budget.Record(ctx, apibudget.ProviderFinnhubNews)
	}
	return mapFinnhubNewsArticles(payload), nil
}

type finnhubNewsArticle struct {
	Datetime int64  `json:"datetime"`
	Headline string `json:"headline"`
	Summary  string `json:"summary"`
	URL      string `json:"url"`
	Related  string `json:"related"`
	Source   string `json:"source"`
}

func mapFinnhubNewsArticles(rows []finnhubNewsArticle) []RawNewsItem {
	out := make([]RawNewsItem, 0, len(rows))
	for _, row := range rows {
		title := strings.TrimSpace(row.Headline)
		link := strings.TrimSpace(row.URL)
		if title == "" || link == "" {
			continue
		}
		var publishedAt time.Time
		if row.Datetime > 0 {
			publishedAt = time.Unix(row.Datetime, 0).UTC()
		}
		out = append(out, RawNewsItem{
			Title:         title,
			Description:   strings.TrimSpace(row.Summary),
			Link:          link,
			OriginalLink:  link,
			PublishedAt:   publishedAt,
			SourceID:      "finnhub",
			RelatedSymbol: strings.ToUpper(strings.TrimSpace(row.Related)),
			PublisherName: strings.TrimSpace(row.Source),
		})
	}
	return out
}
