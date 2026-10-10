package repository

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

/*
`migrations/`의 모든 파일이 **이미 도는 DB에도 적용되는지** 센다.

# 왜 이 시험이 있나 (2026-09-14)

마이그레이션이 적용되는 길이 **둘**이고, 둘이 보는 것이 다르다.

	· **빈 DB** — `applyMigrationFiles`가 디렉터리의 `.sql`을 번호순으로 전부 돌린다.
	  새 파일은 두기만 하면 여기서는 저절로 적용된다.
	· **이미 도는 DB** — 표가 없을 때만 그 파일을 적용하는 함수들이 한다
	  (`MigrateXxxIfNeeded`). 새 파일을 **여기에 등록하는 것은 손으로 하는 일**이다.

그래서 잊어버리면 새로 붓는 DB에는 표가 있고 운영 DB에는 없다. 개발에서는 아무
증상이 없고(늘 빈 DB로 시작한다), 운영에서는 몇 주 뒤에 그 표를 처음 읽는 질의
하나가 터지는 것으로만 나타난다. 이 시험이 그 시차를 없앤다.

**파일 이름이 Go 소스 어딘가에 적혀 있는가**만 센다. 그 이상(정말 불리는가, 분기가
맞는가)은 DB 없이 알 수 없고, 잊어버리는 실수는 언제나 「이름이 아무 데도 없다」
모양으로 나타난다.
*/
func TestEveryMigrationFileIsReferenced(t *testing.T) {
	/*
		등록하지 않는 파일들과 그 이유. 새 파일을 여기 넣으려면 **왜 이미 도는 DB에는
		필요 없는지**를 적어야 한다 — 이유 없이 목록만 늘면 이 시험은 껍데기가 된다.
	*/
	exempt := map[string]string{
		"001_init.sql":                                  "최초 스키마 — 이미 도는 DB에는 정의상 이미 있다",
		"002_snapshots_insights.sql":                    "위와 같음 (RequiredTables가 덮는다)",
		"003_candle_bars.sql":                           "위와 같음",
		"004_stocks.sql":                                "위와 같음",
		"012_candle_bars_datetime.sql":                  "컬럼 타입 교체 — 손으로 한 번 돌린 일회성",
		"026_storage_cleanup.sql":                       "저장 공간 정리 — 사람이 필요할 때 돌린다",
		"045_holding_trades_price_mismatch_cleanup.sql": "잘못된 행 정리 — 일회성",
		"060_drop_60m_candles.sql":                      "표 제거 — 일회성 (다시 돌면 안 된다)",
		"091_screener_phrases.sql":                      "문장 검색 표 — 기능을 내렸고 103이 DROP한다(이미 도는 DB에 다시 만들면 안 된다)",
	}
	dir := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("마이그레이션 디렉터리를 못 읽는다: %v", err)
	}

	sources := goSources(t, filepath.Join("..", ".."))
	numbered := regexp.MustCompile(`^\d{3}_.+\.sql$`)
	seen := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !numbered.MatchString(name) {
			continue
		}
		seen++
		if _, ok := exempt[name]; ok {
			continue
		}
		if !strings.Contains(sources, name) {
			t.Errorf("%s 가 Go 소스 어디에서도 적용되지 않는다 — 빈 DB에는 들어가지만 "+
				"**이미 도는 DB에는 영영 안 들어간다.** `MigrateXxxIfNeeded`를 만들어 "+
				"migrate.go 의 두 자리에 등록하거나, 일회성이면 이 시험의 `exempt`에 "+
				"이유와 함께 적어라", name)
		}
	}
	if seen == 0 {
		t.Fatal("마이그레이션 파일을 하나도 못 찾았다 — 경로가 틀렸다")
	}

	// 사라진 파일이 면제 목록에 남으면, 다음 사람은 그 이름이 무엇이었는지 모른 채
	// 목록만 물려받는다.
	for name := range exempt {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("면제 목록의 %s 가 이제 없다 — 목록에서 지워라", name)
		}
	}
}

// goSources concatenates every .go file under root (vendor and testdata aside).
func goSources(t *testing.T, root string) string {
	t.Helper()
	var sb strings.Builder
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", "vendor", "testdata", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sb.Write(body)
		return nil
	})
	if err != nil {
		t.Fatalf("Go 소스를 못 읽는다: %v", err)
	}
	return sb.String()
}
