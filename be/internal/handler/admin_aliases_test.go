package handler

import "testing"

/*
검증 규칙은 **거부**가 기본이다 (별칭 화면 머리글).

별칭이 틀리면 화면에 오류가 안 뜬다 — 그냥 아무 기사도 안 붙고, 아무도 모른다.
그래서 틀릴 수 있는 값은 저장되기 전에 막는다. 여기 있는 두 함수가 그 막는 자리의
전부라, 이 시험이 곧 규칙의 문서다.
*/

func TestLooksLikeSymbol(t *testing.T) {
	t.Parallel()

	ok := []string{
		"GOOGL",  // 해외 보통주
		"005930", // 국내 — 숫자 여섯 자리
		"BRK.B",  // 점이 들어간 클래스 표기
		"RDS-A",  // 하이픈이 들어간 표기
		"AAPL",
	}
	for _, s := range ok {
		if !looksLikeSymbol(s) {
			t.Fatalf("%q 를 심볼이 아니라고 했다", s)
		}
	}

	bad := []struct{ in, why string }{
		{"", "빈 값"},
		{"구글", "한글 — 별칭 자리에 들어갈 값이 대상 자리에 왔다"},
		{"googl", "소문자 — 호출부가 대문자로 올린 뒤에 부른다"},
		{"AAPL US", "공백"},
		{"AAPL;DROP", "문장부호"},
	}
	for _, tc := range bad {
		if looksLikeSymbol(tc.in) {
			t.Fatalf("%q 를 심볼로 받았다 (%s)", tc.in, tc.why)
		}
	}
}

// 갈래는 표가 아는 둘뿐이다. 다른 값이 들어오면 그 행은 스냅샷의 어느 map 에도
// 안 들어가 — 저장은 되는데 아무 일도 안 일어나는 행이 된다.
func TestValidAliasKind(t *testing.T) {
	t.Parallel()

	for _, k := range []string{"brand", "symbol"} {
		if !validAliasKind(k) {
			t.Fatalf("%q 를 거부했다", k)
		}
	}
	for _, k := range []string{"", "Brand", "ticker", "symbols", " symbol"} {
		if validAliasKind(k) {
			t.Fatalf("%q 를 받아들였다", k)
		}
	}
}
