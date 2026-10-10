package growth

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

/*
이 워커가 앱의 표를 바꾸지 않는다는 약속을 지킨다.

패키지 소스(시험 제외)에서 INSERT INTO · UPDATE · DELETE FROM · REPLACE INTO · ALTER/DROP/TRUNCATE
뒤에 오는 표 이름을 모아, growth_ 로 시작하지 않는 것이 하나라도 있으면 실패한다. 점수 산식 · 금융 원본 ·
사용자 데이터 · 인증 · 법적 고지를 여기서 고치는 코드가 들어오면 이 시험이 먼저 막는다.
*/
func TestWritesOnlyTouchGrowthTables(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	write := regexp.MustCompile(`(?i)\b(INSERT\s+INTO|REPLACE\s+INTO|DELETE\s+FROM|UPDATE|ALTER\s+TABLE|DROP\s+TABLE|TRUNCATE(?:\s+TABLE)?)\s+([a-z_][a-z0-9_]*)`)
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range write.FindAllStringSubmatch(string(raw), -1) {
			table := strings.ToLower(m[2])
			// 「ON DUPLICATE KEY UPDATE col = …」의 UPDATE 뒤는 표가 아니라 열이다.
			if strings.EqualFold(strings.Fields(m[1])[0], "UPDATE") && isUpsertClause(string(raw), m[0]) {
				continue
			}
			checked++
			if !strings.HasPrefix(table, "growth_") {
				t.Errorf("%s: %q writes to %s — worker-growth may only write growth_* tables", f, m[0], table)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no write statements found — the pattern is broken")
	}
}

func isUpsertClause(src, match string) bool {
	for i := strings.Index(src, match); i >= 0; {
		before := strings.ToUpper(src[max(0, i-20):i])
		if !strings.Contains(before, "KEY") {
			return false
		}
		next := strings.Index(src[i+len(match):], match)
		if next < 0 {
			return true
		}
		i = i + len(match) + next
	}
	return true
}
