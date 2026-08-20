package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

const (
	papagoTextTranslateURL = "https://papago.apigw.ntruss.com/nmt/v1/translation"
	papagoWebTranslateURL  = "https://papago.apigw.ntruss.com/web-trans/v1/translate"
	papagoWebHTMLMaxBytes  = 200_000
)

type PapagoClient struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
	budget       *apibudget.Manager
}

func NewPapagoClient(clientID, clientSecret string, httpClient *http.Client, budget *apibudget.Manager) *PapagoClient {
	return &PapagoClient{
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
		httpClient:   httpClient,
		budget:       budget,
	}
}

func (c *PapagoClient) Enabled() bool {
	return c != nil && c.clientID != "" && c.clientSecret != ""
}

func (c *PapagoClient) TranslateENToKO(ctx context.Context, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	if !c.Enabled() {
		return "", fmt.Errorf("papago not configured")
	}
	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderPapagoText); err != nil {
			return "", err
		}
	}

	body := url.Values{}
	body.Set("source", "en")
	body.Set("target", "ko")
	body.Set("text", truncateRunes(text, 4500))

	payload, err := c.postForm(ctx, papagoTextTranslateURL, body)
	if err != nil {
		return "", err
	}

	var parsed struct {
		Message struct {
			Result struct {
				TranslatedText string `json:"translatedText"`
			} `json:"result"`
		} `json:"message"`
	}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return "", err
	}
	out := strings.TrimSpace(parsed.Message.Result.TranslatedText)
	if out == "" {
		return "", fmt.Errorf("papago empty translation")
	}
	if c.budget != nil {
		c.budget.Record(ctx, apibudget.ProviderPapagoText)
	}
	return out, nil
}

func (c *PapagoClient) TranslateWebsiteENToKO(ctx context.Context, html string) (string, error) {
	html = strings.TrimSpace(html)
	if html == "" {
		return "", fmt.Errorf("empty html")
	}
	if !c.Enabled() {
		return "", fmt.Errorf("papago not configured")
	}
	if len(html) > papagoWebHTMLMaxBytes {
		html = truncateUTF8Bytes(html, papagoWebHTMLMaxBytes)
	}
	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderPapagoWeb); err != nil {
			return "", err
		}
	}

	body := url.Values{}
	body.Set("source", "en")
	body.Set("target", "ko")
	body.Set("html", html)

	payload, err := c.postForm(ctx, papagoWebTranslateURL, body)
	if err != nil {
		return "", err
	}

	var parsed struct {
		StatusCode int    `json:"status_code"`
		Data       string `json:"data"`
		Message    struct {
			Result struct {
				TranslatedHTML string `json:"translatedHTML"`
			} `json:"result"`
		} `json:"message"`
	}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		trimmed := strings.TrimSpace(string(payload))
		if trimmed != "" && strings.Contains(trimmed, "<") {
			if c.budget != nil {
				c.budget.Record(ctx, apibudget.ProviderPapagoWeb)
			}
			return trimmed, nil
		}
		return "", err
	}
	out := strings.TrimSpace(parsed.Data)
	if out == "" {
		out = strings.TrimSpace(parsed.Message.Result.TranslatedHTML)
	}
	if out == "" {
		return "", fmt.Errorf("papago empty website translation")
	}
	if c.budget != nil {
		c.budget.Record(ctx, apibudget.ProviderPapagoWeb)
	}
	return out, nil
}

func (c *PapagoClient) postForm(ctx context.Context, endpoint string, body url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-NCP-APIGW-API-KEY-ID", c.clientID)
	req.Header.Set("X-NCP-APIGW-API-KEY", c.clientSecret)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("papago rate limited")
	}
	if res.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("papago status %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	return io.ReadAll(io.LimitReader(res.Body, 4<<20))
}

func truncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n >= max {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

func truncateUTF8Bytes(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	for maxBytes > 0 && maxBytes < len(s) && (s[maxBytes]&0xC0) == 0x80 {
		maxBytes--
	}
	return s[:maxBytes]
}
