package repository

import (
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

/*
한글 발음으로 영문 상장명을 찾는다.

회귀 대상: 「네이버」·「포스코」·「숲」은 `name`(NAVER)에도 `english_name`에도 그 읽기가
없어 어떤 규칙에도 걸리지 않았다 — 실측 0건.
*/
func TestSearchAliasScore_hangulReadingFindsLatinName(t *testing.T) {
	cases := []struct {
		q      string
		symbol string
		name   string
	}{
		{"네이버", "035420", "NAVER"},
		{"네이", "035420", "NAVER"},
		{"포스코", "005490", "POSCO홀딩스"},
		{"숲", "067160", "SOOP"},
		{"아프리카티비", "067160", "SOOP"},
		{"케이티앤지", "033780", "KT&G"},
		{"케이티", "030200", "KT"},
		{"에스오일", "010950", "S-Oil"},
		{"에스케이", "000660", "SK하이닉스"},
		{"엘지", "066570", "LG전자"},
		{"에이치엠엠", "011200", "HMM"},
		{"에이치피에스피", "403870", "HPSP"},
		{"코덱스", "069500", "KODEX 200"},
	}
	for _, c := range cases {
		st := model.Stock{Symbol: c.symbol, Name: c.name, AssetType: model.AssetDomestic}
		if got := searchRelevanceScore(c.q, st, false); got <= 0 {
			t.Fatalf("%q should find %q, got %d", c.q, c.name, got)
		}
	}
}

// 별칭은 한글 이름이 직접 걸린 행을 밀어내면 안 된다 (표는 추측, 이름은 사실).
func TestSearchAliasScore_ranksBelowDirectNameMatch(t *testing.T) {
	viaAlias := model.Stock{Symbol: "037560", Name: "LG헬로비전", AssetType: model.AssetDomestic}
	direct := model.Stock{Symbol: "003550", Name: "엘지생활건강", AssetType: model.AssetDomestic}

	aliasScore := searchRelevanceScore("엘지", viaAlias, false)
	directScore := searchRelevanceScore("엘지", direct, false)
	if aliasScore <= 0 {
		t.Fatal("alias row should still match")
	}
	if directScore < aliasScore {
		t.Fatalf("hangul name match must not rank below an alias match: %d vs %d", directScore, aliasScore)
	}
}

// 별칭이 없는 한글 질의는 아무것도 끌어오지 않는다.
func TestSearchAliasTokens_unknownReading(t *testing.T) {
	if got := SearchAliasTokens("삼성전자"); got != nil {
		t.Fatalf("unknown reading should map to nothing, got %v", got)
	}
	if got := SearchAliasTokens("naver"); got != nil {
		t.Fatalf("latin query needs no alias, got %v", got)
	}
	if got := SearchAliasTokens("ㅅ"); got != nil {
		t.Fatalf("single jamo should map to nothing, got %v", got)
	}
}

// 짧은 접두는 여러 브랜드에 걸린다 — 개수는 상한을 넘지 않고 순서는 결정적이다.
func TestSearchAliasTokens_prefixIsCappedAndDeterministic(t *testing.T) {
	first := SearchAliasTokens("에스")
	if len(first) == 0 {
		t.Fatal("expected prefix matches for 에스")
	}
	if len(first) > searchAliasCap {
		t.Fatalf("alias count must stay within the cap, got %v", first)
	}
	for range 5 {
		if got := SearchAliasTokens("에스"); strings.Join(got, ",") != strings.Join(first, ",") {
			t.Fatalf("alias order must be stable: %v != %v", got, first)
		}
	}
}

// 한글 질의의 후보 조건에 별칭 조각이 실린다 (없으면 NAVER 행이 후보에 없다).
func TestAppendSearchQueryFilter_hangulAliasCandidates(t *testing.T) {
	var q, args = "", []any(nil)
	appendSearchQueryFilter(&q, &args, "네이버", false)
	found := false
	for _, a := range args {
		if s, ok := a.(string); ok && strings.Contains(s, "naver") {
			found = true
		}
	}
	if !found {
		t.Fatalf("alias pattern missing from candidate args: %v", args)
	}
}
