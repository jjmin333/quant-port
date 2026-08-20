package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
)

type NewsArticleRecord struct {
	ID            string
	Market        string
	URLHash       string
	URL           string
	OriginalURL   string
	PortalURL     string
	Headline      string
	HeadlineKo    string
	Summary       string
	SummaryKo     string
	ImageURL      string
	PreviewMode   string
	SourceID      string
	PublisherID   string
	PublisherName string
	Tags          []string
	Symbols       []string
	Keywords      []string
	PublishedAt   time.Time
	ViewCount     int
	Highlighted   bool
	UpdatedAt     time.Time
}

type NewsListParams struct {
	Market   string
	SourceID string
	Sort     string
	Query    string
	Symbol   string
	Limit    int
}

type NewsRepo struct {
	db *sql.DB
}

func NewNewsRepo(db *sql.DB) *NewsRepo {
	return &NewsRepo{db: db}
}

func (r *NewsRepo) Upsert(ctx context.Context, rec NewsArticleRecord) error {
	tags, err := json.Marshal(rec.Tags)
	if err != nil {
		tags = []byte("[]")
	}
	symbols, err := json.Marshal(rec.Symbols)
	if err != nil {
		symbols = []byte("[]")
	}
	keywords, err := json.Marshal(rec.Keywords)
	if err != nil {
		keywords = []byte("[]")
	}
	previewMode := strings.TrimSpace(rec.PreviewMode)
	if previewMode == "" {
		previewMode = "summary"
	}
	publishedAt := NormalizeNewsPublishedAt(rec.PublishedAt)

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO news_articles (
			id, market, url_hash, url, original_url, portal_url,
			headline, headline_ko, summary, summary_ko, image_url, preview_mode,
			source_id, publisher_id, publisher_name,
			tags_json, symbols_json, keywords_json,
			published_at, view_count, highlighted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			headline = VALUES(headline),
			headline_ko = IF(VALUES(headline_ko) != '', VALUES(headline_ko), headline_ko),
			summary = IF(VALUES(summary) != '', VALUES(summary), summary),
			summary_ko = IF(VALUES(summary_ko) != '', VALUES(summary_ko), summary_ko),
			original_url = IF(VALUES(original_url) != '', VALUES(original_url), original_url),
			portal_url = IF(VALUES(portal_url) != '', VALUES(portal_url), portal_url),
			publisher_id = IF(VALUES(publisher_id) != '', VALUES(publisher_id), publisher_id),
			publisher_name = IF(VALUES(publisher_name) != '', VALUES(publisher_name), publisher_name),
			image_url = IF(VALUES(image_url) != '', VALUES(image_url), image_url),
			preview_mode = IF(VALUES(preview_mode) != 'summary', VALUES(preview_mode), preview_mode),
			tags_json = VALUES(tags_json),
			symbols_json = VALUES(symbols_json),
			keywords_json = IF(JSON_LENGTH(VALUES(keywords_json)) > 0, VALUES(keywords_json), keywords_json),
			published_at = CASE
				WHEN VALUES(published_at) IS NULL OR VALUES(published_at) < '2000-01-01' THEN published_at
				WHEN published_at IS NULL OR published_at < '2000-01-01' THEN VALUES(published_at)
				WHEN VALUES(published_at) < published_at THEN VALUES(published_at)
				ELSE published_at
			END,
			updated_at = CURRENT_TIMESTAMP(3)
	`, rec.ID, rec.Market, rec.URLHash, rec.URL,
		nullIfEmpty(rec.OriginalURL), nullIfEmpty(rec.PortalURL),
		rec.Headline, nullIfEmpty(rec.HeadlineKo), nullIfEmpty(rec.Summary), nullIfEmpty(rec.SummaryKo),
		nullIfEmpty(rec.ImageURL), previewMode,
		rec.SourceID, nullIfEmpty(rec.PublisherID), nullIfEmpty(rec.PublisherName),
		tags, symbols, keywords, publishedAt, rec.ViewCount, rec.Highlighted)
	return err
}

func (r *NewsRepo) ExistsByURLHash(ctx context.Context, urlHash string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM news_articles WHERE url_hash = ?
	`, urlHash).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func normalizeNewsHeadlineKey(headline string) string {
	return marketdata.NormalizeNewsHeadlineKey(headline)
}

func (r *NewsRepo) ExistsRecentHeadline(
	ctx context.Context,
	market, sourceID, headline string,
	within time.Duration,
) (bool, error) {
	return r.ExistsRecentHeadlineMatch(ctx, market, headline, "", within, sourceID)
}

func (r *NewsRepo) ExistsRecentHeadlineMatch(
	ctx context.Context,
	market, headline, headlineKo string,
	within time.Duration,
	sourceIDs ...string,
) (bool, error) {
	want := map[string]struct{}{}
	for _, key := range marketdata.NewsHeadlineMatchKeys(headline, headlineKo) {
		want[key] = struct{}{}
	}
	if len(want) == 0 {
		return false, nil
	}
	since := time.Now().UTC().Add(-within)

	query := `
		SELECT headline, COALESCE(headline_ko, '') FROM news_articles
		WHERE market = ? AND published_at >= ?
	`
	args := []any{market, since}
	if len(sourceIDs) == 1 && strings.TrimSpace(sourceIDs[0]) != "" {
		query += " AND source_id = ?"
		args = append(args, strings.TrimSpace(sourceIDs[0]))
	}
	query += " ORDER BY published_at DESC LIMIT 128"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var existing, existingKo string
		if err := rows.Scan(&existing, &existingKo); err != nil {
			return false, err
		}
		for _, key := range marketdata.NewsHeadlineMatchKeys(existing, existingKo) {
			if _, ok := want[key]; ok {
				return true, nil
			}
		}
	}
	return false, rows.Err()
}

func (r *NewsRepo) AppendKeywords(ctx context.Context, urlHash string, keywords []string) error {
	if r == nil || len(keywords) == 0 {
		return nil
	}
	var raw []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT keywords_json FROM news_articles WHERE url_hash = ? LIMIT 1
	`, urlHash).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	existing := parseJSONStringSlice(raw)
	merged := mergeStringSlices(existing, keywords)
	if len(merged) == len(existing) {
		return nil
	}
	payload, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE news_articles SET keywords_json = ?, updated_at = CURRENT_TIMESTAMP(3)
		WHERE url_hash = ?
	`, payload, urlHash)
	return err
}

func parseJSONStringSlice(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func mergeStringSlices(primary, extra []string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	for _, v := range primary {
		add(v)
	}
	for _, v := range extra {
		add(v)
	}
	return out
}

func (r *NewsRepo) PublishedAtByURLHash(ctx context.Context, urlHash string) (time.Time, error) {
	var at sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT published_at FROM news_articles WHERE url_hash = ? LIMIT 1
	`, urlHash).Scan(&at)
	if err != nil {
		return time.Time{}, err
	}
	if at.Valid {
		return at.Time, nil
	}
	return time.Time{}, nil
}

func (r *NewsRepo) IncrementViewCount(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE news_articles SET view_count = view_count + 1, updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, id)
	return err
}

func (r *NewsRepo) UpdateTranslation(ctx context.Context, id, headlineKo, summaryKo string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE news_articles
		SET headline_ko = ?, summary_ko = IF(? != '', ?, summary_ko), updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, headlineKo, summaryKo, summaryKo, id)
	return err
}

func (r *NewsRepo) GetByID(ctx context.Context, id string) (*NewsArticleRecord, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, sql.ErrNoRows
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, market, url_hash, url, COALESCE(original_url, ''), COALESCE(portal_url, ''),
			headline, COALESCE(headline_ko, ''), COALESCE(summary, ''), COALESCE(summary_ko, ''),
			COALESCE(image_url, ''), COALESCE(preview_mode, 'summary'),
			source_id, COALESCE(publisher_id, ''), COALESCE(publisher_name, ''),
			tags_json, symbols_json, keywords_json, published_at, view_count, highlighted, updated_at
		FROM news_articles
		WHERE id = ?
		LIMIT 1
	`, id)
	return scanNewsRow(row)
}

func (r *NewsRepo) ListMissingTranslation(ctx context.Context, market string, limit int) ([]NewsArticleRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, market, url_hash, url, COALESCE(original_url, ''), COALESCE(portal_url, ''),
			headline, COALESCE(headline_ko, ''), COALESCE(summary, ''), COALESCE(summary_ko, ''),
			COALESCE(image_url, ''), COALESCE(preview_mode, 'summary'),
			source_id, COALESCE(publisher_id, ''), COALESCE(publisher_name, ''),
			tags_json, symbols_json, keywords_json, published_at, view_count, highlighted, updated_at
		FROM news_articles
		WHERE market = ?
		  AND (headline_ko IS NULL OR headline_ko = '')
		  AND headline != ''
		ORDER BY published_at DESC
		LIMIT ?
	`, market, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNewsRows(rows)
}

func (r *NewsRepo) ListMissingImages(ctx context.Context, market string, limit int) ([]NewsArticleRecord, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, market, url_hash, url, COALESCE(original_url, ''), COALESCE(portal_url, ''),
			headline, COALESCE(headline_ko, ''), COALESCE(summary, ''), COALESCE(summary_ko, ''),
			COALESCE(image_url, ''), COALESCE(preview_mode, 'summary'),
			source_id, COALESCE(publisher_id, ''), COALESCE(publisher_name, ''),
			tags_json, symbols_json, keywords_json, published_at, view_count, highlighted, updated_at
		FROM news_articles
		WHERE market = ? AND (image_url IS NULL OR image_url = '')
		ORDER BY published_at DESC
		LIMIT ?
	`, market, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNewsRows(rows)
}

func (r *NewsRepo) ListRecentForImageReprobe(ctx context.Context, market string, limit int) ([]NewsArticleRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, market, url_hash, url, COALESCE(original_url, ''), COALESCE(portal_url, ''),
			headline, COALESCE(headline_ko, ''), COALESCE(summary, ''), COALESCE(summary_ko, ''),
			COALESCE(image_url, ''), COALESCE(preview_mode, 'summary'),
			source_id, COALESCE(publisher_id, ''), COALESCE(publisher_name, ''),
			tags_json, symbols_json, keywords_json, published_at, view_count, highlighted, updated_at
		FROM news_articles
		WHERE market = ? AND COALESCE(original_url, '') != ''
		ORDER BY published_at DESC
		LIMIT ?
	`, market, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNewsRows(rows)
}

func (r *NewsRepo) UpdateImageURL(ctx context.Context, id, imageURL string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE news_articles SET image_url = ?, updated_at = CURRENT_TIMESTAMP(3) WHERE id = ?
	`, imageURL, id)
	return err
}

func (r *NewsRepo) List(ctx context.Context, params NewsListParams) ([]NewsArticleRecord, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		if params.Market == "us" {
			if limit > 300 {
				limit = 300
			}
		} else {
			limit = 200
		}
	}

	order := "published_at DESC"
	if params.Sort == "popular" {
		order = "view_count DESC, published_at DESC"
	}

	query := `
		SELECT id, market, url_hash, url, COALESCE(original_url, ''), COALESCE(portal_url, ''),
			headline, COALESCE(headline_ko, ''), COALESCE(summary, ''), COALESCE(summary_ko, ''),
			COALESCE(image_url, ''), COALESCE(preview_mode, 'summary'),
			source_id, COALESCE(publisher_id, ''), COALESCE(publisher_name, ''),
			tags_json, symbols_json, keywords_json, published_at, view_count, highlighted, updated_at
		FROM news_articles
		WHERE market = ?
	`
	args := []any{params.Market}

	if params.Market == "us" {
		query += usTranslationReadySQL()
	}

	if src := strings.TrimSpace(params.SourceID); src != "" && src != "all" {
		query += " AND source_id = ?"
		args = append(args, src)
	}
	if sym := strings.ToUpper(strings.TrimSpace(params.Symbol)); sym != "" {
		query += " AND JSON_CONTAINS(symbols_json, JSON_QUOTE(?))"
		args = append(args, sym)
	}
	if q := strings.TrimSpace(params.Query); q != "" {
		like := "%" + q + "%"
		query += " AND (headline LIKE ? OR summary LIKE ? OR headline_ko LIKE ? OR summary_ko LIKE ?)"
		args = append(args, like, like, like, like)
	}
	query += " ORDER BY " + order + " LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNewsRows(rows)
}

func (r *NewsRepo) LatestUpdatedAt(ctx context.Context, market string) (time.Time, error) {
	var at sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT MAX(updated_at) FROM news_articles WHERE market = ?
	`, market).Scan(&at)
	if err != nil {
		return time.Time{}, err
	}
	if at.Valid {
		return at.Time, nil
	}
	return time.Time{}, nil
}

func (r *NewsRepo) Count(ctx context.Context, market string) (int, error) {
	query := `SELECT COUNT(1) FROM news_articles WHERE market = ?`
	args := []any{market}
	if market == "us" {
		query += usTranslationReadySQL()
	}
	var n int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

func usTranslationReadySQL() string {
	return " AND COALESCE(headline_ko, '') != ''"
}

func (r *NewsRepo) ListPage(ctx context.Context, market string, offset, limit int) ([]NewsArticleRecord, error) {
	if limit <= 0 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, market, url_hash, url, COALESCE(original_url, ''), COALESCE(portal_url, ''),
			headline, COALESCE(headline_ko, ''), COALESCE(summary, ''), COALESCE(summary_ko, ''),
			COALESCE(image_url, ''), COALESCE(preview_mode, 'summary'),
			source_id, COALESCE(publisher_id, ''), COALESCE(publisher_name, ''),
			tags_json, symbols_json, keywords_json, published_at, view_count, highlighted, updated_at
		FROM news_articles
		WHERE market = ?
		ORDER BY id
		LIMIT ? OFFSET ?
	`, market, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNewsRows(rows)
}

// UpdateSymbols rewrites one article's symbols_json. Used by the retag pass
// that re-derives symbols after the extraction rules change — it must not touch
// updated_at, or every retagged row would look freshly ingested to the merge
// logic that orders the feed.
func (r *NewsRepo) UpdateSymbols(ctx context.Context, id string, symbols []string) error {
	if symbols == nil {
		symbols = []string{}
	}
	payload, err := json.Marshal(symbols)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE news_articles SET symbols_json = ? WHERE id = ?`, string(payload), id)
	return err
}

// UpdateMarket moves an article between domestic and US feeds.
func (r *NewsRepo) UpdateMarket(ctx context.Context, id, market string) error {
	market = strings.ToLower(strings.TrimSpace(market))
	if market != "us" && market != "domestic" {
		return fmt.Errorf("invalid market %q", market)
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE news_articles SET market = ?, updated_at = CURRENT_TIMESTAMP(3) WHERE id = ?`,
		market, id)
	return err
}

// UpdateTags rewrites one article's tags_json without touching updated_at.
func (r *NewsRepo) UpdateTags(ctx context.Context, id string, tags []string) error {
	if tags == nil {
		tags = []string{}
	}
	payload, err := json.Marshal(tags)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE news_articles SET tags_json = ? WHERE id = ?`, string(payload), id)
	return err
}

func (r *NewsRepo) DeleteByIDs(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var total int64
	for start := 0; start < len(ids); start += 100 {
		end := start + 100
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		query := "DELETE FROM news_articles WHERE id IN (?" + strings.Repeat(",?", len(chunk)-1) + ")"
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		res, err := r.db.ExecContext(ctx, query, args...)
		if err != nil {
			return total, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func scanNewsRows(rows *sql.Rows) ([]NewsArticleRecord, error) {
	var out []NewsArticleRecord
	for rows.Next() {
		rec, err := scanNewsRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rec)
	}
	return out, rows.Err()
}

func scanNewsRow(scanner interface {
	Scan(dest ...any) error
}) (*NewsArticleRecord, error) {
	var rec NewsArticleRecord
	var tagsRaw, symbolsRaw, keywordsRaw []byte
	if err := scanner.Scan(
		&rec.ID, &rec.Market, &rec.URLHash, &rec.URL, &rec.OriginalURL, &rec.PortalURL,
		&rec.Headline, &rec.HeadlineKo, &rec.Summary, &rec.SummaryKo, &rec.ImageURL, &rec.PreviewMode,
		&rec.SourceID, &rec.PublisherID, &rec.PublisherName,
		&tagsRaw, &symbolsRaw, &keywordsRaw, &rec.PublishedAt, &rec.ViewCount, &rec.Highlighted, &rec.UpdatedAt,
	); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(tagsRaw, &rec.Tags)
	_ = json.Unmarshal(symbolsRaw, &rec.Symbols)
	_ = json.Unmarshal(keywordsRaw, &rec.Keywords)
	rec.PublishedAt = NormalizeNewsPublishedAt(rec.PublishedAt)
	return &rec, nil
}
