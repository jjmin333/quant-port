package growth

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRobotsLongestMatchWins(t *testing.T) {
	// 운영 robots.txt 모양 — Cloudflare가 자기 묶음을 위에 주입하고, 우리 묶음이 /api/를 막은 뒤
	// 공개 접두어를 Allow로 되살린다.
	body := `# Cloudflare managed
User-agent: GPTBot
Disallow: /

User-agent: *
Content-Signal: search=yes, ai-train=no
Allow: /

User-agent: *
Allow: /
Allow: /api/v1/news/
Disallow: /api/
Disallow: /holdings
Disallow: /reports/new
Sitemap: https://www.quant-port.com/sitemap.xml
`
	r := parseRobots(body)
	cases := map[string]bool{
		"/":                 true,
		"/news/domestic":    true,
		"/api/v1/news/list": true,
		"/api/v1/auth/me":   false,
		"/holdings":         false,
		"/holdings/x":       false,
		"/reports/12":       true,
		"/reports/new":      false,
		"/stocks/005930":    true,
	}
	for path, want := range cases {
		if got := r.Allowed(path); got != want {
			t.Errorf("Allowed(%q) = %v, want %v", path, got, want)
		}
	}
	// GPTBot 묶음의 Disallow: / 가 * 에 섞이면 안 된다.
	if containsStr(r.disallow, "/") {
		t.Fatalf("GPTBot rule leaked into *: %v", r.disallow)
	}
}

func TestRobotsWildcardAndAnchor(t *testing.T) {
	r := parseRobots("User-agent: *\nDisallow: /*.json$\nDisallow: /dev*\n")
	if r.Allowed("/data.json") || !r.Allowed("/data.json?x=1") {
		t.Fatal("$ anchor")
	}
	if r.Allowed("/devtools") {
		t.Fatal("* wildcard")
	}
}

const samplePage = `<!doctype html><html><head>
<title>삼성전자 (005930) 주가·AI 점수 · Quant Port</title>
<meta name="description" content="삼성전자는 코스피 반도체 종목입니다.">
<meta name="robots" content="index, follow">
<link rel="canonical" href="/stocks/005930">
<meta property="og:image" content="https://www.quant-port.com/opengraph-image">
<script type="application/ld+json">{"@context":"https://schema.org","@graph":[{"@type":"WebSite"},{"@type":["Organization","Thing"]}]}</script>
</head><body>
<nav><a href="/">홈</a><a href="/market">시장</a><svg><title>아이콘</title></svg></nav>
<main><h1>삼성전자</h1><p>삼성전자는 코스피 반도체 종목입니다.</p>
<a href="/stocks/000660?tab=news#x">SK하이닉스</a>
<a href="https://quant-port.com/reports/3">리포트</a>
<a href="https://example.com/">밖</a>
<script>self.__next_f.push([1,"긴 글자"])</script></main>
</body></html>`

func TestParsePage(t *testing.T) {
	u, _ := url.Parse("https://www.quant-port.com/stocks/005930")
	f := parsePage([]byte(samplePage), u, "www.quant-port.com")
	if f.Title != "삼성전자 (005930) 주가·AI 점수 · Quant Port" || f.TitleCount != 1 {
		t.Fatalf("title %q x%d (svg <title> must not count)", f.Title, f.TitleCount)
	}
	if f.Canonical != "https://www.quant-port.com/stocks/005930" {
		t.Fatalf("canonical %q", f.Canonical)
	}
	if !f.OGImage || f.H1Count != 1 || f.Robots != "index, follow" {
		t.Fatalf("facts %+v", f)
	}
	if strings.Join(f.JSONLDTypes, ",") != "WebSite,Organization,Thing" {
		t.Fatalf("jsonld %v", f.JSONLDTypes)
	}
	// 내비게이션 · 스크립트 글자는 본문에서 빠진다.
	if strings.Contains(f.Text, "시장") || strings.Contains(f.Text, "긴 글자") {
		t.Fatalf("text includes chrome: %q", f.Text)
	}
	want := []string{"/", "/market", "/reports/3", "/stocks/000660"}
	if strings.Join(f.Links, " ") != strings.Join(want, " ") {
		t.Fatalf("links %v", f.Links)
	}
}

func page(u, typ, title, desc, canonical, robots string, links ...string) *PageResult {
	return &PageResult{URL: u, Type: typ, Status: 200, Facts: pageFacts{
		Title: title, TitleCount: 1, Description: desc, Canonical: canonical, Robots: robots,
		H1Count: 1, TextChars: 500, Text: desc, Links: links, OGImage: true,
	}}
}

func codes(p *PageResult) string {
	var out []string
	for _, is := range p.Issues {
		out = append(out, is.Code)
	}
	return strings.Join(out, ",")
}

func TestAuditPages(t *testing.T) {
	site := "https://www.quant-port.com"
	home := page(site+"/", "home", "Quant Port — 흩어진 투자 기록을 한곳에", "홈 설명", site+"/", "index", "/news/domestic", "/stocks/005930")
	news := page(site+"/news/domestic", "news", "국내 뉴스 · Quant Port", "뉴스 설명", site+"/", "index", "/")
	stock := page(site+"/stocks/005930", "stock", "Quant Port — 흩어진 투자 기록을 한곳에", "홈 설명", site+"/stocks/005930", "noindex, follow")
	stock.Facts.TextChars = 20
	orphan := page(site+"/reports/9", "report", "리포트 제목이 들어간다 · Quant Port", "리포트 설명", site+"/reports/9", "index")
	redirect := &PageResult{URL: site + "/old", Type: "section", Status: 301, Location: "/new"}
	pages := []*PageResult{home, news, stock, orphan, redirect}
	auditPages(pages, &robotsRules{}, site)

	if got := codes(news); !strings.Contains(got, "canonical_mismatch") {
		t.Errorf("news: %s", got)
	}
	s := codes(stock)
	for _, want := range []string{"noindex_in_sitemap", "thin_content", "stock_title_fallback", "title_duplicate", "description_duplicate"} {
		if !strings.Contains(s, want) {
			t.Errorf("stock missing %s: %s", want, s)
		}
	}
	if stock.LinksIn != 1 || strings.Contains(s, "orphan") {
		t.Errorf("stock inbound %d: %s", stock.LinksIn, s)
	}
	if !strings.Contains(codes(orphan), "orphan") || !strings.Contains(codes(orphan), "jsonld_missing") {
		t.Errorf("report: %s", codes(orphan))
	}
	if codes(redirect) != "redirect_in_sitemap" {
		t.Errorf("redirect: %s", codes(redirect))
	}
	if strings.Contains(codes(home), "orphan") {
		t.Errorf("home is never an orphan: %s", codes(home))
	}
}

func TestDiffIssuesIgnoresDetailAndNewURLs(t *testing.T) {
	prev := map[string][]Issue{
		"a": {{Code: "slow_response", Severity: sevWarn, Detail: "2600ms"}, {Code: "h1_missing", Severity: sevInfo}},
	}
	pages := []*PageResult{
		{URL: "a", Issues: []Issue{{Code: "slow_response", Severity: sevWarn, Detail: "3100ms"}, {Code: "thin_content", Severity: sevWarn}}},
		{URL: "b", Issues: []Issue{{Code: "http_error", Severity: sevError}}}, // 처음 보는 URL — 「새 문제」로 세지 않는다
	}
	added, resolved := diffIssues(prev, pages)
	if len(added) != 1 || added[0].Issue.Code != "thin_content" {
		t.Fatalf("added %+v", added)
	}
	if len(resolved) != 1 || resolved[0].Issue.Code != "h1_missing" {
		t.Fatalf("resolved %+v", resolved)
	}
}

func TestStockMatcher(t *testing.T) {
	m := newStockMatcher([]stockName{
		{"005930", "삼성전자"}, {"005935", "삼성전자우"}, {"003550", "LG"}, {"AAPL", "Apple"}, {"010140", "삼성중공업"},
	})
	cases := map[string]string{
		"삼성전자 주가":  "005930",
		"삼성전자우 배당": "005935", // 긴 이름이 이긴다
		"삼성 전자 실적": "005930", // 띄어쓰기 무시
		"aapl 목표가": "AAPL",
		"lg 주가":    "003550",
		"lg에너지솔루션": "", // 두 글자 이름은 토큰이 같을 때만
		"코스피 전망":   "",
	}
	for q, want := range cases {
		got, _, _ := m.match(q)
		if got != want {
			t.Errorf("match(%q) = %q, want %q", q, got, want)
		}
	}
}

func day(s string) time.Time {
	t, _ := time.ParseInLocation("2006-01-02", s, kst)
	return t
}

func TestCompareQueriesAndMoves(t *testing.T) {
	prev := []SearchRow{
		{Day: day("2026-09-10"), Query: "삼성전자 주가", Page: "/a", Clicks: 1, Impressions: 10, Position: 12},
		{Day: day("2026-09-11"), Query: "사라진 말", Page: "/b", Clicks: 0, Impressions: 8, Position: 30},
	}
	cur := []SearchRow{
		{Day: day("2026-09-17"), Query: "삼성전자 주가", Page: "/a", Clicks: 2, Impressions: 30, Position: 6},
		{Day: day("2026-09-18"), Query: "삼성전자 주가", Page: "/c", Clicks: 0, Impressions: 10, Position: 10},
		{Day: day("2026-09-18"), Query: "새 말", Page: "/a", Clicks: 0, Impressions: 4, Position: 40},
	}
	stats := compareQueries(cur, prev)
	var q QueryStat
	for _, s := range stats {
		if s.Query == "삼성전자 주가" {
			q = s
		}
	}
	if q.Impressions != 40 || q.TopPage != "/a" || q.PrevImpressions != 10 {
		t.Fatalf("%+v", q)
	}
	if want := (6.0*30 + 10*10) / 40; q.Position != want {
		t.Fatalf("weighted position %v want %v", q.Position, want)
	}
	mv := queryMoves(stats, 5)
	if len(mv.Up) != 1 || len(mv.New) != 1 || len(mv.Lost) != 1 || len(mv.Down) != 0 {
		t.Fatalf("moves %+v", mv)
	}
}

func TestAnalyzeFindsSearchOpportunities(t *testing.T) {
	site := "https://www.quant-port.com"
	queries := []QueryStat{
		{Query: "퀀트 포트폴리오", Clicks: 1, Impressions: 200, Position: 7, TopPage: site + "/"},            // 4~20위 문턱
		{Query: "삼성전자 실적", Clicks: 0, Impressions: 50, Position: 3, TopPage: site + "/news/domestic"}, // 종목 페이지가 안 걸림 + 의도
		{Query: "소형주 주가", Clicks: 0, Impressions: 30, Position: 15, TopPage: site + "/"},              // 색인 밖 종목
	}
	out := analyze(analysisInput{
		Queries:   queries,
		Indexable: map[string]bool{"005930": true},
		Matcher:   newStockMatcher([]stockName{{"005930", "삼성전자"}, {"999999", "소형주"}}),
		SiteURL:   site,
		HasSearch: true,
	})
	kinds := map[string]int{}
	for _, o := range out.Opportunities {
		kinds[o.Kind]++
	}
	for _, k := range []string{"striking", "ctr_gap", "stock_page_missing", "stock_not_indexed", "content_gap"} {
		if kinds[k] == 0 {
			t.Errorf("missing %s: %v", k, kinds)
		}
	}
	for i := 1; i < len(out.Opportunities); i++ {
		if out.Opportunities[i].Gain > out.Opportunities[i-1].Gain {
			t.Fatal("opportunities not sorted by gain")
		}
	}
}

func TestParseBingDate(t *testing.T) {
	d, ok := parseBingDate(`/Date(1758844800000-0700)/`) // 2025-09-26 00:00 UTC
	if !ok || ymd(d) != "2025-09-26" {
		t.Fatalf("%v %v", d, ok)
	}
}

func TestLastSunday(t *testing.T) {
	for today, want := range map[string]string{
		"2026-09-28": "2026-09-27", // 월
		"2026-09-27": "2026-09-20", // 일 — 이번 주가 안 끝났다
		"2026-09-25": "2026-09-20", // 금
	} {
		if got := ymd(lastSunday(day(today))); got != want {
			t.Errorf("lastSunday(%s) = %s, want %s", today, got, want)
		}
	}
}

func TestPickInspectTargets(t *testing.T) {
	pages := []*PageResult{{URL: "s1", Type: "stock"}, {URL: "h", Type: "home"}, {URL: "old", Type: "news"}, {URL: "older", Type: "news"}}
	existing := map[string]inspectionRow{
		"old":   {InspectedAt: day("2026-09-20")},
		"older": {InspectedAt: day("2026-09-10")},
	}
	got := strings.Join(pickInspectTargets(pages, existing, 3), ",")
	if got != "h,s1,older" {
		t.Fatalf("targets %s", got)
	}
}

func TestPublicProbeFailureTopsTheList(t *testing.T) {
	out := analyze(analysisInput{
		Crawl: []*PageResult{{URL: "https://www.quant-port.com/", Type: "home", Status: 200,
			Issues: []Issue{{Code: "thin_content", Severity: sevWarn}}}},
		Probe: &PublicProbe{Tried: 15, OK: 12, Slow: 1, Failed: 2, MaxMS: 30000, Worst: "/"},
	})
	if len(out.Opportunities) == 0 || out.Opportunities[0].Target != "public_probe" {
		t.Fatalf("public probe should lead: %+v", out.Opportunities)
	}
	healthy := analyze(analysisInput{Probe: &PublicProbe{Tried: 15, OK: 15}})
	for _, o := range healthy.Opportunities {
		if o.Target == "public_probe" {
			t.Fatal("healthy probe must not raise an opportunity")
		}
	}
}
