package service

import (
	"strings"
	"testing"
)

/*
미국 종목의 **한국어 이름**으로 검색·태깅한다 (2026-09-12 사용자 요청 2).

무엇이 빠져 있었나: 손으로 적은 별칭 13개에 `구글`은 있고 **`알파벳 A`·`알파벳 C`는
없었다.** 그런데 그 이름은 이미 `stocks.name`에 있다(토스가 준 한국어 종목명,
주식 종류까지) — 손으로 적을 것이 아니라 가진 것을 쓰면 된다.
*/

// usIdx는 addUSNameIndexKeys 로 한 종목을 색인한 결과를 준다.
func usIdx(t *testing.T, sym, english, local string) map[string]string {
	t.Helper()
	idx := map[string]string{}
	addUSNameIndexKeys(idx, map[string]string{}, sym, english, local)
	return idx
}

func TestUSNameIndex_indexesKoreanShareClassNames(t *testing.T) {
	// 이 세 짝이 사용자가 지목한 바로 그 경우다.
	cases := []struct{ sym, english, local string }{
		{"GOOGL", "Alphabet A", "알파벳 A"},
		{"GOOG", "Alphabet C", "알파벳 C"},
		{"BRK.A", "BERKSHIRE HATHAWAY INC", "버크셔 해서웨이 A"},
	}
	for _, c := range cases {
		idx := usIdx(t, c.sym, c.english, c.local)
		if got := idx[strings.ToLower(c.local)]; got != c.sym {
			t.Fatalf("%s: 한국어 이름 %q 가 색인되지 않았다 (got %q)", c.sym, c.local, got)
		}
	}
}

func TestUSNameIndex_koreanNameDoesNotMatchLongerWord(t *testing.T) {
	// 한글 이름의 위험은 더 긴 낱말의 머리로 걸리는 것이다. 그 판정은
	// nameTokenMatch 의 조사 규칙이 한다 — 색인을 열어도 이 성질은 유지돼야 한다.
	for _, c := range []struct {
		name string
		text string
		want bool
	}{
		{"애플", "애플 주가가 상승했다", true},
		{"애플", "애플리케이션 개발", false},   // 리 는 조사가 아니다
		{"메타", "메타가 신제품을 냈다", true},
		{"메타", "메타버스 시장 전망", false}, // 버 는 조사가 아니다
		{"알파벳 a", "알파벳 a 주가 상승", true},
	} {
		got := nameTokenMatch(strings.ToLower(c.text), c.name)
		if got != c.want {
			t.Fatalf("%q in %q = %v, want %v", c.name, c.text, got, c.want)
		}
	}
}

func TestUSNameIndex_skipsOrdinaryWordNames(t *testing.T) {
	// 「비자 발급」 기사에 결제회사가 붙으면 안 된다. 조사 규칙으로는 안 막히는
	// 경우(「비자가 필요하다」)가 있어 색인 자체에서 뺀다.
	idx := usIdx(t, "V", "Visa", "비자")
	if _, indexed := idx["비자"]; indexed {
		t.Fatal("일상어와 같은 한국어 이름이 색인됐다")
	}
	// 영어 이름은 그대로 남아야 한다 — 재현율을 통째로 잃지 않는다.
	if idx["visa"] != "V" {
		t.Fatalf("영어 이름이 색인되지 않았다: %v", idx)
	}
}

func TestUSNameIndex_nonHangulLocalNameStillGoesThroughLatinRules(t *testing.T) {
	/*
		토스가 영문으로 적는 종목(AMD·QQQ·SPY)은 **예전 경로 그대로**여야 한다.

		그 경로의 규칙은 `usNameMinLen`(4)이다 — 세 글자 영문 이름은 이름 인덱스에
		안 들어가고 **티커 인덱스**로 간다. 자유 본문에서 "AMD"를 이름으로 매칭하면
		평범한 약어와 부딪히기 때문이고, 티커 인덱스는 대문자 표기만 본다.
		한글 이름을 연 것이 이 규칙을 건드리지 않았는지 잠근다.
	*/
	idx := map[string]string{}
	tickers := map[string]string{}
	addUSNameIndexKeys(idx, tickers, "AMD", "Advanced Micro Devices", "AMD")

	if _, tooShort := idx["amd"]; tooShort {
		t.Fatalf("세 글자 영문 이름이 이름 인덱스에 들어갔다: %v", idx)
	}
	if tickers["AMD"] != "AMD" {
		t.Fatalf("티커 인덱스에 없다: %v", tickers)
	}
	// 긴 영어 이름은 그대로 이름 인덱스로 간다.
	if idx["advanced micro devices"] != "AMD" {
		t.Fatalf("영어 이름이 색인되지 않았다: %v", idx)
	}
}

func TestAmbiguousUSLocalNames_onlyHoldsWordsParticleRuleCannotSave(t *testing.T) {
	// 조사 규칙이 이미 막는 이름을 목록에 넣으면 재현율만 잃는다.
	for name := range ambiguousUSLocalNames {
		if nameTokenMatch(name+"버스", name) {
			continue // 조사 규칙이 못 막는다 — 목록에 있어야 한다
		}
		if !nameTokenMatch(name+"가 필요하다", name) {
			t.Fatalf("%q 는 조사 규칙만으로 충분하다 — 목록에서 빼야 한다", name)
		}
	}
}

func TestUSLocalName_indexNameIsNotTheExchangeOperator(t *testing.T) {
	// 「나스닥이 1.2% 하락」은 지수다 — NDAQ(Nasdaq, Inc.)으로 붙으면 안 된다(2026-10-10).
	idx := map[string]string{}
	addUSNameIndexKeys(idx, nil, "NDAQ", "Nasdaq, Inc.", "나스닥")
	for _, key := range []string{"나스닥", "nasdaq"} {
		if sym, ok := idx[key]; ok {
			t.Fatalf("이름 키 %q → %s 가 색인됐다", key, sym)
		}
	}
}
