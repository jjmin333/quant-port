package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
)

/*
닉네임은 **겹치지 않는다** (2026-09-27 사용자 결정 — 커뮤니티에서 작성자를 구분해야 한다).

유니크 인덱스를 걸기 전에 이미 겹친 닉네임을 푼다: 같은 이름 중 **가장 먼저 가입한 사람**이
이름을 지키고, 나머지는 뒤에 번호를 붙인다(「투자왕」 → 「투자왕2」). 칸의 collation이
대소문자·공백 끝을 같게 보므로 겹침 판단도 DB의 GROUP BY에 맡긴다 — Go에서 따로 비교하면
DB는 같다고 보는데 Go는 다르다고 보는 이름이 남아 인덱스 생성이 실패한다.

인덱스가 이미 있으면 아무것도 하지 않는다(재기동마다 도는 마커 방식).
*/

const displayNameUniqueIndex = "uniq_users_display_name"

// displayNameColumnMax is users.display_name VARCHAR(100) — 번호를 붙여도 이 안에 들어가야 한다.
const displayNameColumnMax = 100

func MigrateUniqueDisplayNameIfNeeded(ctx context.Context, db *sql.DB) error {
	exists, err := indexExists(ctx, db, "users", displayNameUniqueIndex)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	renamed, err := dedupeDisplayNames(ctx, db)
	if err != nil {
		return fmt.Errorf("dedupe display names: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		`ALTER TABLE users ADD UNIQUE KEY `+displayNameUniqueIndex+` (display_name)`); err != nil {
		if isIgnorableAlterErr(err) {
			return nil
		}
		return fmt.Errorf("add %s: %w", displayNameUniqueIndex, err)
	}
	log.Printf("mysql: users.display_name unique (renamed %d duplicate nicknames)", renamed)
	return nil
}

func dedupeDisplayNames(ctx context.Context, db *sql.DB) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT display_name FROM users GROUP BY display_name HAVING COUNT(*) > 1`)
	if err != nil {
		return 0, err
	}
	var dupes []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return 0, err
		}
		dupes = append(dupes, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	renamed := 0
	for _, name := range dupes {
		// 같은 이름(collation 기준)의 계정들 — 먼저 가입한 순. 첫 번째는 이름을 지킨다.
		idRows, err := db.QueryContext(ctx,
			`SELECT id, display_name FROM users WHERE display_name = ? ORDER BY created_at, id`, name)
		if err != nil {
			return renamed, err
		}
		type acct struct {
			id   int64
			name string
		}
		var accts []acct
		for idRows.Next() {
			var a acct
			if err := idRows.Scan(&a.id, &a.name); err != nil {
				idRows.Close()
				return renamed, err
			}
			accts = append(accts, a)
		}
		idRows.Close()

		n := 2
		for _, a := range accts[1:] {
			for {
				candidate := numberedDisplayName(a.name, n)
				n++
				var taken int
				if err := db.QueryRowContext(ctx,
					`SELECT COUNT(*) FROM users WHERE display_name = ?`, candidate).Scan(&taken); err != nil {
					return renamed, err
				}
				if taken > 0 {
					continue
				}
				if _, err := db.ExecContext(ctx,
					`UPDATE users SET display_name = ? WHERE id = ?`, candidate, a.id); err != nil {
					return renamed, err
				}
				renamed++
				break
			}
		}
	}
	return renamed, nil
}

// numberedDisplayName appends n, trimming the base (by characters) so the
// result still fits the column.
func numberedDisplayName(base string, n int) string {
	suffix := strconv.Itoa(n)
	runes := []rune(base)
	if max := displayNameColumnMax - len(suffix); len(runes) > max {
		runes = runes[:max]
	}
	return string(runes) + suffix
}
