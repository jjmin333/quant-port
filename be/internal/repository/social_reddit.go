package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// 094 — Reddit 커뮤니티 탭: ApeWisdom 종목 요약 · Reddit 글 · 커뮤니티 점수/AI 요약.

// SocialInsight is one social_insights row.
type SocialInsight struct {
	Symbol       string          `json:"-"`
	SnapDate     string          `json:"date"`
	Users        *int            `json:"users,omitempty"`
	SentimentPct *int            `json:"sentimentPct,omitempty"`
	WSBPct       *int            `json:"wsbPct,omitempty"`
	Keywords     json.RawMessage `json:"keywords,omitempty"`
}

func (r *ExternalSignalRepo) UpsertSocialInsight(ctx context.Context, in SocialInsight) error {
	var kw any
	if len(in.Keywords) > 0 {
		kw = string(in.Keywords)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO social_insights (symbol, snap_date, users, sentiment_pct, sentiment_wsb_pct, keywords, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW(3))
		ON DUPLICATE KEY UPDATE users = VALUES(users), sentiment_pct = VALUES(sentiment_pct),
			sentiment_wsb_pct = VALUES(sentiment_wsb_pct), keywords = VALUES(keywords), fetched_at = VALUES(fetched_at)`,
		in.Symbol, in.SnapDate, in.Users, in.SentimentPct, in.WSBPct, kw)
	return err
}

const socialInsightCols = `symbol, DATE_FORMAT(snap_date, '%Y-%m-%d'), users, sentiment_pct, sentiment_wsb_pct, keywords`

func scanSocialInsight(sc interface{ Scan(...any) error }) (SocialInsight, error) {
	var in SocialInsight
	var users, sent, wsb sql.NullInt64
	var kw sql.NullString
	if err := sc.Scan(&in.Symbol, &in.SnapDate, &users, &sent, &wsb, &kw); err != nil {
		return in, err
	}
	in.Users, in.SentimentPct, in.WSBPct = ni(users), ni(sent), ni(wsb)
	if kw.Valid {
		in.Keywords = json.RawMessage(kw.String)
	}
	return in, nil
}

func ni(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	x := int(v.Int64)
	return &x
}

// SocialInsightLatest — 한 종목의 가장 최근 요약(없으면 nil).
func (r *ExternalSignalRepo) SocialInsightLatest(ctx context.Context, symbol string) (*SocialInsight, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	in, err := scanSocialInsight(r.db.QueryRowContext(ctx,
		`SELECT `+socialInsightCols+` FROM social_insights WHERE symbol = ? ORDER BY snap_date DESC LIMIT 1`,
		strings.ToUpper(symbol)))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &in, nil
}

// SocialInsightsBetween — 점수 배치용: 종목마다 [from, to] 안의 가장 최근 요약.
func (r *ExternalSignalRepo) SocialInsightsBetween(ctx context.Context, from, to string) (map[string]SocialInsight, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT i.symbol, DATE_FORMAT(i.snap_date, '%Y-%m-%d'), i.users, i.sentiment_pct, i.sentiment_wsb_pct, i.keywords
		  FROM social_insights i
		  JOIN (SELECT symbol, MAX(snap_date) d FROM social_insights WHERE snap_date BETWEEN ? AND ? GROUP BY symbol) x
		    ON x.symbol = i.symbol AND x.d = i.snap_date`, from, to)
	if err != nil {
		return nil, fmt.Errorf("social insights: %w", err)
	}
	defer rows.Close()
	out := map[string]SocialInsight{}
	for rows.Next() {
		in, err := scanSocialInsight(rows)
		if err != nil {
			return nil, err
		}
		out[in.Symbol] = in
	}
	return out, rows.Err()
}

// SocialTop is one day's all-stocks row with the catalog names.
type SocialTop struct {
	Symbol, Name, EnglishName string
	Mentions, MentionsPrev    int
	Upvotes, Rank             int
	RankPrev                  *int
	LastPostsAt               *time.Time
}

// SocialTopOn — date의 전체 목록을 언급 많은 순으로(글을 마지막으로 받은 시각 포함).
func (r *ExternalSignalRepo) SocialTopOn(ctx context.Context, date string, limit int) ([]SocialTop, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.symbol, s.name, s.english_name, m.mentions, m.mentions_prev, m.upvotes, m.rank_now, m.rank_prev, p.last_at
		  FROM social_mentions m
		  JOIN stocks s ON s.symbol = m.symbol
		  LEFT JOIN (SELECT symbol, MAX(fetched_at) last_at FROM social_posts GROUP BY symbol) p ON p.symbol = m.symbol
		 WHERE m.snap_date = ? AND m.source = 'apewisdom'
		 ORDER BY m.mentions DESC, m.rank_now
		 LIMIT ?`, date, limit)
	if err != nil {
		return nil, fmt.Errorf("social top: %w", err)
	}
	defer rows.Close()
	var out []SocialTop
	for rows.Next() {
		var t SocialTop
		var rp sql.NullInt64
		var last sql.NullTime
		if err := rows.Scan(&t.Symbol, &t.Name, &t.EnglishName, &t.Mentions, &t.MentionsPrev, &t.Upvotes, &t.Rank, &rp, &last); err != nil {
			return nil, err
		}
		t.RankPrev = ni(rp)
		if last.Valid {
			t.LastPostsAt = &last.Time
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// LatestSocialDate — 전체 목록의 가장 최근 스냅샷 날(없으면 "").
func (r *ExternalSignalRepo) LatestSocialDate(ctx context.Context) (string, error) {
	var d sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT DATE_FORMAT(MAX(snap_date), '%Y-%m-%d') FROM social_mentions WHERE source = 'apewisdom'`).Scan(&d)
	return d.String, err
}

// SocialPost is one social_posts row.
type SocialPost struct {
	PostID      string    `json:"id"`
	Subreddit   string    `json:"subreddit"`
	Title       string    `json:"title"`
	TitleKO     string    `json:"titleKo,omitempty"`
	Snippet     string    `json:"snippet,omitempty"`
	SnippetKO   string    `json:"snippetKo,omitempty"`
	URL         string    `json:"url"`
	TopRank     int       `json:"-"`
	PublishedAt time.Time `json:"publishedAt"`
}

// SocialPostTranslations — 이미 옮긴 글은 다시 옮기지 않는다(post_id → 제목·앞부분 번역).
func (r *ExternalSignalRepo) SocialPostTranslations(ctx context.Context, symbol string) (map[string][2]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT post_id, COALESCE(title_ko, ''), COALESCE(snippet_ko, '') FROM social_posts WHERE symbol = ?`, symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][2]string{}
	for rows.Next() {
		var id, t, s string
		if err := rows.Scan(&id, &t, &s); err != nil {
			return nil, err
		}
		out[id] = [2]string{t, s}
	}
	return out, rows.Err()
}

// ReplaceSocialPosts — 한 종목의 글 목록을 이번 검색 결과로 바꾼다(이번 주 인기 = 지금의 목록).
func (r *ExternalSignalRepo) ReplaceSocialPosts(ctx context.Context, symbol string, posts []SocialPost) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM social_posts WHERE symbol = ?`, symbol); err != nil {
		return err
	}
	for _, p := range posts {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO social_posts (symbol, post_id, subreddit, title, title_ko, snippet, snippet_ko, url, top_rank, published_at)
			VALUES (?, ?, ?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), ?, ?, ?)
			ON DUPLICATE KEY UPDATE top_rank = VALUES(top_rank)`,
			symbol, p.PostID, cut(p.Subreddit, 40), cut(p.Title, 400), cut(p.TitleKO, 600), cut(p.Snippet, 600),
			cut(p.SnippetKO, 900), cut(p.URL, 400), p.TopRank, p.PublishedAt.UTC()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func cut(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// SocialPosts — 한 종목의 글(인기 순).
func (r *ExternalSignalRepo) SocialPosts(ctx context.Context, symbol string, limit int) ([]SocialPost, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT post_id, subreddit, title, COALESCE(title_ko, ''), snippet, COALESCE(snippet_ko, ''), url, top_rank, published_at
		  FROM social_posts WHERE symbol = ? ORDER BY top_rank, published_at DESC LIMIT ?`, strings.ToUpper(symbol), limit)
	if err != nil {
		return nil, fmt.Errorf("social posts: %w", err)
	}
	defer rows.Close()
	out := []SocialPost{}
	for rows.Next() {
		var p SocialPost
		if err := rows.Scan(&p.PostID, &p.Subreddit, &p.Title, &p.TitleKO, &p.Snippet, &p.SnippetKO, &p.URL, &p.TopRank, &p.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SocialSummary is one social_summaries row.
type SocialSummary struct {
	Symbol    string          `json:"-"`
	SnapDate  string          `json:"date"`
	Score     int             `json:"score"`
	Body      json.RawMessage `json:"body"`
	Source    string          `json:"source"`
	InputHash string          `json:"-"`
	Model     string          `json:"-"`
	Tokens    int64           `json:"-"`
	CreatedAt time.Time       `json:"createdAt"`
}

func (r *ExternalSignalRepo) UpsertSocialSummary(ctx context.Context, s SocialSummary) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO social_summaries (symbol, snap_date, score, body, source, model, input_hash, tokens, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(3))
		ON DUPLICATE KEY UPDATE snap_date = VALUES(snap_date), score = VALUES(score), body = VALUES(body),
			source = VALUES(source), model = VALUES(model), input_hash = VALUES(input_hash), tokens = VALUES(tokens),
			created_at = VALUES(created_at)`,
		s.Symbol, s.SnapDate, s.Score, string(s.Body), s.Source, s.Model, s.InputHash, s.Tokens)
	return err
}

// SocialSummaryOf reads one symbol's summary (없으면 nil).
func (r *ExternalSignalRepo) SocialSummaryOf(ctx context.Context, symbol string) (*SocialSummary, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	var s SocialSummary
	var body string
	err := r.db.QueryRowContext(ctx, `
		SELECT symbol, DATE_FORMAT(snap_date, '%Y-%m-%d'), score, body, source, input_hash, created_at
		  FROM social_summaries WHERE symbol = ?`, strings.ToUpper(symbol)).
		Scan(&s.Symbol, &s.SnapDate, &s.Score, &body, &s.Source, &s.InputHash, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.Body = json.RawMessage(body)
	return &s, nil
}

// SocialDayMood — 하루치 시장 집계(2026-10-01 요청 3): 전체 언급 합, 지수 ETF·전체의 Σ(감성×사용자)·Σ사용자.
// 「전체」는 종목 요약이 있는 언급 상위 150종목 중 사용자 5명 이상.
type SocialDayMood struct {
	Date                   string
	Mentions               int
	ETFSentUsers, ETFUsers int64
	AllSentUsers, AllUsers int64
}

// SocialMarketDays — 최근 days일(최신 먼저).
func (r *ExternalSignalRepo) SocialMarketDays(ctx context.Context, etfs []string, days int) ([]SocialDayMood, error) {
	if r == nil || r.db == nil || len(etfs) == 0 {
		return nil, nil
	}
	in := strings.TrimSuffix(strings.Repeat("?,", len(etfs)), ",")
	args := make([]any, 0, 2*len(etfs)+1)
	args = append(args, days)
	for i := 0; i < 2; i++ {
		for _, e := range etfs {
			args = append(args, e)
		}
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(m.d, '%Y-%m-%d'), m.mentions,
		       COALESCE(i.etf_su, 0), COALESCE(i.etf_u, 0), COALESCE(i.all_su, 0), COALESCE(i.all_u, 0)
		  FROM (SELECT snap_date d, SUM(mentions) mentions FROM social_mentions
		         WHERE source = 'apewisdom' GROUP BY snap_date ORDER BY snap_date DESC LIMIT ?) m
		  LEFT JOIN (SELECT snap_date d,
		                    SUM(CASE WHEN symbol IN (`+in+`) THEN sentiment_pct * users END) etf_su,
		                    SUM(CASE WHEN symbol IN (`+in+`) THEN users END) etf_u,
		                    SUM(sentiment_pct * users) all_su, SUM(users) all_u
		               FROM social_insights WHERE users >= 5 AND sentiment_pct IS NOT NULL
		              GROUP BY snap_date) i ON i.d = m.d
		 ORDER BY m.d DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("social market days: %w", err)
	}
	defer rows.Close()
	var out []SocialDayMood
	for rows.Next() {
		var d SocialDayMood
		if err := rows.Scan(&d.Date, &d.Mentions, &d.ETFSentUsers, &d.ETFUsers, &d.AllSentUsers, &d.AllUsers); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// SocialSubredditTotals — date의 서브레딧별 언급 합(많은 순).
func (r *ExternalSignalRepo) SocialSubredditTotals(ctx context.Context, date string) ([]SocialSource, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT source, SUM(mentions), SUM(mentions_prev), SUM(upvotes) FROM social_mentions
		 WHERE snap_date = ? AND source LIKE 'r/%' GROUP BY source ORDER BY 2 DESC`, date)
	if err != nil {
		return nil, fmt.Errorf("social subreddit totals: %w", err)
	}
	defer rows.Close()
	out := []SocialSource{}
	for rows.Next() {
		var s SocialSource
		if err := rows.Scan(&s.Source, &s.Mentions, &s.MentionsPrev, &s.Upvotes); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
