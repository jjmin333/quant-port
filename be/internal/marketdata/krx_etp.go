package marketdata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
)

const (
	krxETFTradeURL = "https://data-dbg.krx.co.kr/svc/apis/etp/etf_bydd_trd"
	krxETNTradeURL = "https://data-dbg.krx.co.kr/svc/apis/etp/etn_bydd_trd"
)

// KRXClient fetches domestic ETP listings from KRX data-dbg API.
type KRXClient struct {
	apiKey     string
	httpClient *http.Client
	budget     *apibudget.Manager
}

func NewKRXClient(apiKey string, httpClient *http.Client, budget *apibudget.Manager) *KRXClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 45 * time.Second}
	}
	return &KRXClient{
		apiKey:     strings.TrimSpace(apiKey),
		httpClient: httpClient,
		budget:     budget,
	}
}

func (c *KRXClient) Enabled() bool {
	return c != nil && c.apiKey != ""
}

// ListDomesticETP returns ETF and ETN rows for basDd (YYYYMMDD).
// When basDd is empty or has no rows yet (same-day lag), walks back to the latest KST weekday with data.
func (c *KRXClient) ListDomesticETP(ctx context.Context, basDd string) ([]model.Stock, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("krx api key not configured")
	}
	basDd = strings.TrimSpace(basDd)
	if basDd == "" {
		basDd = KRXTtradeDate(time.Now())
	}

	var etfRows []model.Stock
	var err error
	resolved := basDd
	for i := 0; i < 7; i++ {
		etfRows, err = c.fetchETPEndpoint(ctx, krxETFTradeURL, resolved, "ETF")
		if err != nil {
			return nil, fmt.Errorf("krx etf: %w", err)
		}
		if len(etfRows) > 0 {
			break
		}
		if i < 6 {
			time.Sleep(2 * time.Second)
		}
		resolved = previousKRXWeekday(resolved)
	}
	if len(etfRows) == 0 {
		return nil, fmt.Errorf("krx etp: no etf rows in last 7 weekdays (from %s)", basDd)
	}

	time.Sleep(2 * time.Second)

	etnRows, err := c.fetchETPEndpoint(ctx, krxETNTradeURL, resolved, "ETN")
	if err != nil {
		return nil, fmt.Errorf("krx etn: %w", err)
	}

	out := make([]model.Stock, 0, len(etfRows)+len(etnRows))
	out = append(out, etfRows...)
	out = append(out, etnRows...)
	return out, nil
}

func (c *KRXClient) fetchETPEndpoint(ctx context.Context, endpoint, basDd, secType string) ([]model.Stock, error) {
	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderKRX); err != nil {
			return nil, err
		}
	}

	body, err := json.Marshal(map[string]string{"basDd": basDd})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("AUTH_KEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "quant-port-sync-etp/1.0")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests {
		if c.budget != nil {
			c.budget.BlockFor(ctx, apibudget.ProviderKRX, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		return nil, fmt.Errorf("krx: status 429")
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("krx: status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	rows, err := parseKRXETPResponse(raw)
	if err != nil {
		return nil, err
	}
	if c.budget != nil && len(rows) > 0 {
		c.budget.Record(ctx, apibudget.ProviderKRX)
	}
	return mapKRXETPRows(rows, secType), nil
}

type krxETPRow struct {
	ISUCD     string `json:"ISU_CD"`
	ISUSRTCD  string `json:"ISU_SRT_CD"`
	ISUNM     string `json:"ISU_NM"`
	ISUABBRV  string `json:"ISU_ABBRV"`
	MKTNM     string `json:"MKT_NM"`
	LISTDD    string `json:"LIST_DD"`
}

func parseKRXETPResponse(raw []byte) ([]krxETPRow, error) {
	var direct []krxETPRow
	if err := json.Unmarshal(raw, &direct); err == nil && len(direct) > 0 {
		return direct, nil
	}

	var wrapped struct {
		OutBlock1 []krxETPRow `json:"OutBlock_1"`
		Block1    []krxETPRow `json:"block1"`
		Items     []krxETPRow `json:"items"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, fmt.Errorf("krx etp json: %w", err)
	}
	switch {
	case len(wrapped.OutBlock1) > 0:
		return wrapped.OutBlock1, nil
	case len(wrapped.Block1) > 0:
		return wrapped.Block1, nil
	case len(wrapped.Items) > 0:
		return wrapped.Items, nil
	default:
		return []krxETPRow{}, nil
	}
}

func mapKRXETPRows(rows []krxETPRow, secType string) []model.Stock {
	out := make([]model.Stock, 0, len(rows))
	for _, row := range rows {
		sym := strings.ToUpper(strings.TrimSpace(row.ISUSRTCD))
		if sym == "" {
			sym = strings.ToUpper(strings.TrimSpace(row.ISUCD))
		}
		name := strings.TrimSpace(row.ISUABBRV)
		if name == "" {
			name = strings.TrimSpace(row.ISUNM)
		}
		if sym == "" || name == "" {
			continue
		}
		market := mapKRXMLMarket(row.MKTNM)
		if market == "" {
			market = "KRX"
		}
		var listDate *time.Time
		if row.LISTDD != "" {
			if t, err := time.Parse("20060102", strings.ReplaceAll(row.LISTDD, "/", "")); err == nil {
				listDate = &t
			} else if t, err := time.Parse("2006/01/02", row.LISTDD); err == nil {
				listDate = &t
			}
		}
		out = append(out, model.Stock{
			Symbol:       sym,
			Name:         name,
			EnglishName:  strings.TrimSpace(row.ISUNM),
			ISINCode:     strings.TrimSpace(row.ISUCD),
			Market:       market,
			SecurityType: secType,
			Currency:     model.CurrencyKRW,
			AssetType:    model.AssetETF,
			Status:       "NORMAL",
			ListDate:     listDate,
		})
	}
	return out
}

func mapKRXMLMarket(raw string) string {
	u := strings.ToUpper(strings.TrimSpace(raw))
	switch {
	case strings.Contains(u, "KOSDAQ"):
		return "KOSDAQ"
	case strings.Contains(u, "KONEX"):
		return "KONEX"
	case strings.Contains(u, "KOSPI"), strings.Contains(u, "KRX"):
		return "KRX"
	default:
		return ""
	}
}

// KRXTtradeDate returns YYYYMMDD for the latest KST weekday (skip Sat/Sun).
func KRXTtradeDate(now time.Time) string {
	t := now.In(time.FixedZone("KST", 9*3600))
	for t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		t = t.AddDate(0, 0, -1)
	}
	return t.Format("20060102")
}

func previousKRXWeekday(basDd string) string {
	t, err := time.ParseInLocation("20060102", strings.TrimSpace(basDd), time.FixedZone("KST", 9*3600))
	if err != nil {
		return KRXTtradeDate(time.Now().AddDate(0, 0, -1))
	}
	t = t.AddDate(0, 0, -1)
	for t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		t = t.AddDate(0, 0, -1)
	}
	return t.Format("20060102")
}
