package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

const (
	naverAPIHubBaseURL  = "https://naverapihub.apigw.ntruss.com"
	naverNewsSearchPath = "/search/v1/news"
)

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

type RawNewsItem struct {
	Title       string
	Description string
	Link        string
	PublishedAt time.Time
	SourceID    string
}

type NaverNewsClient struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
	budget       *apibudget.Manager
}

func NewNaverNewsClient(clientID, clientSecret string, httpClient *http.Client, budget *apibudget.Manager) *NaverNewsClient {
	return &NaverNewsClient{
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
		httpClient:   httpClient,
		budget:       budget,
	}
}

func (c *NaverNewsClient) Enabled() bool {
	return c != nil && c.clientID != "" && c.clientSecret != ""
}

func (c *NaverNewsClient) Search(ctx context.Context, query string, display int) ([]RawNewsItem, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("naver news not configured")
	}
	if display <= 0 {
		display = 10
	}
	if display > 20 {
		display = 20
	}
	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderNaver); err != nil {
			return nil, err
		}
	}

	u := fmt.Sprintf(
		"%s%s?query=%s&display=%d&sort=date",
		naverAPIHubBaseURL,
		naverNewsSearchPath,
		url.QueryEscape(strings.TrimSpace(query)),
		display,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-NCP-APIGW-API-KEY-ID", c.clientID)
	req.Header.Set("X-NCP-APIGW-API-KEY", c.clientSecret)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests {
		if c.budget != nil {
			c.budget.BlockFor(ctx, apibudget.ProviderNaver, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		return nil, fmt.Errorf("naver news rate limited")
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("naver news status %d", res.StatusCode)
	}

	var payload struct {
		Items []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Link        string `json:"link"`
			PubDate     string `json:"pubDate"`
		} `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if c.budget != nil {
		c.budget.Record(ctx, apibudget.ProviderNaver)
	}

	out := make([]RawNewsItem, 0, len(payload.Items))
	for _, item := range payload.Items {
		link := strings.TrimSpace(item.Link)
		if link == "" {
			continue
		}
		out = append(out, RawNewsItem{
			Title:       stripHTML(item.Title),
			Description: stripHTML(item.Description),
			Link:        link,
			PublishedAt: ParseNewsPubDate(item.PubDate),
			SourceID:    "naver",
		})
	}
	return out, nil
}

type KakaoNewsClient struct {
	apiKey     string
	httpClient *http.Client
	budget     *apibudget.Manager
}

func NewKakaoNewsClient(apiKey string, httpClient *http.Client, budget *apibudget.Manager) *KakaoNewsClient {
	return &KakaoNewsClient{
		apiKey:     strings.TrimSpace(apiKey),
		httpClient: httpClient,
		budget:     budget,
	}
}

func (c *KakaoNewsClient) Enabled() bool {
	return c != nil && c.apiKey != ""
}

func (c *KakaoNewsClient) Search(ctx context.Context, query string, size int) ([]RawNewsItem, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("kakao news not configured")
	}
	if size <= 0 {
		size = 5
	}
	if size > 10 {
		size = 10
	}
	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderKakao); err != nil {
			return nil, err
		}
	}

	u := fmt.Sprintf(
		"https://dapi.kakao.com/v2/search/web?query=%s&size=%d&sort=recency",
		url.QueryEscape(strings.TrimSpace(query)),
		size,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "KakaoAK "+c.apiKey)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests {
		if c.budget != nil {
			c.budget.BlockFor(ctx, apibudget.ProviderKakao, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		return nil, fmt.Errorf("kakao search rate limited")
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kakao search status %d", res.StatusCode)
	}

	var payload struct {
		Documents []struct {
			Title       string `json:"title"`
			Contents    string `json:"contents"`
			URL         string `json:"url"`
			Datetime    string `json:"datetime"`
		} `json:"documents"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if c.budget != nil {
		c.budget.Record(ctx, apibudget.ProviderKakao)
	}

	out := make([]RawNewsItem, 0, len(payload.Documents))
	for _, doc := range payload.Documents {
		link := strings.TrimSpace(doc.URL)
		if link == "" || !looksLikeNewsURL(link) {
			continue
		}
		out = append(out, RawNewsItem{
			Title:       stripHTML(doc.Title),
			Description: stripHTML(doc.Contents),
			Link:        link,
			PublishedAt: ParseNewsPubDate(doc.Datetime),
			SourceID:    "kakao",
		})
	}
	return out, nil
}

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(s)
}

func looksLikeNewsURL(link string) bool {
	link = strings.ToLower(link)
	keywords := []string{
		"news", "media", "fnnews", "mk.co.kr", "hankyung", "sedaily", "yna.co.kr",
		"etnews", "mt.co.kr", "bizwatch", "newsis", "herald", "chosun", "joongang",
		"donga", "hani", "kbs.co.kr", "sbs.co.kr", "mbn.co.kr", "naver.com",
	}
	for _, kw := range keywords {
		if strings.Contains(link, kw) {
			return true
		}
	}
	return false
}
