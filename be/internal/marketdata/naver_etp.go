package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

const (
	naverETFListURL = "https://finance.naver.com/api/sise/etfItemList.nhn"
	naverETNListURL = "https://finance.naver.com/api/sise/etnItemList.nhn"
)

type naverETPItem struct {
	ItemCode string `json:"itemcode"`
	ItemName string `json:"itemname"`
}

type naverETPListResponse struct {
	Result struct {
		ETFItemList []naverETPItem `json:"etfItemList"`
		ETNItemList []naverETPItem `json:"etnItemList"`
	} `json:"result"`
}

// ListDomesticETPFromNaver fetches domestic ETF+ETN catalog (2 HTTP calls, no API key).
func ListDomesticETPFromNaver(ctx context.Context, httpClient *http.Client) ([]model.Stock, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	etfRows, err := fetchNaverETPList(ctx, httpClient, naverETFListURL, "ETF")
	if err != nil {
		return nil, fmt.Errorf("naver etf: %w", err)
	}
	time.Sleep(500 * time.Millisecond)

	etnRows, err := fetchNaverETPList(ctx, httpClient, naverETNListURL, "ETN")
	if err != nil {
		return nil, fmt.Errorf("naver etn: %w", err)
	}

	out := make([]model.Stock, 0, len(etfRows)+len(etnRows))
	out = append(out, etfRows...)
	out = append(out, etnRows...)
	return out, nil
}

func fetchNaverETPList(ctx context.Context, client *http.Client, endpoint, secType string) ([]model.Stock, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "quant-port-sync-etp/1.0")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 256))
		return nil, fmt.Errorf("status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	utf8JSON, err := decodeNaverFinanceBody(raw, res.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("decode body: %w", err)
	}

	var payload naverETPListResponse
	if err := json.Unmarshal(utf8JSON, &payload); err != nil {
		return nil, err
	}

	items := payload.Result.ETFItemList
	if len(items) == 0 {
		items = payload.Result.ETNItemList
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("empty %s list", secType)
	}

	out := make([]model.Stock, 0, len(items))
	for _, row := range items {
		sym := strings.ToUpper(strings.TrimSpace(row.ItemCode))
		name := strings.TrimSpace(row.ItemName)
		if sym == "" || name == "" {
			continue
		}
		out = append(out, model.Stock{
			Symbol:       sym,
			Name:         name,
			EnglishName:  name,
			Market:       "KRX",
			SecurityType: secType,
			Currency:     model.CurrencyKRW,
			AssetType:    model.AssetETF,
			Status:       "NORMAL",
		})
	}
	return out, nil
}

// IsKRXUnauthorized reports Open API 401 / subscription errors.
func IsKRXUnauthorized(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized")
}
