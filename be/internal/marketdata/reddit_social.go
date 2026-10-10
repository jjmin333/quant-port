package marketdata

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

/*
Reddit 커뮤니티 탭 재료 두 가지 (2026-09-29 사용자 요청 「ApeWisdom 최대 활용」).

	ApeWisdom 종목 페이지  /stocks/{T}/  — 목록 API에 없는 값: 언급한 사용자 수 · 감성(긍정 비율) ·
	                                      WSB 감성 · 함께 나온 낱말. API가 없어 HTML에서 읽는다.
	Reddit 검색 RSS       /r/a+b/search.rss — 글 제목·앞부분·링크. JSON API는 이 서버에서 403이고
	                                      RSS만 열린다. **분당 1회**(x-ratelimit, 실측) — 배치만 부른다.

둘 다 모양이 바뀌면 빈 값으로 떨어진다 — 화면은 그 칸을 숨긴다.
*/

// ApeWisdomInsight — 종목 페이지의 24시간 요약.
type ApeWisdomInsight struct {
	Users        *int
	SentimentPct *int // 긍정 비율 0..100
	WSBPct       *int
	Keywords     []KeywordCount
}

type KeywordCount struct {
	Word string `json:"word"`
	N    int    `json:"n"`
}

var (
	apeTileRe    = regexp.MustCompile(`(?s)<div class="tile-title">\s*([^<]+?)\s*</div>\s*<div class="tile-value">\s*([^<]*)`)
	apeBlockRe   = regexp.MustCompile(`(?s)<span>[A-Z0-9.\-]+ (Overall Sentiment|Sentiment on WSB)</span>(.*?)(?:class="detail-sub-title"|$)`)
	apeBarRe     = regexp.MustCompile(`class="sentiment-bar"[^>]*>\s*<rect width="(\d+)"`)
	apeKeywordRe = regexp.MustCompile(`badge-filter"[^>]*>\s*([^<]+?)\s*<span[^>]*>x(\d+)</span>`)
)

// apeKeywordStop — 뜻 없는 낱말(실측 NVDA: up·back·than·much·like·doing·title …).
var apeKeywordStop = map[string]bool{
	"up": true, "back": true, "than": true, "much": true, "like": true, "doing": true, "title": true,
	"just": true, "will": true, "what": true, "about": true, "more": true, "some": true, "been": true,
	"into": true, "only": true, "should": true, "would": true, "could": true, "they": true, "this": true,
	"that": true, "with": true, "have": true, "from": true, "your": true, "when": true, "then": true,
	"even": true, "still": true, "really": true, "going": true, "think": true, "know": true, "down": true,
	"also": true, "over": true, "here": true, "there": true, "their": true, "were": true, "which": true,
	"right": true, "good": true, "make": true, "need": true, "want": true, "time": true, "because": true,
	"after": true, "before": true, "giving": true, "today": true, "week": true, "next": true, "last": true,
	"year": true, "being": true, "other": true, "every": true, "where": true, "these": true, "those": true,
	"again": true, "since": true, "while": true, "made": true, "many": true, "getting": true, "people": true,
}

// ParseApeWisdomPage reads the insight block from one ticker page.
func ParseApeWisdomPage(page string) ApeWisdomInsight {
	var out ApeWisdomInsight
	for _, m := range apeTileRe.FindAllStringSubmatch(page, -1) {
		title := strings.ToLower(strings.TrimSpace(m[1]))
		n, ok := leadingInt(m[2])
		if !ok {
			continue
		}
		switch title {
		case "mentioning users":
			out.Users = &n
		case "sentiment":
			if n >= 0 && n <= 100 {
				out.SentimentPct = &n
			}
		}
	}
	for _, b := range apeBlockRe.FindAllStringSubmatch(page, -1) {
		bar := apeBarRe.FindStringSubmatch(b[2])
		var pct *int
		if bar != nil {
			if n, err := strconv.Atoi(bar[1]); err == nil && n >= 0 && n <= 100 {
				pct = &n
			}
		}
		if b[1] == "Sentiment on WSB" {
			out.WSBPct = pct
			continue
		}
		if out.SentimentPct == nil {
			out.SentimentPct = pct
		}
		for _, k := range apeKeywordRe.FindAllStringSubmatch(b[2], -1) {
			w := strings.ToLower(html.UnescapeString(strings.TrimSpace(k[1])))
			n, _ := strconv.Atoi(k[2])
			if w == "" || apeKeywordStop[w] || len(out.Keywords) >= 8 {
				continue
			}
			out.Keywords = append(out.Keywords, KeywordCount{Word: w, N: n})
		}
	}
	return out
}

// leadingInt reads "1,159 " / "72%" → 1159 / 72.
func leadingInt(s string) (int, bool) {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) && (s[end] >= '0' && s[end] <= '9' || s[end] == ',') {
		end++
	}
	n, err := strconv.Atoi(strings.ReplaceAll(s[:end], ",", ""))
	return n, err == nil
}

// Insight fetches one ticker page (ApeWisdom 버킷).
func (c *ApeWisdomClient) Insight(ctx context.Context, ticker string) (ApeWisdomInsight, error) {
	u := "https://apewisdom.io/stocks/" + url.PathEscape(ticker) + "/"
	body, err := getRaw(ctx, c.httpClient, c.budget, apibudget.ProviderApeWisdom, u, "text/html", browserUA)
	if err != nil {
		return ApeWisdomInsight{}, err
	}
	ins := ParseApeWisdomPage(string(body))
	if ins.Users == nil && ins.SentimentPct == nil {
		return ins, ErrExternalNotFound
	}
	return ins, nil
}

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0 Safari/537.36"

// RedditPost is one search result.
type RedditPost struct {
	ID        string // t3_xxx
	Subreddit string
	Title     string
	Snippet   string
	URL       string
	Published time.Time
	Rank      int
	body      string // 전체 본문(티커 확인용, 저장 안 함)
}

// RedditSubreddits — 검색할 곳. ApeWisdom이 세는 주식 서브레딧 중 글이 많은 다섯.
var RedditSubreddits = []string{"wallstreetbets", "stocks", "investing", "StockMarket", "options"}

// RedditClient reads Reddit's public search RSS.
type RedditClient struct {
	httpClient *http.Client
	budget     *apibudget.Manager
}

func NewRedditClient(hc *http.Client, budget *apibudget.Manager) *RedditClient {
	return &RedditClient{httpClient: hc, budget: budget}
}

type atomFeed struct {
	Entries []struct {
		ID        string `xml:"id"`
		Title     string `xml:"title"`
		Published string `xml:"published"`
		Updated   string `xml:"updated"`
		Content   string `xml:"content"`
		Link      struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
		Category struct {
			Term string `xml:"term,attr"`
		} `xml:"category"`
	} `xml:"entry"`
}

// SearchTop returns this week's top posts mentioning ticker (티커가 실제로 적힌 글만).
func (c *RedditClient) SearchTop(ctx context.Context, ticker, name string) ([]RedditPost, error) {
	u := fmt.Sprintf("https://www.reddit.com/r/%s/search.rss?q=%s&restrict_sr=1&sort=top&t=week&limit=25",
		strings.Join(RedditSubreddits, "+"), url.QueryEscape(ticker))
	body, err := getRaw(ctx, c.httpClient, c.budget, apibudget.ProviderRedditRSS, u, "application/atom+xml", browserUA)
	if err != nil {
		return nil, err
	}
	posts, err := ParseRedditAtom(body)
	if err != nil {
		return nil, err
	}
	// 제목에 종목이 나온 글을 먼저, 본문에만 나온 글은 뒤에(09-29 실측: MU 검색에 본문만 MU인 SPY·NVDA 글).
	var inTitle, inBody []RedditPost
	for _, p := range posts {
		switch {
		case isMegathread(p.Title):
		case MentionsTicker(p.Title, ticker, name):
			inTitle = append(inTitle, p)
		case MentionsTicker(p.body, ticker, name):
			inBody = append(inBody, p)
		}
	}
	out := append(inTitle, inBody...)
	for i := range out {
		out[i].Rank = i
	}
	return out, nil
}

// RedditMarketSubreddits — 시장 전반 글(2026-10-01 요청 3). wsb는 밈·손익 인증이 많아 뺐다.
var RedditMarketSubreddits = []string{"stocks", "investing", "StockMarket"}

// MarketTop — 오늘의 시장 인기 글(메가스레드·교차 게시 중복 제외). 티커 확인은 하지 않는다.
func (c *RedditClient) MarketTop(ctx context.Context) ([]RedditPost, error) {
	u := fmt.Sprintf("https://www.reddit.com/r/%s/top.rss?t=day&limit=25", strings.Join(RedditMarketSubreddits, "+"))
	body, err := getRaw(ctx, c.httpClient, c.budget, apibudget.ProviderRedditRSS, u, "application/atom+xml", browserUA)
	if err != nil {
		return nil, err
	}
	posts, err := ParseRedditAtom(body)
	if err != nil {
		return nil, err
	}
	return dedupeMarketPosts(posts), nil
}

func dedupeMarketPosts(posts []RedditPost) []RedditPost {
	seen := map[string]bool{}
	var out []RedditPost
	for _, p := range posts {
		key := strings.ToLower(p.Title)
		if isMegathread(p.Title) || seen[key] {
			continue
		}
		seen[key] = true
		p.Rank = len(out)
		out = append(out, p)
	}
	return out
}

var (
	mdDivRe  = regexp.MustCompile(`(?s)<div class="md">(.*?)</div>`)
	tagRe    = regexp.MustCompile(`<[^>]+>`)
	spacesRe = regexp.MustCompile(`\s+`)
)

// ParseRedditAtom decodes a search.rss body.
func ParseRedditAtom(body []byte) ([]RedditPost, error) {
	var feed atomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("reddit rss: %w", err)
	}
	out := make([]RedditPost, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		// 링크는 화면에서 href가 된다 — Reddit 주소만 받는다(javascript: 등 차단).
		if !strings.HasPrefix(e.ID, "t3_") || !strings.HasPrefix(e.Link.Href, "https://www.reddit.com/") {
			continue
		}
		at, err := time.Parse(time.RFC3339, firstNonBlank(e.Published, e.Updated))
		if err != nil {
			continue
		}
		text := ""
		if m := mdDivRe.FindStringSubmatch(e.Content); m != nil {
			text = strings.TrimSpace(spacesRe.ReplaceAllString(html.UnescapeString(tagRe.ReplaceAllString(m[1], " ")), " "))
		}
		out = append(out, RedditPost{
			ID:        e.ID,
			Subreddit: e.Category.Term,
			Title:     strings.TrimSpace(html.UnescapeString(e.Title)),
			Snippet:   cutRunes(text, 280),
			URL:       e.Link.Href,
			Published: at.UTC().Truncate(time.Second),
			body:      text,
		})
	}
	return out, nil
}

func firstNonBlank(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func cutRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:n])) + "…"
}

/*
MentionsTicker — Reddit 검색은 대소문자를 가리지 않아 「ON」이 「on」에 걸린다. 그래서 글에 티커가
**실제로** 적혔는지 다시 본다: $티커(대소문자 무관) · 대문자 티커 낱말 · 4자 이상 티커는 소문자도 ·
회사 이름 첫 낱말(5자 이상, 예: NVIDIA·Microsoft).
*/
func MentionsTicker(text, ticker, name string) bool {
	t := strings.ToUpper(ticker)
	upper := strings.ToUpper(text)
	if containsWord(text, t) || containsWord(upper, "$"+t) {
		return true
	}
	if len(t) >= 4 && containsWord(upper, t) {
		return true
	}
	if first := strings.Fields(name); len(first) > 0 && len(first[0]) >= 5 {
		return containsWord(upper, strings.ToUpper(strings.Trim(first[0], ",.")))
	}
	return false
}

// containsWord reports w in s on word boundaries (영숫자가 아닌 글자 사이).
func containsWord(s, w string) bool {
	for i := 0; ; {
		j := strings.Index(s[i:], w)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(w)
		if (start == 0 || !isWordByte(s[start-1])) && (end == len(s) || !isWordByte(s[end])) {
			return true
		}
		i = start + 1
	}
}

func isWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// 매일 올라오는 토론 글 — 모든 티커 검색에 걸리지만 그 종목 이야기가 아니다.
func isMegathread(title string) bool {
	t := strings.ToLower(title)
	return strings.Contains(t, "discussion thread") || strings.Contains(t, "daily discussion") ||
		strings.Contains(t, "moves tomorrow") || strings.Contains(t, "weekend discussion")
}
