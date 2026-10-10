package marketdata

import (
	"testing"
)

// 2026-09-29 apewisdom.io/stocks/NVDA/ 에서 잘라 온 모양.
const apeNVDAFixture = `<span>NVDA Overall Summary (last 24h)</span>
<div class="details-small-tile"><div class="tile-title">Mentions</div>
<div class="tile-value">126 <span class="percentage-green"> +2,000%</span></div></div>
<div class="details-small-tile"><div class="tile-title">mentioning users</div>
<div class="tile-value">1,087 <span class="percentage-green"> +1,143%</span></div></div>
<div class="details-small-tile"><div class="tile-title">Sentiment</div>
<div class="tile-value">72% <span class="percentage-green"></span></div></div>
<div class="detail-sub-title"><span>NVDA Overall Sentiment</span></div>
<svg class="sentiment-bar" viewBox="0 0 100 5"><rect width="72" height="24" /></svg>
Nearby keywords:
<span class="badge badge-filter" style="font-size: 18px;">up <span style="font-weight:bold;">x9</span></span>
<span class="badge badge-filter" style="font-size: 18px;">calls <span style="font-weight:bold;">x7</span></span>
<div class="detail-sub-title"><span>NVDA Sentiment on WSB</span></div>
<svg class="sentiment-bar" viewBox="0 0 100 5"><rect width="75" height="24" /></svg>
<span class="badge badge-filter" style="font-size: 18px;">buy <span style="font-weight:bold;">x5</span></span>`

func TestParseApeWisdomPage(t *testing.T) {
	in := ParseApeWisdomPage(apeNVDAFixture)
	if in.Users == nil || *in.Users != 1087 {
		t.Fatalf("users = %v", in.Users)
	}
	if in.SentimentPct == nil || *in.SentimentPct != 72 || in.WSBPct == nil || *in.WSBPct != 75 {
		t.Fatalf("sentiment = %v wsb = %v", in.SentimentPct, in.WSBPct)
	}
	// 「up」은 뜻 없는 낱말로 빠지고, WSB 칸의 낱말은 섞이지 않는다.
	if len(in.Keywords) != 1 || in.Keywords[0] != (KeywordCount{"calls", 7}) {
		t.Fatalf("keywords = %+v", in.Keywords)
	}
}

const redditFixture = `<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom">
<entry><category term="stocks" label="r/stocks"/><content type="html">&lt;!-- SC_OFF --&gt;&lt;div class=&quot;md&quot;&gt;&lt;p&gt;Nvidia&amp;#39;s (NVDA) cap depends on Blackwell.&lt;/p&gt;&lt;/div&gt;&lt;!-- SC_ON --&gt; submitted by</content>
<id>t3_1wmeg8b</id><link href="https://www.reddit.com/r/stocks/comments/1wmeg8b/x/" /><updated>2026-09-21T14:31:07+00:00</updated><published>2026-09-21T14:31:07+00:00</published><title>Nvidia CEO shrugs off AI doomsday</title></entry>
<entry><category term="wallstreetbets" label="r/wallstreetbets"/><content type="html">&lt;div class=&quot;md&quot;&gt;&lt;p&gt;turn it on&lt;/p&gt;&lt;/div&gt;</content>
<id>t3_2</id><link href="https://www.reddit.com/r/wallstreetbets/comments/2/y/" /><published>2026-09-22T01:00:00+00:00</published><title>Daily Discussion Thread for September 22</title></entry>
</feed>`

func TestParseRedditAtom(t *testing.T) {
	posts, err := ParseRedditAtom([]byte(redditFixture))
	if err != nil || len(posts) != 2 {
		t.Fatalf("posts=%d err=%v", len(posts), err)
	}
	p := posts[0]
	if p.ID != "t3_1wmeg8b" || p.Subreddit != "stocks" || p.Snippet != "Nvidia's (NVDA) cap depends on Blackwell." {
		t.Fatalf("post = %+v", p)
	}
	if !MentionsTicker(p.Title+"\n"+p.body, "NVDA", "NVIDIA Corp") || !isMegathread(posts[1].Title) {
		t.Fatal("ticker / megathread check")
	}
}

func TestMentionsTicker(t *testing.T) {
	cases := []struct {
		text, ticker, name string
		want               bool
	}{
		{"turn it on please", "ON", "", false}, // 짧은 티커 소문자는 낱말
		{"ON Semi beat", "ON", "", true},       // 대문자
		{"bought $amd calls", "AMD", "", true}, // 캐시태그
		{"amdocs is up", "AMD", "", false},     // 낱말 경계
		{"nvda to the moon", "NVDA", "", true}, // 4자 이상은 소문자도
		{"Microsoft earnings", "MSFT", "Microsoft Corp", true},
	}
	for _, c := range cases {
		if got := MentionsTicker(c.text, c.ticker, c.name); got != c.want {
			t.Errorf("%q %s = %v", c.text, c.ticker, got)
		}
	}
}

// 시장 인기 글(2026-10-01 요청 3): 메가스레드·교차 게시(같은 제목) 빼고 순위를 다시 매긴다.
func TestDedupeMarketPosts(t *testing.T) {
	in := []RedditPost{
		{ID: "t3_a", Title: "Mortgage rates hit 7.58%"},
		{ID: "t3_b", Title: "r/Stocks Daily Discussion Wednesday - Sep 30, 2026"},
		{ID: "t3_c", Title: "Only 1 of the 10 biggest US companies earns more"},
		{ID: "t3_d", Title: "Only 1 of the 10 biggest US companies earns more"},
	}
	out := dedupeMarketPosts(in)
	if len(out) != 2 || out[0].ID != "t3_a" || out[1].ID != "t3_c" || out[1].Rank != 1 {
		t.Fatalf("got %+v", out)
	}
}
