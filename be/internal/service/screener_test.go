package service

import (
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

/*
스크리너 시험 — 재는 것은 **조용히 넘어가지 않는가**다.

이 기능이 틀리면 사용자가 손해를 보는 자리는 셋이다: ① 모르는 조건을 말없이 버리고
결과를 내는가 ② 화면 단위와 저장 단위를 헷갈리는가 ③ 거르지 않는 조건을 저장해 두고
나중에 실행에서 거절하는가.
*/

func screenRule(field string, min, max *float64) model.ScreenRule {
	return model.ScreenRule{Field: field, Min: min, Max: max}
}

/*
# 모르는 조건은 **버리지 않고 거절한다**

버리면 사용자가 건 조건이 빠진 채로 결과가 나오는데, 화면은 그 조건이 걸린 줄 안다.
「ROE 15% 이상」이라고 적힌 화면에 ROE 3%짜리가 앉아 있게 되고, 사용자는 그것을
데이터 오류로 읽는다 — 가장 나쁜 종류의 조용한 실패다.
*/
func TestScreenRejectsUnknownField(t *testing.T) {
	min := 15.0
	_, _, _, err := normalizeScreenQuery(model.ScreenQuery{
		Market: "domestic",
		Rules:  []model.ScreenRule{screenRule("roe", &min, nil), screenRule("ebitda", &min, nil)},
	})
	if err == nil {
		t.Fatal("모르는 조건은 거절해야 한다")
	}
	if !strings.Contains(err.Error(), "ebitda") {
		t.Errorf("무엇이 문제인지 이름을 말해야 한다: %v", err)
	}
}

// 같은 조건을 두 번 걸면 어느 쪽이 이겼는지 화면이 말할 수 없다.
func TestScreenRejectsDuplicateField(t *testing.T) {
	a, b := 10.0, 20.0
	_, _, _, err := normalizeScreenQuery(model.ScreenQuery{
		Rules: []model.ScreenRule{screenRule("roe", &a, nil), screenRule("roe", &b, nil)},
	})
	if err == nil {
		t.Fatal("같은 조건을 두 번 걸면 거절해야 한다")
	}
}

// 최솟값이 최댓값보다 크면 결과는 언제나 0이다 — 0을 주기 전에 말해 준다.
func TestScreenRejectsInvertedRange(t *testing.T) {
	min, max := 50.0, 10.0
	_, _, _, err := normalizeScreenQuery(model.ScreenQuery{
		Rules: []model.ScreenRule{screenRule("rsi14", &min, &max)},
	})
	if err == nil {
		t.Fatal("뒤집힌 범위는 거절해야 한다")
	}
}

/*
# 시가총액은 화면이 **조(兆)**로 보내고 DB는 **원**으로 받는다

실측: 삼성전자 `inputs.marketCap`은 1482031627128000(원)이다. 화면이 「1조 이상」을
보냈는데 그대로 비교하면 시가총액 1원 이상인 모든 종목이 통과한다 — 조건이 없는 것과
같아지고, 사용자는 조건이 걸렸다고 믿는다.
*/
func TestScreenScalesMarketCapToStoredUnit(t *testing.T) {
	min := 1.0 // 1조원
	norm, active, _, err := normalizeScreenQuery(model.ScreenQuery{
		Rules: []model.ScreenRule{screenRule("marketCap", &min, nil)},
	})
	if err != nil {
		t.Fatalf("정상 조건이 거절됐다: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("거르는 조건 하나여야 한다: %d", len(active))
	}
	if norm.Rules[0].Min == nil || *norm.Rules[0].Min != 1e12 {
		t.Errorf("1조원은 1e12로 바뀌어야 한다: %v", norm.Rules[0].Min)
	}

	// 배율이 없는 필드는 건드리지 않는다.
	roeMin := 15.0
	plain, _, _, err := normalizeScreenQuery(model.ScreenQuery{
		Rules: []model.ScreenRule{screenRule("roe", &roeMin, nil)},
	})
	if err != nil {
		t.Fatalf("정상 조건이 거절됐다: %v", err)
	}
	if plain.Rules[0].Min == nil || *plain.Rules[0].Min != 15 {
		t.Errorf("ROE는 그대로여야 한다: %v", plain.Rules[0].Min)
	}
}

/*
# 값이 없는 줄은 **조건이 아니지만 열이다**

「PER로 거르지는 않되 PER은 보고 싶다」가 실제로 흔한 요구다. 그런 줄은 거르는 데
쓰이지 않고(`active`에 안 들어감) 결과표의 열로만 나간다(`cells`에 들어감).
*/
func TestScreenKeepsValuelessRuleAsColumnOnly(t *testing.T) {
	min := 70.0
	_, active, cells, err := normalizeScreenQuery(model.ScreenQuery{
		Rules: []model.ScreenRule{screenRule("score", &min, nil), screenRule("per", nil, nil)},
	})
	if err != nil {
		t.Fatalf("정상 조건이 거절됐다: %v", err)
	}
	if len(active) != 1 || active[0].ID != "score" {
		t.Errorf("거르는 조건은 종합 점수 하나다: %+v", active)
	}
	if len(cells) != 2 {
		t.Errorf("열은 둘이어야 한다 (PER은 보기만): %d", len(cells))
	}
}

// 정렬 기준도 화이트리스트를 통과해야 한다 — 컬럼 이름을 받으면 그것도 주입 통로다.
func TestScreenRejectsUnknownSort(t *testing.T) {
	if _, _, _, err := normalizeScreenQuery(model.ScreenQuery{SortBy: "sc.score; DROP"}); err == nil {
		t.Fatal("모르는 정렬 기준은 거절해야 한다")
	}
	if _, _, _, err := normalizeScreenQuery(model.ScreenQuery{SortBy: "roe"}); err != nil {
		t.Errorf("정상 정렬 기준이 거절됐다: %v", err)
	}
}

// 상한을 넘겨 요청하면 상한으로 깎는다 — 거절할 일은 아니다.
func TestScreenClampsLimit(t *testing.T) {
	norm, _, _, err := normalizeScreenQuery(model.ScreenQuery{Limit: 9999})
	if err != nil {
		t.Fatalf("정상 요청이 거절됐다: %v", err)
	}
	if norm.Limit != model.ScreenMaxLimit {
		t.Errorf("상한으로 깎아야 한다: %d", norm.Limit)
	}
	if def, _, _, _ := normalizeScreenQuery(model.ScreenQuery{}); def.Limit != model.ScreenDefaultLimit {
		t.Errorf("기본값이 있어야 한다: %d", def.Limit)
	}
}

// 시장 이름이 이상하면 국내로 떨어진다 — 랭킹의 기본값과 같다.
func TestScreenNormalizesMarket(t *testing.T) {
	cases := map[string]string{
		"overseas": "overseas", "OVERSEAS": "overseas",
		"domestic": "domestic", "": "domestic", "화성": "domestic",
	}
	for in, want := range cases {
		got, _, _, err := normalizeScreenQuery(model.ScreenQuery{Market: in})
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got.Market != want {
			t.Errorf("%q → %q, want %q", in, got.Market, want)
		}
	}
}

// 조건 수 상한. 넘으면 거절한다.
func TestScreenRejectsTooManyRules(t *testing.T) {
	min := 1.0
	rules := make([]model.ScreenRule, 0, model.ScreenMaxRules+1)
	for i := 0; i <= model.ScreenMaxRules; i++ {
		rules = append(rules, screenRule("roe", &min, nil))
	}
	if _, _, _, err := normalizeScreenQuery(model.ScreenQuery{Rules: rules}); err == nil {
		t.Fatal("조건 수 상한을 넘기면 거절해야 한다")
	}
}

/*
# 화이트리스트가 비어 있지 않고, 식이 새어 나가지 않는다

`Expr`은 JSON 응답에 **절대 나가면 안 된다**(`json:"-"`). 나가면 내부 스키마가 그대로
공개되고, 다음 사람이 그것을 API 계약으로 오해한다.
*/
func TestScreenFieldCatalogIsSane(t *testing.T) {
	if len(model.ScreenFields) == 0 {
		t.Fatal("걸 수 있는 조건이 하나는 있어야 한다")
	}
	seen := map[string]struct{}{}
	for _, f := range model.ScreenFields {
		if f.ID == "" || f.Label == "" || f.Expr == "" {
			t.Errorf("빈 칸이 있는 필드: %+v", f)
		}
		if _, dup := seen[f.ID]; dup {
			t.Errorf("중복 ID: %s", f.ID)
		}
		seen[f.ID] = struct{}{}
		if _, ok := model.ScreenGroupLabels[f.Group]; !ok {
			t.Errorf("%s의 묶음에 이름이 없다: %s", f.ID, f.Group)
		}
	}
	// 실측으로 확인한 저장 경로들 — 이름이 바뀌면 조용히 전부 NULL이 된다.
	for _, id := range []string{"score", "roe", "debtRatio", "per", "pbr", "rsi14",
		"pctFromHigh52Week", "marketCap", "axis.growth", "axis.sentiment"} {
		if _, ok := model.LookupScreenField(id); !ok {
			t.Errorf("있어야 할 조건이 없다: %s", id)
		}
	}
}

/*
# 저장한 조건은 **화면 단위 그대로** 남아야 한다

실측으로 드러난 왕복 결함: 시가총액 「1조」를 저장할 때 조회용 환산값(1e12)을 적으면,
화면이 그것을 다시 불러 보낼 때 1e12에 또 1e12가 곱해진다. 결과는 언제나 0이고
사용자는 자기 조건이 고장 났다고 읽는다.

환산은 **실행 지점 한 곳**(`Screen`)에만 있어야 한다.
*/
func TestSavedScreenKeepsDisplayUnits(t *testing.T) {
	min := 1.0 // 1조원
	in := model.UserScreen{
		Name:   "대형 저평가",
		Market: "domestic",
		Rules:  []model.ScreenRule{screenRule("marketCap", &min, nil)},
	}

	// SaveScreen의 저장 직전 모양을 그대로 재현한다 (DB 없이 재는 부분).
	norm, active, _, err := normalizeScreenQuery(model.ScreenQuery{Market: in.Market, Rules: in.Rules})
	if err != nil || len(active) != 1 {
		t.Fatalf("정상 조건이어야 한다: %v", err)
	}
	if norm.Rules[0].Min == nil || *norm.Rules[0].Min != 1e12 {
		t.Fatalf("조회용 값은 환산돼야 한다: %v", norm.Rules[0].Min)
	}
	// 저장용은 원본이어야 한다 — 같은 포인터가 아니라 같은 **값**인지를 본다.
	if in.Rules[0].Min == nil || *in.Rules[0].Min != 1 {
		t.Errorf("환산이 입력을 덮어썼다: %v", in.Rules[0].Min)
	}

	// 저장된 값을 다시 넣었을 때 같은 조회값이 나와야 한다 (왕복).
	again, _, _, err := normalizeScreenQuery(model.ScreenQuery{Market: in.Market, Rules: in.Rules})
	if err != nil {
		t.Fatalf("왕복이 거절됐다: %v", err)
	}
	if again.Rules[0].Min == nil || *again.Rules[0].Min != 1e12 {
		t.Errorf("왕복하면 값이 달라진다: %v", again.Rules[0].Min)
	}
}
