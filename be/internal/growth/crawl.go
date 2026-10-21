package growth

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
)

/*
사이트 크롤 — 사이트맵에 실린 URL만 연다.

사이트맵이 「색인해 달라고 내민 목록」이므로 그 약속이 지켜지는지(200 · index · 자기 canonical ·
robots 허용 · 본문 있음)를 재는 것이 이 크롤의 전부다. 링크를 따라 사이트 전체를 뒤지지는 않는다 —
사이트맵 밖 주소는 대부분 개인 화면이라 크롤러가 볼 것이 없고, 링크는 **들어오는 링크 수**를 세는
재료로만 쓴다.

리다이렉트는 따라가지 않는다. 사이트맵 URL이 3xx면 그 자체가 문제다.
*/

// robotsRules — `User-agent: *` 묶음의 규칙. 같은 에이전트 묶음이 여럿이면 합친다
// (Google 규칙. Cloudflare가 자기 묶음을 우리 것 위에 주입하는 경우가 실제로 있다).
type robotsRules struct {
	allow, disallow []string
}

func parseRobots(body string) robotsRules {
	var out robotsRules
	var agents []string
	inRules := false
	for _, line := range strings.Split(body, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k, v = strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(v)
		switch k {
		case "user-agent":
			if inRules {
				agents, inRules = nil, false
			}
			agents = append(agents, strings.ToLower(v))
		case "allow", "disallow":
			inRules = true
			if !containsStr(agents, "*") || v == "" {
				continue
			}
			if k == "allow" {
				out.allow = append(out.allow, v)
			} else {
				out.disallow = append(out.disallow, v)
			}
		default:
			// Sitemap · Content-Signal 등은 묶음을 끊지 않는다.
		}
	}
	return out
}

func containsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// robotsMatch — `*`와 끝의 `$`를 지원하는 접두 일치.
func robotsMatch(pattern, path string) bool {
	anchored := strings.HasSuffix(pattern, "$")
	pattern = strings.TrimSuffix(pattern, "$")
	var b strings.Builder
	b.WriteString("^")
	for i, part := range strings.Split(pattern, "*") {
		if i > 0 {
			b.WriteString(".*")
		}
		b.WriteString(regexp.QuoteMeta(part))
	}
	if anchored {
		b.WriteString("$")
	}
	re, err := regexp.Compile(b.String())
	return err == nil && re.MatchString(path)
}

// Allowed — 가장 긴 규칙이 이긴다. 길이가 같으면 Allow가 이긴다(Google 규칙).
func (r robotsRules) Allowed(path string) bool {
	best, allowed := -1, true
	for _, p := range r.disallow {
		if robotsMatch(p, path) && len(p) > best {
			best, allowed = len(p), false
		}
	}
	for _, p := range r.allow {
		if robotsMatch(p, path) && len(p) >= best {
			best, allowed = len(p), true
		}
	}
	return allowed
}

type sitemapDoc struct {
	URLs []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

// pageFacts — HTML 한 장에서 뽑은 사실. 판정은 audit.go가 한다.
type pageFacts struct {
	Title       string
	TitleCount  int
	Description string
	Canonical   string
	Robots      string
	H1Count     int
	TextChars   int
	Text        string // 본문(내비게이션 제외) — 요약 줄이 보이게 그려졌는지 확인하는 데 쓴다. 저장하지 않는다.
	Links       []string
	OGImage     bool
	JSONLDTypes []string
}

// 본문 글자 수에서 빼는 요소 — 모든 화면에 똑같이 들어가는 틀과 실행 코드.
// 2026-09-08 실측에서 색인 대상 8페이지의 글자 대부분이 사이드바였다(docs/SEO.md).
var skipTextTags = map[string]bool{
	"script": true, "style": true, "noscript": true, "template": true, "svg": true,
	"nav": true, "header": true, "footer": true, "aside": true, "head": true,
}

func parsePage(body []byte, pageURL *url.URL, siteHost string) pageFacts {
	var f pageFacts
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return f
	}
	var text strings.Builder
	links := map[string]bool{}
	robots := []string{}

	// inSVG — <svg> 안의 <title>은 그림 설명이지 문서 제목이 아니다.
	var walk func(n *html.Node, skip, inSVG bool)
	walk = func(n *html.Node, skip, inSVG bool) {
		if n.Type == html.ElementNode {
			tag := n.Data
			if tag == "svg" {
				inSVG = true
			}
			switch tag {
			case "title":
				if inSVG {
					break
				}
				if f.TitleCount == 0 && n.FirstChild != nil {
					f.Title = strings.TrimSpace(n.FirstChild.Data)
				}
				f.TitleCount++
			case "meta":
				name := strings.ToLower(attr(n, "name"))
				prop := strings.ToLower(attr(n, "property"))
				content := strings.TrimSpace(attr(n, "content"))
				switch {
				case name == "description" && f.Description == "":
					f.Description = content
				case name == "robots" || name == "googlebot":
					robots = append(robots, strings.ToLower(content))
				case prop == "og:image" && content != "":
					f.OGImage = true
				}
			case "link":
				if strings.EqualFold(attr(n, "rel"), "canonical") && f.Canonical == "" {
					if u, err := pageURL.Parse(attr(n, "href")); err == nil {
						f.Canonical = u.String()
					}
				}
			case "h1":
				f.H1Count++
			case "a":
				if href := attr(n, "href"); href != "" {
					if key, ok := internalLinkKey(pageURL, href, siteHost); ok {
						links[key] = true
					}
				}
			case "script":
				if strings.EqualFold(attr(n, "type"), "application/ld+json") && n.FirstChild != nil {
					f.JSONLDTypes = append(f.JSONLDTypes, jsonLDTypes(n.FirstChild.Data)...)
				}
			}
			if skipTextTags[tag] {
				skip = true
			}
		}
		if n.Type == html.TextNode && !skip {
			if s := strings.TrimSpace(n.Data); s != "" {
				text.WriteString(s)
				text.WriteByte(' ')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, skip, inSVG)
		}
	}
	walk(doc, false, false)

	f.Robots = strings.Join(dedupe(robots), ", ")
	f.Text = collapseSpace(text.String())
	f.TextChars = countVisibleRunes(f.Text)
	for k := range links {
		f.Links = append(f.Links, k)
	}
	sort.Strings(f.Links)
	f.JSONLDTypes = dedupe(f.JSONLDTypes)
	return f
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

/*
internalLinkKey — 같은 사이트로 가는 링크를 비교 가능한 열쇠로 접는다.

쿼리와 조각은 버린다(`/analysis?symbol=…`은 같은 문서). 경로는 **디코드한 값**으로 비교한다 —
사이트맵은 `/stocks/%EC…`처럼 인코딩해 싣고 화면 링크는 그렇지 않을 수 있다. apex와 www는
같은 사이트로 본다(Cloudflare가 apex→www 301).
*/
func internalLinkKey(base *url.URL, href, siteHost string) (string, bool) {
	if strings.HasPrefix(href, "#") || strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "javascript:") {
		return "", false
	}
	u, err := base.Parse(href)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	if bareHost(u.Host) != bareHost(siteHost) {
		return "", false
	}
	return pathKey(u.Path), true
}

func bareHost(h string) string { return strings.TrimPrefix(strings.ToLower(h), "www.") }

func pathKey(p string) string {
	if p == "" {
		return "/"
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	return p
}

func jsonLDTypes(raw string) []string {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return []string{"(invalid)"}
	}
	var out []string
	var visit func(any)
	visit = func(x any) {
		switch t := x.(type) {
		case []any:
			for _, e := range t {
				visit(e)
			}
		case map[string]any:
			switch ty := t["@type"].(type) {
			case string:
				out = append(out, ty)
			case []any:
				for _, e := range ty {
					if s, ok := e.(string); ok {
						out = append(out, s)
					}
				}
			}
			if g, ok := t["@graph"]; ok {
				visit(g)
			}
		}
	}
	visit(v)
	return out
}

func dedupe(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

var spaceRe = regexp.MustCompile(`\s+`)

func collapseSpace(s string) string { return strings.TrimSpace(spaceRe.ReplaceAllString(s, " ")) }

// countVisibleRunes — 공백을 뺀 글자 수. 한글 한 글자가 1이다(바이트가 아니라).
func countVisibleRunes(s string) int {
	n := 0
	for _, r := range s {
		if !unicode.IsSpace(r) {
			n++
		}
	}
	return n
}

// ── HTTP ───────────────────────────────────────────────────────────────────

type crawler struct {
	cfg      Config
	hc       *http.Client
	siteHost string
}

func newCrawler(cfg Config) (*crawler, error) {
	site, err := url.Parse(cfg.SiteURL)
	if err != nil || site.Host == "" {
		return nil, fmt.Errorf("GROWTH_SITE_URL: %q", cfg.SiteURL)
	}
	return &crawler{
		cfg:      cfg,
		siteHost: site.Host,
		hc: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

type fetched struct {
	Status   int
	Location string
	Body     []byte
	Elapsed  time.Duration
	Err      error
}

const maxPageBytes = 5 << 20

/*
get — 한 번 실패하면 한 번 더.

2026-09-25 실측: 공개 주소가 가끔 수 초~30초 멈추고 Cloudflare 524를 낸다. 그 순간 nginx 로그에는
요청이 아예 없다(오리진 앞 구간). 사이트맵을 못 받으면 그날 크롤이 통째로 빠지므로, 연결 오류와
에지·게이트웨이 5xx(502·503·504·520~524)는 2초 뒤 한 번 다시 연다. 두 번째도 실패하면 그대로 적는다 —
계속 실패하는 URL은 보고서가 보여 줘야 한다.
*/
func (c *crawler) get(ctx context.Context, rawURL string) fetched {
	return c.fetch(ctx, rawURL, c.cfg.CrawlOrigin != "")
}

// getPublic — CrawlOrigin과 상관없이 공개 주소로. Cloudflare가 끼워 넣는 것(robots 묶음)과
// 공개 경로의 도달성은 오리진에서는 안 보인다.
func (c *crawler) getPublic(ctx context.Context, rawURL string) fetched {
	return c.fetch(ctx, rawURL, false)
}

func (c *crawler) fetch(ctx context.Context, rawURL string, viaOrigin bool) fetched {
	res := c.getOnce(ctx, rawURL, viaOrigin)
	if res.Err == nil && !retryableStatus(res.Status) {
		return res
	}
	select {
	case <-ctx.Done():
		return res
	case <-time.After(2 * time.Second):
	}
	return c.getOnce(ctx, rawURL, viaOrigin)
}

func retryableStatus(code int) bool {
	return code == 502 || code == 503 || code == 504 || (code >= 520 && code <= 524)
}

func (c *crawler) getOnce(ctx context.Context, rawURL string, viaOrigin bool) fetched {
	target := rawURL
	if viaOrigin {
		if u, err := url.Parse(rawURL); err == nil {
			target = c.cfg.CrawlOrigin + u.RequestURI()
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return fetched{Err: err}
	}
	if viaOrigin {
		req.Host = c.siteHost
		req.Header.Set("X-Forwarded-Proto", "https")
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ko-KR,ko;q=0.9")
	start := time.Now()
	resp, err := c.hc.Do(req)
	if err != nil {
		return fetched{Err: err, Elapsed: time.Since(start)}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPageBytes))
	return fetched{
		Status:   resp.StatusCode,
		Location: resp.Header.Get("Location"),
		Body:     body,
		Elapsed:  time.Since(start),
		Err:      err,
	}
}

// sitemapURLs — sitemap.xml을 받는다. 색인 파일이면 한 단계만 내려간다.
func (c *crawler) sitemapURLs(ctx context.Context) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	queue := []string{c.cfg.SiteURL + "/sitemap.xml"}
	for depth := 0; depth < 2 && len(queue) > 0; depth++ {
		var next []string
		for _, loc := range queue {
			res := c.get(ctx, loc)
			if res.Err != nil {
				return nil, fmt.Errorf("sitemap %s: %w", loc, res.Err)
			}
			if res.Status != http.StatusOK {
				return nil, fmt.Errorf("sitemap %s: HTTP %d", loc, res.Status)
			}
			var doc sitemapDoc
			if err := xml.Unmarshal(res.Body, &doc); err != nil {
				return nil, fmt.Errorf("sitemap %s: %w", loc, err)
			}
			for _, u := range doc.URLs {
				if s := strings.TrimSpace(u.Loc); s != "" && !seen[s] {
					seen[s] = true
					out = append(out, s)
				}
			}
			for _, s := range doc.Sitemaps {
				next = append(next, strings.TrimSpace(s.Loc))
			}
		}
		queue = next
	}
	return out, nil
}

// robots — 공개 주소의 것을 먼저 본다(Cloudflare가 자기 묶음을 끼워 넣는다). 공개 경로가 막혔으면 오리진.
func (c *crawler) robots(ctx context.Context) (robotsRules, string, error) {
	res := c.getPublic(ctx, c.cfg.SiteURL+"/robots.txt")
	if (res.Err != nil || res.Status != http.StatusOK) && c.cfg.CrawlOrigin != "" {
		res = c.get(ctx, c.cfg.SiteURL+"/robots.txt")
	}
	if res.Err != nil {
		return robotsRules{}, "", res.Err
	}
	if res.Status != http.StatusOK {
		return robotsRules{}, "", fmt.Errorf("robots.txt: HTTP %d", res.Status)
	}
	body := string(res.Body)
	return parseRobots(body), body, nil
}

/*
PublicProbe — 공개 주소 도달성(Cloudflare → 오리진 구간).

크롤을 오리진으로 돌리면 이 구간이 안 보인다. 2026-09-25에 바로 여기서 문제가 났다 — 공개 요청이
간헐적으로 8~30초 멈추거나 524였는데 nginx에는 요청이 아예 없었다. 그래서 매번 몇 개를 공개
주소로 따로 열어 본다. 재시도하지 않는다 — 방문자도 재시도하지 않는다.
*/
type PublicProbe struct {
	Tried  int    `json:"tried"`
	OK     int    `json:"ok"`
	Slow   int    `json:"slow"` // 3초 초과
	Failed int    `json:"failed"`
	MaxMS  int    `json:"maxMs"`
	Worst  string `json:"worst,omitempty"`
}

const probeSlowMS = 3000

func (c *crawler) probePublic(ctx context.Context, paths []string, rounds int) PublicProbe {
	var p PublicProbe
	for r := 0; r < rounds; r++ {
		for _, path := range paths {
			if ctx.Err() != nil {
				return p
			}
			res := c.getOnce(ctx, c.cfg.SiteURL+path, false)
			ms := int(res.Elapsed / time.Millisecond)
			p.Tried++
			if ms > p.MaxMS {
				p.MaxMS = ms
				p.Worst = path
			}
			switch {
			case res.Err != nil || res.Status >= 500:
				p.Failed++
			case ms > probeSlowMS:
				p.Slow++
			default:
				p.OK++
			}
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
			}
		}
	}
	return p
}

// crawlAll — URL마다 한 번씩. 동시 N개 · 요청 사이 간격을 둔다(운영 인프라가 작다).
func (c *crawler) crawlAll(ctx context.Context, urls []string) []*PageResult {
	out := make([]*PageResult, len(urls))
	jobs := make(chan int)
	var wg sync.WaitGroup
	workers := c.cfg.CrawlConcurrency
	if workers < 1 {
		workers = 1
	}
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				out[i] = c.crawlOne(ctx, urls[i])
				select {
				case <-ctx.Done():
				case <-time.After(c.cfg.CrawlDelay):
				}
			}
		}()
	}
	for i := range urls {
		if ctx.Err() != nil {
			break
		}
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	// ctx가 끊겨 못 연 URL은 빠진다.
	kept := out[:0]
	for _, p := range out {
		if p != nil {
			kept = append(kept, p)
		}
	}
	return kept
}

func (c *crawler) crawlOne(ctx context.Context, rawURL string) *PageResult {
	p := &PageResult{URL: rawURL, Type: pageType(rawURL)}
	res := c.get(ctx, rawURL)
	p.ResponseMS = int(res.Elapsed / time.Millisecond)
	if res.Err != nil {
		p.FetchErr = res.Err.Error()
		return p
	}
	p.Status, p.Location, p.Bytes = res.Status, res.Location, len(res.Body)
	if res.Status == http.StatusOK && utf8.Valid(res.Body) {
		u, _ := url.Parse(rawURL)
		p.Facts = parsePage(res.Body, u, c.siteHost)
	}
	return p
}

// pageType — 보고서가 묶어 보는 단위. 경로 첫 마디로 정한다.
func pageType(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "other"
	}
	p := pathKey(u.Path)
	switch {
	case p == "/":
		return "home"
	// 업종·스타일 허브(`/stocks/sector/kr/…`)와 작성자 화면(`/reports/authors/…`)은 목록이다 —
	// 종목·리포트 검사(요약 줄·Article)를 걸면 거짓 경고가 난다(2026-10-21 티켓 QP-07, 39건).
	case strings.HasPrefix(p, "/stocks/") && !strings.Contains(strings.TrimPrefix(p, "/stocks/"), "/"):
		return "stock"
	case strings.HasPrefix(p, "/reports/") && !strings.HasPrefix(p, "/reports/authors/"):
		return "report"
	case strings.HasPrefix(p, "/news"):
		return "news"
	case strings.HasPrefix(p, "/earnings"):
		return "earnings"
	case strings.HasPrefix(p, "/legal"):
		return "legal"
	default:
		return "section"
	}
}
