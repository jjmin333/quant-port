package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

type EdgarConfig struct {
	UserAgent string
}

func (c EdgarConfig) Enabled() bool {
	return strings.TrimSpace(c.UserAgent) != ""
}

type EdgarClient struct {
	cfg        EdgarConfig
	httpClient *http.Client
	budget     *apibudget.Manager

	mu          sync.RWMutex
	tickerToCIK map[string]string
	tickersAt   time.Time
}

func NewEdgarClient(cfg EdgarConfig, budget *apibudget.Manager) *EdgarClient {
	return &EdgarClient{
		cfg:         cfg,
		httpClient:  &http.Client{Timeout: 45 * time.Second},
		budget:      budget,
		tickerToCIK: map[string]string{},
	}
}

func (e *EdgarClient) LookupCIK(ctx context.Context, ticker string) (string, error) {
	if !e.cfg.Enabled() {
		return "", fmt.Errorf("sec edgar user agent not configured")
	}
	if err := e.ensureTickers(ctx); err != nil {
		return "", err
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	cik, ok := e.tickerToCIK[strings.ToUpper(strings.TrimSpace(ticker))]
	if !ok || cik == "" {
		return "", fmt.Errorf("edgar: cik not found for ticker %s", ticker)
	}
	return cik, nil
}

func (e *EdgarClient) FetchFinancialSnapshots(ctx context.Context, ticker string) (current, prior model.FinancialSnapshot, shares *int64, err error) {
	cik, err := e.LookupCIK(ctx, ticker)
	if err != nil {
		return current, prior, nil, err
	}
	facts, err := e.fetchCompanyFacts(ctx, cik)
	if err != nil {
		return current, prior, nil, err
	}

	shares = PickSharesOutstanding(facts)

	revCur, revPrior := pickAnnualPairValues(facts, revenueConceptFallbacks)
	opCur, opPrior := pickAnnualPairValues(facts, operatingIncomeConceptFallbacks)
	niCur, niPrior := pickAnnualPairValues(facts, netIncomeConceptFallbacks)
	eqCur, eqPrior := pickAnnualPairValues(facts, equityConceptFallbacks)
	asCur, asPrior := pickAnnualPairValues(facts, assetsConceptFallbacks)
	liCur, _ := pickAnnualPairValues(facts, liabilitiesConceptFallbacks)
	caCur, _ := pickAnnualPairValues(facts, currentAssetsConceptFallbacks)
	clCur, _ := pickAnnualPairValues(facts, currentLiabilitiesConceptFallbacks)
	epsCur, epsPrior := pickAnnualPairFloats(facts, epsBasicConceptFallbacks)
	epsDilCur, epsDilPrior := pickAnnualPairFloats(facts, epsDilutedConceptFallbacks)

	current = model.FinancialSnapshot{
		FiscalPeriod:       "FY",
		Revenue:            revCur,
		OperatingIncome:    opCur,
		NetIncome:          niCur,
		TotalAssets:        asCur,
		TotalEquity:        eqCur,
		TotalLiabilities:   liCur,
		CurrentAssets:      caCur,
		CurrentLiabilities: clCur,
		EPSBasic:           epsCur,
		EPSDiluted:         firstNonNilFloat(epsDilCur, epsCur),
		SharesOutstanding:  shares,
	}
	prior = model.FinancialSnapshot{
		FiscalPeriod: "FY",
		Revenue:      revPrior,
		OperatingIncome: opPrior,
		NetIncome:    niPrior,
		TotalAssets:  asPrior,
		TotalEquity:  eqPrior,
		EPSBasic:     epsPrior,
		EPSDiluted:   firstNonNilFloat(epsDilPrior, epsPrior),
	}

	if revCurFact, _ := pickAnnualPairFacts(facts, revenueConceptFallbacks); revCurFact != nil {
		current.FiscalYear = revCurFact.FY
		current.PeriodEnd = parseFactDate(revCurFact.End)
		current.FiledAt = parseFactDate(revCurFact.Filed)
	}
	_ = opPrior
	return current, prior, shares, nil
}

func pickAnnualPairFacts(facts map[string]map[string][]XBRLFact, tags []string) (*XBRLFact, *XBRLFact) {
	items := pickConceptFacts(facts, tags)
	cur, prior := pickLatestAnnualPair(items)
	return cur, prior
}

func pickAnnualPairValues(facts map[string]map[string][]XBRLFact, tags []string) (*int64, *int64) {
	cur, prior := pickAnnualPairFacts(facts, tags)
	return factInt64(cur), factInt64(prior)
}

func pickAnnualPairFloats(facts map[string]map[string][]XBRLFact, tags []string) (*float64, *float64) {
	cur, prior := pickAnnualPairFacts(facts, tags)
	return factFloat64(cur), factFloat64(prior)
}

func firstNonNilFloat(values ...*float64) *float64 {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func (e *EdgarClient) ensureTickers(ctx context.Context) error {
	e.mu.RLock()
	if len(e.tickerToCIK) > 0 && time.Since(e.tickersAt) < 24*time.Hour {
		e.mu.RUnlock()
		return nil
	}
	e.mu.RUnlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.tickerToCIK) > 0 && time.Since(e.tickersAt) < 24*time.Hour {
		return nil
	}

	if e.budget != nil {
		if err := e.budget.Check(ctx, apibudget.ProviderEdgar); err != nil {
			return err
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.sec.gov/files/company_tickers.json", nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", e.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")

	res, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		if e.budget != nil {
			e.budget.BlockFor(ctx, apibudget.ProviderEdgar, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("edgar tickers: %s", strings.TrimSpace(string(body)))
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("edgar tickers: %s", strings.TrimSpace(string(body)))
	}

	var payload map[string]struct {
		CIKStr json.Number `json:"cik_str"`
		Ticker string      `json:"ticker"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return err
	}

	next := make(map[string]string, len(payload))
	for _, item := range payload {
		ticker := strings.ToUpper(strings.TrimSpace(item.Ticker))
		if ticker == "" {
			continue
		}
		n, err := item.CIKStr.Int64()
		if err != nil {
			continue
		}
		next[ticker] = fmt.Sprintf("%010d", n)
	}
	e.tickerToCIK = next
	e.tickersAt = time.Now()
	if e.budget != nil {
		e.budget.Record(ctx, apibudget.ProviderEdgar)
	}
	return nil
}

func (e *EdgarClient) fetchCompanyFacts(ctx context.Context, cik string) (map[string]map[string][]XBRLFact, error) {
	if e.budget != nil {
		if err := e.budget.Check(ctx, apibudget.ProviderEdgar); err != nil {
			return nil, err
		}
	}
	u := fmt.Sprintf("https://data.sec.gov/api/xbrl/companyfacts/CIK%s.json", cik)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", e.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")

	res, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		if e.budget != nil {
			e.budget.BlockFor(ctx, apibudget.ProviderEdgar, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("edgar companyfacts: %s", strings.TrimSpace(string(body)))
	}
	if res.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("edgar: company facts not found for cik %s", cik)
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("edgar companyfacts: %s", strings.TrimSpace(string(body)))
	}

	var payload struct {
		Facts map[string]map[string]struct {
			Units map[string][]struct {
				End   string  `json:"end"`
				Start string  `json:"start"`
				Val   float64 `json:"val"`
				FY    int     `json:"fy"`
				FP    string  `json:"fp"`
				Form  string  `json:"form"`
				Filed string  `json:"filed"`
			} `json:"units"`
		} `json:"facts"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if e.budget != nil {
		e.budget.Record(ctx, apibudget.ProviderEdgar)
	}

	out := make(map[string]map[string][]XBRLFact)
	for taxonomy, tags := range payload.Facts {
		if out[taxonomy] == nil {
			out[taxonomy] = make(map[string][]XBRLFact)
		}
		for tag, concept := range tags {
			for _, unitItems := range concept.Units {
				for _, item := range unitItems {
					out[taxonomy][tag] = append(out[taxonomy][tag], XBRLFact{
						End:   item.End,
						Start: item.Start,
						Val:   item.Val,
						FY:    item.FY,
						FP:    item.FP,
						Form:  item.Form,
						Filed: item.Filed,
					})
				}
			}
		}
	}
	return out, nil
}
