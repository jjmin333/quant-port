package growth

import (
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"
)

/*
분석 — 전부 규칙이다. 숫자는 추정이고, 보고서가 그렇게 적는다.

# 기대 클릭률 곡선

순위별 기대 클릭률은 공개된 업계 관측치를 둥글린 값이다(1위 약 28% → 10위 약 2.5%). 사이트마다
다르므로 **절대값이 아니라 순서를 매기는 데만** 쓴다 — 「이 검색어를 3위로 올리면 주당 몇 클릭」은
어림이고, 기회끼리 줄을 세우는 잣대로는 충분하다.
*/
func expectedCTR(position float64) float64 {
	switch {
	case position <= 0:
		return 0
	case position < 1.5:
		return 0.28
	case position < 2.5:
		return 0.15
	case position < 3.5:
		return 0.10
	case position < 4.5:
		return 0.07
	case position < 5.5:
		return 0.05
	case position < 10.5:
		return 0.03
	case position < 20.5:
		return 0.01
	default:
		return 0.003
	}
}

// QueryStat — 한 구간의 검색어 집계. 순위는 노출 가중 평균.
type QueryStat struct {
	Query           string  `json:"query"`
	Clicks          int     `json:"clicks"`
	Impressions     int     `json:"impressions"`
	Position        float64 `json:"position"`
	TopPage         string  `json:"topPage"`
	PrevClicks      int     `json:"prevClicks"`
	PrevImpressions int     `json:"prevImpressions"`
	PrevPosition    float64 `json:"prevPosition"`
}

func (q QueryStat) CTR() float64 {
	if q.Impressions == 0 {
		return 0
	}
	return float64(q.Clicks) / float64(q.Impressions)
}

// PageStat — 한 구간의 페이지 집계.
type PageStat struct {
	Page            string  `json:"page"`
	Clicks          int     `json:"clicks"`
	Impressions     int     `json:"impressions"`
	Position        float64 `json:"position"`
	PrevClicks      int     `json:"prevClicks"`
	PrevImpressions int     `json:"prevImpressions"`
}

type agg struct {
	clicks, impr int
	posW         float64
	pages        map[string]int
}

func aggregate(rows []SearchRow, key func(SearchRow) string) map[string]*agg {
	out := map[string]*agg{}
	for _, r := range rows {
		k := key(r)
		if k == "" {
			continue
		}
		a := out[k]
		if a == nil {
			a = &agg{pages: map[string]int{}}
			out[k] = a
		}
		a.clicks += r.Clicks
		a.impr += r.Impressions
		a.posW += r.Position * float64(r.Impressions)
		if r.Page != "" {
			a.pages[r.Page] += r.Impressions
		}
	}
	return out
}

func (a *agg) position() float64 {
	if a == nil || a.impr == 0 {
		return 0
	}
	return a.posW / float64(a.impr)
}

func (a *agg) topPage() string {
	best, n := "", -1
	for p, v := range a.pages {
		if v > n || (v == n && p < best) {
			best, n = p, v
		}
	}
	return best
}

// compareQueries — 이번 구간과 지난 구간을 검색어로 맞붙인다.
func compareQueries(cur, prev []SearchRow) []QueryStat {
	byQ := func(r SearchRow) string { return r.Query }
	c, p := aggregate(cur, byQ), aggregate(prev, byQ)
	keys := map[string]bool{}
	for k := range c {
		keys[k] = true
	}
	for k := range p {
		keys[k] = true
	}
	out := make([]QueryStat, 0, len(keys))
	for k := range keys {
		s := QueryStat{Query: k}
		if a := c[k]; a != nil {
			s.Clicks, s.Impressions, s.Position, s.TopPage = a.clicks, a.impr, a.position(), a.topPage()
		}
		if a := p[k]; a != nil {
			s.PrevClicks, s.PrevImpressions, s.PrevPosition = a.clicks, a.impr, a.position()
			if s.TopPage == "" {
				s.TopPage = a.topPage()
			}
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Impressions != out[j].Impressions {
			return out[i].Impressions > out[j].Impressions
		}
		return out[i].Query < out[j].Query
	})
	return out
}

func comparePages(cur, prev []SearchRow) []PageStat {
	byP := func(r SearchRow) string { return r.Page }
	c, p := aggregate(cur, byP), aggregate(prev, byP)
	keys := map[string]bool{}
	for k := range c {
		keys[k] = true
	}
	for k := range p {
		keys[k] = true
	}
	out := make([]PageStat, 0, len(keys))
	for k := range keys {
		s := PageStat{Page: k}
		if a := c[k]; a != nil {
			s.Clicks, s.Impressions, s.Position = a.clicks, a.impr, a.position()
		}
		if a := p[k]; a != nil {
			s.PrevClicks, s.PrevImpressions = a.clicks, a.impr
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Impressions != out[j].Impressions {
			return out[i].Impressions > out[j].Impressions
		}
		return out[i].Page < out[j].Page
	})
	return out
}

// QueryMoves — 검색어 변화(매일 항목 4).
type QueryMoves struct {
	Up   []QueryStat `json:"up"`
	Down []QueryStat `json:"down"`
	New  []QueryStat `json:"new"`
	Lost []QueryStat `json:"lost"`
}

func queryMoves(stats []QueryStat, limit int) QueryMoves {
	var m QueryMoves
	for _, s := range stats {
		switch {
		case s.PrevImpressions == 0 && s.Impressions >= 3:
			m.New = append(m.New, s)
		case s.Impressions == 0 && s.PrevImpressions >= 5:
			m.Lost = append(m.Lost, s)
		case s.Impressions > 0 && s.PrevImpressions > 0:
			// 순위가 오른 것(숫자가 줄어든 것) 또는 클릭이 는 것.
			if s.PrevPosition-s.Position >= 2 || s.Clicks-s.PrevClicks >= 3 {
				m.Up = append(m.Up, s)
			} else if s.Position-s.PrevPosition >= 2 || s.PrevClicks-s.Clicks >= 3 {
				m.Down = append(m.Down, s)
			}
		}
	}
	byImpr := func(xs []QueryStat, prev bool) {
		sort.SliceStable(xs, func(i, j int) bool {
			if prev {
				return xs[i].PrevImpressions > xs[j].PrevImpressions
			}
			return xs[i].Impressions > xs[j].Impressions
		})
	}
	byImpr(m.New, false)
	byImpr(m.Lost, true)
	byImpr(m.Up, false)
	byImpr(m.Down, false)
	m.Up, m.Down, m.New, m.Lost = head(m.Up, limit), head(m.Down, limit), head(m.New, limit), head(m.Lost, limit)
	return m
}

func head[T any](xs []T, n int) []T {
	if len(xs) > n {
		return xs[:n]
	}
	return xs
}

// ── 종목 이름 ↔ 검색어 ─────────────────────────────────────────────────────

type stockMatcher struct {
	bySymbol map[string]string // SYMBOL → name
	names    []stockName       // 긴 이름부터
}

func newStockMatcher(names []stockName) *stockMatcher {
	m := &stockMatcher{bySymbol: map[string]string{}}
	for _, n := range names {
		m.bySymbol[strings.ToUpper(n.Symbol)] = n.Name
		m.names = append(m.names, n)
	}
	sort.SliceStable(m.names, func(i, j int) bool {
		return utf8.RuneCountInString(m.names[i].Name) > utf8.RuneCountInString(m.names[j].Name)
	})
	return m
}

// match — 검색어가 가리키는 종목. 토큰이 심볼과 같으면 그것, 아니면 검색어에 통째로 든 가장 긴 이름.
// 두 글자 이름(「LG」 같은)은 한 토큰과 같을 때만 인정한다 — 부분 일치는 오탐이 너무 많다.
func (m *stockMatcher) match(query string) (symbol, name string, ok bool) {
	if m == nil {
		return "", "", false
	}
	q := strings.ToLower(strings.TrimSpace(query))
	tokens := strings.Fields(q)
	for _, t := range tokens {
		if n, found := m.bySymbol[strings.ToUpper(t)]; found && len(t) >= 2 {
			return strings.ToUpper(t), n, true
		}
	}
	compact := strings.ReplaceAll(q, " ", "")
	for _, s := range m.names {
		ln := strings.ToLower(strings.ReplaceAll(s.Name, " ", ""))
		if utf8.RuneCountInString(ln) <= 2 {
			if containsStr(tokens, ln) {
				return strings.ToUpper(s.Symbol), s.Name, true
			}
			continue
		}
		if strings.Contains(compact, ln) {
			return strings.ToUpper(s.Symbol), s.Name, true
		}
	}
	return "", "", false
}

// intentWords — 종목 검색어에 붙는 의도. 콘텐츠 후보의 모양을 정한다.
var intentWords = []struct{ word, content string }{
	{"실적", "실적 요약·발표 일정"},
	{"배당", "배당 이력"},
	{"전망", "점수 근거·변화 요약"},
	{"목표가", "컨센서스·목표가"},
	{"주가", "주가·점수 요약"},
	{"뉴스", "종목 뉴스 모음"},
	{"공시", "최근 공시"},
	{"per", "밸류에이션 지표"},
	{"시가총액", "밸류에이션 지표"},
}

func intentOf(query string) (word, content string) {
	q := strings.ToLower(query)
	for _, w := range intentWords {
		if strings.Contains(q, w.word) {
			return w.word, w.content
		}
	}
	return "", ""
}

// ── 기회 ───────────────────────────────────────────────────────────────────

// Opportunity — 한 줄짜리 할 일. Gain은 「주당 추가 클릭」 어림(없으면 기술 가중치).
type Opportunity struct {
	Kind     string  `json:"kind"`
	Title    string  `json:"title"`
	Target   string  `json:"target"`
	Gain     float64 `json:"gain"`
	Evidence string  `json:"evidence"`
	Action   string  `json:"action"`
	Where    string  `json:"where,omitempty"`
}

var opportunityKinds = map[string]string{
	"striking":           "상위 10위 문턱",
	"ctr_gap":            "노출 대비 클릭 부족",
	"stock_not_indexed":  "수요 있는 종목이 색인 밖",
	"stock_page_missing": "종목 검색에 종목 페이지가 안 걸림",
	"content_gap":        "전용 페이지 없는 검색 의도",
	"tech":               "기술 문제",
	"link":               "내부 링크",
}

// whereFor — 고칠 자리 안내. 보고서가 「어디를 열면 되나」까지 말하게.
func whereFor(page string) string {
	switch pageType(page) {
	case "stock":
		return "fe/src/app/(main)/stocks/[symbol]/page.tsx · 요약 문장은 BE /seo/stocks/{symbol}(repository/seo_stocks.go)"
	case "report":
		return "fe/src/app/(main)/reports/[id]/page.tsx (리포트별 generateMetadata)"
	case "home", "news", "earnings", "section":
		return "관리자 › 페이지 메타(제목·설명 덮어쓰기) 또는 fe/src/lib/page-meta-routes.ts"
	default:
		return ""
	}
}

type analysisInput struct {
	Queries     []QueryStat
	Pages       []PageStat
	Crawl       []*PageResult
	Indexable   map[string]bool // 사이트맵의 종목 심볼(대문자)
	Matcher     *stockMatcher
	SiteURL     string
	Earnings    []string // 7일 안 실적 발표 · 색인 대상
	HasSearch   bool
	Probe       *PublicProbe
	WindowLabel string
}

type analysisOutput struct {
	Opportunities []Opportunity `json:"opportunities"`
	Content       []Opportunity `json:"content"`
	Links         []Opportunity `json:"links"`
	QueryActions  []Opportunity `json:"queryActions"`
}

func analyze(in analysisInput) analysisOutput {
	var out analysisOutput
	pageDemand := map[string]PageStat{}
	for _, p := range in.Pages {
		pageDemand[keyOf(p.Page)] = p
	}

	if in.HasSearch {
		stockDemand := map[string]*QueryStat{} // 색인 밖 종목별 합
		stockDemandQueries := map[string][]string{}
		for _, q := range in.Queries {
			if q.Impressions == 0 {
				continue
			}
			ctr, pos := q.CTR(), q.Position

			// 1) 4~20위 — 조금만 오르면 첫 화면. 3위까지 오른다고 보고 어림한다.
			if pos >= 4 && pos <= 20 && q.Impressions >= 5 {
				gain := float64(q.Impressions) * math.Max(0, expectedCTR(3)-ctr)
				out.QueryActions = append(out.QueryActions, Opportunity{
					Kind: "striking", Title: fmt.Sprintf("「%s」 평균 %.1f위 → 상위권", q.Query, pos),
					Target: q.Query, Gain: gain,
					Evidence: fmt.Sprintf("노출 %d · 클릭 %d · CTR %.1f%%", q.Impressions, q.Clicks, ctr*100),
					Action:   "이 검색어가 닿는 페이지의 제목·첫 문단에 검색어를 그대로 넣고, 관련 페이지에서 이 페이지로 링크를 건다",
					Where:    whereFor(q.TopPage),
				})
			}
			// 2) 상위권인데 클릭이 적다 — 제목·설명이 검색 결과에서 안 눌린다.
			if pos < 10.5 && q.Impressions >= 20 && ctr < expectedCTR(pos)*0.5 {
				gain := float64(q.Impressions) * (expectedCTR(pos) - ctr)
				out.QueryActions = append(out.QueryActions, Opportunity{
					Kind: "ctr_gap", Title: fmt.Sprintf("「%s」 %.1f위인데 CTR %.1f%% (기대 %.0f%%)", q.Query, pos, ctr*100, expectedCTR(pos)*100),
					Target: q.Query, Gain: gain,
					Evidence: fmt.Sprintf("노출 %d · 클릭 %d · 페이지 %s", q.Impressions, q.Clicks, shortPath(q.TopPage)),
					Action:   "검색 결과에 보이는 제목·설명을 이 검색어의 의도(무엇을 알고 싶은가)에 맞게 다시 쓴다",
					Where:    whereFor(q.TopPage),
				})
			}

			// 3) 종목 검색어.
			sym, name, ok := in.Matcher.match(q.Query)
			if !ok {
				// 종목이 아닌데 전용 페이지가 없는 의도 — 홈·목록 페이지가 20위 밖에서 받고 있다.
				if pos > 10 && q.Impressions >= 10 && (pageType(q.TopPage) == "home" || pageType(q.TopPage) == "section") {
					out.Content = append(out.Content, Opportunity{
						Kind: "content_gap", Title: fmt.Sprintf("「%s」 전용 페이지 후보", q.Query), Target: q.Query,
						Gain:     float64(q.Impressions) * expectedCTR(5),
						Evidence: fmt.Sprintf("노출 %d · %.1f위 · 지금은 %s가 받음", q.Impressions, pos, shortPath(q.TopPage)),
						Action:   "이 검색어에 답하는 페이지(또는 기존 페이지의 절)를 만든다 — 사람이 쓸 문안",
					})
				}
				continue
			}
			if !in.Indexable[sym] {
				agg := stockDemand[sym]
				if agg == nil {
					agg = &QueryStat{Query: name + " (" + sym + ")"}
					stockDemand[sym] = agg
				}
				agg.Impressions += q.Impressions
				agg.Clicks += q.Clicks
				stockDemandQueries[sym] = append(stockDemandQueries[sym], q.Query)
				continue
			}
			stockPath := "/stocks/" + sym
			if keyOf(q.TopPage) != stockPath && q.Impressions >= 5 {
				out.QueryActions = append(out.QueryActions, Opportunity{
					Kind: "stock_page_missing", Title: fmt.Sprintf("「%s」에 %s 페이지가 아니라 %s가 걸림", q.Query, sym, shortPath(q.TopPage)),
					Target: q.Query, Gain: float64(q.Impressions) * expectedCTR(5),
					Evidence: fmt.Sprintf("노출 %d · %.1f위", q.Impressions, pos),
					Action:   "걸린 페이지에서 " + stockPath + "로 링크를 걸고, 종목 페이지 제목에 검색어 표현을 맞춘다",
					Where:    whereFor(in.SiteURL + stockPath),
				})
			}
			if word, content := intentOf(q.Query); word != "" && q.Impressions >= 5 {
				out.Content = append(out.Content, Opportunity{
					Kind: "content_gap", Title: fmt.Sprintf("%s × 「%s」 — %s", name, word, content), Target: q.Query,
					Gain:     float64(q.Impressions) * math.Max(0, expectedCTR(3)-ctr),
					Evidence: fmt.Sprintf("노출 %d · %.1f위 · 페이지 %s", q.Impressions, pos, shortPath(q.TopPage)),
					Action:   "종목 페이지 서버 렌더 요약에 이 의도의 답(" + content + ")을 한 줄 더 그린다 — 저장된 값만",
					Where:    whereFor(in.SiteURL + stockPath),
				})
			}
		}
		for sym, agg := range stockDemand {
			if agg.Impressions < 3 {
				continue
			}
			out.Opportunities = append(out.Opportunities, Opportunity{
				Kind: "stock_not_indexed", Title: agg.Query + " 검색 수요 — 종목 페이지가 noindex", Target: sym,
				Gain:     float64(agg.Impressions) * expectedCTR(5),
				Evidence: fmt.Sprintf("노출 %d · 검색어 %s", agg.Impressions, strings.Join(head(stockDemandQueries[sym], 3), ", ")),
				Action:   "색인 대상 잣대(거래대금 상위 1,000)에 「검색 수요 있음」 예외를 더할지 결정한다",
				Where:    "be/internal/repository/seo_stocks.go (SEOIndexableStocks · seoIndexablePerMarket)",
			})
		}
	}

	// 4) 기술 문제 — 코드별로 묶는다. 수요가 있는 페이지에 걸린 문제는 그 클릭을 위험액으로 더한다.
	type techAgg struct {
		sev    string
		pages  []string
		atRisk float64
		byType map[string]int
	}
	tech := map[string]*techAgg{}
	for _, p := range in.Crawl {
		for _, is := range p.Issues {
			t := tech[is.Code]
			if t == nil {
				t = &techAgg{sev: is.Severity, byType: map[string]int{}}
				tech[is.Code] = t
			}
			t.pages = append(t.pages, p.URL)
			t.byType[p.Type]++
			if d, ok := pageDemand[keyOf(p.URL)]; ok {
				t.atRisk += float64(d.Clicks) + float64(d.Impressions)*expectedCTR(d.Position)
			}
		}
	}
	for code, t := range tech {
		if t.sev == sevInfo && code != "orphan" {
			continue
		}
		weight := map[string]float64{sevError: 5, sevWarn: 1, sevInfo: 0.2}[t.sev]
		gain := weight*math.Sqrt(float64(len(t.pages))) + t.atRisk
		if code == "orphan" {
			continue // 아래 링크 절에서 다룬다
		}
		out.Opportunities = append(out.Opportunities, Opportunity{
			Kind: "tech", Title: fmt.Sprintf("%s — %d페이지", issueLabel(code), len(t.pages)), Target: code,
			Gain:     gain,
			Evidence: fmt.Sprintf("%s · 예: %s", typeBreakdown(t.byType), shortPath(t.pages[0])),
			Action:   techAction(code),
			Where:    whereFor(t.pages[0]),
		})
	}

	// 5) 내부 링크(매일 항목 9).
	orphansByType := map[string]int{}
	totalByType := map[string]int{}
	for _, p := range in.Crawl {
		totalByType[p.Type]++
		if p.Type != "home" && p.LinksIn == 0 && p.Status == 200 {
			orphansByType[p.Type]++
		}
		if d, ok := pageDemand[keyOf(p.URL)]; ok && d.Impressions >= 10 && p.LinksIn <= 1 {
			out.Links = append(out.Links, Opportunity{
				Kind: "link", Title: fmt.Sprintf("%s — 노출 %d인데 들어오는 링크 %d개", shortPath(p.URL), d.Impressions, p.LinksIn),
				Target: p.URL, Gain: float64(d.Impressions) * math.Max(0, expectedCTR(math.Max(1, d.Position-3))-expectedCTR(d.Position)),
				Evidence: fmt.Sprintf("%.1f위 · 클릭 %d", d.Position, d.Clicks),
				Action:   linkAction(p.Type),
				Where:    whereFor(p.URL),
			})
		}
	}
	for typ, n := range orphansByType {
		total := totalByType[typ]
		if total == 0 || float64(n)/float64(total) < 0.3 || n < 5 {
			continue
		}
		out.Links = append(out.Links, Opportunity{
			Kind: "link", Title: fmt.Sprintf("%s 페이지 %d/%d개가 사이트맵으로만 발견된다", typeLabel(typ), n, total),
			Target: typ, Gain: math.Sqrt(float64(n)),
			Evidence: "크롤한 사이트맵 페이지 어디에서도 이 페이지로 오는 링크가 없다",
			Action:   hubAction(typ),
		})
	}

	// 5-1) 공개 주소 도달성 — 크롤러(구글·네이버)도 이 구간을 지난다. 실패가 있으면 맨 위로 올린다.
	if p := in.Probe; p != nil && (p.Failed > 0 || p.Slow > 0) {
		out.Opportunities = append(out.Opportunities, Opportunity{
			Kind: "tech", Title: fmt.Sprintf("공개 주소 응답 실패 %d/%d · 3초 초과 %d", p.Failed, p.Tried, p.Slow),
			Target: "public_probe", Gain: 20*float64(p.Failed) + 5*float64(p.Slow),
			Evidence: fmt.Sprintf("가장 느린 응답 %s %.1f초 · 오리진(nginx) 크롤은 따로 잰다", p.Worst, float64(p.MaxMS)/1000),
			Action:   "nginx 접근 로그에 그 요청이 있는지 본다 — 없으면 Cloudflare → 오리진 구간(호스트 네트워크 · 포트 포워딩)",
		})
	}

	// 6) 실적 발표 예정(콘텐츠 후보).
	if len(in.Earnings) > 0 {
		out.Content = append(out.Content, Opportunity{
			Kind: "content_gap", Title: fmt.Sprintf("7일 안 실적 발표 — 색인 대상 %d종목", len(in.Earnings)),
			Target: "earnings", Gain: math.Sqrt(float64(len(in.Earnings))),
			Evidence: "예: " + strings.Join(head(in.Earnings, 8), ", "),
			Action:   "종목 페이지 요약 줄에 「다음 실적 발표 MM-DD」를 넣거나 실적 페이지에서 종목 페이지로 링크한다(저장된 일정만)",
			Where:    "fe/src/app/(main)/earnings · fe/src/app/(main)/stocks/[symbol]/page.tsx",
		})
	}

	sortOpps := func(xs []Opportunity) {
		sort.SliceStable(xs, func(i, j int) bool {
			if xs[i].Gain != xs[j].Gain {
				return xs[i].Gain > xs[j].Gain
			}
			return xs[i].Title < xs[j].Title
		})
	}
	sortOpps(out.QueryActions)
	sortOpps(out.Content)
	sortOpps(out.Links)
	all := append([]Opportunity{}, out.Opportunities...)
	all = append(all, out.QueryActions...)
	all = append(all, out.Content...)
	all = append(all, out.Links...)
	sortOpps(all)
	out.Opportunities = all
	return out
}

func techAction(code string) string {
	switch code {
	case "fetch_failed", "http_error":
		return "해당 URL을 직접 열어 원인을 본다 — 사이트맵에서 빼야 할 주소인지, 서버 오류인지"
	case "redirect_in_sitemap":
		return "사이트맵에 최종 주소를 싣는다(fe/src/app/sitemap.ts)"
	case "noindex_in_sitemap":
		return "사이트맵과 색인 정책을 맞춘다 — noindex면 사이트맵에서 빼고, 색인 대상이면 noindex를 푼다"
	case "blocked_by_robots":
		return "robots.txt 규칙을 확인한다(fe/src/app/robots.ts)"
	case "canonical_missing", "canonical_mismatch":
		return "화면의 pageMetadata({ path })가 자기 주소를 canonical로 내는지 확인한다"
	case "title_duplicate", "description_duplicate", "stock_title_fallback":
		return "요약을 못 받아 기본값으로 나간 페이지인지 본다 — 종목이면 BE /seo/stocks/{symbol} 응답 시간(SSR 3초 제한)"
	case "thin_content":
		return "서버가 그리는 본문을 늘린다(SSR 선반입) — 클라이언트에서만 그리는 내용은 크롤러가 못 본다"
	case "slow_response":
		return "느린 경로를 nginx 로그 p95로 확인하고 캐시(SWR)를 붙인다"
	case "stock_summary_hidden":
		return "메타 설명과 같은 문장이 화면에 보이게 그려지는지 확인한다(숨긴 글자는 속임수로 본다)"
	case "title_too_short", "title_multiple", "description_missing", "jsonld_invalid":
		return "해당 화면의 메타를 채운다"
	default:
		return "해당 화면을 확인한다"
	}
}

func linkAction(typ string) string {
	if typ == "stock" {
		return "같은 섹터 비교 종목 · 뉴스 종목 태그 · 변화 목록에서 이 종목 페이지로 링크가 걸리는지 확인한다"
	}
	return "수요가 있는 페이지다 — 홈·시장·분석 같은 허브에서 한 번 더 링크한다"
}

func hubAction(typ string) string {
	switch typ {
	case "stock":
		return "종목 페이지를 모아 거는 허브(예: 시장별 인기 종목 목록, 섹터별 목록)를 서버 렌더로 둔다 — 새 화면이라 사람이 설계"
	case "report":
		return "리포트 목록 첫 화면이 서버 렌더로 상세 링크를 내는지 확인한다"
	default:
		return "상단 내비게이션이나 허브 페이지에서 링크한다"
	}
}

func typeLabel(t string) string {
	switch t {
	case "stock":
		return "종목"
	case "report":
		return "리포트"
	case "news":
		return "뉴스"
	case "earnings":
		return "실적"
	case "legal":
		return "약관"
	case "home":
		return "홈"
	default:
		return "기타"
	}
}

func typeBreakdown(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] })
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", typeLabel(k), m[k]))
	}
	return strings.Join(parts, " · ")
}

// shortPath — 보고서에는 호스트를 떼고 경로만(디코드해서 읽히게).
func shortPath(raw string) string {
	if raw == "" {
		return "(없음)"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Path == "" {
		return raw
	}
	return u.Path
}
