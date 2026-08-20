package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// perfIndexes — 050이 만드는 인덱스들. 마커가 테이블/컬럼이 아니라 인덱스 이름이라
// 별도 목록으로 둔다. 두 인덱스가 서로 다른 테이블에 걸려 있어 하나만 마커로 쓰면
// 나머지 하나가 영원히 적용되지 않는다.
var perfIndexes = []struct{ table, index string }{
	{"stock_scores", "idx_stock_scores_rank"},
	{"news_articles", "idx_news_market_updated"},
}

// MigratePerfIndexesIfNeeded applies 050_perf_indexes.sql.
//
// 순수 인덱스 추가라 데이터는 건드리지 않는다. 컨테이너 여섯 개가 함께 부팅해
// 같은 CREATE INDEX를 동시에 실행하면 진 쪽이 1061(ER_DUP_KEYNAME)을 받는데,
// 그건 이 배포 형태에서 정상적인 결과지 실패가 아니다 — isSchemaRaceError로 넘긴다.
func MigratePerfIndexesIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	missing := false
	for _, ix := range perfIndexes {
		exists, err := tableExists(ctx, db, ix.table)
		if err != nil {
			return err
		}
		if !exists {
			// 테이블 자체가 아직 없으면 그 테이블을 만드는 마이그레이션이 인덱스까지
			// 책임진다. 여기서 먼저 손대면 CREATE INDEX가 없는 테이블에 나간다.
			continue
		}
		has, err := indexExists(ctx, db, ix.table, ix.index)
		if err != nil {
			return err
		}
		if !has {
			missing = true
		}
	}
	if !missing {
		return nil
	}

	path := filepath.Join(migrationsDir, "050_perf_indexes.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 050_perf_indexes.sql: %w", err)
	}
	applied := false
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if isSchemaRaceError(err) {
				continue
			}
			return fmt.Errorf("050_perf_indexes.sql: %w", err)
		}
		applied = true
	}
	if applied {
		log.Println("mysql: applied 050_perf_indexes.sql (score ranking + news updated_at indexes)")
	}
	return nil
}

func indexExists(ctx context.Context, db *sql.DB, table, index string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?
	`, table, index).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
