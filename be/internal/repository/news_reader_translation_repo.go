package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type NewsReaderTranslationRecord struct {
	URLHash            string
	ArticleID          string
	Market             string
	TitleKo            string
	ExcerptKo          string
	ContentHTMLKo      string
	TextContentKo      string
	SourceContentHash  string
	UpdatedAt          time.Time
}

type NewsReaderTranslationRepo struct {
	db *sql.DB
}

func NewNewsReaderTranslationRepo(db *sql.DB) *NewsReaderTranslationRepo {
	return &NewsReaderTranslationRepo{db: db}
}

func (r *NewsReaderTranslationRepo) GetByURLHash(ctx context.Context, urlHash string) (*NewsReaderTranslationRecord, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	urlHash = strings.TrimSpace(urlHash)
	if urlHash == "" {
		return nil, sql.ErrNoRows
	}
	var rec NewsReaderTranslationRecord
	var articleID sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT url_hash, COALESCE(article_id, ''), market,
			title_ko, COALESCE(excerpt_ko, ''), COALESCE(content_html_ko, ''), COALESCE(text_content_ko, ''),
			COALESCE(source_content_hash, ''), updated_at
		FROM news_reader_translations
		WHERE url_hash = ? AND content_html_ko IS NOT NULL AND content_html_ko != ''
		LIMIT 1
	`, urlHash).Scan(
		&rec.URLHash, &articleID, &rec.Market,
		&rec.TitleKo, &rec.ExcerptKo, &rec.ContentHTMLKo, &rec.TextContentKo,
		&rec.SourceContentHash, &rec.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if articleID.Valid {
		rec.ArticleID = articleID.String
	}
	return &rec, nil
}

func (r *NewsReaderTranslationRepo) Upsert(ctx context.Context, rec NewsReaderTranslationRecord) error {
	if r == nil || r.db == nil {
		return sql.ErrConnDone
	}
	market := strings.TrimSpace(rec.Market)
	if market == "" {
		market = "us"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO news_reader_translations (
			url_hash, article_id, market, title_ko, excerpt_ko, content_html_ko, text_content_ko, source_content_hash
		) VALUES (?, NULLIF(?, ''), ?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), NULLIF(?, ''))
		ON DUPLICATE KEY UPDATE
			article_id = IF(VALUES(article_id) IS NOT NULL AND VALUES(article_id) != '', VALUES(article_id), article_id),
			title_ko = IF(VALUES(title_ko) != '', VALUES(title_ko), title_ko),
			excerpt_ko = IF(VALUES(excerpt_ko) != '', VALUES(excerpt_ko), excerpt_ko),
			content_html_ko = IF(VALUES(content_html_ko) != '', VALUES(content_html_ko), content_html_ko),
			text_content_ko = IF(VALUES(text_content_ko) != '', VALUES(text_content_ko), text_content_ko),
			source_content_hash = IF(VALUES(source_content_hash) != '', VALUES(source_content_hash), source_content_hash),
			updated_at = CURRENT_TIMESTAMP(3)
	`, rec.URLHash, rec.ArticleID, market,
		rec.TitleKo, rec.ExcerptKo, rec.ContentHTMLKo, rec.TextContentKo, rec.SourceContentHash)
	return err
}
