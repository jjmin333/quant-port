package repository

import (
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

/*
조각을 만드는 함수들의 테스트다. SQL 자체는 DB 없이 못 돌리므로 **모양과 인자 개수**를
검사한다 — 실제로 났던 사고가 그 둘이었다: placeholder 수와 args 수가 어긋나면 드라이버가
런타임에야 실패한다.
*/

func TestRankableProductClauseBindsEveryPlaceholder(t *testing.T) {
	clause, args := scoreRankableProductClause("s")

	if got := strings.Count(clause, "?"); got != len(args) {
		t.Fatalf("placeholders = %d, args = %d — 개수가 맞아야 한다", got, len(args))
	}
	// asset_type 하나 + security_type 목록.
	if want := len(scoreRankingExcludedSecurityTypes) + 1; len(args) != want {
		t.Errorf("args = %d, want %d", len(args), want)
	}
	if args[0] != scoreRankingExcludedAssetType {
		t.Errorf("첫 인자 = %v, want %q", args[0], scoreRankingExcludedAssetType)
	}
	if !strings.Contains(clause, "s.asset_type <> ?") {
		t.Errorf("asset_type 조건이 없다: %s", clause)
	}
	/*
		`security_type`을 함께 보는 것이 이 조각의 요점이다. 실측 분포에서 미국 상장
		ETF는 `asset_type='us'`, 국내 ETF·ETN은 `asset_type='domestic'`이라
		asset_type만으로는 3,950종목이 그대로 통과한다.
	*/
	if !strings.Contains(clause, "s.security_type") {
		t.Errorf("security_type 조건이 없다: %s", clause)
	}
	// 별칭을 인자로 받는다 — 호출부가 stocks를 다른 이름으로 조인할 수 있다.
	other, _ := scoreRankableProductClause("st")
	if strings.Contains(other, "s.asset_type") || !strings.Contains(other, "st.asset_type") {
		t.Errorf("별칭이 안 먹었다: %s", other)
	}
}

func TestRankableProductClauseCoversEveryIndexProductType(t *testing.T) {
	clause, _ := scoreRankableProductClause("s")
	// 대소문자가 섞여 들어오므로 비교 전에 정규화해야 한다.
	if !strings.Contains(clause, "UPPER(TRIM(s.security_type))") {
		t.Errorf("security_type 비교가 정규화되지 않았다: %s", clause)
	}
	for _, want := range []string{"ETF", "ETN", "ETP"} {
		found := false
		for _, t2 := range scoreRankingExcludedSecurityTypes {
			if t2 == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%s가 제외 목록에 없다 — 지수 추종 상품이 랭킹에 오른다", want)
		}
	}
}

/*
축 개수 식은 `model.ScoreAxes`에서 만든다. 손으로 다섯 줄을 적어 두면 축이 하나
늘었을 때 이 식만 옛날 다섯 개를 세고, 그 사실이 **아무 데서도 안 터진다**.
*/
func TestAxisCountExprCoversEveryAxis(t *testing.T) {
	expr := scoreAxisCountExpr("sc")
	for _, axis := range model.ScoreAxes {
		if !strings.Contains(expr, "'$."+string(axis)+"'") {
			t.Errorf("%s 축이 개수 식에 없다: %s", axis, expr)
		}
	}
	if got := strings.Count(expr, "JSON_EXTRACT"); got != len(model.ScoreAxes)*2 {
		t.Errorf("JSON_EXTRACT 수 = %d, want %d", got, len(model.ScoreAxes)*2)
	}
	// JSON null을 「있다」로 세면 안 된다 — 옛 행에 `"sentiment": null`이 남아 있다.
	if !strings.Contains(expr, "JSON_TYPE") {
		t.Errorf("JSON null 구분이 없다: %s", expr)
	}
}

func TestMinAxesClauseUsesTheConfiguredThreshold(t *testing.T) {
	clause := scoreMinAxesClause("sc")
	if !strings.HasSuffix(strings.TrimSpace(clause), ">= 3") {
		t.Errorf("문턱이 3이 아니다: %s", clause)
	}
	if scoreRankingMinAxes != 3 {
		t.Errorf("scoreRankingMinAxes = %d, want 3 (2026-09-04 사용자 요청)", scoreRankingMinAxes)
	}
	// 조각은 이미 WHERE가 있는 쿼리 뒤에 붙는다 — AND로 시작해야 한다.
	if !strings.HasPrefix(strings.TrimSpace(clause), "AND ") {
		t.Errorf("AND로 시작하지 않는다: %s", clause)
	}
}
