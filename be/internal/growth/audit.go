package growth

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"
)

// PageResult — URL 하나의 크롤 결과와 판정.
type PageResult struct {
	URL        string
	Type       string
	Status     int
	Location   string
	ResponseMS int
	Bytes      int
	FetchErr   string
	Facts      pageFacts
	LinksIn    int
	Issues     []Issue
}

const (
	sevError = "error" // 색인이 안 되거나 틀린 문서가 색인된다
	sevWarn  = "warn"  // 색인은 되지만 순위·클릭을 잃는다
	sevInfo  = "info"  // 다듬을 거리
)

type Issue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Detail   string `json:"detail,omitempty"`
}

// issueLabels — 보고서에 쓰는 이름. 코드는 저장·비교용이라 바꾸지 않는다.
var issueLabels = map[string]string{
	"fetch_failed":          "응답 없음",
	"http_error":            "HTTP 오류",
	"redirect_in_sitemap":   "사이트맵 URL이 리다이렉트",
	"noindex_in_sitemap":    "사이트맵에 실렸는데 noindex",
	"blocked_by_robots":     "robots.txt가 막음",
	"canonical_missing":     "canonical 없음",
	"canonical_mismatch":    "canonical이 자기 주소가 아님",
	"title_missing":         "제목 없음",
	"title_multiple":        "<title> 여러 개",
	"title_too_long":        "제목이 김(잘림)",
	"title_too_short":       "제목이 짧음",
	"title_duplicate":       "다른 페이지와 제목이 같음",
	"description_missing":   "설명 없음",
	"description_too_long":  "설명이 김",
	"description_duplicate": "다른 페이지와 설명이 같음",
	"h1_missing":            "H1 없음",
	"h1_multiple":           "H1 여러 개",
	"thin_content":          "서버가 그린 본문이 얇음",
	"slow_response":         "응답이 느림",
	"og_image_missing":      "공유 이미지 없음",
	"orphan":                "들어오는 내부 링크 없음",
	"jsonld_missing":        "구조화 데이터 없음",
	"jsonld_invalid":        "구조화 데이터 JSON 오류",
	"stock_title_fallback":  "종목 페이지가 기본 제목으로 나감",
	"stock_summary_hidden":  "종목 요약 줄이 본문에 없음",
}

func issueLabel(code string) string {
	if l, ok := issueLabels[code]; ok {
		return l
	}
	return code
}

// 문턱값. 한글 제목은 검색 결과에서 대략 32~36자에서 잘린다 — 템플릿(「 · Quant Port」 13자)을
// 포함한 전체 길이로 잰다.
const (
	titleMaxRunes    = 45
	titleMinRunes    = 10
	descMaxRunes     = 160
	thinContentRunes = 300
	thinStockRunes   = 80 // 종목 페이지는 서버가 요약 한 줄만 그린다(2026-09-24 설계)
	slowResponseMS   = 2500
)

// auditPages — 페이지별 판정과 페이지 사이 판정(중복 · 고아)을 한 번에 채운다.
func auditPages(pages []*PageResult, robots *robotsRules, siteURL string) {
	site, _ := url.Parse(siteURL)
	siteHost := ""
	if site != nil {
		siteHost = site.Host
	}

	// 들어오는 링크 — 사이트맵 페이지끼리만 센다. 자기 자신으로 가는 링크는 빼고,
	// 한 페이지에서 여러 번 걸린 링크는 하나로 친다(parsePage가 이미 접었다).
	inbound := map[string]int{}
	for _, p := range pages {
		self := keyOf(p.URL)
		for _, l := range p.Facts.Links {
			if l != self {
				inbound[l]++
			}
		}
	}

	titles := map[string][]*PageResult{}
	descs := map[string][]*PageResult{}

	for _, p := range pages {
		p.Issues = nil
		p.LinksIn = inbound[keyOf(p.URL)]
		add := func(code, sev, detail string) {
			p.Issues = append(p.Issues, Issue{Code: code, Severity: sev, Detail: detail})
		}

		u, _ := url.Parse(p.URL)
		if robots != nil && u != nil && !robots.Allowed(u.EscapedPath()) {
			add("blocked_by_robots", sevError, u.EscapedPath())
		}
		if u != nil && siteHost != "" && !strings.EqualFold(u.Host, siteHost) {
			add("redirect_in_sitemap", sevWarn, "사이트맵 호스트가 "+u.Host)
		}

		switch {
		case p.FetchErr != "":
			add("fetch_failed", sevError, truncate(p.FetchErr, 120))
			continue
		case p.Status >= 300 && p.Status < 400:
			add("redirect_in_sitemap", sevWarn, fmt.Sprintf("%d → %s", p.Status, p.Location))
			continue
		case p.Status != 200:
			add("http_error", sevError, fmt.Sprintf("HTTP %d", p.Status))
			continue
		}

		f := p.Facts
		if strings.Contains(f.Robots, "noindex") {
			add("noindex_in_sitemap", sevError, f.Robots)
		}
		switch {
		case f.Canonical == "":
			add("canonical_missing", sevWarn, "")
		case keyOf(f.Canonical) != keyOf(p.URL) || bareHost(hostOf(f.Canonical)) != bareHost(hostOf(p.URL)):
			add("canonical_mismatch", sevError, f.Canonical)
		}

		tl := utf8.RuneCountInString(f.Title)
		switch {
		case f.Title == "":
			add("title_missing", sevError, "")
		case tl > titleMaxRunes:
			add("title_too_long", sevInfo, fmt.Sprintf("%d자", tl))
		case tl < titleMinRunes:
			add("title_too_short", sevWarn, fmt.Sprintf("%d자", tl))
		}
		if f.TitleCount > 1 {
			add("title_multiple", sevWarn, fmt.Sprintf("%d개", f.TitleCount))
		}
		if f.Title != "" {
			titles[f.Title] = append(titles[f.Title], p)
		}

		dl := utf8.RuneCountInString(f.Description)
		switch {
		case f.Description == "":
			add("description_missing", sevWarn, "")
		case dl > descMaxRunes:
			add("description_too_long", sevInfo, fmt.Sprintf("%d자", dl))
		}
		if f.Description != "" {
			descs[f.Description] = append(descs[f.Description], p)
		}

		if f.H1Count == 0 {
			add("h1_missing", sevInfo, "")
		} else if f.H1Count > 1 {
			add("h1_multiple", sevInfo, fmt.Sprintf("%d개", f.H1Count))
		}

		thin := thinContentRunes
		if p.Type == "stock" {
			thin = thinStockRunes
		}
		if p.Type != "legal" && f.TextChars < thin {
			add("thin_content", sevWarn, fmt.Sprintf("%d자", f.TextChars))
		}
		if p.ResponseMS > slowResponseMS {
			add("slow_response", sevWarn, fmt.Sprintf("%dms", p.ResponseMS))
		}
		if !f.OGImage {
			add("og_image_missing", sevInfo, "")
		}
		if containsStr(f.JSONLDTypes, "(invalid)") {
			add("jsonld_invalid", sevWarn, "")
		} else if p.Type == "report" && !containsStr(f.JSONLDTypes, "Article") {
			add("jsonld_missing", sevInfo, "Article 기대")
		}
		if p.Type != "home" && p.LinksIn == 0 {
			add("orphan", sevWarn, "사이트맵으로만 발견된다")
		}
		if p.Type == "stock" {
			auditStockPage(p, add)
		}
	}

	// 중복 — 종목 페이지끼리는 제목·설명에 이름이 들어가 겹칠 일이 없으므로, 겹치면 그 자체가
	// 요약을 못 받아 기본값으로 나간 신호다(stock_title_fallback이 따로 잡는다).
	markDuplicates(titles, "title_duplicate")
	markDuplicates(descs, "description_duplicate")
}

func markDuplicates(groups map[string][]*PageResult, code string) {
	for _, ps := range groups {
		if len(ps) < 2 {
			continue
		}
		for _, p := range ps {
			p.Issues = append(p.Issues, Issue{Code: code, Severity: sevWarn, Detail: fmt.Sprintf("%d개 페이지", len(ps))})
		}
	}
}

/*
auditStockPage — 종목 페이지 품질(매일 항목 8).

종목 페이지는 서버가 요약을 받아야 제목이 「이름 (심볼) 주가·AI 점수」가 되고 설명·보이는 요약 줄이
같은 문장으로 채워진다(`app/(main)/stocks/[symbol]/page.tsx`). 요약을 못 받으면(BE 지연 · 3초 SSR
제한) 사이트 기본 제목·설명으로 나가는데, 그 상태로 색인되면 1,000장이 같은 문서가 된다.
*/
func auditStockPage(p *PageResult, add func(code, sev, detail string)) {
	symbol := stockSymbolOf(p.URL)
	f := p.Facts
	if symbol != "" && !strings.Contains(strings.ToUpper(f.Title), "("+strings.ToUpper(symbol)+")") {
		add("stock_title_fallback", sevError, truncate(f.Title, 60))
	}
	if f.Description != "" && !strings.Contains(f.Text, f.Description) {
		add("stock_summary_hidden", sevWarn, "")
	}
}

func stockSymbolOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	rest, ok := strings.CutPrefix(pathKey(u.Path), "/stocks/")
	if !ok || strings.Contains(rest, "/") {
		return ""
	}
	return rest
}

func keyOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return pathKey(u.Path)
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}

// ── 이전 크롤과의 차이 ───────────────────────────────────────────────────────

type issueChange struct {
	URL   string
	Issue Issue
}

// diffIssues — 이번에 새로 생긴 문제와 사라진 문제. 비교 단위는 (URL, 코드)다 — 설명 문구(몇 자 ·
// 몇 ms)가 바뀐 것은 새 문제가 아니다.
func diffIssues(prev map[string][]Issue, pages []*PageResult) (added, resolved []issueChange) {
	seen := map[string]bool{}
	for _, p := range pages {
		seen[p.URL] = true
		before := map[string]bool{}
		for _, is := range prev[p.URL] {
			before[is.Code] = true
		}
		now := map[string]bool{}
		for _, is := range p.Issues {
			now[is.Code] = true
			if _, known := prev[p.URL]; known && !before[is.Code] {
				added = append(added, issueChange{URL: p.URL, Issue: is})
			}
		}
		for _, is := range prev[p.URL] {
			if !now[is.Code] {
				resolved = append(resolved, issueChange{URL: p.URL, Issue: is})
			}
		}
	}
	sortChanges := func(xs []issueChange) {
		sort.SliceStable(xs, func(i, j int) bool {
			if severityRank(xs[i].Issue.Severity) != severityRank(xs[j].Issue.Severity) {
				return severityRank(xs[i].Issue.Severity) < severityRank(xs[j].Issue.Severity)
			}
			return xs[i].URL < xs[j].URL
		})
	}
	sortChanges(added)
	sortChanges(resolved)
	return added, resolved
}

func severityRank(s string) int {
	switch s {
	case sevError:
		return 0
	case sevWarn:
		return 1
	default:
		return 2
	}
}
