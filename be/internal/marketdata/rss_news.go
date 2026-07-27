package marketdata

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type rssFeed struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

var DefaultDomesticRSSFeeds = []struct {
	FeedURL  string
	SourceID string
}{
	{FeedURL: "https://www.hankyung.com/feed/economy", SourceID: "rss"},
	{FeedURL: "https://www.mk.co.kr/rss/30000001/", SourceID: "rss"},
	{FeedURL: "https://www.sedaily.com/RSS/SediNews.xml", SourceID: "rss"},
	{FeedURL: "https://www.yonhapnewstv.co.kr/category/news/economy/feed/", SourceID: "yonhap"},
}

type RSSNewsClient struct {
	httpClient *http.Client
	feeds      []struct {
		FeedURL  string
		SourceID string
	}
}

func NewRSSNewsClient(httpClient *http.Client) *RSSNewsClient {
	return &RSSNewsClient{
		httpClient: httpClient,
		feeds:      DefaultDomesticRSSFeeds,
	}
}

func (c *RSSNewsClient) FetchAll(ctx context.Context) ([]RawNewsItem, error) {
	if c == nil || c.httpClient == nil {
		return nil, fmt.Errorf("rss client not configured")
	}
	var out []RawNewsItem
	for _, feed := range c.feeds {
		items, err := c.fetchFeed(ctx, feed.FeedURL, feed.SourceID)
		if err != nil {
			continue
		}
		out = append(out, items...)
	}
	return out, nil
}

func (c *RSSNewsClient) fetchFeed(ctx context.Context, feedURL, sourceID string) ([]RawNewsItem, error) {
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
		return nil, fmt.Errorf("rss %s status %d", feedURL, res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, err
	}

	var parsed rssFeed
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	out := make([]RawNewsItem, 0, len(parsed.Channel.Items))
	for _, item := range parsed.Channel.Items {
		link := strings.TrimSpace(item.Link)
		title := stripHTML(item.Title)
		if link == "" || title == "" {
			continue
		}
		out = append(out, RawNewsItem{
			Title:       title,
			Description: stripHTML(item.Description),
			Link:        link,
			PublishedAt: ParseNewsPubDate(item.PubDate),
			SourceID:    sourceID,
		})
	}
	return out, nil
}
