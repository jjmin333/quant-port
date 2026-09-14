package service

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/marketdata"
)

// 기사 텍스트에서 종목을 찾아내는 규칙.
//
// 여기서는 **정밀도가 재현율보다 중요하다.** 화면(뉴스 화면의 「오늘의 이슈 종목」과
// 행마다 붙는 종목 칩)이 이 태그를 그대로 믿고 시세까지 끌어다 붙이기 때문에, 틀린
// 종목 하나가 붙는 것이 맞는 종목을 놓치는 것보다 나쁘다.
//
// 고치기 전에는 두 가지가 동시에 잘못돼 있었다:
//
//	국내 — 이름 인덱스를 "누군가 관심·보유에 담은 종목"으로만 만들어서, 아무도 담지
//	      않은 종목은 기사 제목에 이름이 나와도 영원히 태깅되지 않았다.
//	해외 — 티커 자체를 이름 인덱스 키로 넣고(`idx["v"] = "V"`) 경계 없는 부분 문자열로
//	      맞춰서, 알파벳 v가 하나라도 든 모든 기사에 Visa가 붙었다(실측 73건 중 51건).
//
// 그래서 이 파일의 모든 매칭은 **경계 검사**를 거치고, 짧은 이름·짧은 티커는 추가
// 근거를 요구한다.

// hangulRune reports whether r is a Hangul syllable or jamo — the "letters" a
// Korean company name is made of.
func hangulRune(r rune) bool {
	return (r >= 0xAC00 && r <= 0xD7A3) || (r >= 0x1100 && r <= 0x11FF) || (r >= 0x3130 && r <= 0x318F)
}

func latinOrDigit(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || unicode.IsDigit(r)
}

// nameTokenMatch reports whether name occurs in text as its own token.
//
// "Its own token" differs by script. A Latin name must not be glued to letters
// or digits on either side, so "arm" does not match "farmer" or "army". A
// Korean name must not have Hangul immediately *before* it — but Hangul after
// it is allowed, because Korean particles attach directly to the noun
// ("삼성전자가", "삼성전자는"). Requiring a boundary on both sides would reject
// almost every real sentence.
//
// text and name must both already be lowercased.
func nameTokenMatch(text, name string) bool {
	if name == "" || text == "" {
		return false
	}
	offset := 0
	for {
		i := strings.Index(text[offset:], name)
		if i < 0 {
			return false
		}
		start := offset + i
		end := start + len(name)

		beforeOK := true
		if start > 0 {
			r, _ := utf8.DecodeLastRuneInString(text[:start])
			// 앞이 한글이든 라틴 문자든, 이름이 더 긴 단어의 꼬리인 상황이다.
			beforeOK = !hangulRune(r) && !latinOrDigit(r)
		}

		afterOK := true
		if end < len(text) {
			r, _ := utf8.DecodeRuneInString(text[end:])
			switch {
			case latinOrDigit(r):
				// 라틴 이름 뒤에 글자가 붙으면 다른 단어다. 한글 이름 뒤의
				// 라틴 문자(예: "카카오M")도 다른 종목이므로 같이 막는다.
				afterOK = false
			case hangulRune(r):
				// 한글 뒤에는 조사가 바로 붙는다("삼성전자가"). 다만 아무
				// 글자나 허용하면 이름이 더 긴 단어의 머리로 걸린다 —
				// 링크드(193250)가 "링크드인"에 붙던 식이다. 그래서 조사로
				// 쓰이는 음절만 통과시킨다.
				afterOK = koreanParticleStarts[r]
			}
		}

		if beforeOK && afterOK {
			return true
		}
		offset = start + 1
		if offset >= len(text) {
			return false
		}
	}
}

// koreanParticleStarts are the syllables that may legitimately follow a company
// name with no space — the first syllable of a Korean particle or conjunctive
// ending. Anything else means the name was only the head of a longer word.
var koreanParticleStarts = map[rune]bool{
	'은': true, '는': true, '이': true, '가': true, '을': true, '를': true,
	'의': true, '에': true, '와': true, '과': true, '도': true, '로': true,
	'으': true, '만': true, '부': true, '까': true, '보': true, '랑': true,
	'께': true, '나': true, '든': true, '여': true, '야': true, '며': true,
	'라': true, '처': true, '한': true, '엔': true, '선': true,
}

// ambiguousDomesticNames are listed company names that are also ordinary Korean
// words. "인수 대상은…"은 대상(001680)에 관한 문장이 아니고, "우리 회사"는
// 우리기술이 아니다. 짧은 이름일수록 이런 충돌이 흔해서, 이 목록에 있으면 종목코드가
// 본문에 같이 나올 때만 인정한다.
var ambiguousDomesticNames = map[string]struct{}{
	"대상": {}, "동양": {}, "한일": {}, "우리": {}, "선진": {}, "국보": {},
	"일신": {}, "신라": {}, "미래": {}, "대신": {}, "상신": {}, "경동": {},
	"자이": {}, "다우": {}, "세방": {}, "진양": {}, "한창": {}, "삼진": {},
	"백광": {}, "대륙": {}, "이건": {}, "삼일": {}, "부산": {}, "제주": {},
	"광주": {}, "대구": {}, "전북": {}, "한국": {}, "코리아": {}, "에이스": {},
	"그린": {}, "하이": {}, "케이": {}, "지에스": {}, "체리": {},
	// 상호이면서 업계 일반명사이기도 한 이름들. 조사 규칙만으로는 "디바이스
	// 시장", "링크드인" 같은 문장을 걸러내지 못한다.
	"디바이스": {}, "링크드": {}, "시스템": {}, "솔루션": {}, "미디어": {},
	"커머스": {}, "바이오": {}, "에너지": {}, "모바일": {}, "글로벌": {},
	"인터넷": {}, "디지털": {}, "네트웍스": {}, "데이터": {}, "플랫폼": {},
	"클라우드": {}, "테크놀로지": {}, "스마트": {}, "컴퓨터": {}, "반도체": {},
	"엔터테인먼트": {}, "홀딩스": {}, "인베스트먼트": {}, "파트너스": {},
}

// domesticNameNeedsCode reports whether a domestic name is too weak to stand on
// its own — two-glyph names and known ordinary words. For those we require the
// article to also carry the 6-digit code, which no ordinary sentence does.
func domesticNameNeedsCode(name string) bool {
	if _, ok := ambiguousDomesticNames[name]; ok {
		return true
	}
	return utf8.RuneCountInString(name) <= 2
}

// containsDomesticCode reports whether text carries the 6-digit KRX code as its
// own token, so a price like "1248500원" cannot stand in for symbol 248500.
func containsDomesticCode(text, code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, r := range code {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	offset := 0
	for {
		i := strings.Index(text[offset:], code)
		if i < 0 {
			return false
		}
		start := offset + i
		end := start + len(code)
		okBefore := start == 0
		if !okBefore {
			r, _ := utf8.DecodeLastRuneInString(text[:start])
			okBefore = !unicode.IsDigit(r)
		}
		okAfter := end == len(text)
		if !okAfter {
			r, _ := utf8.DecodeRuneInString(text[end:])
			okAfter = !unicode.IsDigit(r)
		}
		if okBefore && okAfter {
			return true
		}
		offset = start + 1
		if offset >= len(text) {
			return false
		}
	}
}

// usTickerMinFreeTextLen is the shortest ticker we will accept from running
// text. One- and two-letter tickers (V, MA, F, GM, …) collide with ordinary
// words in every language we ingest, so they only arrive through the explicit
// forms — "$V", "NYSE: V" — that the marketdata extractors already handle.
const usTickerMinFreeTextLen = 3

// usTickerTokenMatch reports whether an uppercase ticker appears in text as a
// standalone token. text is the original (mixed-case) article text: a ticker is
// written in capitals, and lowercasing first is what let "v" match every word
// containing the letter.
func usTickerTokenMatch(text, ticker string) bool {
	if len(ticker) < usTickerMinFreeTextLen {
		return false
	}
	offset := 0
	for {
		i := strings.Index(text[offset:], ticker)
		if i < 0 {
			return false
		}
		start := offset + i
		end := start + len(ticker)
		okBefore := start == 0
		if !okBefore {
			r, _ := utf8.DecodeLastRuneInString(text[:start])
			okBefore = !latinOrDigit(r) && !hangulRune(r)
		}
		okAfter := end == len(text)
		if !okAfter {
			r, _ := utf8.DecodeRuneInString(text[end:])
			okAfter = !latinOrDigit(r) && !hangulRune(r)
		}
		if okBefore && okAfter {
			return true
		}
		offset = start + 1
		if offset >= len(text) {
			return false
		}
	}
}

// isPrefixOfLongerMatch reports whether name is contained in some other
// matched name — i.e. the shorter name was only the head of a company name
// that also matched the same article.
func isPrefixOfLongerMatch(name string, matched []string) bool {
	for _, other := range matched {
		if other == name || len(other) <= len(name) {
			continue
		}
		if strings.Contains(other, name) {
			return true
		}
	}
	return false
}

// mergeTickerCandidates adds Latin ticker candidates to an existing symbol
// list without re-filtering what is already there.
//
// marketdata.MergeTickerSymbols runs *every* entry — including the ones the
// caller already resolved — through isLikelyTicker, which only accepts 1–5
// uppercase A–Z letters. A KRX code like "005930" has digits, so passing an
// already-correct domestic symbol list through it silently deleted the whole
// list. That is why 국내 기사에 종목이 거의 붙지 않았다: the name matcher found
// 삼성전자 and the very next line threw it away.
func mergeTickerCandidates(existing []string, extra ...string) []string {
	out := append([]string(nil), existing...)
	seen := make(map[string]struct{}, len(out)+len(extra))
	for _, sym := range out {
		seen[strings.ToUpper(strings.TrimSpace(sym))] = struct{}{}
	}
	// 새로 들어오는 후보만 티커 규칙으로 거른다.
	for _, sym := range marketdata.MergeTickerSymbols(nil, extra...) {
		if _, dup := seen[sym]; dup {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}
	return out
}

// usNameMinLen is the shortest company-name key allowed into the US name
// index. Below this the "name" is really an abbreviation and behaves like a
// short ticker.
const usNameMinLen = 4

// corporateSuffixes are the legal-entity tails that appear in the security
// master but never in a headline.
var corporateSuffixes = []string{
	" incorporated", " inc.", " inc", " corporation", " corp.", " corp",
	" company", " co.", " limited", " ltd.", " ltd", " plc", " n.v.", " s.a.",
	" holdings", " holding", " group", " class a", " class b", " class c",
	" common stock", " ordinary shares", " adr", " sa", " ag", " se",
}

// trimCorporateSuffix strips one trailing legal-entity token, repeatedly, so
// "tesla, inc." → "tesla" and "arm holdings plc" → "arm". Input must already be
// lowercased.
func trimCorporateSuffix(name string) string {
	out := strings.TrimSpace(name)
	for changed := true; changed; {
		changed = false
		for _, suffix := range corporateSuffixes {
			if strings.HasSuffix(out, suffix) {
				out = strings.TrimSpace(strings.TrimSuffix(out, suffix))
				out = strings.TrimSuffix(out, ",")
				out = strings.TrimSpace(out)
				changed = true
				break
			}
		}
	}
	return out
}

// webDomainSuffixes are the tails that make a security-master name a domain
// rather than the word a headline uses.
var webDomainSuffixes = []string{".com", ".net", ".org", ".co.uk", ".io", ".ai"}

/*
trimHeadlineNameTail removes the two tails that keep a security-master name from
ever matching a headline. Input must already be lowercased.

Both come from real misses found in stored articles —

 1. **도메인 꼬리.** AMZN의 마스터 이름은 `Amazon.com`이다. 법인 접미사 목록에는
    `.com`이 없어 인덱스에 들어가는 키가 `amazon.com` 하나뿐이었고, "Amazon rising,
    Apple sinking"처럼 **회사를 그냥 Amazon이라 부르는 헤드라인은 전부 놓쳤다**.
    실제로 저장된 기사에서 Amazon과 Apple이 함께 나오는 10건 중 AMZN이 붙은 것은
    본문에 "Amazon.com"이 그대로 적힌 SEC 공시 기사뿐이었다.

 2. **클래스 표기 한 글자.** `Alphabet A`·`Berkshire Hathaway B`처럼 `class`라는
    단어 없이 한 글자만 붙는 형태다. 접미사 목록의 ` class a`는 이걸 못 잡는다.

두 경우 모두 잘라낸 **뒤에도** 경계 검사(nameTokenMatch)를 그대로 통과해야 하므로
"amazonian"에 amazon이 붙는 일은 없다. 자른 결과가 usNameMinLen보다 짧아지면
호출부가 버린다 — 한두 글자짜리 키는 티커처럼 굴어 오탐의 원천이 된다.
*/
func trimHeadlineNameTail(name string) string {
	out := strings.TrimSpace(name)
	for changed := true; changed; {
		changed = false
		for _, suffix := range webDomainSuffixes {
			if strings.HasSuffix(out, suffix) {
				out = strings.TrimSpace(strings.TrimSuffix(out, suffix))
				changed = true
				break
			}
		}
		// 마지막 토큰이 한 글자면 주식 클래스 표기다("alphabet a").
		if i := strings.LastIndexByte(out, ' '); i > 0 && len(out)-i == 2 {
			if c := out[i+1]; c >= 'a' && c <= 'z' {
				out = strings.TrimSpace(out[:i])
				changed = true
			}
		}
	}
	return out
}

/*
usNameStopwords — 이름 인덱스에 넣으면 안 되는 흔한 영어 단어.

이름 인덱스는 소문자 본문에 부분문자열(경계 검사 포함)로 맞춘다. 그래서 회사명에서
뽑은 토큰이 **일반 단어와 겹치면** 그 단어가 든 모든 기사에 그 종목이 붙는다.

실측(2026-08-24, 최근 3일 기사):

	HERE 49건 · BOND 39 · OPEN 27 · GOLD 26 · HONG 14 · FIVE 11

「금값」·「채권」·「홍콩」 기사에 GOLD.COM INC · PIMCO ACTIVE BOND ETF ·
LEVERAGE SHARES 2X HON이 붙었다. 그 태그는 화면 품질만 해친 게 아니라,
뉴스 브리핑을 통해 폴러의 시청 집합까지 밀어 올려 상류 호출을 만들었다.

**티커를 지우자는 게 아니다.** 그 종목들은 실재하고, 기사가 진짜로 그 종목을
다루면 `usTickerTokenMatch`(원문 대문자 + 경계)나 `$GOLD`·`NYSE: GOLD` 같은
명시 표기가 잡는다. 여기서 막는 것은 **이름으로 오인되는 경로**뿐이다.

목록은 「금융 기사에 흔히 나오면서 미국 티커와 겹치는 단어」로 한정한다.
길게 만들수록 진짜 회사명을 놓칠 위험이 커지므로, 실측으로 확인된 것과
같은 부류만 넣는다.
*/
/*
usNameStopwords — **이름**으로 매치하면 안 되는 낱말.

티커가 낱말인 경우(AMID·HIGH·MOVE…)는 여기가 아니라 `isWordLikeTicker`가 막는다
— 티커는 이제 대문자 인덱스로 가므로 소문자 낱말과 애초에 안 부딪힌다
(news_symbol_wordlike.go). 여기 남는 것은 **회사의 진짜 이름이 낱말인** 경우다.

둘로 나뉜다:

  ① 그 이름이 실은 **지수·시장 이름**이다. NDAQ의 상호는 "Nasdaq, Inc."라서
     나스닥 지수를 말하는 기사 전부가 이 회사 기사가 됐다 — 실측 1,000건 중 55건,
     엔비디아 다음으로 많았다. 진짜 Nasdaq Inc. 기사는 `$NDAQ`·`NASDAQ: NDAQ`로 잡는다.

  ② 그 이름이 **금융 기사에서 낱말로 훨씬 자주** 쓰인다. TGT의 상호는 "Target
     Corporation"인데, 증권 기사에서 target은 거의 언제나 「목표주가」다
     (실측 18건). 소매업체 타깃 기사는 `$TGT`로 잡는다.

이름을 막는 것은 **정보를 버리는 선택**이라 실측 없이 넣지 않는다. 위 둘은
2026-09-03 최근 1,000건 집계에서 오탐률이 사실상 100%였다.
*/
var usNameStopwords = map[string]struct{}{
	"gold": {}, "bond": {}, "open": {}, "here": {}, "hong": {}, "five": {},
	"cash": {}, "fund": {}, "growth": {}, "value": {}, "income": {},
	"real": {}, "core": {}, "next": {}, "best": {}, "safe": {}, "wall": {},
	"main": {}, "peak": {}, "wave": {}, "boss": {}, "hope": {}, "well": {},
	// ① 지수·시장 이름
	"nasdaq": {}, "nasdaq inc": {}, "dow jones": {}, "russell": {},
	"nyse": {}, "cboe": {}, "nikkei": {}, "ftse": {},
	// ② 금융 기사의 상용어와 겹치는 상호
	"target": {}, "target corp": {}, "target corporation": {},
	// ③ 중앙은행·기관 약어. 헤드라인에서 「Fed's …」·「BoJ's …」로 자주 나오고,
	//    그때 말하는 것은 통화정책이지 같은 글자로 시작하는 ETF가 아니다.
	"fed": {}, "boc": {}, "boj": {}, "ecb": {}, "pboc": {}, "rba": {},
	"snb": {}, "bok": {}, "imf": {}, "opec": {}, "commerce": {},
	// ④ 회사 이름을 줄이면 흔한 낱말이 되는 경우. 실측: "Hold Post Q2 Earnings?"가
	//    포스트 홀딩스(POST) 기사로 잡혔다 — 여기서 post는 「~ 이후」다.
	"post": {},
}

/*
addIdxKey — 이름 인덱스에 키를 넣되, 불용어는 거른다.

인덱스에 쓰는 모든 지점이 이 함수를 거쳐야 한다. 한 곳이라도 맵에 직접 넣으면
그 경로만 조용히 오탐을 만든다 — 실제로 티커를 직접 넣던 한 줄이 그랬다.
*/
func addIdxKey(idx map[string]string, key, sym string) {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" || len(key) < usNameMinLen {
		return
	}
	if _, blocked := usNameStopwords[key]; blocked {
		return
	}
	idx[key] = sym
}
