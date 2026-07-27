package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type NewsArticleRecord struct {
	ID          string
	Market      string
	URLHash     string
	URL         string
	Headline    string
	Summary     string
	SourceID    string
	Tags        []string
	Symbols     []string
	Keywords    []string
	PublishedAt time.Time
	ViewCount   int
	Highlighted bool
	UpdatedAt   time.Time
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

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO news_articles (
			id, market, url_hash, url, headline, summary, source_id,
			tags_json, symbols_json, keywords_json,
			published_at, view_count, highlighted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			headline = VALUES(headline),
			summary = IF(VALUES(summary) != '', VALUES(summary), summary),
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
	`, rec.ID, rec.Market, rec.URLHash, rec.URL, rec.Headline, nullIfEmpty(rec.Summary),
		rec.SourceID, tags, symbols, keywords, rec.PublishedAt, rec.ViewCount, rec.Highlighted)
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

func (r *NewsRepo) List(ctx context.Context, params NewsListParams) ([]NewsArticleRecord, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}

	order := "published_at DESC"
	if params.Sort == "popular" {
		order = "view_count DESC, published_at DESC"
	}

	query := `
		SELECT id, market, url_hash, url, headline, COALESCE(summary, ''), source_id,
			tags_json, symbols_json, keywords_json, published_at, view_count, highlighted, updated_at
		FROM news_articles
		WHERE market = ?
	`
	args := []any{params.Market}

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
		query += " AND (headline LIKE ? OR summary LIKE ?)"
		args = append(args, like, like)
	}
	query += " ORDER BY " + order + " LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

func scanNewsRow(scanner interface {
	Scan(dest ...any) error
}) (*NewsArticleRecord, error) {
	var rec NewsArticleRecord
	var tagsRaw, symbolsRaw, keywordsRaw []byte
	if err := scanner.Scan(
		&rec.ID, &rec.Market, &rec.URLHash, &rec.URL, &rec.Headline, &rec.Summary, &rec.SourceID,
		&tagsRaw, &symbolsRaw, &keywordsRaw, &rec.PublishedAt, &rec.ViewCount, &rec.Highlighted, &rec.UpdatedAt,
	); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(tagsRaw, &rec.Tags)
	_ = json.Unmarshal(symbolsRaw, &rec.Symbols)
	_ = json.Unmarshal(keywordsRaw, &rec.Keywords)
	return &rec, nil
}
