package translate

import (
	"sort"
	"strings"
	"unicode"
)

/*
회사 이름을 **모델이 건드리지 못하게** 잠그고 번역한 뒤 한국어 이름으로 되돌린다.

이 파일이 있는 이유는 실측 하나다. LMT-60-1.7B에 실제 헤드라인 20건을 넣었더니
길이·한글·반복 검사를 **전부 통과하면서** 이런 것들이 나왔다:

	Jim Cramer has a strong message for Nvidia stock investors
	→ 짐 크라머가 **인텔** 주식 투자자들에게 강한 메시지를 보냈다

	Fidelity's FHLC vs. Invesco's …  → **퍼실리티**의 FHLC vs. 인베스코의 …
	Lilly's Record Stock Price …     → **리리**의 기록적인 주가 …

**엔비디아 기사가 인텔 기사가 되는 것**은 어떤 검사로도 못 잡는다. 문장은 자연스럽고
길이도 맞고 한글도 있다. 「번역이 이상하다」가 아니라 **사실이 바뀐 것**이라, 이건
품질 문제가 아니라 정확성 문제다.

그래서 이름은 번역기에게 맡기지 않는다. 우리는 그 기사에 어떤 종목이 붙어 있는지
이미 알고(태깅 파이프라인), 그 종목의 한국어 이름도 카탈로그에 들고 있다. 모델에는
**자리표시자**를 주고, 그 자리에 우리가 아는 이름을 넣는다.

자리표시자 모양은 실측으로 골랐다:

	@@1@@   → 통째로 사라졌다
	Kv1     → KV1로 대문자화됐다 (되돌리기 실패)
	$NV1    → 그대로 살아남았다
	Xq1     → 그대로 살아남았다

`$` 접두사에 **대문자**를 쓴다 — 티커처럼 보이는 토큰이라 모델이 손대지 않고,
대문자면 대문자화가 일어나도 그대로다.

# 숫자 접미사를 걷어 낸 이유 (2026-09-14)

위 실측은 **자체 모델(LMT-60) 기준**이었고, 엔진을 Azure로 바꾸면서 그대로 깨졌다.
Azure는 글자 뒤에 붙은 숫자를 **낱말에서 떼어낸다**:

	$QPZ0: Ignore The Usual Naysayers    → $QPZ 0: 평소 부정적인 말을 무시하라
	… Increases Stock Holdings in $QPZ0  → 알렉타…, $QPZ년 주식 보유 증가   (!)

떼어지든 「년」이 되든 `RestoreNames`는 자리표시자를 못 찾아 **실패**한다. 그리고
그 실패는 조용하다 — 제목이 영어로 남을 뿐 오류가 없어서, 같은 24건을 2분마다
영원히 다시 보내며 **문자만 청구됐다**(실측 2026-09-14: 하루 12,559요청 ·
1,069,514자, 그날 새 기사는 511건뿐이었다).

접미사를 **글자**로 바꾸면 그대로 살아남는다. 같은 제목 틀 셋으로 잰 결과:

	$QPZ0   0/3      $AAPL0  0/3     ← 숫자가 있으면 진다
	$QPZA   3/3      QPZA    3/3     ← 글자면 이긴다
	XQPZAX  3/3      ZQZA    3/3

**두 글자 고정폭**을 쓴다(AA, AB, … ZZ). 한 글자면 이름이 27개를 넘을 때
`$QPZA`가 `$QPZAA`의 접두가 되어 되돌리기가 서로를 먹는다 — 고정폭이면 그 경우가
아예 없다. 늘어난 한 글자는 제목당 1~2자이고, 그 대가로 되돌리기가 성립한다.

Azure 공식 「동적 사전」(`<mstrans:dictionary translation="…">…</mstrans:dictionary>`)
도 4/4 통과했지만 **마크업이 그대로 과금된다** — 이름 하나에 70자가 붙어 제목이
세 배가 된다. 자리표시자는 6자다.
*/

// NamePair is one company's English name and the Korean name we want printed.
type NamePair struct {
	English string
	Korean  string
}

/*
namePlaceholder builds the token handed to the model: $QPZAA, $QPZAB, …

QPZ는 영어 본문·티커 어디에도 잘 나오지 않는 세 글자다. 접두사를 바꾸려면
되돌리기 쪽(`RestoreNames`)이 같은 것을 봐야 한다.

접미사는 **두 글자 고정폭**이다 — 숫자를 쓰면 Azure가 떼어낸다(머리글 참고).
676개를 넘으면 빈 문자열을 준다. 그때 그 이름은 잠기지 않고 번역기에게 그냥
간다(`MaskNames`가 빈 자리표시자를 보고 건너뛴다) — 제목 하나에 회사 이름이
676개 나오는 일은 없지만, 없는 경우를 패닉으로 두지 않는다.
*/
func namePlaceholder(i int) string {
	if i < 0 || i >= 26*26 {
		return ""
	}
	return "$QPZ" + string(rune('A'+i/26)) + string(rune('A'+i%26))
}

/*
builtinNamePairs — **카탈로그에 없는데 자주 나오는 이름들** (2026-09-15).

이름 잠그기는 지금까지 「그 기사에 붙은 종목」만 봤다(`namePairsFor`). 그래서
상장사가 아닌 이름은 모델에게 맡겨졌고, 모델은 그것을 **낱말로 읽는다** — 실측:

	Perplexity local agent platform → 「**당혹감** 현지 에이전트 플랫폼」
	Anthropic's Amodei             → 「**인간학**의 아모디」
	… - Yahoo Finance              → 「… - 야후 **재정**」

도메인을 잠그는 이유와 같은 문제다(아래 MaskNames의 도메인 머리글) — 다만
`- Yahoo Finance`는 도메인이 아니라 낱말이라 그 그물에 안 걸린다. 30일 실측에서
헤드라인 꼬리의 출처 표기는 **Yahoo Finance만 1,374건**이었다.

목록을 늘리는 규칙: **실측으로 자주 나오는 것만.** 여기 있는 이름은 번역문에서
통째로 치환되므로, 드물게 나오는 이름을 넣으면 얻는 것 없이 오탐 위험만 는다.
`.com`으로 끝나는 출처(Investing.com·Bloomberg.com·fool.com)는 이미 도메인
그물이 걷어 가므로 넣지 않는다.
*/
var builtinNamePairs = []NamePair{
	// 비상장 AI 기업 — 낱말로 읽히는 것이 확인된 것들.
	{English: "Anthropic", Korean: "앤스로픽"},
	{English: "Perplexity", Korean: "퍼플렉시티"},
	{English: "OpenAI", Korean: "오픈AI"},

	// 헤드라인 꼬리에 붙는 출처. 30일 실측 상위에서 **낱말로 번역될 수 있는
	// 것**만 골랐다 — CNBC·WSJ 같은 약어는 모델이 그대로 둔다.
	// 지역판이 따로 온다 — 긴 것부터 잠기므로(길이순 정렬) 셋을 다 적어 둔다.
	// 실측 30일: Yahoo Finance 1,374 · UK 128 · Canada 82.
	{English: "Yahoo! Finance Canada", Korean: "야후 파이낸스 캐나다"},
	{English: "Yahoo Finance UK", Korean: "야후 파이낸스 UK"},
	{English: "Yahoo Finance", Korean: "야후 파이낸스"},
	{English: "Seeking Alpha", Korean: "시킹알파"},
	{English: "MarketWatch", Korean: "마켓워치"},
	{English: "Investor's Business Daily", Korean: "인베스터스 비즈니스 데일리"},
	{English: "Morningstar", Korean: "모닝스타"},
	{English: "TradingView", Korean: "트레이딩뷰"},
	{English: "TipRanks", Korean: "팁랭크스"},
	{English: "Benzinga", Korean: "벤징가"},
	{English: "Barron's", Korean: "배런스"},
	{English: "Reuters", Korean: "로이터"},
	{English: "The Globe and Mail", Korean: "글로브앤메일"},

	/*
		**그대로 두기 위해** 잠그는 이름도 있다 — 한국어 표기가 없어서 옮기면
		오히려 못 알아본다. 실측(2026-09-15 드레인): 같은 출처가 한 줄에서는
		`FJElite`, 다른 줄에서는 「FJ엘리트」로 나왔다. 어느 쪽이 옳고 그르기
		이전에 **한 화면에서 갈리는 것**이 문제다.
	*/
	{English: "FJElite", Korean: "FJElite"},
}

/*
MaskNames는 원문에서 아는 회사 이름을 자리표시자로 바꾸고, 되돌릴 이름표를 함께 준다.

**긴 이름부터** 바꾼다. "Alto Ingredients"를 "Alto"보다 먼저 보지 않으면 앞부분만
잘려 나가고 뒤에 "Ingredients"가 남는다.

이름이 하나도 안 걸리면 원문을 그대로 돌려준다 — 그 경우 되돌릴 것도 없다.
*/
func MaskNames(text string, pairs []NamePair) (string, []string) {
	if text == "" {
		return text, nil
	}
	ordered := make([]NamePair, 0, len(pairs)+len(builtinNamePairs))
	for _, p := range pairs {
		if strings.TrimSpace(p.English) == "" || strings.TrimSpace(p.Korean) == "" {
			continue
		}
		ordered = append(ordered, p)
	}
	// 카탈로그에 없는 이름도 잠근다 — 아래 목록의 근거는 머리글에 있다.
	ordered = append(ordered, builtinNamePairs...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return len(ordered[i].English) > len(ordered[j].English)
	})

	masked := text
	var restore []string

	/*
		**도메인부터 잠근다.** 헤드라인 꼬리에 붙는 출처(`- finance.yahoo.com`)를
		모델이 낱말로 읽고 옮긴다 — 실측: "Nvidia Just Broke the Stock Market -
		finance.yahoo.com"이 「… - **재정**.yahoo.com」이 됐다. 도메인은 번역할
		것이 아니라 그대로 두어야 할 것이라, 회사 이름과 같은 방식으로 잠근다.
	*/
	for _, host := range findDomains(masked) {
		ph := namePlaceholder(len(restore))
		if ph == "" {
			break
		}
		replaced, ok := replaceWordFold(masked, host, ph)
		if !ok {
			continue
		}
		masked = replaced
		restore = append(restore, host)
	}
	for _, p := range ordered {
		ph := namePlaceholder(len(restore))
		if ph == "" {
			break
		}
		replaced, ok := replaceWordFold(masked, p.English, ph)
		if !ok {
			continue
		}
		masked = replaced
		restore = append(restore, p.Korean)
	}
	return masked, restore
}

/*
RestoreNames는 자리표시자를 한국어 이름으로 되돌린다.

**하나라도 사라졌으면 실패로 친다**(두 번째 반환값 false). 모델이 자리표시자를
지웠다는 것은 그 자리의 회사를 통째로 빼먹었다는 뜻이라, 그대로 내보내면 주어가
없는 제목이 된다. 실패한 건은 다음 엔진으로 내려간다.

# 왜 느슨하게 찾나

엔진마다 토큰을 조금씩 다르게 건드린다. 자체 모델은 대문자화했고(`$qpz0`), Azure는
**낱말 안에 공백을 넣었다**(`$QPZ AA`). 지금 쓰는 두 글자 접미사는 Azure에서 쪼개지지
않지만(머리글의 실측), 다음 엔진이 무엇을 할지는 모른다 — 그리고 이 실패는 **조용해서**
알아채는 데 하루가 걸렸다. 그래서 대소문자와 **공백 끼어들기**를 둘 다 받아 준다.

느슨함의 한계는 분명히 둔다: 자리표시자의 **글자 순서가 그대로**여야 하고 사이에
들어와도 되는 것은 공백뿐이다. 「$QPZ년」처럼 글자가 바뀐 것은 여전히 실패다 —
그 자리에 무슨 회사가 있었는지 더는 알 수 없기 때문이다.
*/
func RestoreNames(translated string, restore []string) (string, bool) {
	if len(restore) == 0 {
		return translated, true
	}
	out := translated
	for i, korean := range restore {
		ph := namePlaceholder(i)
		if ph == "" {
			return translated, false
		}
		if strings.Contains(out, ph) {
			out = strings.ReplaceAll(out, ph, korean)
			continue
		}
		// 대문자화만 일어났을 수 있다 — `$qpzaa` 같은 변형도 받아 준다.
		if idx := indexFold(out, ph); idx >= 0 {
			out = out[:idx] + korean + out[idx+len(ph):]
			continue
		}
		// 공백이 끼어들었을 수 있다 — `$QPZ AA`.
		if start, end, ok := indexSpaced(out, ph); ok {
			out = out[:start] + korean + out[end:]
			continue
		}
		/*
			**달러 기호만 떨어져 나갔을 수 있다** — `$QPZAA` → `QPZAA` (2026-09-15
			실측: "Fantastic News for Broadcom Stock Investors"가 「QPZAA 주식
			투자자에게 환상적인 소식」으로 왔다. 20건 중 1건).

			여기까지 느슨하게 가도 되는 이유는 접두사 `QPZ`가 영어 본문·티커
			어디에도 안 나오도록 고른 세 글자이기 때문이다 — 남은 다섯 글자가
			그대로면 어느 자리였는지 모호하지 않다. 공백까지 허용하는 위 규칙과
			달리 여기서는 **글자가 붙어 있어야** 한다.
		*/
		if bare := strings.TrimPrefix(ph, "$"); bare != ph {
			if idx := indexFold(out, bare); idx >= 0 {
				out = out[:idx] + korean + out[idx+len(bare):]
				continue
			}
		}
		return translated, false
	}
	return out, true
}

/*
indexSpaced finds `sub` in `s` allowing whitespace between its characters.

번역기가 토큰을 쪼개 놓았을 때를 위한 것이다(`$QPZ AA`). 대소문자는 무시하고,
**건너뛸 수 있는 것은 공백뿐**이다 — 아무 글자나 건너뛰게 하면 서로 다른 두
자리표시자가 같은 자리에 맞아 회사가 뒤바뀐다.

돌려주는 것은 원문에서의 시작·끝 바이트 위치다(찾은 구간을 통째로 이름으로 바꾼다).
*/
func indexSpaced(s, sub string) (int, int, bool) {
	if sub == "" {
		return 0, 0, false
	}
	want := []rune(strings.ToLower(sub))
	runes := []rune(s)
	lower := []rune(strings.ToLower(s))
	// 각 룬의 시작 바이트 위치. 끝 위치를 바이트로 돌려주기 위해 미리 센다.
	offsets := make([]int, len(runes)+1)
	pos := 0
	for i, r := range runes {
		offsets[i] = pos
		pos += len(string(r))
	}
	offsets[len(runes)] = pos

	for start := 0; start < len(runes); start++ {
		if lower[start] != want[0] {
			continue
		}
		matched, i := 1, start+1
		for i < len(runes) && matched < len(want) {
			switch {
			case unicode.IsSpace(runes[i]):
				i++
			case lower[i] == want[matched]:
				matched++
				i++
			default:
				matched = -1
			}
			if matched < 0 {
				break
			}
		}
		if matched == len(want) {
			return offsets[start], offsets[i], true
		}
	}
	return 0, 0, false
}

// replaceWordFold replaces the first case-insensitive whole-word occurrence.
//
// 경계를 보는 이유는 이름 인덱스와 같다 — "Live"가 "Delivery"의 가운데에 걸리면
// 회사 이름이 아니라 낱말을 지우게 된다.
func replaceWordFold(text, word, with string) (string, bool) {
	if word == "" {
		return text, false
	}
	lowerText := strings.ToLower(text)
	lowerWord := strings.ToLower(word)
	offset := 0
	for {
		i := strings.Index(lowerText[offset:], lowerWord)
		if i < 0 {
			return text, false
		}
		start := offset + i
		end := start + len(lowerWord)
		if boundaryOK(text, start, end) {
			return text[:start] + with + text[end:], true
		}
		offset = start + 1
		if offset >= len(lowerText) {
			return text, false
		}
	}
}

func boundaryOK(text string, start, end int) bool {
	if start > 0 {
		r := rune(text[start-1])
		if isWordRune(r) {
			return false
		}
	}
	if end < len(text) {
		r := rune(text[end])
		// 소유격 's는 이름의 일부가 아니다 — "Lilly's"의 Lilly를 잡아야 한다.
		if isWordRune(r) && r != '\'' {
			return false
		}
	}
	return true
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func indexFold(s, sub string) int {
	return strings.Index(strings.ToLower(s), strings.ToLower(sub))
}

/*
findDomains는 본문에 그대로 두어야 할 도메인을 찾는다.

점이 있고 공백이 없으며 알려진 최상위 도메인으로 끝나는 토큰만 본다. "vs."나
"Inc."처럼 문장부호로 끝나는 낱말을 도메인으로 잘못 잡으면 그 자리가 통째로
안 옮겨진다.
*/
func findDomains(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, tok := range strings.FieldsFunc(text, unicode.IsSpace) {
		tok = strings.Trim(tok, "(),;:\"'")
		if !strings.Contains(tok, ".") || len(tok) < 5 {
			continue
		}
		lower := strings.ToLower(tok)
		for _, tld := range []string{".com", ".net", ".org", ".io", ".co.kr", ".co.uk", ".gov"} {
			if strings.HasSuffix(lower, tld) && !seen[tok] {
				seen[tok] = true
				out = append(out, tok)
				break
			}
		}
	}
	return out
}
