package toss

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/config"
)

const (
	accountHeader = "X-Tossinvest-Account"
	defaultTimeout  = 20 * time.Second
)

// Client is the reusable Toss Securities Open API HTTP client.
// Domain methods are grouped in separate files (market_data.go, asset.go, ...).
type Client struct {
	baseURL    string
	httpClient *http.Client
	auth       *TokenManager
}

func NewClient(cfg config.TossConfig, tokenStore TokenStore) (*Client, error) {
	return NewClientForCredentials(cfg.BaseURL, cfg.ClientID, cfg.ClientSecret, tokenStore, tokenCacheKey)
}

func NewClientForCredentials(baseURL, clientID, clientSecret string, tokenStore TokenStore, redisTokenKey string) (*Client, error) {
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("toss credentials are not configured")
	}

	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		auth: NewTokenManagerWithKey(baseURL, clientID, clientSecret, tokenStore, redisTokenKey),
	}, nil
}

func (c *Client) Enabled() bool {
	return c != nil && c.auth != nil
}

// StartTokenRefresh runs proactive background OAuth token renewal.
func (c *Client) StartTokenRefresh(ctx context.Context) {
	if c != nil && c.auth != nil {
		c.auth.StartBackgroundRefresh(ctx)
	}
}

func (c *Client) get(ctx context.Context, path string, query url.Values, accountSeq *int64, dest any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, accountSeq, dest)
}

func (c *Client) postJSON(ctx context.Context, path string, body any, accountSeq *int64, dest any) error {
	return c.do(ctx, http.MethodPost, path, nil, body, accountSeq, dest)
}

func (c *Client) delete(ctx context.Context, path string, accountSeq *int64, dest any) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil, accountSeq, dest)
}

func (c *Client) do(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	accountSeq *int64,
	dest any,
) error {
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.auth.AccessToken(ctx)
		if err != nil {
			return err
		}

		raw, status, rl, err := c.doOnce(ctx, method, path, query, body, accountSeq, token)
		if err != nil {
			return err
		}

		if status == http.StatusNoContent {
			return nil
		}

		if status >= 400 {
			var errBody ErrorBody
			if json.Unmarshal(raw, &errBody) == nil && errBody.Error.Code != "" {
				apiErr := newErrorFromAPI(status, errBody, rl)
				if IsInvalidTokenError(apiErr) && attempt == 0 {
					c.auth.Invalidate(ctx)
					continue
				}
				return apiErr
			}
			if status == http.StatusUnauthorized && attempt == 0 {
				c.auth.Invalidate(ctx)
				continue
			}
			return fmt.Errorf("toss api %d: %s", status, strings.TrimSpace(string(raw)))
		}

		if dest == nil {
			return nil
		}
		if err := json.Unmarshal(raw, dest); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return nil
	}
	return fmt.Errorf("toss api: unauthorized after token refresh")
}

func (c *Client) doOnce(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	accountSeq *int64,
	token string,
) ([]byte, int, RateLimitStatus, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, 0, RateLimitStatus{}, fmt.Errorf("parse url: %w", err)
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, 0, RateLimitStatus{}, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, 0, RateLimitStatus{}, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accountSeq != nil {
		req.Header.Set(accountHeader, strconv.FormatInt(*accountSeq, 10))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, RateLimitStatus{}, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	rl := parseRateLimitStatus(resp.Header)
	logRateLimitStatus(method, path, resp.StatusCode, rl)

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, rl, fmt.Errorf("read body: %w", err)
	}
	return raw, resp.StatusCode, rl, nil
}

// WithAccount is a helper for account-scoped calls.
func WithAccount(accountSeq int64) *int64 {
	v := accountSeq
	return &v
}
