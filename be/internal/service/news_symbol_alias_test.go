package service

import (
	"context"
	"testing"
)

/*
사용자 신고 (2026-09-03): 「네이버, 공격·방어 '국가 보안 AI' 만든다(종합)」에
NAVER(035420)가 안 붙고 엔비디아만 붙었다.

원인은 이름 인덱스가 상장명(`NAVER`)만 담고 있어 한글 「네이버」가 어디에도 없었던
것이다. 아래는 그 인덱스를 만드는 규칙에 대한 시험이다 — DB 없이 인덱스 맵만 준다.
*/

func TestAddHangulAliasNames_mapsHangulReadingToLatinListing(t *testing.T) {
	idx := map[string]string{
		"naver": "035420",
		"kt":    "030200",
		"삼성전자":  "005930",
	}
	addHangulAliasNames(idx)

	if got := idx["네이버"]; got != "035420" {
		t.Fatalf("네이버 = %q, want 035420", got)
	}
	// 「케이티」는 세 글자라 얹힌다.
	if got := idx["케이티"]; got != "030200" {
		t.Fatalf("케이티 = %q, want 030200", got)
	}
}

// 두 글자 읽기는 얹지 않는다 — 어차피 종목코드를 요구받고, 일상어와 부딪히기만 한다.
func TestAddHangulAliasNames_skipsTwoSyllableReadings(t *testing.T) {
	idx := map[string]string{"lg": "003550"}
	addHangulAliasNames(idx)

	if _, ok := idx["엘지"]; ok {
		t.Fatal("two-syllable reading 엘지 was added to the index")
	}
}

/*
상장명이 **정확히** 그 토큰인 종목만 받는다.

「포스코」의 토큰은 `posco`인데 상장명은 「POSCO홀딩스」 꼴이라 정확히 일치하는 종목이
없다 — 그때는 얹지 않는다. 접두로 받으면 「포스코」 한 단어에 지주와 계열사 중 어느
쪽을 붙일지 정할 수 없다.
*/
func TestAddHangulAliasNames_requiresAnExactListingName(t *testing.T) {
	idx := map[string]string{"posco홀딩스": "005490", "poscoi": "058430"}
	addHangulAliasNames(idx)

	if sym, ok := idx["포스코"]; ok {
		t.Fatalf("포스코 resolved to %q on a prefix match", sym)
	}
}

// ETF 브랜드는 저절로 빠진다 — 상장명이 정확히 「KODEX」인 종목은 없다.
func TestAddHangulAliasNames_dropsEtfBrandsWithNoExactListing(t *testing.T) {
	idx := map[string]string{"kodex 200": "069500", "tiger 200": "102110"}
	addHangulAliasNames(idx)

	for _, reading := range []string{"코덱스", "타이거"} {
		if sym, ok := idx[reading]; ok {
			t.Fatalf("ETF brand %s resolved to %q", reading, sym)
		}
	}
}

// 실제 상장명이 그 한글이면 별칭이 덮지 않는다 — DB에 있는 사실이 먼저다.
func TestAddHangulAliasNames_neverOverwritesARealListingName(t *testing.T) {
	idx := map[string]string{"네이버": "999999", "naver": "035420"}
	addHangulAliasNames(idx)

	if got := idx["네이버"]; got != "999999" {
		t.Fatalf("네이버 = %q, want the real listing 999999", got)
	}
}

// 일상어와 겹치는 이름은 별칭으로도 우회시키지 않는다(`ambiguousDomesticNames`).
func TestAddHangulAliasNames_respectsTheAmbiguousNameList(t *testing.T) {
	idx := map[string]string{"ace": "111111"}
	addHangulAliasNames(idx)

	if sym, ok := idx["에이스"]; ok {
		t.Fatalf("ambiguous reading 에이스 resolved to %q", sym)
	}
}

// 토큰 여럿이 서로 다른 종목으로 풀리면 포기한다 — 찍지 않는다.
func TestResolveAliasSymbol_bailsOnConflict(t *testing.T) {
	idx := map[string]string{"a": "111111", "b": "222222"}
	if sym, ok := resolveAliasSymbol(idx, []string{"a", "b"}); ok {
		t.Fatalf("conflicting tokens resolved to %q", sym)
	}
	if sym, ok := resolveAliasSymbol(idx, []string{"a", "missing"}); !ok || sym != "111111" {
		t.Fatalf("single resolvable token = %q/%v, want 111111/true", sym, ok)
	}
}

// 빈 인덱스에는 아무것도 얹지 않는다 — 카탈로그를 못 읽은 상황이다.
func TestAddHangulAliasNames_emptyIndexStaysEmpty(t *testing.T) {
	idx := map[string]string{}
	addHangulAliasNames(idx)
	if len(idx) != 0 {
		t.Fatalf("empty index gained %d entries", len(idx))
	}
}

/*
신고된 기사 제목 그대로 — 인덱스에 별칭이 얹힌 뒤에는 NAVER가 붙는다.

`addHangulAliasNames`가 채운 것과 같은 모양의 인덱스를 만들어 실제 추출 경로를 탄다.
*/
func TestExtractSymbols_taggingNaverFromHangulHeadline(t *testing.T) {
	idx := map[string]string{"naver": "035420", "카카오": "035720"}
	addHangulAliasNames(idx)
	st := &NewsStore{nameIndex: idx}

	got := st.extractSymbols(context.Background(), "네이버, 공격·방어 '국가 보안 AI' 만든다(종합)")

	if len(got) != 1 || got[0] != "035420" {
		t.Fatalf("extractSymbols = %v, want [035420]", got)
	}
}

// 「네이버지도」처럼 이름이 더 긴 낱말의 머리로 나오면 붙지 않는다 — 별칭도
// 이름 매칭의 경계 규칙을 그대로 따른다.
func TestExtractSymbols_aliasRespectsTokenBoundaries(t *testing.T) {
	idx := map[string]string{"naver": "035420"}
	addHangulAliasNames(idx)
	st := &NewsStore{nameIndex: idx}

	if got := st.extractSymbols(context.Background(), "네이버지도 개편 소식"); len(got) != 0 {
		t.Fatalf("extractSymbols = %v, want none", got)
	}
	// 조사는 붙어도 된다 — 「네이버가」는 여전히 네이버다.
	if got := st.extractSymbols(context.Background(), "네이버가 AI를 만든다"); len(got) != 1 {
		t.Fatalf("extractSymbols = %v, want [035420]", got)
	}
}
