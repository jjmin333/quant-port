package marketdata

import "net/http"

// DefaultUSRSSFeeds — 해외 증시·매크로 중심 (개인금융/브로드 라이프스타일 피드 제외).
var DefaultUSRSSFeeds = []struct {
	FeedURL  string
	SourceID string
}{
	// FinancialJuice — analyst-curated market wire (savetickers-class headlines).
	{FeedURL: "https://www.financialjuice.com/feed.ashx?xy=rss", SourceID: "financial-juice"},
	// Reuters — markets, finance, macro/geopolitics (may require non-bot User-Agent from some networks).
	{FeedURL: "https://www.reuters.com/markets/us/rss", SourceID: "reuters"},
	{FeedURL: "https://www.reuters.com/business/finance/rss", SourceID: "reuters"},
	{FeedURL: "https://www.reuters.com/world/us/rss", SourceID: "reuters"},
	{FeedURL: "https://www.reuters.com/markets/rss", SourceID: "reuters"},
	// Tier-1 finance wires.
	{FeedURL: "https://feeds.bloomberg.com/markets/news.rss", SourceID: "rss"},
	{FeedURL: "https://feeds.a.dj.com/rss/RSSMarketsMain.xml", SourceID: "rss"},
	{FeedURL: "https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=100003114", SourceID: "rss"},
	{FeedURL: "https://feeds.marketwatch.com/marketwatch/marketpulse/", SourceID: "rss"},
	{FeedURL: "https://www.nasdaq.com/feed/rssoutbound?category=Markets", SourceID: "rss"},
	// Fed — monetary policy press releases only (press_all includes payments studies, enforcement, etc.).
	{FeedURL: "https://www.federalreserve.gov/feeds/press_monetary.xml", SourceID: "rss"},
	// Index / sector ETFs.
	{FeedURL: "https://feeds.finance.yahoo.com/rss/2.0/headline?s=%5EGSPC&region=US&lang=en-US", SourceID: "rss"},
	{FeedURL: "https://feeds.finance.yahoo.com/rss/2.0/headline?s=%5EIXIC&region=US&lang=en-US", SourceID: "rss"},
	{FeedURL: "https://feeds.finance.yahoo.com/rss/2.0/headline?s=%5EDJI&region=US&lang=en-US", SourceID: "rss"},
	{FeedURL: "https://feeds.finance.yahoo.com/rss/2.0/headline?s=SPY&region=US&lang=en-US", SourceID: "rss"},
	{FeedURL: "https://feeds.finance.yahoo.com/rss/2.0/headline?s=QQQ&region=US&lang=en-US", SourceID: "rss"},
	{FeedURL: "https://feeds.finance.yahoo.com/rss/2.0/headline?s=XLF&region=US&lang=en-US", SourceID: "rss"},
	{FeedURL: "https://feeds.finance.yahoo.com/rss/2.0/headline?s=XLK&region=US&lang=en-US", SourceID: "rss"},
}

func NewUSRSSNewsClient(httpClient *http.Client) *RSSNewsClient {
	return &RSSNewsClient{
		httpClient: httpClient,
		feeds:      DefaultUSRSSFeeds,
	}
}
