package marketdata

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

type rssFeed struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title         string `xml:"title"`
	Link          string `xml:"link"`
	Description   string `xml:"description"`
	PubDate       string `xml:"pubDate"`
	Creator       string `xml:"http://purl.org/dc/elements/1.1/ creator"`
	NasdaqTickers string `xml:"http://nasdaq.com/reference/feeds/1.0 tickers"`
}

var DefaultDomesticRSSFeeds = []struct {
	FeedURL  string
	SourceID string
}{
	{FeedURL: "https://www.hankyung.com/feed/finance", SourceID: "rss"},
	{FeedURL: "https://www.mk.co.kr/rss/50200011/", SourceID: "rss"},
	{FeedURL: "https://www.sedaily.com/RSS/SediNews.xml", SourceID: "rss"},
	{FeedURL: "https://www.yonhapnewstv.co.kr/category/news/economy/feed/", SourceID: "yonhap"},
	{FeedURL: "https://www.yna.co.kr/rss/economy.xml", SourceID: "yonhap"},
	{FeedURL: "https://www.tokenpost.kr/rss", SourceID: "tokenpost"},
	{FeedURL: "https://www.blockmedia.co.kr/feed", SourceID: "blockmedia"},
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
		title := normalizeRSSNewsTitle(stripHTML(item.Title))
		if link == "" || title == "" {
			continue
		}
		desc := normalizeRSSNewsDescription(title, stripHTML(item.Description))
		creator := strings.TrimSpace(item.Creator)
		feedSymbols := ParseRSSTickerList(item.NasdaqTickers)
		related := ""
		if len(feedSymbols) > 0 {
			related = feedSymbols[0]
		}
		out = append(out, RawNewsItem{
			Title:         title,
			Description:   desc,
			Link:          link,
			PublishedAt:   ParseNewsPubDate(item.PubDate),
			SourceID:      sourceID,
			PublisherName: creator,
			RelatedSymbol: related,
			FeedSymbols:   feedSymbols,
		})
	}
	return out, nil
}

func normalizeRSSNewsTitle(title string) string {
	title = strings.TrimSpace(title)
	for _, prefix := range []string{"FinancialJuice: ", "financialjuice: "} {
		if strings.HasPrefix(title, prefix) {
			return strings.TrimSpace(title[len(prefix):])
		}
	}
	return title
}

func normalizeRSSNewsDescription(title, description string) string {
	description = strings.TrimSpace(description)
	description = normalizeKeyPointsPrefix(description)
	if description != "" {
		return description
	}
	return title
}

var keyPointsPrefixRe = regexp.MustCompile(`(?i)^Key Points(?::|\s+)?`)

func normalizeKeyPointsPrefix(description string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return ""
	}
	if keyPointsPrefixRe.MatchString(description) {
		rest := strings.TrimSpace(keyPointsPrefixRe.ReplaceAllString(description, ""))
		if rest != "" {
			return "Key points: " + rest
		}
	}
	return description
}
