package growth

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Service — 하루 한 번 도는 본체. 월요일(또는 지난주 보고서가 없을 때)에는 주간 보고서도 만든다.
type Service struct {
	cfg Config
	st  *store
	act *activity
	hc  *http.Client
}

func New(db *sql.DB, cfg Config) *Service {
	return &Service{
		cfg: cfg,
		st:  &store{db: db},
		act: &activity{db: db},
		hc:  &http.Client{Timeout: 60 * time.Second},
	}
}

// sourceState — 보고서 머리의 「데이터 소스」 줄.
type sourceState struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok | off | error
	Note   string `json:"note,omitempty"`
}

// runState — 한 번의 실행이 모은 것. 보고서 두 종류가 같은 재료를 쓴다.
type runState struct {
	now, today  time.Time
	sources     []sourceState
	pages       []*PageResult
	sitemapURLs int
	crawlOK     bool
	probe       *PublicProbe
	added       []issueChange
	resolved    []issueChange
	removed     int64
	inspections map[string]inspectionRow
	indexable   map[string]bool
	matcher     *stockMatcher
	earnings    []string
	shares      []ShareCandidate
	hasGoogle   bool
	gscSitemaps []sitemapStatus
	ga          *ga4Client
}

/*
Run — 배치 한 번. 실패는 소스 단위로 가둔다: Search Console이 죽어도 크롤과 DB 지표로 보고서는 나간다.

에러를 돌려주는 경우는 **크롤이 통째로 실패했을 때뿐**이다(사이트맵을 못 받음 = 사이트가 크롤러에게
안 보인다는 뜻이라 그것 자체가 경보감이다). 나머지 소스 실패는 보고서와 결과의 sources에 남는다.
*/
func (s *Service) Run(ctx context.Context, forceWeekly bool) (map[string]any, string, error) {
	st := &runState{now: time.Now().In(kst)}
	st.today = dayOf(st.now)

	crawlErr := s.crawl(ctx, st)
	s.searchConsole(ctx, st)
	s.analytics(ctx, st)
	s.bing(ctx, st)
	s.dbActivity(ctx, st)
	s.candidates(ctx, st)
	if err := s.st.prune(ctx, st.today); err != nil {
		log.Printf("growth: prune: %v", err)
	}

	result := map[string]any{"sources": st.sources, "pages": len(st.pages), "newIssues": len(st.added), "resolvedIssues": len(st.resolved)}

	daily, err := s.buildDaily(ctx, st)
	if err != nil {
		return result, "", fmt.Errorf("daily report: %w", err)
	}
	result["dailyReportId"] = daily

	weekEnd := lastSunday(st.today)
	needWeekly := forceWeekly || st.today.Weekday() == time.Monday
	if !needWeekly {
		if ok, err := s.st.hasReport(ctx, "weekly", weekEnd); err == nil && !ok {
			needWeekly = true // 월요일 실행을 놓쳤다 — 지난주 것을 지금 만든다
		}
	}
	if needWeekly {
		id, err := s.buildWeekly(ctx, st, weekEnd)
		if err != nil {
			return result, "", fmt.Errorf("weekly report: %w", err)
		}
		result["weeklyReportId"] = id
	}

	msg := fmt.Sprintf("크롤 %d/%d · 새 문제 %d · 해결 %d", len(st.pages), st.sitemapURLs, len(st.added), len(st.resolved))
	for _, src := range st.sources {
		if src.Status == "error" {
			msg += " · " + src.Name + " 실패"
		}
	}
	if crawlErr != nil {
		return result, msg, fmt.Errorf("crawl: %w", crawlErr)
	}
	return result, msg, nil
}

func (st *runState) source(name, status, note string) {
	st.sources = append(st.sources, sourceState{Name: name, Status: status, Note: truncate(note, 200)})
}

func lastSunday(today time.Time) time.Time {
	back := int(today.Weekday()) // 일요일=0
	if back == 0 {
		back = 7
	}
	return today.AddDate(0, 0, -back)
}

// ── 1. 크롤 ────────────────────────────────────────────────────────────────

func (s *Service) crawl(ctx context.Context, st *runState) error {
	c, err := newCrawler(s.cfg)
	if err != nil {
		st.source("크롤", "error", err.Error())
		return err
	}
	// 공개 주소 도달성은 크롤보다 먼저 잰다 — 사이트맵을 못 받아 크롤이 서도 이 값은 남는다.
	probe := c.probePublic(ctx, []string{"/", "/news/domestic", "/market", "/robots.txt", "/sitemap.xml"}, 3)
	st.probe = &probe
	if err := s.st.mergeKPI(ctx, st.today, "public", probe); err != nil {
		log.Printf("growth: public probe kpi: %v", err)
	}

	robots, _, robotsErr := c.robots(ctx)
	urls, err := c.sitemapURLs(ctx)
	if err != nil {
		st.source("크롤", "error", err.Error())
		return err
	}
	st.sitemapURLs = len(urls)
	st.indexable = map[string]bool{}
	for _, u := range urls {
		if sym := stockSymbolOf(u); sym != "" {
			st.indexable[strings.ToUpper(sym)] = true
		}
	}

	prev, err := s.st.loadPrevCrawl(ctx)
	if err != nil {
		st.source("크롤", "error", err.Error())
		return err
	}
	started := time.Now()
	pages := c.crawlAll(ctx, urls)
	var rules *robotsRules
	if robotsErr == nil {
		rules = &robots
	}
	auditPages(pages, rules, s.cfg.SiteURL)
	st.pages = pages
	st.crawlOK = len(pages) == len(urls)
	st.added, st.resolved = diffIssues(prev.issues, pages)

	removed, err := s.st.saveCrawl(ctx, pages, prev, started, st.crawlOK)
	if err != nil {
		st.source("크롤", "error", err.Error())
		return err
	}
	st.removed = removed

	note := fmt.Sprintf("%d/%d URL · %s", len(pages), len(urls), time.Since(started).Round(time.Second))
	if robotsErr != nil {
		note += " · robots.txt 못 받음: " + robotsErr.Error()
	}
	st.source("크롤", "ok", note)
	return s.st.mergeKPI(ctx, st.today, "site", siteSummary(pages, len(urls), len(st.added), len(st.resolved)))
}

// SiteDay — KPI의 site 절.
type SiteDay struct {
	SitemapURLs     int            `json:"sitemapUrls"`
	Crawled         int            `json:"crawled"`
	OK              int            `json:"ok"`
	Errors          int            `json:"errors"`
	Warnings        int            `json:"warnings"`
	PagesWithErrors int            `json:"pagesWithErrors"`
	IssueCounts     map[string]int `json:"issueCounts"`
	P50MS           int            `json:"p50Ms"`
	P95MS           int            `json:"p95Ms"`
	NewIssues       int            `json:"newIssues"`
	Resolved        int            `json:"resolved"`
	Indexed         int            `json:"indexed,omitempty"`
	Inspected       int            `json:"inspected,omitempty"`
}

func siteSummary(pages []*PageResult, sitemap, added, resolved int) SiteDay {
	d := SiteDay{SitemapURLs: sitemap, Crawled: len(pages), IssueCounts: map[string]int{}, NewIssues: added, Resolved: resolved}
	var ms []int
	for _, p := range pages {
		if p.Status == 200 {
			d.OK++
			ms = append(ms, p.ResponseMS)
		}
		hasErr := false
		for _, is := range p.Issues {
			d.IssueCounts[is.Code]++
			switch is.Severity {
			case sevError:
				d.Errors++
				hasErr = true
			case sevWarn:
				d.Warnings++
			}
		}
		if hasErr {
			d.PagesWithErrors++
		}
	}
	sort.Ints(ms)
	d.P50MS, d.P95MS = percentile(ms, 0.5), percentile(ms, 0.95)
	return d
}

func percentile(sorted []int, q float64) int {
	if len(sorted) == 0 {
		return 0
	}
	i := int(math.Ceil(q*float64(len(sorted)))) - 1
	return sorted[max(0, min(i, len(sorted)-1))]
}

// ── 2. Search Console ──────────────────────────────────────────────────────

func (s *Service) googleAuth() (*googleAuth, error) {
	if s.cfg.GoogleCredentialsFile == "" {
		return nil, nil
	}
	// compose는 비밀 폴더를 늘 마운트하고 경로도 늘 넘긴다 — 파일이 없으면 「미설정」이지 오류가 아니다.
	if _, err := os.Stat(s.cfg.GoogleCredentialsFile); os.IsNotExist(err) {
		return nil, nil
	}
	return loadGoogleAuth(s.cfg.GoogleCredentialsFile, s.hc, scopeWebmastersRO, scopeAnalyticsRO)
}

// GoogleDay — KPI의 google 절(날짜만으로 받은 합계 — 익명화된 검색어까지 포함한다).
type GoogleDay struct {
	Clicks      float64 `json:"clicks"`
	Impressions float64 `json:"impressions"`
	Position    float64 `json:"position"`
}

func (s *Service) searchConsole(ctx context.Context, st *runState) {
	auth, err := s.googleAuth()
	if auth == nil {
		if err != nil {
			st.source("Search Console", "error", err.Error())
		} else {
			st.source("Search Console", "off", "서비스 계정 JSON 없음 — docs/GROWTH.md")
		}
		return
	}
	gsc := &gscClient{auth: auth, property: s.cfg.GSCProperty}
	st.ga = &ga4Client{auth: auth, property: s.cfg.GA4PropertyID}

	// 처음이면 90일을 받아 추세의 바탕을 깐다. 이후엔 최근 10일을 다시 받아 잠정치를 확정치로 덮는다.
	from := st.today.AddDate(0, 0, -10)
	if has, err := s.st.hasSearchRows(ctx, engineGoogle); err == nil && !has {
		from = st.today.AddDate(0, 0, -90)
	}
	to := st.today.AddDate(0, 0, -1)

	totals, err := gsc.searchAnalytics(ctx, from, to, []string{"date"})
	if err != nil {
		st.source("Search Console", "error", err.Error())
		return
	}
	for _, r := range totals {
		if len(r.Keys) < 1 {
			continue
		}
		day, err := time.ParseInLocation("2006-01-02", r.Keys[0], kst)
		if err != nil {
			continue
		}
		if err := s.st.mergeKPI(ctx, day, "google", GoogleDay{Clicks: r.Clicks, Impressions: r.Impressions, Position: r.Position}); err != nil {
			st.source("Search Console", "error", err.Error())
			return
		}
	}

	detail, err := gsc.searchAnalytics(ctx, from, to, []string{"date", "query", "page"})
	if err != nil {
		st.source("Search Console", "error", err.Error())
		return
	}
	rows := make([]SearchRow, 0, len(detail))
	for _, r := range detail {
		if len(r.Keys) < 3 {
			continue
		}
		day, err := time.ParseInLocation("2006-01-02", r.Keys[0], kst)
		if err != nil {
			continue
		}
		rows = append(rows, SearchRow{Day: day, Query: r.Keys[1], Page: r.Keys[2],
			Clicks: int(r.Clicks), Impressions: int(r.Impressions), Position: r.Position})
	}
	if err := s.st.replaceSearchRows(ctx, engineGoogle, from, to, rows); err != nil {
		st.source("Search Console", "error", err.Error())
		return
	}
	st.hasGoogle = true

	if sm, err := gsc.sitemaps(ctx); err == nil {
		st.gscSitemaps = sm
	}

	inspected, inspectErr := s.inspect(ctx, st, gsc)
	note := fmt.Sprintf("%s~%s · 행 %d · URL 검사 %d", ymd(from), ymd(to), len(rows), inspected)
	if inspectErr != nil {
		note += " · 검사 중단: " + inspectErr.Error()
	}
	st.source("Search Console", "ok", note)
}

/*
inspect — URL 검사를 하루 할당만큼. 대상 순서: 한 번도 안 본 URL(정적 → 리포트 → 종목) → 가장 오래전에 본 URL.

한도(분당 600)보다 한참 느리게 부른다(0.3초 간격). 한도 초과 응답이 오면 그날은 멈춘다.
*/
func (s *Service) inspect(ctx context.Context, st *runState, gsc *gscClient) (int, error) {
	existing, err := s.st.loadInspections(ctx)
	if err != nil {
		return 0, err
	}
	keep := map[string]bool{}
	for _, p := range st.pages {
		keep[p.URL] = true
	}
	if st.crawlOK {
		_ = s.st.dropInspectionsNotIn(ctx, keep)
	}
	targets := pickInspectTargets(st.pages, existing, s.cfg.InspectPerDay)
	done := 0
	for _, u := range targets {
		if ctx.Err() != nil {
			break
		}
		res, err := gsc.inspect(ctx, u)
		if err != nil {
			if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") {
				break
			}
			st.inspections, _ = s.st.loadInspections(ctx)
			return done, err
		}
		if err := s.st.saveInspection(ctx, u, res, time.Now()); err != nil {
			return done, err
		}
		done++
		select {
		case <-ctx.Done():
		case <-time.After(300 * time.Millisecond):
		}
	}
	st.inspections, _ = s.st.loadInspections(ctx)
	return done, nil
}

func pickInspectTargets(pages []*PageResult, existing map[string]inspectionRow, n int) []string {
	if n <= 0 {
		return nil
	}
	typeRank := map[string]int{"home": 0, "news": 1, "earnings": 1, "section": 1, "report": 2, "legal": 3, "stock": 4}
	var fresh, seen []*PageResult
	for _, p := range pages {
		if _, ok := existing[p.URL]; ok {
			seen = append(seen, p)
		} else {
			fresh = append(fresh, p)
		}
	}
	sort.SliceStable(fresh, func(i, j int) bool { return typeRank[fresh[i].Type] < typeRank[fresh[j].Type] })
	sort.SliceStable(seen, func(i, j int) bool {
		return existing[seen[i].URL].InspectedAt.Before(existing[seen[j].URL].InspectedAt)
	})
	var out []string
	for _, p := range append(fresh, seen...) {
		if len(out) >= n {
			break
		}
		out = append(out, p.URL)
	}
	return out
}

// ── 3. GA4 ─────────────────────────────────────────────────────────────────

// GADay — KPI의 ga 절.
type GADay struct {
	Sessions        float64 `json:"sessions"`
	ActiveUsers     float64 `json:"activeUsers"`
	NewUsers        float64 `json:"newUsers"`
	EngagedSessions float64 `json:"engagedSessions"`
	OrganicSessions float64 `json:"organicSessions"`
}

func (s *Service) analytics(ctx context.Context, st *runState) {
	if s.cfg.GA4PropertyID == "" {
		st.source("GA4", "off", "GROWTH_GA4_PROPERTY_ID 미설정")
		return
	}
	if st.ga == nil {
		st.source("GA4", "off", "Google 자격 증명 미설정")
		return
	}
	from, to := st.today.AddDate(0, 0, -10), st.today.AddDate(0, 0, -1)
	totals, err := st.ga.runReport(ctx, from, to, []string{"date"},
		[]string{"sessions", "activeUsers", "newUsers", "engagedSessions"}, nil, 0)
	if err != nil {
		st.source("GA4", "error", err.Error())
		st.ga = nil
		return
	}
	channels, err := st.ga.runReport(ctx, from, to, []string{"date", "sessionDefaultChannelGroup"}, []string{"sessions"}, nil, 0)
	if err != nil {
		st.source("GA4", "error", err.Error())
		st.ga = nil
		return
	}
	days := map[string]*GADay{}
	get := func(raw string) *GADay {
		if days[raw] == nil {
			days[raw] = &GADay{}
		}
		return days[raw]
	}
	for _, r := range totals {
		if len(r.Dims) < 1 || len(r.Metrics) < 4 {
			continue
		}
		d := get(r.Dims[0])
		d.Sessions, d.ActiveUsers, d.NewUsers, d.EngagedSessions = r.Metrics[0], r.Metrics[1], r.Metrics[2], r.Metrics[3]
	}
	for _, r := range channels {
		if len(r.Dims) < 2 || len(r.Metrics) < 1 {
			continue
		}
		if r.Dims[1] == "Organic Search" {
			get(r.Dims[0]).OrganicSessions += r.Metrics[0]
		}
	}
	for raw, d := range days {
		day, err := time.ParseInLocation("20060102", raw, kst) // GA4는 날짜를 YYYYMMDD로 준다
		if err != nil {
			continue
		}
		if err := s.st.mergeKPI(ctx, day, "ga", d); err != nil {
			st.source("GA4", "error", err.Error())
			return
		}
	}
	st.source("GA4", "ok", fmt.Sprintf("%s~%s · %d일", ymd(from), ymd(to), len(days)))
}

// landingPages — 주간 보고서용: 자연 검색으로 들어온 첫 페이지.
func (s *Service) landingPages(ctx context.Context, ga *ga4Client, from, to time.Time) ([]ga4Row, error) {
	filter := map[string]any{"filter": map[string]any{
		"fieldName":    "sessionDefaultChannelGroup",
		"stringFilter": map[string]any{"value": "Organic Search"},
	}}
	return ga.runReport(ctx, from, to, []string{"landingPage"}, []string{"sessions", "engagedSessions"}, filter, 20)
}

// ── 4. Bing ────────────────────────────────────────────────────────────────

// BingDay — KPI의 bing 절(페이지 행의 합).
type BingDay struct {
	Clicks      int `json:"clicks"`
	Impressions int `json:"impressions"`
}

func (s *Service) bing(ctx context.Context, st *runState) {
	if s.cfg.BingAPIKey == "" {
		st.source("Bing", "off", "GROWTH_BING_API_KEY 미설정")
		return
	}
	b := &bingClient{key: s.cfg.BingAPIKey, siteURL: s.cfg.SiteURL, hc: s.hc}
	queries, err := b.stats(ctx, "GetQueryStats")
	if err != nil {
		st.source("Bing", "error", err.Error())
		return
	}
	pages, err := b.stats(ctx, "GetPageStats")
	if err != nil {
		st.source("Bing", "error", err.Error())
		return
	}
	save := func(engine string, rows []bingRow, asQuery bool) error {
		if len(rows) == 0 {
			return nil
		}
		from, to := rows[0].Day, rows[0].Day
		out := make([]SearchRow, 0, len(rows))
		for _, r := range rows {
			if r.Day.Before(from) {
				from = r.Day
			}
			if r.Day.After(to) {
				to = r.Day
			}
			row := SearchRow{Day: r.Day, Clicks: r.Clicks, Impressions: r.Impressions, Position: r.Position}
			if asQuery {
				row.Query = r.Key
			} else {
				row.Page = r.Key
			}
			out = append(out, row)
		}
		return s.st.replaceSearchRows(ctx, engine, from, to, out)
	}
	if err := save(engineBing, queries, true); err != nil {
		st.source("Bing", "error", err.Error())
		return
	}
	if err := save(engineBingPage, pages, false); err != nil {
		st.source("Bing", "error", err.Error())
		return
	}
	perDay := map[time.Time]*BingDay{}
	for _, r := range pages {
		d := perDay[r.Day]
		if d == nil {
			d = &BingDay{}
			perDay[r.Day] = d
		}
		d.Clicks += r.Clicks
		d.Impressions += r.Impressions
	}
	for day, d := range perDay {
		if err := s.st.mergeKPI(ctx, day, "bing", d); err != nil {
			st.source("Bing", "error", err.Error())
			return
		}
	}
	st.source("Bing", "ok", fmt.Sprintf("검색어 %d행 · 페이지 %d행", len(queries), len(pages)))
}

// ── 5. DB 활동 ──────────────────────────────────────────────────────────────

func (s *Service) dbActivity(ctx context.Context, st *runState) {
	// 최근 28일(보고서가 읽는 가장 긴 구간)을 매번 다시 센다 — 작은 표의 COUNT라 싸고, 세는 규칙
	// (예: 시험 계정 제외)을 고치면 다음 실행이 지난 값까지 바로잡는다.
	const days = 28
	for i := 1; i <= days; i++ {
		day := st.today.AddDate(0, 0, -i)
		a, err := s.act.day(ctx, day)
		if err != nil {
			st.source("DB 활동", "error", err.Error())
			return
		}
		if err := s.st.mergeKPI(ctx, day, "db", a); err != nil {
			st.source("DB 활동", "error", err.Error())
			return
		}
	}
	st.source("DB 활동", "ok", fmt.Sprintf("최근 %d일", days))
	st.source("네이버 서치어드바이저", "off", "공개 API 없음 — 기술 점검은 크롤이 대신한다(검색 성과는 콘솔에서 직접)")
}

func (s *Service) candidates(ctx context.Context, st *runState) {
	names, err := s.act.stockNames(ctx)
	if err != nil {
		log.Printf("growth: stock names: %v", err)
	}
	st.matcher = newStockMatcher(names)
	if st.indexable == nil {
		st.indexable = map[string]bool{}
	}
	if st.earnings, err = s.act.upcomingEarnings(ctx, st.today, st.indexable); err != nil {
		log.Printf("growth: earnings: %v", err)
	}
	if st.shares, err = s.act.shareCandidates(ctx, st.today, st.indexable); err != nil {
		log.Printf("growth: share candidates: %v", err)
	}
	if st.inspections == nil {
		st.inspections, _ = s.st.loadInspections(ctx)
	}
}

// writeReportFile — 보고서 사본. 실패해도 DB 본은 남았으므로 로그만.
func (s *Service) writeReportFile(kind string, end time.Time, md string) {
	if s.cfg.ReportDir == "" {
		return
	}
	if err := os.MkdirAll(s.cfg.ReportDir, 0o755); err != nil {
		log.Printf("growth: report dir: %v", err)
		return
	}
	path := filepath.Join(s.cfg.ReportDir, fmt.Sprintf("%s-%s.md", kind, ymd(end)))
	if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
		log.Printf("growth: write %s: %v", path, err)
	}
}
