package marketdata

import (
	"context"

	"encoding/json"

	"fmt"

	"net/http"

	"net/url"

	"strconv"

	"log"
	"strings"

	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

type FREDClient struct {
	apiKey string

	httpClient *http.Client

	budget *apibudget.Manager
}

func NewFREDClient(apiKey string, httpClient *http.Client, budget *apibudget.Manager) *FREDClient {
	return &FREDClient{apiKey: strings.TrimSpace(apiKey), httpClient: httpClient, budget: budget}
}

func (c *FREDClient) Enabled() bool {
	return c != nil && c.apiKey != ""
}

func (c *FREDClient) LatestQuote(ctx context.Context, seriesID string) (Quote, error) {
	return c.QuoteWithSparkline(ctx, seriesID, 0)
}

func (c *FREDClient) QuoteWithSparkline(ctx context.Context, seriesID string, sparkLimit int) (Quote, error) {
	out, err := c.QuoteBatch(ctx, []string{seriesID}, sparkLimit)

	if err != nil {
		return Quote{}, err
	}

	q, ok := out[seriesID]

	if !ok {
		return Quote{}, fmt.Errorf("fred %s: missing from batch", seriesID)
	}

	return q, nil
}

/*
QuoteBatch — 여러 시리즈를 한 번의 Check 로 묶어 받는다.

# Check 는 한 번, **Record 는 실제 호출 수만큼** (2026-09-11 사용자 요청 7)

Check 를 한 번만 하는 것은 의도다 — 최소 간격(3분)이 배치 중간에 끼어들면 시리즈
일부만 받고 끝난다(`apibudget.MinInterval` 머리글의 그 함정).

그런데 Record 도 한 번이었다. 이 함수는 시리즈마다 **HTTP 2회**를 낸다
(`recentValuesNoBudget` + `latestObservationDate`). 지수 6종이면 실제 12회인데
카운터에는 1이 적혔다 — 즉 우리 계기가 사용량을 **12분의 1로** 말하고 있었다.

FRED 한도(5,000/일)에 비해 실사용이 작아 사고로 이어지진 않았지만, 이 저장소의
규칙이 그것을 허용하지 않는다: 「우리 헤드룸이 상대의 잔여와 다른 숫자를 가리키면
상한을 올릴 수 없다」(quotaZone 머리글). 못 믿는 계기는 없는 계기보다 나쁘다.

# 부분 실패는 부분 실패라고 말한다

시리즈 하나가 실패하면 조용히 빠지고 **전부 실패했을 때만** 오류였다. 호출부는
「그 지수가 원래 없는 것」과 「이번에 못 받은 것」을 구분할 수 없었다. 이제 빠진
시리즈를 로그에 남긴다 — 지수 패널에 구멍이 났을 때 원인을 로그에서 찾을 수 있어야 한다.
*/
func (c *FREDClient) QuoteBatch(ctx context.Context, seriesIDs []string, sparkLimit int) (map[string]Quote, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("fred not configured")
	}

	if len(seriesIDs) == 0 {
		return map[string]Quote{}, nil
	}

	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderFRED); err != nil {
			return nil, err
		}
	}

	out := make(map[string]Quote, len(seriesIDs))
	// 시리즈당 상류 HTTP 2회(관측값 + 최신 관측일). 실패한 시리즈도 최소 1회는
	// 나갔으므로 함께 센다 — 안 센 호출이 상대의 카운터에는 남는다.
	httpCalls := 0
	var missing []string
	for _, seriesID := range seriesIDs {
		q, err := c.quoteWithSparklineNoBudget(ctx, seriesID, sparkLimit)
		if err != nil {
			httpCalls++
			missing = append(missing, seriesID)
			continue
		}
		httpCalls += 2
		out[seriesID] = q
	}
	if c.budget != nil {
		c.budget.RecordN(ctx, apibudget.ProviderFRED, httpCalls)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("fred batch: all series failed")
	}
	if len(missing) > 0 {
		// 부분 실패를 조용히 넘기지 않는다 — 지수 패널의 구멍이 여기서 설명된다.
		log.Printf("fred batch: %d/%d 시리즈 실패 — %s", len(missing), len(seriesIDs), strings.Join(missing, ","))
	}
	return out, nil
}

func (c *FREDClient) quoteWithSparklineNoBudget(ctx context.Context, seriesID string, sparkLimit int) (Quote, error) {
	limit := 2

	if sparkLimit > limit {
		limit = sparkLimit
	}

	values, err := c.recentValuesNoBudget(ctx, seriesID, limit)

	if err != nil {
		return Quote{}, err
	}

	if len(values) == 0 {
		return Quote{}, fmt.Errorf("fred %s: no observations", seriesID)
	}

	latest := values[0]

	out := Quote{Value: latest}

	if obsDate, ok := c.latestObservationDate(ctx, seriesID); ok {
		out.AsOf = obsDate
	}

	if len(values) >= 2 {
		prev := values[1]

		if prev > 0 {
			out.Change = latest - prev

			out.ChangeRate = (out.Change / prev) * 100
		}
	}

	if sparkLimit > 1 && len(values) > 1 {
		out.Sparkline = reverseFloats(values)
	}

	return out, nil
}

func (c *FREDClient) latestObservationDate(ctx context.Context, seriesID string) (time.Time, bool) {
	q := url.Values{
		"series_id": {seriesID},

		"api_key": {c.apiKey},

		"file_type": {"json"},

		"sort_order": {"desc"},

		"limit": {"1"},
	}

	req, err := http.NewRequestWithContext(

		ctx,

		http.MethodGet,

		"https://api.stlouisfed.org/fred/series/observations?"+q.Encode(),

		nil,
	)

	if err != nil {
		return time.Time{}, false
	}

	res, err := c.httpClient.Do(req)

	if err != nil {
		return time.Time{}, false
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return time.Time{}, false
	}

	var payload struct {
		Observations []struct {
			Date string `json:"date"`
		} `json:"observations"`
	}

	if json.NewDecoder(res.Body).Decode(&payload) != nil || len(payload.Observations) == 0 {
		return time.Time{}, false
	}

	t, err := time.Parse("2006-01-02", strings.TrimSpace(payload.Observations[0].Date))

	if err != nil {
		return time.Time{}, false
	}

	return t, true
}

func (c *FREDClient) recentValuesNoBudget(ctx context.Context, seriesID string, limit int) ([]float64, error) {
	q := url.Values{
		"series_id": {seriesID},

		"api_key": {c.apiKey},

		"file_type": {"json"},

		"sort_order": {"desc"},

		"limit": {strconv.Itoa(limit)},
	}

	req, err := http.NewRequestWithContext(

		ctx,

		http.MethodGet,

		"https://api.stlouisfed.org/fred/series/observations?"+q.Encode(),

		nil,
	)

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
			c.budget.BlockFor(ctx, apibudget.ProviderFRED, time.Minute)
		}

		return nil, fmt.Errorf("fred %s: status 429", seriesID)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fred %s: status %d", seriesID, res.StatusCode)
	}

	var payload struct {
		Observations []struct {
			Value string `json:"value"`
		} `json:"observations"`
	}

	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}

	var values []float64

	for _, obs := range payload.Observations {
		v, err := parseFredValue(obs.Value)

		if err != nil || v <= 0 {
			continue
		}

		values = append(values, v)
	}

	if len(values) == 0 {
		return nil, fmt.Errorf("fred %s: no valid observations", seriesID)
	}

	return values, nil
}

func parseFredValue(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)

	if raw == "" || raw == "." {
		return 0, fmt.Errorf("missing value")
	}

	return strconv.ParseFloat(raw, 64)
}

func reverseFloats(values []float64) []float64 {
	out := make([]float64, len(values))

	for i, v := range values {
		out[len(values)-1-i] = v
	}

	return out
}

// DatedObservation is one FRED data point with the date it belongs to.
type DatedObservation struct {
	Date  time.Time
	Value float64
}

// SeriesHistory returns dated observations, newest first, for backfilling an
// index's daily close history.
//
// The existing helpers deliberately drop the dates — a sparkline only needs the
// shape — but writing index_daily_closes rows requires knowing which session
// each value belongs to, so this keeps them.
func (c *FREDClient) SeriesHistory(ctx context.Context, seriesID string, limit int) ([]DatedObservation, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("fred not configured")
	}
	if limit <= 0 {
		limit = 60
	}
	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderFRED); err != nil {
			return nil, err
		}
		c.budget.Record(ctx, apibudget.ProviderFRED)
	}

	q := url.Values{
		"series_id":  {seriesID},
		"api_key":    {c.apiKey},
		"file_type":  {"json"},
		"sort_order": {"desc"},
		"limit":      {strconv.Itoa(limit)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.stlouisfed.org/fred/series/observations?"+q.Encode(), nil)
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
			c.budget.BlockFor(ctx, apibudget.ProviderFRED, time.Minute)
		}
		return nil, fmt.Errorf("fred %s: status 429", seriesID)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fred %s: status %d", seriesID, res.StatusCode)
	}

	var payload struct {
		Observations []struct {
			Date  string `json:"date"`
			Value string `json:"value"`
		} `json:"observations"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}

	out := make([]DatedObservation, 0, len(payload.Observations))
	for _, obs := range payload.Observations {
		// FRED marks missing sessions with "."; those are holidays, not zeros.
		raw := strings.TrimSpace(obs.Value)
		if raw == "" || raw == "." {
			continue
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v <= 0 {
			continue
		}
		d, err := time.Parse("2006-01-02", strings.TrimSpace(obs.Date))
		if err != nil {
			continue
		}
		out = append(out, DatedObservation{Date: d, Value: v})
	}
	return out, nil
}
