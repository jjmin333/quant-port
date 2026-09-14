package service

import (
	"os"
	"strings"
	"testing"
)

// 소스를 읽어 구조적 사실을 잠그는 시험용 도우미.
//
// 런타임으로 재현하려면 거래소 달력·Redis·Toss가 필요한 판정이 있다. 그런 경우
// 「그 호출이 그 자리에 있는가」만이라도 잠가 두면, 리팩터링이 조용히 빼 가는 것을
// 막을 수 있다. 동작을 시험하는 것이 아니므로 남용하지 않는다.

func readServiceSource(t *testing.T, file string) string {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	return string(raw)
}

func countOccurrences(s, sub string) int { return strings.Count(s, sub) }

func containsStr(s, sub string) bool { return strings.Contains(s, sub) }

// funcBody returns the text from the given func signature to the next
// top-level closing brace.
func funcBody(src, signature string) (string, bool) {
	i := strings.Index(src, signature)
	if i < 0 {
		return "", false
	}
	rest := src[i:]
	if end := strings.Index(rest, "\n}\n"); end >= 0 {
		return rest[:end], true
	}
	return rest, true
}
