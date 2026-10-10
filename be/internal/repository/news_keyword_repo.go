package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

var ErrNewsKeywordNotFound = errors.New("news keyword not found")
var ErrNewsKeywordDuplicate = errors.New("keyword already exists for market")

type NewsKeywordRepo struct {
	db *sql.DB
}

func NewNewsKeywordRepo(db *sql.DB) *NewsKeywordRepo {
	return &NewsKeywordRepo{db: db}
}

func (r *NewsKeywordRepo) List(ctx context.Context, market string, includeDisabled bool) ([]model.NewsPollKeyword, error) {
	query := `
		SELECT id, market, keyword, topic, enabled, sort_order, created_at, updated_at
		FROM news_poll_keywords
		WHERE market = ?
	`
	if !includeDisabled {
		query += ` AND enabled = 1`
	}
	query += ` ORDER BY sort_order ASC, keyword ASC`

	rows, err := r.db.QueryContext(ctx, query, market)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.NewsPollKeyword
	for rows.Next() {
		item, err := scanNewsKeyword(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ListEnabledTexts — 종목 뉴스 큐에 섞이는 일반 키워드만(주제 검색어는 ListTopicKeywords).
func (r *NewsKeywordRepo) ListEnabledTexts(ctx context.Context, market string) ([]string, error) {
	items, err := r.List(ctx, market, false)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.Topic == "" {
			out = append(out, item.Keyword)
		}
	}
	return out, nil
}

// ListTopicKeywords — 켜진 국내 주제 검색어(부동산·금리·예적금·대출), 정렬 순.
func (r *NewsKeywordRepo) ListTopicKeywords(ctx context.Context) ([]model.NewsPollKeyword, error) {
	items, err := r.List(ctx, "domestic", false)
	if err != nil {
		return nil, err
	}
	out := items[:0]
	for _, item := range items {
		if item.Topic != "" {
			out = append(out, item)
		}
	}
	return out, nil
}

func (r *NewsKeywordRepo) GetByID(ctx context.Context, id int64) (*model.NewsPollKeyword, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, market, keyword, topic, enabled, sort_order, created_at, updated_at
		FROM news_poll_keywords WHERE id = ?
	`, id)
	item, err := scanNewsKeyword(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNewsKeywordNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *NewsKeywordRepo) Create(ctx context.Context, market, keyword, topic string, enabled bool, sortOrder int) (*model.NewsPollKeyword, error) {
	market = normalizeNewsMarket(market)
	keyword = strings.TrimSpace(keyword)
	if market == "" || keyword == "" {
		return nil, fmt.Errorf("market and keyword required")
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO news_poll_keywords (market, keyword, topic, enabled, sort_order)
		VALUES (?, ?, ?, ?, ?)
	`, market, keyword, topic, boolToTiny(enabled), sortOrder)
	if err != nil {
		if isDuplicateKey(err) {
			return nil, ErrNewsKeywordDuplicate
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return r.GetByID(ctx, id)
}

func (r *NewsKeywordRepo) Update(ctx context.Context, id int64, keyword, topic string, enabled bool, sortOrder int) (*model.NewsPollKeyword, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, fmt.Errorf("keyword required")
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE news_poll_keywords
		SET keyword = ?, topic = ?, enabled = ?, sort_order = ?
		WHERE id = ?
	`, keyword, topic, boolToTiny(enabled), sortOrder, id)
	if err != nil {
		if isDuplicateKey(err) {
			return nil, ErrNewsKeywordDuplicate
		}
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrNewsKeywordNotFound
	}
	return r.GetByID(ctx, id)
}

func (r *NewsKeywordRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM news_poll_keywords WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNewsKeywordNotFound
	}
	return nil
}

func normalizeNewsMarket(market string) string {
	m := strings.ToLower(strings.TrimSpace(market))
	switch m {
	case "domestic", "us":
		return m
	default:
		return ""
	}
}

func boolToTiny(v bool) int {
	if v {
		return 1
	}
	return 0
}

type newsKeywordScanner interface {
	Scan(dest ...any) error
}

func scanNewsKeyword(row newsKeywordScanner) (model.NewsPollKeyword, error) {
	var item model.NewsPollKeyword
	var enabled int
	if err := row.Scan(
		&item.ID, &item.Market, &item.Keyword, &item.Topic, &enabled, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return item, err
	}
	item.Enabled = enabled == 1
	return item, nil
}

// MigrateAdminNewsKeywordsIfNeeded applies 020_admin_news_keywords.sql.
func MigrateAdminNewsKeywordsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	hasRole, err := columnExists(ctx, db, "users", "role")
	if err != nil {
		return err
	}
	hasTable, err := tableExists(ctx, db, "news_poll_keywords")
	if err != nil {
		return err
	}
	if hasRole && hasTable {
		return MigrateNewsPollSettingsIfNeeded(ctx, db, migrationsDir)
	}

	path := filepath.Join(migrationsDir, "020_admin_news_keywords.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 020_admin_news_keywords.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if isIgnorableAlterErr(err) {
				continue
			}
			return fmt.Errorf("020_admin_news_keywords.sql: %w", err)
		}
	}
	return MigrateNewsPollSettingsIfNeeded(ctx, db, migrationsDir)
}

// MigrateNewsIndexKeywordsIfNeeded applies 040_news_index_keywords.sql.
func MigrateNewsIndexKeywordsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	hasTable, err := tableExists(ctx, db, "news_poll_keywords")
	if err != nil {
		return err
	}
	if !hasTable {
		return nil
	}
	var count int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM news_poll_keywords
		WHERE market = 'domestic' AND keyword = '비트코인'
	`).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	path := filepath.Join(migrationsDir, "040_news_index_keywords.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 040_news_index_keywords.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("040_news_index_keywords.sql: %w", err)
		}
	}
	return nil
}
