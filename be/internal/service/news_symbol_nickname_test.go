package service

import (
	"sort"
	"testing"
)

// 이름 인덱스의 최소 재현 — `rebuildNameIndex`가 만드는 것과 같은 모양이다
// (소문자 상장명 → 대문자 심볼).
func nicknameTestIndex() map[string]string {
	return map[string]string{
		"삼성전자":     "005930",
		"sk하이닉스":   "000660",
		"현대차":      "005380",
		"lg에너지솔루션": "373220",
	}
}

/*
2026-09-04 사용자 지적 10의 회귀 테스트.

「37% 빠진 삼전닉스…노무라 "심각한 저평가"」에 종목이 하나도 안 붙었다. 삼성전자와
SK하이닉스 둘 다에 대한 기사인데, 「삼전닉스」는 어느 회사의 상장명도 아니라
이름 인덱스 어디에도 없었다.
*/
func TestNicknameSymbols_samjeonNix(t *testing.T) {
	got := matchNicknameSymbols(`37% 빠진 삼전닉스…노무라 "심각한 저평가"`, nicknameTestIndex())
	sort.Strings(got)
	want := []string{"000660", "005930"}
	if len(got) != len(want) {
		t.Fatalf("symbols = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("symbols = %v, want %v", got, want)
		}
	}
}

// 별명이 가리키는 종목 중 **인덱스에 있는 것만** 붙는다. 상장폐지·이름 변경이 나면
// 틀린 종목을 붙이는 대신 조용히 빠진다.
func TestNicknameSymbols_skipsUnknownListing(t *testing.T) {
	idx := map[string]string{"삼성전자": "005930"} // SK하이닉스가 없다
	got := matchNicknameSymbols("삼전닉스 반등", idx)
	if len(got) != 1 || got[0] != "005930" {
		t.Fatalf("symbols = %v, want [005930]", got)
	}
}

// 단독 통칭도 푼다 — 상장명이 「SK하이닉스」라 「하이닉스」만으로는 앞 경계 검사에
// 막힌다(한글 앞 경계를 요구하므로 "sk"에 붙은 꼬리로 읽힌다).
func TestNicknameSymbols_standaloneNicknames(t *testing.T) {
	idx := nicknameTestIndex()
	for _, tc := range []struct{ text, want string }{
		{"하이닉스 3분기 실적", "000660"},
		{"삼전 7만원 회복", "005930"},
		{"현차 판매량 증가", "005380"},
		{"엘지엔솔 수주", "373220"},
	} {
		got := matchNicknameSymbols(tc.text, idx)
		if len(got) != 1 || got[0] != tc.want {
			t.Errorf("%q → %v, want [%s]", tc.text, got, tc.want)
		}
	}
}

/*
**더 긴 단어의 꼬리로 걸리지 않는다.** 매칭은 `nameTokenMatch`가 하므로 앞 경계가
한글이면 거부된다 — 이 검사가 없으면 별명 하나가 온갖 기사에 붙는다(해외 티커
"V"가 v 든 기사 전부에 Visa를 붙이던 것과 같은 종류의 사고).
*/
func TestNicknameSymbols_requiresTokenBoundary(t *testing.T) {
	idx := nicknameTestIndex()
	if got := matchNicknameSymbols("우리삼전닉스", idx); len(got) != 0 {
		t.Errorf("앞에 한글이 붙었는데 매칭됐다: %v", got)
	}
}

// 별명이 아무것도 없는 본문에는 아무것도 붙지 않는다.
func TestNicknameSymbols_noMatch(t *testing.T) {
	if got := matchNicknameSymbols("오늘 코스피는 보합권에 머물렀다", nicknameTestIndex()); len(got) != 0 {
		t.Errorf("symbols = %v, want none", got)
	}
}

// 별명이 실제 상장명과 같으면 이 경로를 타지 않는다 — 이름 인덱스가 이미 맞추고,
// 무엇보다 `domesticNameNeedsCode`(짧은 이름은 종목코드를 요구한다)를 우회시키면 안 된다.
func TestNicknameSymbols_skipsWhenNicknameIsAListedName(t *testing.T) {
	idx := map[string]string{"삼전": "999999", "삼성전자": "005930"}
	if got := matchNicknameSymbols("삼전 강세", idx); len(got) != 0 {
		t.Errorf("상장명과 같은 별명을 별명으로 풀었다: %v", got)
	}
}
