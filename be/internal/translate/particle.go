package translate

import (
	"strings"
	"unicode/utf8"
)

/*
낱말을 바꾸면 **뒤에 붙은 조사도 같이 바뀌어야 한다.**

「요금이」에서 요금만 금리로 갈면 「금리이」가 된다. 한국어의 이/가·은/는·을/를은
앞 음절에 받침이 있느냐로 갈리는데, 요금은 ㅁ 받침이 있고 금리는 없다.

이걸 표로 해결하려고 하면 낱말 하나마다 「요금이→금리가」·「요금은→금리는」·
「요금을→금리를」…을 다 적어야 하고, 그러면 용어 하나 늘릴 때마다 다섯 줄이 는다.
**한 번 제대로 짜 두면 앞으로 추가하는 용어는 낱말만 적으면 된다.**

으로/로는 예외가 하나 더 있다 — ㄹ 받침은 받침이 있는데도 「로」를 쓴다(「서울로」).
*/

// hasFinalConsonant reports whether the last Hangul syllable carries a 받침.
func hasFinalConsonant(s string) (has bool, isRieul bool, ok bool) {
	r, size := utf8.DecodeLastRuneInString(s)
	if size == 0 || r < 0xAC00 || r > 0xD7A3 {
		return false, false, false
	}
	final := (r - 0xAC00) % 28
	// 8 == ㄹ
	return final != 0, final == 8, true
}

// particleForms lists the pairs that depend on the preceding 받침:
// [받침 있음, 받침 없음].
var particleForms = [][2]string{
	{"이", "가"},
	{"은", "는"},
	{"을", "를"},
	{"과", "와"},
	{"으로", "로"},
	{"이나", "나"},
	{"이라", "라"},
}

/*
replaceNoun swaps `from` for `to` and fixes the particle that follows.

조사가 아닌 글자가 뒤에 오면 그냥 두고 낱말만 바꾼다 — 「요금 인상」의 「인」을
조사로 오해하면 안 되므로, 조사 뒤가 낱말 경계(공백·문장부호·끝)일 때만 고친다.
*/
func replaceNoun(text, from, to string) string {
	if from == "" || to == "" || !strings.Contains(text, from) {
		return text
	}
	has, rieul, ok := hasFinalConsonant(to)
	var out strings.Builder
	rest := text
	for {
		i := strings.Index(rest, from)
		if i < 0 {
			out.WriteString(rest)
			break
		}
		out.WriteString(rest[:i])
		out.WriteString(to)
		rest = rest[i+len(from):]

		if !ok {
			continue
		}
		// 바로 뒤에 조사가 오면 새 낱말에 맞는 형태로 바꾼다.
		for _, form := range particleForms {
			withFinal, withoutFinal := form[0], form[1]
			var matched, want string
			switch {
			case strings.HasPrefix(rest, withFinal):
				matched = withFinal
			case strings.HasPrefix(rest, withoutFinal):
				matched = withoutFinal
			default:
				continue
			}
			// 조사 뒤가 낱말 경계여야 조사다.
			if !boundaryAfterParticle(rest[len(matched):]) {
				continue
			}
			if has && !(rieul && withoutFinal == "로") {
				want = withFinal
			} else {
				want = withoutFinal
			}
			out.WriteString(want)
			rest = rest[len(matched):]
			break
		}
	}
	return out.String()
}

func boundaryAfterParticle(rest string) bool {
	if rest == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(rest)
	// 한글이 이어지면 조사가 아니라 낱말의 일부다 (「요금인상」).
	return !(r >= 0xAC00 && r <= 0xD7A3)
}
