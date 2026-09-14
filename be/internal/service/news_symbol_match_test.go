package service

import "testing"

func TestNameTokenMatchAcceptsKoreanParticles(t *testing.T) {
	cases := []struct {
		text string
		name string
		want bool
	}{
		// 조사가 붙어도 종목명이다 — 한국어 문장 대부분이 이 형태다.
		{"삼성전자가 파운드리 수주를 늘렸다", "삼성전자", true},
		{"sk하이닉스는 hbm4 양산을 확정했다", "sk하이닉스", true},
		{"현대차·기아 실적 발표", "현대차", true},
		// 더 긴 이름의 꼬리로 걸린 경우는 아니다.
		{"한국항공우주 주가가 올랐다", "항공우주", false},
		{"에스케이하이닉스 관련 기사", "하이닉스", false},
		// 라틴 이름은 양쪽 모두 경계여야 한다.
		{"arm holdings ipo", "arm", true},
		{"the farmer said", "arm", false},
		{"army recruiting", "arm", false},
		// 한글 이름 뒤에 라틴 문자가 붙으면 다른 종목이다.
		{"카카오m 매각설", "카카오", false},
		{"카카오 자회사 매각설", "카카오", true},
		// 이름이 더 긴 한글 단어의 머리인 경우 — 조사가 아니면 다른 단어다.
		{"링크드인 채용 사기", "링크드", false},
		{"링크드가 상장했다", "링크드", true},
		// "에"는 조사라 "두산에너빌리티"의 "두산"도 여기서는 통과한다 —
		// 짧은 이름을 걷어내는 것은 extractSymbols의 최장 일치 규칙이다.
		{"두산은 SMR 사업을 키운다", "두산", true},
	}
	for _, c := range cases {
		if got := nameTokenMatch(c.text, c.name); got != c.want {
			t.Errorf("nameTokenMatch(%q, %q) = %v, want %v", c.text, c.name, got, c.want)
		}
	}
}

func TestUSTickerTokenMatchRejectsShortAndEmbedded(t *testing.T) {
	cases := []struct {
		text   string
		ticker string
		want   bool
	}{
		// 실제로 붙던 오탐: 티커 V가 알파벳 v 하나에 걸렸다.
		{"Ripple (XRP) holds above a dollar", "V", false},
		{"Reddit joins the S&P 500", "MA", false},
		{"Visa and Mastercard settle", "V", false}, // 짧은 티커는 자유 텍스트에서 안 받는다
		// 3글자 이상 티커는 경계가 맞으면 받는다.
		{"NVDA earnings beat expectations", "NVDA", true},
		{"엔비디아(NVDA) 실적 발표", "NVDA", true},
		{"NVDAX fund launches", "NVDA", false},
		{"buy TSLA now", "TSLA", true},
		{"TSLAQ crowd reacts", "TSLA", false},
	}
	for _, c := range cases {
		if got := usTickerTokenMatch(c.text, c.ticker); got != c.want {
			t.Errorf("usTickerTokenMatch(%q, %q) = %v, want %v", c.text, c.ticker, got, c.want)
		}
	}
}

func TestAddUSNameIndexKeysDropsShortTickerKeys(t *testing.T) {
	idx := map[string]string{}
	tickers := map[string]string{}
	addUSNameIndexKeys(idx, tickers, "V", "Visa Inc.", "비자")
	addUSNameIndexKeys(idx, tickers, "MA", "Mastercard Incorporated", "마스터카드")
	addUSNameIndexKeys(idx, tickers, "NVDA", "NVIDIA Corporation", "엔비디아")

	// 티커는 어느 쪽이든 **소문자 이름 인덱스**에 들어가지 않는다.
	for _, key := range []string{"v", "ma", "nvda"} {
		if sym, ok := idx[key]; ok {
			t.Errorf(`idx[%q] should not exist, got %q`, key, sym)
		}
	}
	// 짧은 티커는 티커 인덱스에도 안 들어간다 — "V"가 든 모든 기사에 붙는다.
	for _, key := range []string{"V", "MA"} {
		if sym, ok := tickers[key]; ok {
			t.Errorf(`tickers[%q] should not exist, got %q`, key, sym)
		}
	}
	if tickers["NVDA"] != "NVDA" {
		t.Errorf(`tickers["NVDA"] = %q, want "NVDA"`, tickers["NVDA"])
	}
	// 회사명은 여전히 들어간다.
	if idx["visa inc."] != "V" {
		t.Errorf(`idx["visa inc."] = %q, want "V"`, idx["visa inc."])
	}
	if idx["visa"] != "V" {
		t.Errorf(`idx["visa"] = %q, want "V" (comma/paren prefix token)`, idx["visa"])
	}
}

// 마스터 이름이 도메인·클래스 표기로 끝나면 헤드라인이 쓰는 형태와 영영 안 맞는다.
// 실측: "Amazon rising, Apple sinking …" 기사에 AAPL만 붙고 AMZN이 빠져 있었다.
func TestAddUSNameIndexKeysTrimsDomainAndClassTails(t *testing.T) {
	idx := map[string]string{}
	addUSNameIndexKeys(idx, nil, "AMZN", "Amazon.com", "아마존")
	addUSNameIndexKeys(idx, nil, "GOOGL", "Alphabet A", "알파벳")
	addUSNameIndexKeys(idx, nil, "BRK.B", "Berkshire Hathaway B", "버크셔")

	for _, c := range []struct{ key, want string }{
		{"amazon.com", "AMZN"}, // 원래 형태도 남는다
		{"amazon", "AMZN"},
		{"alphabet", "GOOGL"},
		{"berkshire hathaway", "BRK.B"},
	} {
		if idx[c.key] != c.want {
			t.Errorf("idx[%q] = %q, want %q", c.key, idx[c.key], c.want)
		}
	}
}

func TestTrimHeadlineNameTailKeepsOrdinaryNames(t *testing.T) {
	cases := []struct{ in, want string }{
		{"amazon.com", "amazon"},
		{"alphabet a", "alphabet"},
		{"berkshire hathaway b", "berkshire hathaway"},
		{"costco wholesale", "costco wholesale"}, // 일반 단어는 건드리지 않는다
		{"apple", "apple"},
		{"3m", "3m"},
	}
	for _, c := range cases {
		if got := trimHeadlineNameTail(c.in); got != c.want {
			t.Errorf("trimHeadlineNameTail(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDomesticNameNeedsCode(t *testing.T) {
	// 두 글자 이름과 일상어와 겹치는 이름은 코드가 있어야 인정한다.
	for _, name := range []string{"대상", "동양", "한국", "코리아", "농심"} {
		if !domesticNameNeedsCode(name) {
			t.Errorf("domesticNameNeedsCode(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"삼성전자", "sk하이닉스", "알테오젠", "오뚜기"} {
		if domesticNameNeedsCode(name) {
			t.Errorf("domesticNameNeedsCode(%q) = true, want false", name)
		}
	}
}

func TestContainsDomesticCodeRequiresTokenBoundary(t *testing.T) {
	cases := []struct {
		text string
		code string
		want bool
	}{
		{"삼성전자(005930) 실적", "005930", true},
		{"005930 주가", "005930", true},
		{"코드 1005930 은 다른 숫자", "005930", false},
		{"체결가 2485000원", "248500", false},
		{"종목코드 248500", "248500", true},
	}
	for _, c := range cases {
		if got := containsDomesticCode(c.text, c.code); got != c.want {
			t.Errorf("containsDomesticCode(%q, %q) = %v, want %v", c.text, c.code, got, c.want)
		}
	}
}
