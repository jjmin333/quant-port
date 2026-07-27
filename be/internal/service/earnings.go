package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

type EarningsConfig struct {
	FinnhubAPIKey string
}

func (c EarningsConfig) Enabled() bool {
	return c.FinnhubAPIKey != ""
}

type EarningsClient struct {
	cfg        EarningsConfig
	httpClient *http.Client
	budget     *apibudget.Manager
}

func NewEarningsClient(cfg EarningsConfig, budget *apibudget.Manager) *EarningsClient {
	return &EarningsClient{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		budget:     budget,
	}
}

func (e *EarningsClient) UpcomingForSymbols(ctx context.Context, symbols []string) ([]model.BriefingEvent, error) {
	if !e.cfg.Enabled() || len(symbols) == 0 {
		return nil, nil
	}
	all, err := e.FetchCalendar(ctx)
	if err != nil {
		return nil, err
	}
	held := map[string]struct{}{}
	for _, s := range symbols {
		held[strings.ToUpper(strings.TrimSpace(s))] = struct{}{}
	}
	var out []model.BriefingEvent
	for _, ev := range all {
		if _, ok := held[strings.ToUpper(ev.Symbol)]; ok {
			out = append(out, ev)
		}
	}
	return out, nil
}

func (e *EarningsClient) FetchCalendar(ctx context.Context) ([]model.BriefingEvent, error) {
	if !e.cfg.Enabled() {
		return nil, nil
	}

	from := time.Now().Format("2006-01-02")
	to := time.Now().Add(14 * 24 * time.Hour).Format("2006-01-02")
	if e.budget != nil {
		if err := e.budget.Check(ctx, apibudget.ProviderFinnhub); err != nil {
			return nil, err
		}
	}
	u := fmt.Sprintf(
		"https://finnhub.io/api/v1/calendar/earnings?from=%s&to=%s&token=%s",
		from, to, url.QueryEscape(e.cfg.FinnhubAPIKey),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		if e.budget != nil {
			e.budget.BlockFor(ctx, apibudget.ProviderFinnhub, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		return nil, fmt.Errorf("finnhub earnings: status 429")
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub earnings: status %d", res.StatusCode)
	}

	var payload struct {
		EarningsCalendar []struct {
			Date    string  `json:"date"`
			Symbol  string  `json:"symbol"`
			Quarter float64 `json:"quarter"`
		} `json:"earningsCalendar"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if e.budget != nil {
		e.budget.Record(ctx, apibudget.ProviderFinnhub)
	}

	var out []model.BriefingEvent
	for _, row := range payload.EarningsCalendar {
		sym := strings.ToUpper(strings.TrimSpace(row.Symbol))
		if sym == "" {
			continue
		}
		label := "실적발표"
		if row.Quarter > 0 {
			label = fmt.Sprintf("Q%.0f 실적발표", row.Quarter)
		}
		out = append(out, model.BriefingEvent{
			Date:   formatBriefDate(row.Date),
			Symbol: sym,
			Event:  label,
		})
	}
	return out, nil
}

func formatBriefDate(raw string) string {
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return raw
	}
	return t.Format("1/2")
}

func domesticSymbols(holdings []model.UnifiedHolding) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, h := range holdings {
		if h.AssetType != "domestic" && h.Currency != "KRW" {
			continue
		}
		if len(h.Symbol) != 6 {
			continue
		}
		if _, ok := seen[h.Symbol]; ok {
			continue
		}
		seen[h.Symbol] = struct{}{}
		out = append(out, h.Symbol)
	}
	return out
}

func usSymbols(holdings []model.UnifiedHolding) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, h := range holdings {
		if h.AssetType != "us" && h.Currency != "USD" {
			continue
		}
		sym := strings.ToUpper(h.Symbol)
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}
	return out
}
