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
	"github.com/redis/go-redis/v9"
)

const goldSpotCacheKeyFmt = "market:gold:spot:v1:%s:%s"
const goldSpotFreshFor = 90 * time.Minute
const goldSpotStoreTTL = 45 * 24 * time.Hour

type GoldPriceClient struct {
	apiKey     string
	httpClient *http.Client
	budget     *apibudget.Manager
	redis      *redis.Client
}

type cachedGoldQuote struct {
	Quote     Quote     `json:"quote"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func NewGoldPriceClient(apiKey string, httpClient *http.Client, budget *apibudget.Manager, redisClient *redis.Client) *GoldPriceClient {
	return &GoldPriceClient{
		apiKey:     strings.TrimSpace(apiKey),
		httpClient: httpClient,
		budget:     budget,
		redis:      redisClient,
	}
}

func (c *GoldPriceClient) Enabled() bool {
	return c != nil && c.apiKey != ""
}

func (c *GoldPriceClient) Spot(ctx context.Context, symbol, currency string) (Quote, error) {
	if !c.Enabled() {
		return Quote{}, fmt.Errorf("goldprice not configured")
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		symbol = "XAU"
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		currency = "USD"
	}

	if q, ok := c.loadCacheFresh(ctx, symbol, currency); ok {
		return q, nil
	}

	stale, hasStale := c.loadCacheAny(ctx, symbol, currency)

	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderGoldPrice); err != nil {
			if hasStale {
				return stale, nil
			}
			return Quote{}, err
		}
	}

	q, err := c.fetchSpot(ctx, symbol, currency)
	if err != nil {
		if isProviderRateLimit(err) {
			if c.budget != nil {
				c.budget.BlockFor(ctx, apibudget.ProviderGoldPrice, time.Hour)
			}
		}
		if hasStale {
			return stale, nil
		}
		return Quote{}, err
	}

	if c.budget != nil {
		c.budget.Record(ctx, apibudget.ProviderGoldPrice)
	}
	c.storeCache(ctx, symbol, currency, q)
	return q, nil
}

func (c *GoldPriceClient) fetchSpot(ctx context.Context, symbol, currency string) (Quote, error) {
	q := url.Values{"currency": {currency}}
	u := fmt.Sprintf("https://api.goldprice.dev/v1/spot/%s?%s", url.PathEscape(symbol), q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Quote{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	res, err := c.httpClient.Do(req)
	if err != nil {
		return Quote{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		return Quote{}, fmt.Errorf("goldprice %s/%s: status 429", symbol, currency)
	}
	if res.StatusCode != http.StatusOK {
		return Quote{}, fmt.Errorf("goldprice %s/%s: status %d", symbol, currency, res.StatusCode)
	}
	var payload struct {
		Price string `json:"price"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return Quote{}, err
	}
	price, err := strconv.ParseFloat(strings.TrimSpace(payload.Price), 64)
	if err != nil || price <= 0 {
		return Quote{}, fmt.Errorf("goldprice %s/%s: invalid price", symbol, currency)
	}
	return Quote{Value: price, AsOf: time.Now().UTC()}, nil
}

func (c *GoldPriceClient) loadCacheFresh(ctx context.Context, symbol, currency string) (Quote, bool) {
	q, cached, ok := c.readCache(ctx, symbol, currency)
	if !ok || cached.UpdatedAt.IsZero() {
		return Quote{}, false
	}
	if time.Since(cached.UpdatedAt) > goldSpotFreshFor {
		return Quote{}, false
	}
	return q, true
}

func (c *GoldPriceClient) loadCacheAny(ctx context.Context, symbol, currency string) (Quote, bool) {
	q, _, ok := c.readCache(ctx, symbol, currency)
	return q, ok
}

func (c *GoldPriceClient) readCache(ctx context.Context, symbol, currency string) (Quote, cachedGoldQuote, bool) {
	if c.redis == nil {
		return Quote{}, cachedGoldQuote{}, false
	}
	raw, err := c.redis.Get(ctx, fmt.Sprintf(goldSpotCacheKeyFmt, symbol, currency)).Bytes()
	if err != nil || len(raw) == 0 {
		return Quote{}, cachedGoldQuote{}, false
	}
	var cached cachedGoldQuote
	if json.Unmarshal(raw, &cached) != nil || cached.Quote.Value <= 0 {
		return Quote{}, cachedGoldQuote{}, false
	}
	q := cached.Quote
	if q.AsOf.IsZero() {
		q.AsOf = cached.UpdatedAt
	}
	return q, cached, true
}

func (c *GoldPriceClient) storeCache(ctx context.Context, symbol, currency string, q Quote) {
	if c.redis == nil || q.Value <= 0 {
		return
	}
	payload, err := json.Marshal(cachedGoldQuote{Quote: q, UpdatedAt: time.Now().UTC()})
	if err != nil {
		return
	}
	_ = c.redis.Set(ctx, fmt.Sprintf(goldSpotCacheKeyFmt, symbol, currency), payload, goldSpotStoreTTL).Err()
}

func isProviderRateLimit(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "quota") ||
		strings.Contains(msg, "한도")
}
