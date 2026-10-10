package stocklogo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const naverStockBasicURL = "https://m.stock.naver.com/api/stock/%s/basic"

// NaverLogoURL is the Naver CDN PNG for a KRX symbol — the same file `itemLogoPngUrl` points at.
func NaverLogoURL(symbol string) string {
	return "https://ssl.pstatic.net/imgstock/fn/real/logo/png/stock/Stock" + url.PathEscape(symbol) + ".png"
}

type naverStockBasic struct {
	ItemLogoPngURL string `json:"itemLogoPngUrl"`
	ItemLogoURL    string `json:"itemLogoUrl"`
}

// FetchNaverLogoPNGURL returns the Naver Finance CDN PNG URL for a KRX symbol (6 digits).
func FetchNaverLogoPNGURL(ctx context.Context, client *http.Client, symbol string) (string, error) {
	sym := strings.TrimSpace(symbol)
	if sym == "" {
		return "", fmt.Errorf("empty symbol")
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(naverStockBasicURL, sym), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "quant-port-migrate-logos/1.0")

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("naver basic api: HTTP %d", res.StatusCode)
	}

	var body naverStockBasic
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return "", err
	}
	url := strings.TrimSpace(body.ItemLogoPngURL)
	if url == "" {
		url = strings.TrimSpace(body.ItemLogoURL)
	}
	if url == "" {
		return "", fmt.Errorf("naver basic api: no logo url")
	}
	return url, nil
}
