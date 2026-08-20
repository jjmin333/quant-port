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

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
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
	// provider — 어느 한도 버킷으로 셀지. 기본은 요청 경로용 ProviderEdgar이고,
	// 배치 워커는 UseBatchBudget()으로 전용 버킷으로 옮긴다.
	provider apibudget.Provider

	mu          sync.RWMutex
	tickerToCIK map[string]string
	tickersAt   time.Time
}

func NewEdgarClient(cfg EdgarConfig, budget *apibudget.Manager) *EdgarClient {
	return &EdgarClient{
		cfg:         cfg,
		httpClient:  &http.Client{Timeout: 45 * time.Second},
		budget:      budget,
		provider:    apibudget.ProviderEdgar,
		tickerToCIK: map[string]string{},
	}
}

// UseBatchBudget switches this client to the batch-only quota bucket.
//
// ProviderEdgar의 8/min은 사용자 요청 경로를 위한 값이라, 수천 종목을 도는
// 배치가 그 버킷을 쓰면 첫 두 종목에서 멈춘다. 같은 소스·같은 User-Agent이고
// 나누는 것은 "누가 언제 부르는지"다 — EarningsClient가 Finnhub에 대해 같은
// 일을 먼저 했다.
//
// 인스턴스 단위 설정이므로 워커의 클라이언트만 영향을 받는다. company_tickers
// 조회까지 함께 옮겨가는 것은 의도한 것이다: 그 한 호출이 요청 경로의 8/min에
// 걸리면 배치가 첫 종목에서 죽는다.
func (e *EdgarClient) UseBatchBudget() *EdgarClient {
	if e != nil {
		e.provider = apibudget.ProviderEdgarBulk
	}
	return e
}

// pace waits out the provider's configured minimum interval before the next
// call.
//
// apibudget.Check는 간격이 안 지나면 **에러를 반환한다** — 기다려 주지 않는다.
// 요청 경로에서는 그게 맞다(사용자를 세워둘 수 없다). 배치에서는 정반대라,
// 부르는 쪽이 스스로 간격을 지켜야 한다. 안 지키면 두 번째 호출부터 전부
// "min interval not elapsed"로 떨어지고, 그건 재시도해도 같다.
func (e *EdgarClient) pace(ctx context.Context) {
	if e == nil || e.budget == nil {
		return
	}
	interval := e.budget.LimitsFor(e.provider).MinInterval
	if interval <= 0 {
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(interval):
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
		FiscalPeriod:    "FY",
		Revenue:         revPrior,
		OperatingIncome: opPrior,
		NetIncome:       niPrior,
		TotalAssets:     asPrior,
		TotalEquity:     eqPrior,
		EPSBasic:        epsPrior,
		EPSDiluted:      firstNonNilFloat(epsDilPrior, epsPrior),
	}

	if revCurFact, _ := pickAnnualPairFacts(facts, revenueConceptFallbacks); revCurFact != nil {
		current.FiscalYear = revCurFact.FY
		current.PeriodEnd = parseFactDate(revCurFact.End)
		current.FiledAt = parseFactDate(revCurFact.Filed)
	}
	_ = opPrior
	return current, prior, shares, nil
}

// FetchQuarterlySnapshots returns up to limit real reported periods (10-Q/10-K),
// most recent first — for the 실적 tab's 분기별 실적 상세 section. Unlike
// FetchFinancialSnapshots (annual pair only), this surfaces whatever
// individual quarters/years SEC has on file, with no derived/backfilled data.
func (e *EdgarClient) FetchQuarterlySnapshots(ctx context.Context, ticker string, limit int) ([]model.FinancialSnapshot, error) {
	cik, err := e.LookupCIK(ctx, ticker)
	if err != nil {
		return nil, err
	}
	facts, err := e.fetchCompanyFacts(ctx, cik)
	if err != nil {
		return nil, err
	}

	revItems := pickConceptFacts(facts, revenueConceptFallbacks)
	periods := recentReportPeriods(revItems, limit)
	if len(periods) == 0 {
		return nil, fmt.Errorf("edgar: no quarterly periods found for %s", ticker)
	}

	opItems := pickConceptFacts(facts, operatingIncomeConceptFallbacks)
	niItems := pickConceptFacts(facts, netIncomeConceptFallbacks)
	epsItems := pickConceptFacts(facts, epsBasicConceptFallbacks)
	epsDilItems := pickConceptFacts(facts, epsDilutedConceptFallbacks)
	asItems := pickConceptFacts(facts, assetsConceptFallbacks)
	eqItems := pickConceptFacts(facts, equityConceptFallbacks)
	liItems := pickConceptFacts(facts, liabilitiesConceptFallbacks)
	caItems := pickConceptFacts(facts, currentAssetsConceptFallbacks)
	clItems := pickConceptFacts(facts, currentLiabilitiesConceptFallbacks)

	out := make([]model.FinancialSnapshot, 0, len(periods))
	for _, key := range periods {
		snap := model.FinancialSnapshot{FiscalYear: key.FY, FiscalPeriod: key.FP}
		if f := pickFactForPeriod(revItems, key); f != nil {
			snap.Revenue = factInt64(f)
			snap.PeriodEnd = parseFactDate(f.End)
			snap.FiledAt = parseFactDate(f.Filed)
		}
		snap.OperatingIncome = factInt64(pickFactForPeriod(opItems, key))
		snap.NetIncome = factInt64(pickFactForPeriod(niItems, key))
		eps := factFloat64(pickFactForPeriod(epsItems, key))
		epsDil := factFloat64(pickFactForPeriod(epsDilItems, key))
		snap.EPSBasic = eps
		snap.EPSDiluted = firstNonNilFloat(epsDil, eps)
		// Balance-sheet items are point-in-time — matched by End date only,
		// not the standalone-duration rule that applies to flow items.
		snap.TotalAssets = factInt64(pickInstantFactForPeriod(asItems, key))
		snap.TotalEquity = factInt64(pickInstantFactForPeriod(eqItems, key))
		snap.TotalLiabilities = factInt64(pickInstantFactForPeriod(liItems, key))
		snap.CurrentAssets = factInt64(pickInstantFactForPeriod(caItems, key))
		snap.CurrentLiabilities = factInt64(pickInstantFactForPeriod(clItems, key))
		out = append(out, snap)
	}
	return out, nil
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
		if err := e.budget.Check(ctx, e.provider); err != nil {
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
			e.budget.BlockFor(ctx, e.provider, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
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
		e.budget.Record(ctx, e.provider)
	}
	return nil
}

func (e *EdgarClient) fetchCompanyFacts(ctx context.Context, cik string) (map[string]map[string][]XBRLFact, error) {
	if e.budget != nil {
		if err := e.budget.Check(ctx, e.provider); err != nil {
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
			e.budget.BlockFor(ctx, e.provider, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
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
		e.budget.Record(ctx, e.provider)
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
