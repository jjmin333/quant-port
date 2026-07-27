package llm

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

type Config struct {
	APIKey  string
	BaseURL string
	Model   string
}

func (c Config) Enabled() bool {
	return c.APIKey != ""
}

type Client struct {
	cfg        Config
	httpClient *http.Client
}

func NewClient(cfg Config) *Client {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	model := cfg.Model
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &Client{
		cfg: Config{APIKey: cfg.APIKey, BaseURL: base, Model: model},
		httpClient: &http.Client{Timeout: 45 * time.Second},
	}
}

type BriefingLLMOutput struct {
	Summary    string   `json:"summary"`
	Highlights []string `json:"highlights"`
	Volatility string   `json:"volatility"`
}

func (c *Client) Enabled() bool {
	return c.cfg.Enabled()
}

func (c *Client) GenerateBriefing(ctx context.Context, factsJSON string) (*BriefingLLMOutput, error) {
	if !c.cfg.Enabled() {
		return nil, fmt.Errorf("llm not configured")
	}

	system := `You are a portfolio briefing assistant for a Korean investor.
Use ONLY the JSON facts provided. Do not invent events or prices.
Respond with valid JSON only: {"summary":"...","highlights":["..."],"volatility":"낮음|보통|높음"}
Write summary in Korean (2 sentences max). highlights: up to 4 short Korean phrases.
This is informational only, not investment advice.`

	reqBody := map[string]any{
		"model": c.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": factsJSON},
		},
		"temperature":     0.2,
		"response_format": map[string]string{"type": "json_object"},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("llm api %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}
	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("llm empty response")
	}

	var out BriefingLLMOutput
	if err := json.Unmarshal([]byte(apiResp.Choices[0].Message.Content), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
