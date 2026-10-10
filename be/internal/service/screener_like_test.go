package service

import (
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

// 「닮은꼴」 정렬(2026-10-01 요청 1): 모르는 업종은 식에 들어가지 않고, 기준 종목 이름은 거절 규칙을 따른다.
func TestScreenLikeSortExpr(t *testing.T) {
	ref := map[string]float64{"axis.growth": 63, "axis.quality": 76, "score": 62}
	expr := screenLikeSortExpr(ref, "information_technology")
	if !strings.Contains(expr, "ss.sector = 'information_technology'") {
		t.Fatalf("same-sector bonus missing: %s", expr)
	}
	if strings.Count(expr, "POW(LEAST(") != 2 {
		t.Fatalf("want one term per known axis (2), got: %s", expr)
	}
	if strings.Contains(expr, "sc.score") {
		t.Fatalf("composite score must not be a distance term: %s", expr)
	}
	// 화이트리스트 밖 업종은 식에 문자열로 들어가면 안 된다.
	if e := screenLikeSortExpr(ref, "x' OR 1=1 --"); strings.Contains(e, "OR 1=1") || strings.Contains(e, "CASE") {
		t.Fatalf("invalid sector leaked into SQL: %s", e)
	}
	if screenLikeSortExpr(map[string]float64{}, "") != "" {
		t.Fatal("no axes → no expression (fall back to score order)")
	}
}

func TestNormalizeScreenQueryLike(t *testing.T) {
	q, _, _, err := normalizeScreenQuery(model.ScreenQuery{Market: "overseas", Like: " mu "})
	if err != nil || q.Like != "MU" {
		t.Fatalf("like normalized = %q, %v", q.Like, err)
	}
	if _, _, _, err := normalizeScreenQuery(model.ScreenQuery{Like: "MU'; DROP"}); err == nil {
		t.Fatal("malformed like symbol must be rejected")
	}
}
