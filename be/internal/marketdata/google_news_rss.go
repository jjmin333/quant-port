package marketdata

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const googleNewsRSSBase = "https://news.google.com/rss/search"

type googleRSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Source      string `xml:"source"`
}

type googleRSSFeed struct {
	Channel struct {
		Items []googleRSSItem `xml:"item"`
	} `xml:"channel"`
}

type GoogleNewsRSSClient struct {
	httpClient *http.Client
}

func NewGoogleNewsRSSClient(httpClient *http.Client) *GoogleNewsRSSClient {
	return &GoogleNewsRSSClient{httpClient: httpClient}
}

func (c *GoogleNewsRSSClient) Enabled() bool {
	return c != nil && c.httpClient != nil
}

func (c *GoogleNewsRSSClient) Search(ctx context.Context, query string, maxItems int) ([]RawNewsItem, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query required")
	}
	if maxItems <= 0 {
		maxItems = 10
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("hl", "en-US")
	params.Set("gl", "US")
	params.Set("ceid", "US:en")

	feedURL := googleNewsRSSBase + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "quant-port-news/1.0")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("google news rss %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, err
	}

	var parsed googleRSSFeed
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	out := make([]RawNewsItem, 0, len(parsed.Channel.Items))
	for _, item := range parsed.Channel.Items {
		if len(out) >= maxItems {
			break
		}
		title := stripHTML(strings.TrimSpace(item.Title))
		link := strings.TrimSpace(item.Link)
		if title == "" || link == "" {
			continue
		}
		original := ResolveGoogleNewsURL(ctx, c.httpClient, link)
		if original == "" {
			original = link
		}
		pub := parseRSSPubDate(item.PubDate)
		desc := stripHTML(strings.TrimSpace(item.Description))
		publisher := strings.TrimSpace(item.Source)
		if publisher == "" {
			publisher = extractPublisherFromGoogleTitle(title)
		}

		out = append(out, RawNewsItem{
			Title:         title,
			Description:   desc,
			Link:          link,
			OriginalLink:  original,
			PublishedAt:   pub,
			SourceID:      "google_news",
			PublisherName: publisher,
		})
	}
	return out, nil
}

func extractPublisherFromGoogleTitle(title string) string {
	if idx := strings.LastIndex(title, " - "); idx > 0 && idx < len(title)-3 {
		return strings.TrimSpace(title[idx+3:])
	}
	return ""
}

func ResolveGoogleNewsURL(ctx context.Context, client *http.Client, rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || client == nil {
		return rawURL
	}
	lower := strings.ToLower(rawURL)
	if !strings.Contains(lower, "news.google.com") {
		return rawURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return rawURL
	}
	req.Header.Set("User-Agent", "quant-port-news/1.0")

	res, err := client.Do(req)
	if err != nil {
		return rawURL
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))

	finalURL := res.Request.URL.String()
	if finalURL != "" && !strings.Contains(strings.ToLower(finalURL), "news.google.com") {
		return finalURL
	}
	return rawURL
}

func parseRSSPubDate(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC1123,
		time.RFC1123Z,
		time.RFC822,
		time.RFC822Z,
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
