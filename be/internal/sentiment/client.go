// Package sentiment talks to the local headline sentiment classifier
// (be/sentiment — KR-FinBert-SC for Korean, FinBERT for English).
package sentiment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Item is one headline to classify.
type Item struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Lang string `json:"lang"` // ko | en
}

// Result is the classifier's answer for one item.
type Result struct {
	ID    string  `json:"id"`
	Label string  `json:"label"` // positive | neutral | negative
	Score float64 `json:"score"` // P(positive) − P(negative), −1..1
	Model string  `json:"model"`
}

// Client is a thin HTTP client. A zero BaseURL means "not configured".
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// New returns a client for baseURL ("" disables it).
func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		// 64건 한 묶음이 CPU에서 1~2초다. 모델 적재 직후 첫 요청만 느리다.
		HTTP: &http.Client{Timeout: 60 * time.Second},
	}
}

// Enabled reports whether a classifier URL is configured.
func (c *Client) Enabled() bool { return c != nil && c.BaseURL != "" }

// Healthy pings /health — the batch checks it once before labelling so a
// missing container is one log line, not thousands of failed requests.
func (c *Client) Healthy(ctx context.Context) error {
	if !c.Enabled() {
		return fmt.Errorf("sentiment classifier not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/health", nil)
	if err != nil {
		return err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("sentiment health: HTTP %d", res.StatusCode)
	}
	return nil
}

// Classify sends one batch and returns results keyed by the caller's ids.
func (c *Client) Classify(ctx context.Context, items []Item) ([]Result, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("sentiment classifier not configured")
	}
	body, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/classify", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sentiment classify: HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(raw[:min(len(raw), 200)])))
	}
	var payload struct {
		Results []Result `json:"results"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("sentiment classify: %w", err)
	}
	return payload.Results, nil
}
