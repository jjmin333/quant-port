package marketdata



import (

	"context"

	"encoding/json"

	"fmt"

	"net/http"

	"net/url"

	"strconv"

	"strings"

	"time"



	"github.com/enact/quant-port/be/internal/marketdata/apibudget"

)



type FREDClient struct {

	apiKey     string

	httpClient *http.Client

	budget     *apibudget.Manager

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



// QuoteBatch fetches multiple FRED series with a single apibudget Check/Record.

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

	for _, seriesID := range seriesIDs {

		q, err := c.quoteWithSparklineNoBudget(ctx, seriesID, sparkLimit)

		if err != nil {

			continue

		}

		out[seriesID] = q

	}

	if len(out) == 0 {

		return nil, fmt.Errorf("fred batch: all series failed")

	}

	if c.budget != nil {

		c.budget.Record(ctx, apibudget.ProviderFRED)

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

		"series_id":  {seriesID},

		"api_key":    {c.apiKey},

		"file_type":  {"json"},

		"sort_order": {"desc"},

		"limit":      {"1"},

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



func (c *FREDClient) recentValues(ctx context.Context, seriesID string, limit int) ([]float64, error) {

	if c.budget != nil {

		if err := c.budget.Check(ctx, apibudget.ProviderFRED); err != nil {

			return nil, err

		}

	}

	values, err := c.recentValuesNoBudget(ctx, seriesID, limit)

	if err != nil {

		return nil, err

	}

	if c.budget != nil {

		c.budget.Record(ctx, apibudget.ProviderFRED)

	}

	return values, nil

}



func (c *FREDClient) recentValuesNoBudget(ctx context.Context, seriesID string, limit int) ([]float64, error) {

	q := url.Values{

		"series_id":  {seriesID},

		"api_key":    {c.apiKey},

		"file_type":  {"json"},

		"sort_order": {"desc"},

		"limit":      {strconv.Itoa(limit)},

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
