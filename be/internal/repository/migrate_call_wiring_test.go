package repository

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

/*
`Migrate…IfNeeded` 함수가 **어딘가에서 불리는지** 센다 (2026-09-23).

`TestEveryMigrationFileIsReferenced`는 마이그레이션 파일 이름이 Go 소스에 적혀 있는지만
본다. 그런데 파일 이름은 `Migrate…IfNeeded` 함수 **안에** 적히므로, 함수를 만들어 놓고
`migrate.go`에 등록하는 것을 잊어도 그 시험은 통과한다 — 079(기사 감성 라벨)가 실제로
그렇게 빠질 뻔했다. 운영 DB에서는 그 표가 영영 안 생기고, 처음 읽는 질의가 터질 때에야 보인다.
*/
func TestEveryMigrateIfNeededIsCalled(t *testing.T) {
	exempt := map[string]string{
		// 기존(이 시험 이전)부터 호출처가 없다. 운영 DB에는 news_articles가 이미 있어
		// 증상이 없지만, 빈 표가 필요한 새 환경이라면 migrate.go에 등록해야 한다.
		"MigrateNewsIfNeeded": "기존 미호출 — 2026-09-23 발견, 별도 확인 대상",
	}
	sources := goSources(t, filepath.Join("..", ".."))
	defs := regexp.MustCompile(`func (Migrate[A-Za-z0-9]+IfNeeded)\(`).FindAllStringSubmatch(sources, -1)
	if len(defs) == 0 {
		t.Fatal("Migrate…IfNeeded 정의를 하나도 못 찾았다 — 경로가 틀렸다")
	}
	for _, m := range defs {
		name := m[1]
		if _, ok := exempt[name]; ok {
			continue
		}
		calls := strings.Count(sources, name+"(") - strings.Count(sources, "func "+name+"(")
		if calls <= 0 {
			t.Errorf("%s 가 정의만 있고 불리지 않는다 — migrate.go의 「all required tables present」 "+
				"경로에 등록하지 않으면 이미 도는 DB에는 그 표가 영영 안 생긴다", name)
		}
	}
}
