package growth

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

/*
store — growth_* 표 읽고 쓰기.

**이 파일의 쓰기 문장은 growth_ 표만 겨눈다.** 앱의 다른 표는 `activity.go`가 읽기만 한다.
`store_guard_test.go`가 이 패키지 소스의 INSERT/UPDATE/DELETE/REPLACE 대상을 세어 지킨다.
*/
type store struct {
	db *sql.DB
}

// ── 크롤 ───────────────────────────────────────────────────────────────────

type prevCrawl struct {
	issues    map[string][]Issue
	firstSeen map[string]time.Time
}

func (s *store) loadPrevCrawl(ctx context.Context) (prevCrawl, error) {
	out := prevCrawl{issues: map[string][]Issue{}, firstSeen: map[string]time.Time{}}
	rows, err := s.db.QueryContext(ctx, `SELECT url, issues, first_seen_at FROM growth_crawl_pages`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var u string
		var raw sql.NullString
		var first time.Time
		if err := rows.Scan(&u, &raw, &first); err != nil {
			return out, err
		}
		var is []Issue
		if raw.Valid {
			_ = json.Unmarshal([]byte(raw.String), &is)
		}
		out.issues[u] = is
		out.firstSeen[u] = first
	}
	return out, rows.Err()
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// saveCrawl — 이번 결과로 덮는다. complete일 때만 사이트맵에서 빠진 URL을 지운다 — 크롤이 중간에
// 끊겼는데 지우면 못 연 URL이 「빠진 것」으로 사라진다.
func (s *store) saveCrawl(ctx context.Context, pages []*PageResult, prev prevCrawl, at time.Time, complete bool) (removed int64, err error) {
	at = at.In(kst).Truncate(time.Second)
	const batch = 100
	for i := 0; i < len(pages); i += batch {
		end := min(i+batch, len(pages))
		var b strings.Builder
		b.WriteString(`INSERT INTO growth_crawl_pages
			(url, page_type, status_code, location, response_ms, html_bytes, title, description, canonical,
			 robots, h1_count, text_chars, links_out, links_in, has_og_image, jsonld_types, issues, first_seen_at, crawled_at)
			VALUES `)
		args := make([]any, 0, (end-i)*19)
		for j, p := range pages[i:end] {
			if j > 0 {
				b.WriteString(",")
			}
			b.WriteString("(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
			issues, _ := json.Marshal(p.Issues)
			first, ok := prev.firstSeen[p.URL]
			if !ok {
				first = at
			}
			f := p.Facts
			args = append(args, clip(p.URL, 600), p.Type, p.Status, clip(p.Location, 600), p.ResponseMS, p.Bytes,
				clip(f.Title, 300), clip(f.Description, 600), clip(f.Canonical, 600), clip(f.Robots, 160),
				f.H1Count, f.TextChars, len(f.Links), p.LinksIn, f.OGImage, clip(strings.Join(f.JSONLDTypes, ","), 200),
				string(issues), first, at)
		}
		b.WriteString(` ON DUPLICATE KEY UPDATE
			page_type=VALUES(page_type), status_code=VALUES(status_code), location=VALUES(location),
			response_ms=VALUES(response_ms), html_bytes=VALUES(html_bytes), title=VALUES(title),
			description=VALUES(description), canonical=VALUES(canonical), robots=VALUES(robots),
			h1_count=VALUES(h1_count), text_chars=VALUES(text_chars), links_out=VALUES(links_out),
			links_in=VALUES(links_in), has_og_image=VALUES(has_og_image), jsonld_types=VALUES(jsonld_types),
			issues=VALUES(issues), crawled_at=VALUES(crawled_at)`)
		if _, err := s.db.ExecContext(ctx, b.String(), args...); err != nil {
			return 0, fmt.Errorf("save crawl: %w", err)
		}
	}
	if !complete {
		return 0, nil
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM growth_crawl_pages WHERE crawled_at < ?`, at)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ── 검색 성과 ───────────────────────────────────────────────────────────────

// SearchRow — 검색 성과 한 줄. Day는 서울 날짜.
type SearchRow struct {
	Day         time.Time
	Query       string
	Page        string
	Clicks      int
	Impressions int
	Position    float64
}

const (
	engineGoogle   = "google"
	engineBing     = "bing"     // 검색어 행 (page = '')
	engineBingPage = "bingpage" // 페이지 행 (query = '')
)

func rowKey(query, page string) []byte {
	sum := md5.Sum([]byte(query + "\x00" + page))
	return sum[:]
}

// replaceSearchRows — [from, to] 구간을 통째로 갈아 끼운다. 잠정치가 확정치로 바뀌는 것을 받는다.
func (s *store) replaceSearchRows(ctx context.Context, engine string, from, to time.Time, rows []SearchRow) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM growth_search_rows WHERE engine = ? AND day BETWEEN ? AND ?`,
		engine, ymd(from), ymd(to)); err != nil {
		return err
	}
	const batch = 500
	for i := 0; i < len(rows); i += batch {
		end := min(i+batch, len(rows))
		var b strings.Builder
		b.WriteString(`INSERT INTO growth_search_rows (engine, day, row_key, query, page, clicks, impressions, position) VALUES `)
		args := make([]any, 0, (end-i)*8)
		for j, r := range rows[i:end] {
			if j > 0 {
				b.WriteString(",")
			}
			b.WriteString("(?,?,?,?,?,?,?,?)")
			q, p := clip(r.Query, 400), clip(r.Page, 600)
			args = append(args, engine, ymd(r.Day), rowKey(q, p), q, p, r.Clicks, r.Impressions, r.Position)
		}
		// 같은 날 같은 키가 두 번 오면(자른 문자열이 겹칠 때) 합친다.
		b.WriteString(` ON DUPLICATE KEY UPDATE clicks = clicks + VALUES(clicks), impressions = impressions + VALUES(impressions)`)
		if _, err := tx.ExecContext(ctx, b.String(), args...); err != nil {
			return fmt.Errorf("save search rows: %w", err)
		}
	}
	return tx.Commit()
}

func (s *store) loadSearchRows(ctx context.Context, engine string, from, to time.Time) ([]SearchRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT day, query, page, clicks, impressions, position
		  FROM growth_search_rows WHERE engine = ? AND day BETWEEN ? AND ?`, engine, ymd(from), ymd(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SearchRow
	for rows.Next() {
		var r SearchRow
		if err := rows.Scan(&r.Day, &r.Query, &r.Page, &r.Clicks, &r.Impressions, &r.Position); err != nil {
			return nil, err
		}
		r.Day = dayOf(r.Day)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *store) hasSearchRows(ctx context.Context, engine string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (SELECT 1 FROM growth_search_rows WHERE engine = ? LIMIT 1) t`, engine).Scan(&n)
	return n > 0, err
}

// ── KPI ────────────────────────────────────────────────────────────────────

// mergeKPI — 그날 줄의 한 절만 바꾼다. 다른 절(다른 소스가 쓴 값)은 그대로 둔다.
func (s *store) mergeKPI(ctx context.Context, day time.Time, section string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	now := time.Now().In(kst).Truncate(time.Second)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO growth_kpi_daily (day, metrics, updated_at)
		VALUES (?, JSON_OBJECT(?, CAST(? AS JSON)), ?)
		ON DUPLICATE KEY UPDATE metrics = JSON_SET(metrics, CONCAT('$.', ?), CAST(? AS JSON)), updated_at = VALUES(updated_at)`,
		ymd(day), section, string(raw), now, section, string(raw))
	return err
}

// kpiDay — 하루의 절들. 절은 소스마다 모양이 달라 여기서는 풀지 않는다.
type kpiDay map[string]json.RawMessage

func (s *store) loadKPI(ctx context.Context, from, to time.Time) (map[string]kpiDay, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT day, metrics FROM growth_kpi_daily WHERE day BETWEEN ? AND ?`, ymd(from), ymd(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]kpiDay{}
	for rows.Next() {
		var d time.Time
		var raw string
		if err := rows.Scan(&d, &raw); err != nil {
			return nil, err
		}
		var m kpiDay
		if err := json.Unmarshal([]byte(raw), &m); err == nil {
			out[ymd(d)] = m
		}
	}
	return out, rows.Err()
}

// ── URL 검사 ────────────────────────────────────────────────────────────────

type inspectionRow struct {
	URL             string
	InspectedAt     time.Time
	Verdict         string
	CoverageState   string
	IndexingState   string
	GoogleCanonical string
	UserCanonical   string
	PrevCoverage    string
	ChangedAt       *time.Time
}

func (s *store) loadInspections(ctx context.Context) (map[string]inspectionRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT url, inspected_at, verdict, coverage_state, indexing_state, google_canonical, user_canonical, prev_coverage, changed_at
		  FROM growth_url_inspections`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]inspectionRow{}
	for rows.Next() {
		var r inspectionRow
		var changed sql.NullTime
		if err := rows.Scan(&r.URL, &r.InspectedAt, &r.Verdict, &r.CoverageState, &r.IndexingState,
			&r.GoogleCanonical, &r.UserCanonical, &r.PrevCoverage, &changed); err != nil {
			return nil, err
		}
		if changed.Valid {
			t := changed.Time
			r.ChangedAt = &t
		}
		out[r.URL] = r
	}
	return out, rows.Err()
}

// saveInspection — 상태가 바뀌면 이전 값을 남긴다. MySQL은 ON DUPLICATE KEY UPDATE를 왼쪽부터
// 적용하고 앞에서 바꾼 열을 뒤에서 새 값으로 읽으므로, prev_·changed_at을 coverage_state보다 먼저 쓴다.
func (s *store) saveInspection(ctx context.Context, pageURL string, in inspection, at time.Time) error {
	at = at.In(kst).Truncate(time.Second)
	var lastCrawl any
	if in.LastCrawl != nil {
		lastCrawl = in.LastCrawl.Truncate(time.Second)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO growth_url_inspections
			(url, inspected_at, verdict, coverage_state, indexing_state, robots_state, fetch_state,
			 google_canonical, user_canonical, last_crawl_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE
			prev_coverage = IF(coverage_state <> VALUES(coverage_state), coverage_state, prev_coverage),
			changed_at    = IF(coverage_state <> VALUES(coverage_state), VALUES(inspected_at), changed_at),
			coverage_state = VALUES(coverage_state),
			inspected_at = VALUES(inspected_at), verdict = VALUES(verdict), indexing_state = VALUES(indexing_state),
			robots_state = VALUES(robots_state), fetch_state = VALUES(fetch_state),
			google_canonical = VALUES(google_canonical), user_canonical = VALUES(user_canonical),
			last_crawl_at = VALUES(last_crawl_at)`,
		clip(pageURL, 600), at, in.Verdict, clip(in.CoverageState, 200), in.IndexingState, in.RobotsState, in.FetchState,
		clip(in.GoogleCanonical, 600), clip(in.UserCanonical, 600), lastCrawl)
	return err
}

// dropInspectionsNotIn — 사이트맵에서 빠진 URL의 검사 결과는 지운다(보고서 합계가 사이트맵과 맞게).
func (s *store) dropInspectionsNotIn(ctx context.Context, keep map[string]bool) error {
	existing, err := s.loadInspections(ctx)
	if err != nil {
		return err
	}
	for u := range existing {
		if !keep[u] {
			if _, err := s.db.ExecContext(ctx, `DELETE FROM growth_url_inspections WHERE url = ?`, u); err != nil {
				return err
			}
		}
	}
	return nil
}

// ── 보고서 ─────────────────────────────────────────────────────────────────

func (s *store) saveReport(ctx context.Context, kind string, start, end time.Time, markdown string, data any) (int64, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return 0, err
	}
	now := time.Now().In(kst).Truncate(time.Second)
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO growth_reports (kind, period_start, period_end, created_at, markdown, data)
		VALUES (?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id), period_start = VALUES(period_start),
			created_at = VALUES(created_at), markdown = VALUES(markdown), data = VALUES(data)`,
		kind, ymd(start), ymd(end), now, markdown, string(raw))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *store) hasReport(ctx context.Context, kind string, end time.Time) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM growth_reports WHERE kind = ? AND period_end = ?`, kind, ymd(end)).Scan(&n)
	return n > 0, err
}

// prune — 원자료는 16개월(Search Console 보관 기간과 같게), 일간 보고서는 120일. 주간 보고서는 남긴다.
func (s *store) prune(ctx context.Context, today time.Time) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM growth_search_rows WHERE day < ?`, ymd(today.AddDate(0, -16, 0))); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM growth_reports WHERE kind = 'daily' AND period_end < ?`, ymd(today.AddDate(0, 0, -120)))
	return err
}

// ── 관리자 조회(API 프로세스가 쓴다) ────────────────────────────────────────

// ReportSummary — 목록 한 줄.
type ReportSummary struct {
	ID          int64     `json:"id"`
	Kind        string    `json:"kind"`
	PeriodStart string    `json:"periodStart"`
	PeriodEnd   string    `json:"periodEnd"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Report — 본문 포함.
type Report struct {
	ReportSummary
	Markdown string          `json:"markdown"`
	Data     json.RawMessage `json:"data,omitempty"`
}

// ReportReader — 관리자 API가 보고서를 읽는다. 쓰기는 없다.
type ReportReader struct{ db *sql.DB }

func NewReportReader(db *sql.DB) *ReportReader { return &ReportReader{db: db} }

func (r *ReportReader) List(ctx context.Context, kind string, limit int) ([]ReportSummary, error) {
	if limit <= 0 || limit > 200 {
		limit = 30
	}
	q := `SELECT id, kind, DATE_FORMAT(period_start, '%Y-%m-%d'), DATE_FORMAT(period_end, '%Y-%m-%d'), created_at FROM growth_reports`
	args := []any{}
	if kind != "" {
		q += ` WHERE kind = ?`
		args = append(args, kind)
	}
	q += ` ORDER BY period_end DESC, kind LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReportSummary{}
	for rows.Next() {
		var s ReportSummary
		if err := rows.Scan(&s.ID, &s.Kind, &s.PeriodStart, &s.PeriodEnd, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Get — id가 0이면 그 종류의 최신 보고서. 없으면 nil.
func (r *ReportReader) Get(ctx context.Context, id int64, kind string) (*Report, error) {
	q := `SELECT id, kind, DATE_FORMAT(period_start, '%Y-%m-%d'), DATE_FORMAT(period_end, '%Y-%m-%d'), created_at, markdown, data
	        FROM growth_reports`
	var args []any
	switch {
	case id > 0:
		q += ` WHERE id = ?`
		args = append(args, id)
	case kind != "":
		q += ` WHERE kind = ? ORDER BY period_end DESC LIMIT 1`
		args = append(args, kind)
	default:
		q += ` ORDER BY period_end DESC, created_at DESC LIMIT 1`
	}
	var out Report
	var data sql.NullString
	err := r.db.QueryRowContext(ctx, q, args...).Scan(&out.ID, &out.Kind, &out.PeriodStart, &out.PeriodEnd, &out.CreatedAt, &out.Markdown, &data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if data.Valid {
		out.Data = json.RawMessage(data.String)
	}
	return &out, nil
}
