package growth

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// ── KPI 구간 합 ────────────────────────────────────────────────────────────

type kpiWindow struct {
	GoogleDays                             int
	GoogleClicks, GoogleImpr, googlePosW   float64
	GADays                                 int
	GASessions, GAOrganic, GAActive, GANew float64
	GAEngaged                              float64
	BingDays                               int
	BingClicks, BingImpr                   float64
	DBDays                                 int
	Signups, Active, FirstHolding          int
	Holdings, Watchlist, Reports           int
}

func (w kpiWindow) GooglePosition() float64 {
	if w.GoogleImpr == 0 {
		return 0
	}
	return w.googlePosW / w.GoogleImpr
}

func (w kpiWindow) GoogleCTR() float64 {
	if w.GoogleImpr == 0 {
		return 0
	}
	return w.GoogleClicks / w.GoogleImpr
}

func sumKPI(kpi map[string]kpiDay, from, to time.Time) kpiWindow {
	var w kpiWindow
	for d := dayOf(from); !d.After(to); d = d.AddDate(0, 0, 1) {
		day, ok := kpi[ymd(d)]
		if !ok {
			continue
		}
		var g GoogleDay
		if raw, ok := day["google"]; ok && json.Unmarshal(raw, &g) == nil {
			w.GoogleDays++
			w.GoogleClicks += g.Clicks
			w.GoogleImpr += g.Impressions
			w.googlePosW += g.Position * g.Impressions
		}
		var ga GADay
		if raw, ok := day["ga"]; ok && json.Unmarshal(raw, &ga) == nil {
			w.GADays++
			w.GASessions += ga.Sessions
			w.GAOrganic += ga.OrganicSessions
			w.GAActive += ga.ActiveUsers
			w.GANew += ga.NewUsers
			w.GAEngaged += ga.EngagedSessions
		}
		var b BingDay
		if raw, ok := day["bing"]; ok && json.Unmarshal(raw, &b) == nil {
			w.BingDays++
			w.BingClicks += float64(b.Clicks)
			w.BingImpr += float64(b.Impressions)
		}
		var a DayActivity
		if raw, ok := day["db"]; ok && json.Unmarshal(raw, &a) == nil {
			w.DBDays++
			w.Signups += a.Signups
			w.Active += a.ActiveUsers
			w.FirstHolding += a.FirstHoldingUsers
			w.Holdings += a.HoldingsAdded
			w.Watchlist += a.WatchlistAdded
			w.Reports += a.ReportsPublished
		}
	}
	return w
}

// latestGoogleDay — Search Console 값이 있는 가장 최근 날. 보통 오늘보다 2~3일 앞이다.
func latestGoogleDay(kpi map[string]kpiDay, today time.Time) (time.Time, bool) {
	for i := 1; i <= 14; i++ {
		d := today.AddDate(0, 0, -i)
		if day, ok := kpi[ymd(d)]; ok {
			if _, ok := day["google"]; ok {
				return d, true
			}
		}
	}
	return time.Time{}, false
}

// ── Markdown 도우미 ────────────────────────────────────────────────────────

type md struct{ b strings.Builder }

func (m *md) line(format string, a ...any) { fmt.Fprintf(&m.b, format+"\n", a...) }
func (m *md) blank()                       { m.b.WriteString("\n") }
func (m *md) h(level int, title string) {
	m.blank()
	m.line("%s %s", strings.Repeat("#", level), title)
	m.blank()
}

func (m *md) table(header []string, rows [][]string) {
	if len(rows) == 0 {
		m.line("_없음_")
		return
	}
	m.line("| %s |", strings.Join(header, " | "))
	seps := make([]string, len(header))
	for i := range seps {
		seps[i] = "---"
	}
	m.line("| %s |", strings.Join(seps, " | "))
	for _, r := range rows {
		cells := make([]string, len(r))
		for i, c := range r {
			cells[i] = strings.ReplaceAll(c, "|", "\\|")
		}
		m.line("| %s |", strings.Join(cells, " | "))
	}
}

func num(v float64) string {
	if math.Abs(v-math.Round(v)) < 1e-9 {
		return fmt.Sprintf("%s", groupThousands(int64(math.Round(v))))
	}
	return fmt.Sprintf("%.1f", v)
}

func groupThousands(n int64) string {
	s := fmt.Sprintf("%d", n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var out []string
	for len(s) > 3 {
		out = append([]string{s[len(s)-3:]}, out...)
		s = s[:len(s)-3]
	}
	out = append([]string{s}, out...)
	r := strings.Join(out, ",")
	if neg {
		r = "-" + r
	}
	return r
}

// delta — 「+12 (+8%)」. 이전이 0이면 비율을 쓰지 않는다.
func delta(cur, prev float64) string {
	d := cur - prev
	sign := "+"
	if d < 0 {
		sign = "−"
	}
	if prev == 0 {
		if d == 0 {
			return "0"
		}
		return sign + num(math.Abs(d))
	}
	return fmt.Sprintf("%s%s (%s%.0f%%)", sign, num(math.Abs(d)), sign, math.Abs(d)/prev*100)
}

// kpiRows — 이번/이전 구간 비교 표의 줄. 소스가 없는 줄은 뺀다.
func kpiRows(cur, prev kpiWindow) [][]string {
	var rows [][]string
	add := func(name string, c, p float64) {
		rows = append(rows, []string{name, num(c), num(p), delta(c, p)})
	}
	if cur.GoogleDays > 0 || prev.GoogleDays > 0 {
		add("Google 클릭", cur.GoogleClicks, prev.GoogleClicks)
		add("Google 노출", cur.GoogleImpr, prev.GoogleImpr)
		rows = append(rows, []string{"Google CTR", pct(cur.GoogleCTR()), pct(prev.GoogleCTR()), fmt.Sprintf("%+.2f%%p", (cur.GoogleCTR()-prev.GoogleCTR())*100)})
		rows = append(rows, []string{"Google 평균 순위", fmt.Sprintf("%.1f", cur.GooglePosition()), fmt.Sprintf("%.1f", prev.GooglePosition()),
			fmt.Sprintf("%+.1f", cur.GooglePosition()-prev.GooglePosition())})
	}
	if cur.BingDays > 0 || prev.BingDays > 0 {
		add("Bing 클릭", cur.BingClicks, prev.BingClicks)
		add("Bing 노출", cur.BingImpr, prev.BingImpr)
	}
	if cur.GADays > 0 || prev.GADays > 0 {
		add("GA 세션", cur.GASessions, prev.GASessions)
		add("GA 자연 검색 세션", cur.GAOrganic, prev.GAOrganic)
		add("GA 활성 사용자(일 합)", cur.GAActive, prev.GAActive)
		add("GA 참여 세션", cur.GAEngaged, prev.GAEngaged)
	}
	if cur.DBDays > 0 || prev.DBDays > 0 {
		add("가입", float64(cur.Signups), float64(prev.Signups))
		add("로그인 사용자(일 합)", float64(cur.Active), float64(prev.Active))
		add("첫 보유 등록 사용자", float64(cur.FirstHolding), float64(prev.FirstHolding))
		add("보유 종목 추가", float64(cur.Holdings), float64(prev.Holdings))
		add("관심 종목 추가", float64(cur.Watchlist), float64(prev.Watchlist))
		add("리포트 발행", float64(cur.Reports), float64(prev.Reports))
	}
	return rows
}

func pct(v float64) string { return fmt.Sprintf("%.2f%%", v*100) }

// countRows — {이름: 개수}를 많은 순 표 줄로.
func countRows(counts map[string]int, label func(string) string) [][]string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	rows := make([][]string, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, []string{label(k), fmt.Sprintf("%d", counts[k])})
	}
	return rows
}

func sourceLines(m *md, sources []sourceState) {
	icon := map[string]string{"ok": "✅", "off": "⚪", "error": "❌"}
	for _, s := range sources {
		note := ""
		if s.Note != "" {
			note = " — " + s.Note
		}
		m.line("- %s **%s**%s", icon[s.Status], s.Name, note)
	}
}

func oppRows(xs []Opportunity, n int, withWhere bool) [][]string {
	rows := [][]string{}
	for i, o := range head(xs, n) {
		r := []string{fmt.Sprintf("%d", i+1), opportunityKinds[o.Kind], o.Title, o.Evidence, gainLabel(o), o.Action}
		if withWhere {
			r = append(r, o.Where)
		}
		rows = append(rows, r)
	}
	return rows
}

// gainLabel — 검색 수요로 잰 기회만 「+N 클릭/주」로 적는다. 기술 가중치는 숫자로 말하지 않는다.
func gainLabel(o Opportunity) string {
	switch o.Kind {
	case "tech", "link":
		if o.Gain >= 1 && strings.Contains(o.Title, "노출") {
			return fmt.Sprintf("+%.0f", o.Gain)
		}
		return "—"
	default:
		if o.Gain < 0.5 {
			return "<1"
		}
		return fmt.Sprintf("+%.0f", o.Gain)
	}
}

// ── 공통 재료 ──────────────────────────────────────────────────────────────

type searchWindows struct {
	curFrom, curTo, prevFrom, prevTo time.Time
	queries                          []QueryStat
	pages                            []PageStat
	has                              bool
}

func (s *Service) loadWindows(ctx context.Context, end time.Time) (searchWindows, error) {
	w := searchWindows{curTo: end, curFrom: end.AddDate(0, 0, -6), prevTo: end.AddDate(0, 0, -7), prevFrom: end.AddDate(0, 0, -13)}
	rows, err := s.st.loadSearchRows(ctx, engineGoogle, w.prevFrom, w.curTo)
	if err != nil {
		return w, err
	}
	var cur, prev []SearchRow
	for _, r := range rows {
		if r.Day.Before(w.curFrom) {
			prev = append(prev, r)
		} else {
			cur = append(cur, r)
		}
	}
	w.has = len(rows) > 0
	w.queries = compareQueries(cur, prev)
	w.pages = comparePages(cur, prev)
	return w, nil
}

func (s *Service) analysisFor(st *runState, w searchWindows) analysisOutput {
	return analyze(analysisInput{
		Queries: w.queries, Pages: w.pages, Crawl: st.pages, Indexable: st.indexable,
		Matcher: st.matcher, SiteURL: s.cfg.SiteURL, Earnings: st.earnings, HasSearch: w.has, Probe: st.probe,
	})
}

func writeCrawlSection(m *md, st *runState) {
	if p := st.probe; p != nil {
		m.line("**공개 주소 도달성**(Cloudflare 경유, 재시도 없음): %d회 중 정상 %d · 3초 초과 %d · 실패 %d · 가장 느림 %s %.1f초",
			p.Tried, p.OK, p.Slow, p.Failed, p.Worst, float64(p.MaxMS)/1000)
		m.blank()
	}
	sum := siteSummary(st.pages, st.sitemapURLs, len(st.added), len(st.resolved))
	m.line("사이트맵 %d URL 중 %d개를 열었다 · 200 응답 %d · 오류 %d건(%d페이지) · 경고 %d건 · 응답 p50 %dms / p95 %dms",
		sum.SitemapURLs, sum.Crawled, sum.OK, sum.Errors, sum.PagesWithErrors, sum.Warnings, sum.P50MS, sum.P95MS)
	if st.removed > 0 {
		m.line("사이트맵에서 빠진 URL %d개를 목록에서 지웠다.", st.removed)
	}
	m.blank()
	type codeCount struct {
		code string
		n    int
		sev  string
	}
	sevOf := map[string]string{}
	byType := map[string]map[string]int{}
	for _, p := range st.pages {
		for _, is := range p.Issues {
			sevOf[is.Code] = is.Severity
			if byType[is.Code] == nil {
				byType[is.Code] = map[string]int{}
			}
			byType[is.Code][p.Type]++
		}
	}
	var counts []codeCount
	for code, n := range sum.IssueCounts {
		counts = append(counts, codeCount{code, n, sevOf[code]})
	}
	sort.Slice(counts, func(i, j int) bool {
		if severityRank(counts[i].sev) != severityRank(counts[j].sev) {
			return severityRank(counts[i].sev) < severityRank(counts[j].sev)
		}
		return counts[i].n > counts[j].n
	})
	rows := [][]string{}
	for _, c := range counts {
		rows = append(rows, []string{c.sev, issueLabel(c.code), fmt.Sprintf("%d", c.n), typeBreakdown(byType[c.code])})
	}
	m.table([]string{"등급", "문제", "페이지", "종류별"}, rows)

	if len(st.added) > 0 {
		m.blank()
		m.line("**새로 생긴 문제** (%d건, 앞 15건)", len(st.added))
		m.blank()
		for _, c := range head(st.added, 15) {
			m.line("- [%s] %s — `%s` %s", c.Issue.Severity, issueLabel(c.Issue.Code), shortPath(c.URL), c.Issue.Detail)
		}
	}
	if len(st.resolved) > 0 {
		m.blank()
		m.line("**고쳐진 문제** (%d건, 앞 10건)", len(st.resolved))
		m.blank()
		for _, c := range head(st.resolved, 10) {
			m.line("- %s — `%s`", issueLabel(c.Issue.Code), shortPath(c.URL))
		}
	}
}

// writeStockQuality — 매일 항목 8. 종목 페이지만 따로 센다.
func writeStockQuality(m *md, st *runState) {
	total, ok := 0, 0
	codes := map[string]int{}
	var textSum int
	for _, p := range st.pages {
		if p.Type != "stock" {
			continue
		}
		total++
		if p.Status == 200 {
			textSum += p.Facts.TextChars
		}
		clean := true
		for _, is := range p.Issues {
			if is.Severity == sevInfo || is.Code == "orphan" {
				continue
			}
			codes[is.Code]++
			clean = false
		}
		if clean {
			ok++
		}
	}
	if total == 0 {
		m.line("_사이트맵에 종목 페이지가 없다._")
		return
	}
	m.line("색인 대상 종목 페이지 %d개 중 **%d개(%.0f%%)가 오류·경고 없음**(고아 링크 제외) · 서버가 그린 본문 평균 %d자",
		total, ok, float64(ok)/float64(total)*100, textSum/max(1, total))
	if len(codes) > 0 {
		m.blank()
		m.table([]string{"문제", "종목 페이지"}, countRows(codes, issueLabel))
	}
}

// writeIndexSection — 매일 항목 3.
func writeIndexSection(m *md, st *runState) {
	if len(st.inspections) == 0 {
		m.line("_URL 검사 기록이 없다 — Search Console 자격 증명이 설정되면 매일 할당만큼 돌려 본다(GROWTH_INSPECT_PER_DAY)._")
		return
	}
	byCoverage := map[string]int{}
	pass := 0
	var changed []inspectionRow
	var canonDiff []inspectionRow
	for _, r := range st.inspections {
		byCoverage[r.CoverageState]++
		if r.Verdict == "PASS" {
			pass++
		}
		if r.ChangedAt != nil && r.PrevCoverage != "" && time.Since(*r.ChangedAt) < 36*time.Hour {
			changed = append(changed, r)
		}
		if r.GoogleCanonical != "" && r.UserCanonical != "" && keyOf(r.GoogleCanonical) != keyOf(r.UserCanonical) {
			canonDiff = append(canonDiff, r)
		}
	}
	m.line("검사한 URL %d개 / 사이트맵 %d개 · **색인됨(PASS) %d개**", len(st.inspections), st.sitemapURLs, pass)
	m.blank()
	m.table([]string{"Google 상태", "URL"}, countRows(byCoverage, func(k string) string {
		if k == "" {
			return "(없음)"
		}
		return k
	}))
	if len(changed) > 0 {
		m.blank()
		m.line("**최근 상태가 바뀐 URL**")
		m.blank()
		for _, r := range head(changed, 15) {
			m.line("- `%s` %s → %s", shortPath(r.URL), r.PrevCoverage, r.CoverageState)
		}
	}
	if len(canonDiff) > 0 {
		m.blank()
		m.line("**Google이 다른 canonical을 골랐다** (%d개)", len(canonDiff))
		m.blank()
		for _, r := range head(canonDiff, 10) {
			m.line("- `%s` → Google: `%s`", shortPath(r.UserCanonical), shortPath(r.GoogleCanonical))
		}
	}
	if len(st.gscSitemaps) > 0 {
		m.blank()
		for _, sm := range st.gscSitemaps {
			submitted := ""
			for _, c := range sm.Contents {
				submitted += fmt.Sprintf(" %s %s", c.Type, c.Submitted)
			}
			m.line("- 사이트맵 `%s` 마지막 수집 %s · 오류 %s · 경고 %s ·%s", sm.Path, sm.LastDownloaded, sm.Errors, sm.Warnings, submitted)
		}
	}
}

func writeMoves(m *md, w searchWindows, limit int) {
	mv := queryMoves(w.queries, limit)
	row := func(q QueryStat) []string {
		return []string{q.Query, fmt.Sprintf("%d → %d", q.PrevImpressions, q.Impressions), fmt.Sprintf("%d → %d", q.PrevClicks, q.Clicks),
			posMove(q.PrevPosition, q.Position), shortPath(q.TopPage)}
	}
	hdr := []string{"검색어", "노출", "클릭", "순위", "페이지"}
	for _, part := range []struct {
		title string
		xs    []QueryStat
	}{{"오른 검색어", mv.Up}, {"떨어진 검색어", mv.Down}, {"새 검색어", mv.New}, {"사라진 검색어", mv.Lost}} {
		m.line("**%s**", part.title)
		m.blank()
		var rows [][]string
		for _, q := range part.xs {
			rows = append(rows, row(q))
		}
		m.table(hdr, rows)
		m.blank()
	}
}

func posMove(prev, cur float64) string {
	switch {
	case prev == 0 && cur == 0:
		return "—"
	case prev == 0:
		return fmt.Sprintf("→ %.1f", cur)
	case cur == 0:
		return fmt.Sprintf("%.1f →", prev)
	}
	return fmt.Sprintf("%.1f → %.1f", prev, cur)
}

func writeShares(m *md, st *runState, siteURL string) {
	if len(st.shares) == 0 {
		m.line("_후보 없음_")
		return
	}
	for _, c := range st.shares {
		m.line("- %s — %s%s (%s)", c.Title, siteURL, c.Path, c.Reason)
	}
}

const guardNote = "이 보고서는 **제안만** 한다. worker-growth는 growth_* 표와 보고서 파일만 쓰며, 투자 알고리즘 · 점수 산식 · 금융 원본 · 백테스트 · 수익률 · 개인정보 · 인증 · 결제 · 법적 고지를 바꾸지 않는다. 코드 변경은 사람이 브랜치 → 시험 → 미리보기 → SEO 점검 → 승인 → 배포 순서로 한다."

// ── 일간 ───────────────────────────────────────────────────────────────────

func (s *Service) buildDaily(ctx context.Context, st *runState) (int64, error) {
	kpi, err := s.st.loadKPI(ctx, st.today.AddDate(0, 0, -30), st.today)
	if err != nil {
		return 0, err
	}
	end := st.today.AddDate(0, 0, -1)
	if g, ok := latestGoogleDay(kpi, st.today); ok {
		end = g
	}
	w, err := s.loadWindows(ctx, end)
	if err != nil {
		return 0, err
	}
	an := s.analysisFor(st, w)
	curK := sumKPI(kpi, end.AddDate(0, 0, -6), end)
	prevK := sumKPI(kpi, end.AddDate(0, 0, -13), end.AddDate(0, 0, -7))
	// DB 활동은 어제까지 확정이다 — 검색 지연(2~3일)에 끌려가지 않게 따로 잰다.
	dbCur := sumKPI(kpi, st.today.AddDate(0, 0, -7), st.today.AddDate(0, 0, -1))
	dbPrev := sumKPI(kpi, st.today.AddDate(0, 0, -14), st.today.AddDate(0, 0, -8))
	curK.DBDays, curK.Signups, curK.Active, curK.FirstHolding, curK.Holdings, curK.Watchlist, curK.Reports =
		dbCur.DBDays, dbCur.Signups, dbCur.Active, dbCur.FirstHolding, dbCur.Holdings, dbCur.Watchlist, dbCur.Reports
	prevK.DBDays, prevK.Signups, prevK.Active, prevK.FirstHolding, prevK.Holdings, prevK.Watchlist, prevK.Reports =
		dbPrev.DBDays, dbPrev.Signups, dbPrev.Active, dbPrev.FirstHolding, dbPrev.Holdings, dbPrev.Watchlist, dbPrev.Reports

	var m md
	m.line("# 검색 유입 일간 — %s", ymd(st.today))
	m.blank()
	m.line("> 생성 %s KST · 검색 지표 구간 %s~%s(직전 7일과 비교) · 가입·활동은 어제까지 7일", st.now.Format("2006-01-02 15:04"), ymd(w.curFrom), ymd(w.curTo))

	m.h(2, "데이터 소스")
	sourceLines(&m, st.sources)

	m.h(2, "KPI")
	m.table([]string{"지표", "최근 7일", "이전 7일", "변화"}, kpiRows(curK, prevK))

	m.h(2, "오늘의 기회 Top 5")
	m.table([]string{"#", "종류", "할 일", "근거", "클릭/주(어림)", "방법"}, oppRows(an.Opportunities, 5, false))

	m.h(2, "크롤 · 오류")
	writeCrawlSection(&m, st)

	m.h(2, "종목 페이지 품질")
	writeStockQuality(&m, st)

	m.h(2, "색인 상태")
	writeIndexSection(&m, st)

	if w.has {
		m.h(2, "검색어 변화")
		writeMoves(&m, w, 5)

		m.h(2, "페이지 성과")
		m.table([]string{"페이지", "노출", "클릭", "순위"}, pageRows(w.pages, 10))
	}

	m.h(2, "신규 콘텐츠 후보")
	m.table([]string{"#", "종류", "후보", "근거", "클릭/주(어림)", "방법"}, oppRows(an.Content, 5, false))

	m.h(2, "내부 링크 개선안")
	m.table([]string{"#", "종류", "대상", "근거", "클릭/주(어림)", "방법"}, oppRows(an.Links, 5, false))

	m.h(2, "외부 유통 후보")
	writeShares(&m, st, s.cfg.SiteURL)

	m.blank()
	m.line("---")
	m.line("_%s_", guardNote)

	text := m.b.String()
	data := map[string]any{
		"sources": st.sources, "kpi": map[string]any{"current": curK, "previous": prevK},
		"opportunities": head(an.Opportunities, 20), "content": head(an.Content, 20), "links": head(an.Links, 20),
		"shares": st.shares, "newIssues": len(st.added), "resolvedIssues": len(st.resolved),
	}
	id, err := s.st.saveReport(ctx, "daily", st.today, st.today, text, data)
	if err == nil {
		s.writeReportFile("daily", st.today, text)
	}
	return id, err
}

func pageRows(pages []PageStat, n int) [][]string {
	var rows [][]string
	for _, p := range head(pages, n) {
		rows = append(rows, []string{shortPath(p.Page), fmt.Sprintf("%d → %d", p.PrevImpressions, p.Impressions),
			fmt.Sprintf("%d → %d", p.PrevClicks, p.Clicks), fmt.Sprintf("%.1f", p.Position)})
	}
	return rows
}

// ── 주간 ───────────────────────────────────────────────────────────────────

func (s *Service) buildWeekly(ctx context.Context, st *runState, weekEnd time.Time) (int64, error) {
	weekStart := weekEnd.AddDate(0, 0, -6)
	kpi, err := s.st.loadKPI(ctx, weekStart.AddDate(0, 0, -14), weekEnd)
	if err != nil {
		return 0, err
	}
	w, err := s.loadWindows(ctx, weekEnd)
	if err != nil {
		return 0, err
	}
	an := s.analysisFor(st, w)
	curK := sumKPI(kpi, weekStart, weekEnd)
	prevK := sumKPI(kpi, weekStart.AddDate(0, 0, -7), weekStart.AddDate(0, 0, -1))

	var m md
	year, week := weekEnd.ISOWeek()
	m.line("# 검색 유입 주간 — %d-W%02d (%s ~ %s)", year, week, ymd(weekStart), ymd(weekEnd))
	m.blank()
	m.line("> 생성 %s KST · Search Console은 2~3일 늦게 확정된다 — 주 끝 하루이틀은 잠정치다.", st.now.Format("2006-01-02 15:04"))

	m.h(2, "데이터 소스")
	sourceLines(&m, st.sources)

	m.h(2, "KPI — 이번 주 vs 지난주")
	m.table([]string{"지표", "이번 주", "지난주", "변화"}, kpiRows(curK, prevK))

	// 활성화 코호트: 지난주·그 전 주 가입자의 7일 안 활성화(보유 또는 관심 종목 하나 이상).
	c1, err1 := s.act.cohort(ctx, weekStart.AddDate(0, 0, -7), weekStart.AddDate(0, 0, -1))
	c2, err2 := s.act.cohort(ctx, weekStart.AddDate(0, 0, -14), weekStart.AddDate(0, 0, -8))
	if err1 == nil && err2 == nil {
		m.blank()
		rate := func(c Cohort) string {
			if c.Signups == 0 {
				return "—"
			}
			return fmt.Sprintf("%.0f%%", float64(c.Activated)/float64(c.Signups)*100)
		}
		m.table([]string{"가입 주", "가입", "7일 안 활성화", "비율"}, [][]string{
			{c1.From + "~" + c1.To, fmt.Sprintf("%d", c1.Signups), fmt.Sprintf("%d", c1.Activated), rate(c1)},
			{c2.From + "~" + c2.To, fmt.Sprintf("%d", c2.Signups), fmt.Sprintf("%d", c2.Activated), rate(c2)},
		})
		m.line("_활성화 = 가입 후 7일 안에 보유 또는 관심 종목을 하나 이상 담음. 이번 주 가입자는 7일이 안 지나 다음 주에 잰다._")
	}

	m.h(2, "Top 20 기회")
	m.line("「클릭/주」는 기대 클릭률 곡선으로 어림한 추가 클릭이다 — 기회끼리 줄 세우는 잣대로만 읽는다.")
	m.blank()
	m.table([]string{"#", "종류", "할 일", "근거", "클릭/주(어림)", "방법", "고칠 자리"}, oppRows(an.Opportunities, 20, true))

	m.h(2, "다음 주 우선순위")
	for i, o := range head(an.Opportunities, 5) {
		m.line("%d. **%s** — %s", i+1, o.Title, o.Action)
	}
	if len(an.Opportunities) == 0 {
		m.line("_기회 목록이 비었다 — 검색 데이터가 없으면 기술 점검만으로는 순서를 매길 근거가 적다._")
	}

	m.h(2, "기술 수정 제안")
	m.line("PR은 만들지 않는다. 아래를 사람이 이슈 → 브랜치로 옮긴다.")
	m.blank()
	var tech []Opportunity
	for _, o := range an.Opportunities {
		if o.Kind == "tech" {
			tech = append(tech, o)
		}
	}
	m.table([]string{"#", "종류", "문제", "근거", "—", "방법", "고칠 자리"}, oppRows(tech, 30, true))

	m.h(2, "콘텐츠 제안")
	m.table([]string{"#", "종류", "후보", "근거", "클릭/주(어림)", "방법", "고칠 자리"}, oppRows(an.Content, 15, true))

	m.h(2, "검색어별 개선안")
	m.table([]string{"#", "종류", "검색어", "근거", "클릭/주(어림)", "방법", "고칠 자리"}, oppRows(an.QueryActions, 20, true))

	m.h(2, "내부 링크")
	m.table([]string{"#", "종류", "대상", "근거", "클릭/주(어림)", "방법", "고칠 자리"}, oppRows(an.Links, 10, true))

	if w.has {
		m.h(2, "검색어 변화 (주간)")
		writeMoves(&m, w, 10)

		m.h(2, "페이지 성과")
		m.table([]string{"페이지", "노출", "클릭", "순위"}, pageRows(w.pages, 20))
		var dropping []PageStat
		for _, p := range w.pages {
			if p.PrevImpressions >= 20 && float64(p.Impressions) < float64(p.PrevImpressions)*0.5 {
				dropping = append(dropping, p)
			}
		}
		if len(dropping) > 0 {
			m.blank()
			m.line("**노출이 절반 아래로 준 페이지**")
			m.blank()
			m.table([]string{"페이지", "노출", "클릭", "순위"}, pageRows(dropping, 10))
		}
	}

	if st.ga != nil {
		if rows, err := s.landingPages(ctx, st.ga, weekStart, weekEnd); err == nil && len(rows) > 0 {
			m.h(2, "자연 검색 첫 페이지 (GA4)")
			var t [][]string
			for _, r := range rows {
				if len(r.Dims) < 1 || len(r.Metrics) < 2 {
					continue
				}
				rate := 0.0
				if r.Metrics[0] > 0 {
					rate = r.Metrics[1] / r.Metrics[0]
				}
				t = append(t, []string{r.Dims[0], num(r.Metrics[0]), fmt.Sprintf("%.0f%%", rate*100)})
			}
			m.table([]string{"첫 페이지", "세션", "참여율"}, t)
		}
	}

	m.h(2, "외부 유통 후보")
	writeShares(&m, st, s.cfg.SiteURL)

	m.h(2, "색인 상태")
	writeIndexSection(&m, st)

	m.blank()
	m.line("---")
	m.line("_%s_", guardNote)

	text := m.b.String()
	data := map[string]any{
		"sources": st.sources, "kpi": map[string]any{"current": curK, "previous": prevK},
		"top20": head(an.Opportunities, 20), "content": an.Content, "queryActions": head(an.QueryActions, 50), "links": an.Links,
	}
	id, err := s.st.saveReport(ctx, "weekly", weekStart, weekEnd, text, data)
	if err == nil {
		s.writeReportFile("weekly", weekEnd, text)
	}
	return id, err
}
