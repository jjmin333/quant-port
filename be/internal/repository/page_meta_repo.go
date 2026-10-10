package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

/*
`page_meta` 표를 읽고 쓴다 (2026-09-15 사용자 요청 2).

표가 없어도 **오류를 내지 않는다.** 이 자료는 덮어쓰기라, 없으면 사이트가 코드의
기본값으로 그대로 돌아간다 — 마이그레이션이 아직 안 돈 환경에서 제목 때문에
화면이 죽는 것이 더 나쁘다. 그래서 읽기는 빈 목록을, 쓰기는 「표가 없다」는 오류를
돌려준다(쓰기는 관리자가 보고 있으므로 조용히 삼키면 안 된다).
*/
type PageMetaRepo struct {
	db    *sql.DB
	table *tableExistsCache
}

func NewPageMetaRepo(db *sql.DB) *PageMetaRepo {
	return &PageMetaRepo{db: db, table: newTableExistsCache("page_meta")}
}

// ErrPageMetaUnavailable is returned by writes when the table is not there yet.
var ErrPageMetaUnavailable = fmt.Errorf("page_meta table not available")

func (r *PageMetaRepo) ready(ctx context.Context) bool {
	return r != nil && r.db != nil && r.table.Has(ctx, r.db)
}

/*
NormalizePath brings a caller-supplied path to the one spelling the table
stores: leading slash, no trailing slash (except the root).

정규화를 **저장 전에** 하는 이유는 읽는 쪽이 경로 하나로 찾기 때문이다.
`/market`과 `/market/`이 각각 행이 되면, 화면은 둘 중 하나만 보고 관리자는
「저장했는데 안 바뀐다」를 만난다.

# 대소문자는 **안 건드린다**

한 번 소문자로 내렸다가 되돌렸다. 화면 목록에 `/reports/authors/[userId]`가 있는데
그것을 내리면 `[userid]`가 되어 FE 의 그 화면과 영영 안 맞는다 — 저장은 되고 아무
오류도 안 나지만 제목은 바뀌지 않는다.

내리려던 이유(`/Market` 같은 오타)는 애초에 생기지 않는다. 경로는 관리 화면의
목록에서 **고르는** 값이지 손으로 적는 값이 아니다. URL 경로 자체가 대소문자를
구분하기도 한다.
*/
func NormalizePath(raw string) string {
	path := strings.TrimSpace(raw)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	for len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}

// List returns every override, ordered by path so the admin screen and the
// site read the same sequence.
func (r *PageMetaRepo) List(ctx context.Context) ([]model.PageMeta, error) {
	if !r.ready(ctx) {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT path, title, description, og_title, og_description, og_image, noindex,
		       DATE_FORMAT(updated_at, '%Y-%m-%dT%H:%i:%sZ')
		  FROM page_meta
		 ORDER BY path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.PageMeta, 0, 24)
	for rows.Next() {
		var item model.PageMeta
		var noindex sql.NullBool
		var updatedAt sql.NullString
		if err := rows.Scan(
			&item.Path, &item.Title, &item.Description, &item.OGTitle,
			&item.OGDescription, &item.OGImage, &noindex, &updatedAt,
		); err != nil {
			return nil, err
		}
		if noindex.Valid {
			v := noindex.Bool
			item.NoIndex = &v
		}
		item.UpdatedAt = updatedAt.String
		out = append(out, item)
	}
	return out, rows.Err()
}

// Upsert writes one override. An override that would change nothing deletes
// the row instead — see model.PageMeta.IsEmpty.
func (r *PageMetaRepo) Upsert(ctx context.Context, item model.PageMeta) error {
	if !r.ready(ctx) {
		return ErrPageMetaUnavailable
	}
	path := NormalizePath(item.Path)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	if item.IsEmpty() {
		return r.Delete(ctx, path)
	}
	var noindex any
	if item.NoIndex != nil {
		noindex = *item.NoIndex
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO page_meta (path, title, description, og_title, og_description, og_image, noindex)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			title = VALUES(title),
			description = VALUES(description),
			og_title = VALUES(og_title),
			og_description = VALUES(og_description),
			og_image = VALUES(og_image),
			noindex = VALUES(noindex)`,
		path, item.Title, item.Description, item.OGTitle, item.OGDescription,
		item.OGImage, noindex,
	)
	return err
}

// Delete removes one override, putting that screen back on the code default.
func (r *PageMetaRepo) Delete(ctx context.Context, path string) error {
	if !r.ready(ctx) {
		return ErrPageMetaUnavailable
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM page_meta WHERE path = ?`, NormalizePath(path))
	return err
}

// MigratePageMetaIfNeeded creates the table on an existing database.
func MigratePageMetaIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "page_meta")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	path := filepath.Join(migrationsDir, "074_page_meta.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 074_page_meta.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			// API 와 워커 여덟이 동시에 스키마를 맞춘다 — 하나가 이기고 나머지는
			// 끝난 일을 보러 온다(`isSchemaRaceError`).
			if isSchemaRaceError(err) {
				continue
			}
			return fmt.Errorf("074_page_meta.sql: %w", err)
		}
	}
	log.Println("mysql: applied 074_page_meta.sql")
	return nil
}
