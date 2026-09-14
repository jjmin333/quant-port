// Package translate turns English news headlines into Korean.
//
// **왜 패키지로 뺐나.** 예전에는 `NewsStore`가 `st.papago`를 직접 들고 있었다.
// 그러면 구현을 바꾸려면 호출부를 고쳐야 하고, 무엇보다 **두 엔진을 나란히 돌려
// 비교할 수가 없다** — 자체 모델로 갈아타기 전에 반드시 해야 하는 일이 그것인데.
//
// 지금은 호출부가 `Translator` 하나만 본다. 어느 엔진을 어떤 순서로 쓸지는 설정이
// 정하고(`TRANSLATE_PROVIDER`), 되돌리기는 그 값 하나다.
//
// 배경과 모델 선정 근거: be/docs/TRANSLATION_SELFHOST.md
package translate

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Translator turns English text into Korean.
//
// **한 건이 아니라 묶음을 받는다.** 제목 백필은 2분마다 24건을 처리하는데, 한 건씩
// 부르면 지역 모델의 프롬프트 처리를 24번 되풀이한다. 묶음으로 받으면 구현이
// 알아서 슬롯을 나눠 쓴다.
//
// 반환 슬라이스는 **입력과 같은 길이**여야 하고, 옮기지 못한 자리는 빈 문자열이다.
// 길이가 다르면 호출부가 제목을 엉뚱한 기사에 붙인다.
type Translator interface {
	// Name은 로그·메트릭에 찍히는 이름이다(지금은 "local" 하나).
	Name() string
	// Available은 이 엔진이 지금 쓸 수 있는지. 설정이 없으면 false다.
	Available() bool
	TranslateENtoKO(ctx context.Context, texts []string) ([]string, error)
}

/*
Accept — 번역을 **받아들이기 전** 검사.

기계 번역을 그대로 믿으면 안 된다는 증거는 이미 코드에 있었다. `fixUSStockTranslation`은
Papago가 "stock"을 「재고」로 옮겨서 생긴 함수다. 모델을 바꾼다고 이 문제가 사라지지
않는다 — 종류만 달라진다. 실측(LMT-60-0.6B): "Nvidia stock slips…"가 「에버넷의 주가가
하락」이 됐다. 없는 회사 이름을 지어낸 것이다.

그래서 세 가지를 본다. 하나라도 걸리면 이 번역을 버리고 다음 엔진으로 내려간다.

  - **한글이 없다** — 번역이 아예 안 됐거나 원문을 그대로 뱉었다.
  - **길이가 이상하다** — 한국어는 같은 뜻을 영어보다 짧게 쓴다(실측 0.5~0.9배).
    2.5배를 넘으면 모델이 말을 지어낸 것이고, 0.25배 미만이면 문장이 잘린 것이다.
  - **같은 말을 되풀이한다** — 소형 모델의 전형적인 실패다. 한 어절이 네 번 넘게
    나오면 루프에 빠진 것으로 본다.

여기서 막힌 건수는 부르는 쪽이 세어 둔다. **그 수가 올라가면 모델이 나빠진 것이고,
모르면 화면에 이상한 제목이 조용히 쌓인다.**
*/
func Accept(source, translated string) bool {
	translated = strings.TrimSpace(translated)
	if translated == "" {
		return false
	}
	if !containsHangul(translated) {
		return false
	}
	src := utf8.RuneCountInString(strings.TrimSpace(source))
	got := utf8.RuneCountInString(translated)
	if src > 0 {
		if got > src*5/2 || got*4 < src {
			return false
		}
	}
	return !hasRunawayRepeat(translated)
}

func containsHangul(s string) bool {
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7A3 {
			return true
		}
	}
	return false
}

// hasRunawayRepeat reports whether one word occupies the output four times over.
//
// 두 번, 세 번은 정상적인 문장에도 있다("그리고 … 그리고"). 네 번부터는 사람이 쓴
// 제목에서 사실상 나오지 않는다.
func hasRunawayRepeat(s string) bool {
	counts := map[string]int{}
	for _, w := range strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	}) {
		if utf8.RuneCountInString(w) < 2 {
			continue
		}
		counts[w]++
		if counts[w] >= 4 {
			return true
		}
	}
	return false
}

// NeedsTranslation reports whether text looks like untranslated English.
//
// 이미 한글이 섞여 있으면 옮길 것이 없다. 라틴 문자가 하나도 없으면 영어가 아니다.
func NeedsTranslation(text string) bool {
	if containsHangul(text) {
		return false
	}
	for _, r := range text {
		if unicode.IsLetter(r) && r <= unicode.MaxASCII {
			return true
		}
	}
	return false
}
