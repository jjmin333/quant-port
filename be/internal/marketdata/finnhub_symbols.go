package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
)

type finnhubSymbol struct {
	Currency      string `json:"currency"`
	Description   string `json:"description"`
	DisplaySymbol string `json:"displaySymbol"`
	Symbol        string `json:"symbol"`
	Type          string `json:"type"`
	MIC           string `json:"mic"`
}

// IsUSExchangeTradedProduct reports Finnhub types that represent ETFs/ETNs/ETPs.
func IsUSExchangeTradedProduct(typeLabel string) bool {
	t := strings.ToUpper(strings.TrimSpace(typeLabel))
	if t == "" {
		return false
	}
	switch t {
	case "ETP", "ETF", "ETN", "ETC", "FUND":
		return true
	}
	return strings.Contains(t, "ETF") || strings.Contains(t, "ETN")
}

// ListUSExchangeTradedProducts fetches all US symbols in one Finnhub call and filters ETPs.
func (c *FinnhubClient) ListUSExchangeTradedProducts(ctx context.Context) ([]model.Stock, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("finnhub not configured")
	}
	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderFinnhubMigrate); err != nil {
			return nil, err
		}
	}

	u := fmt.Sprintf(
		"https://finnhub.io/api/v1/stock/symbol?exchange=US&token=%s",
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
			c.budget.BlockFor(ctx, apibudget.ProviderFinnhubMigrate, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		return nil, fmt.Errorf("finnhub symbols: status 429")
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub symbols: status %d", res.StatusCode)
	}

	var payload []finnhubSymbol
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if c.budget != nil {
		c.budget.Record(ctx, apibudget.ProviderFinnhubMigrate)
	}

	out := make([]model.Stock, 0, 512)
	for _, row := range payload {
		if !IsUSExchangeTradedProduct(row.Type) {
			continue
		}
		sym := strings.ToUpper(strings.TrimSpace(row.Symbol))
		if sym == "" {
			sym = strings.ToUpper(strings.TrimSpace(row.DisplaySymbol))
		}
		if sym == "" {
			continue
		}
		name := strings.TrimSpace(row.Description)
		if name == "" {
			name = sym
		}
		market := mapFinnhubMIC(row.MIC)
		out = append(out, model.Stock{
			Symbol:       sym,
			Name:         name,
			EnglishName:  name,
			Market:       market,
			SecurityType: strings.ToUpper(strings.TrimSpace(row.Type)),
			Currency:     model.CurrencyUSD,
			AssetType:    model.AssetETF,
			Status:       "NORMAL",
		})
	}
	return out, nil
}

func mapFinnhubMIC(mic string) string {
	m := strings.ToUpper(strings.TrimSpace(mic))
	switch m {
	case "XNAS", "NASDAQ":
		return "NASDAQ"
	case "XNYS", "NYSE":
		return "NYSE"
	case "XASE", "AMEX":
		return "AMEX"
	default:
		if m != "" {
			return m
		}
		return "US"
	}
}
