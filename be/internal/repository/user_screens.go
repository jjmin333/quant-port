package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

/*
저장한 스크리너 조건 — 표를 왜 이렇게 두는지는 `migrations/076_user_screens.sql`에.
*/

// UserScreenRepo stores each user's saved screener conditions.
type UserScreenRepo struct{ db *sql.DB }

func NewUserScreenRepo(db *sql.DB) *UserScreenRepo { return &UserScreenRepo{db: db} }

// UserScreenLimit — 사용자당 저장 개수 상한. 마이그레이션 머리글 참조.
const UserScreenLimit = 20

func (r *UserScreenRepo) List(ctx context.Context, userID int64) ([]model.UserScreen, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, market, rules_json, sort_by, sort_asc,
		       DATE_FORMAT(updated_at, '%Y-%m-%d')
		  FROM user_screens
		 WHERE user_id = ?
		 ORDER BY updated_at DESC, id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("user screens list: %w", err)
	}
	defer rows.Close()

	out := []model.UserScreen{}
	for rows.Next() {
		var (
			s        model.UserScreen
			rulesRaw []byte
			sortAsc  int
		)
		if err := rows.Scan(&s.ID, &s.Name, &s.Market, &rulesRaw, &s.SortBy, &sortAsc, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.SortAsc = sortAsc == 1
		/*
			읽을 때 **한 번 더** 화이트리스트를 통과시킨다.

			지표는 사라진다 — 공급자가 항목을 빼거나 우리가 축을 바꾸면, 저장된 조건에
			지금은 없는 필드가 남는다. 그것을 그대로 화면에 보내면 조건 이름이 빈칸인
			줄이 생기고, 그대로 실행하면 조회가 거절된다. 여기서 걸러 내고 **몇 개를
			걸렀는지 세어** 화면이 「조건 1개는 더 이상 쓸 수 없습니다」를 말하게 한다.
		*/
		var stored []model.ScreenRule
		if len(rulesRaw) > 0 {
			if err := json.Unmarshal(rulesRaw, &stored); err != nil {
				return nil, fmt.Errorf("user screen %d rules: %w", s.ID, err)
			}
		}
		for _, rule := range stored {
			if _, ok := model.LookupScreenField(rule.Field); ok {
				s.Rules = append(s.Rules, rule)
				continue
			}
			s.DroppedRules++
		}
		if s.Rules == nil {
			s.Rules = []model.ScreenRule{}
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

/*
Upsert — 같은 이름이면 덮어쓴다.

새로 만드는 것과 고치는 것을 한 경로로 두는 이유: 화면의 동작이 「이름을 적고
저장」 하나뿐이라, 이름이 같으면 사용자가 뜻한 것은 언제나 덮어쓰기다. 다른 뜻이었다면
다른 이름을 적는다.
*/
func (r *UserScreenRepo) Upsert(ctx context.Context, userID int64, s model.UserScreen) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("user screens not configured")
	}
	rulesRaw, err := json.Marshal(s.Rules)
	if err != nil {
		return 0, err
	}
	sortAsc := 0
	if s.SortAsc {
		sortAsc = 1
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO user_screens (user_id, name, market, rules_json, sort_by, sort_asc)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  market = VALUES(market), rules_json = VALUES(rules_json),
		  sort_by = VALUES(sort_by), sort_asc = VALUES(sort_asc), id = LAST_INSERT_ID(id)`,
		userID, strings.TrimSpace(s.Name), s.Market, rulesRaw, s.SortBy, sortAsc)
	if err != nil {
		return 0, fmt.Errorf("user screen save: %w", err)
	}
	return res.LastInsertId()
}

func (r *UserScreenRepo) Delete(ctx context.Context, userID, id int64) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("user screens not configured")
	}
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM user_screens WHERE user_id = ? AND id = ?`, userID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// MigrateUserScreensIfNeeded creates the table on an existing database.
func MigrateUserScreensIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "user_screens")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir,
		"076_user_screens.sql", "저장한 스크리너 조건")
}
