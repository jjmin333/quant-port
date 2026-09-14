package service

import (
	"strings"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/repository"
)

/*
상장명이 영문인 종목의 **한글 읽기**를 이름 인덱스에 얹는다.

# 무엇이 잘못돼 있었나

「네이버, 공격·방어 '국가 보안 AI' 만든다」 기사에 NAVER(035420)가 안 붙었다 — 제목
첫 단어가 종목명인데도. 이름 인덱스는 종목 마스터의 `name`으로만 만드는데 NAVER의
`name`은 「NAVER」라서, 한글 「네이버」는 인덱스 어디에도 없었다. 같은 이유로 KT·SK·
LG·CJ·HMM… 이 전부 한글 표기로는 영원히 태깅되지 않았다.

검색 쪽은 이미 이 문제를 표로 풀어 뒀다(`repository.HangulNameAliases`). 표를 두 벌
두면 한쪽만 고쳐지므로 **그 표를 그대로 쓴다.**

# 왜 이렇게 좁게 받는가

이 파일이 사는 곳은 「정밀도가 재현율보다 중요하다」는 규칙 아래다
(`news_symbol_match.go` 머리말). 검색은 후보를 넓게 뽑아 **점수로 줄 세우면** 되지만,
태깅은 붙거나 안 붙거나 둘뿐이라 틀린 종목 하나가 시세까지 달고 화면에 나간다.
그래서 세 가지를 요구한다:

 1. **읽기가 세 글자 이상.** 「엘지」·「디비」 같은 두 글자는 어차피
    `domesticNameNeedsCode`가 종목코드를 요구하므로 얹어도 얻는 게 없고, 일상어와
    부딪힐 자리만 넓힌다.
 2. **상장명과 정확히 같은 토큰.** 검색은 접두로 계열사까지 끌어오지만(「에스케이」
    하나로 SK하이닉스·SK텔레콤이 따라온다), 태깅에서 그렇게 하면 「에스케이」 한 단어에
    어느 계열사를 붙일지 정할 수 없다. 상장명이 **정확히** 그 토큰인 종목만 받는다.
 3. **결과가 하나일 것.** 토큰 여럿이 서로 다른 종목으로 풀리면 포기한다.

이 셋 덕분에 ETF 브랜드(「코덱스」→ kodex)는 저절로 빠진다 — 상장명이 정확히
「KODEX」인 종목은 없고 전부 「KODEX 200」 꼴이다.
*/

// hangulAliasMinRunes — 이보다 짧은 읽기는 얹지 않는다(위 1번).
const hangulAliasMinRunes = 3

func addHangulAliasNames(idx map[string]string) {
	if len(idx) == 0 {
		return
	}
	for reading, tokens := range repository.HangulNameAliases() {
		reading = strings.ToLower(strings.TrimSpace(reading))
		if utf8.RuneCountInString(reading) < hangulAliasMinRunes {
			continue
		}
		// 실제 상장명이 그 한글이면 그것이 진실이다 — 별칭으로 덮지 않는다.
		if _, taken := idx[reading]; taken {
			continue
		}
		// 일상어와 겹치는 이름은 종목코드가 있어야만 인정되는데(`domesticNameNeedsCode`),
		// 별칭으로 그 규칙을 우회시킬 이유가 없다.
		if _, ambiguous := ambiguousDomesticNames[reading]; ambiguous {
			continue
		}
		if sym, ok := resolveAliasSymbol(idx, tokens); ok {
			idx[reading] = sym
		}
	}
}

// resolveAliasSymbol picks the one stock whose listing name *is* the alias
// token. Zero matches or two different symbols both mean "don't guess".
func resolveAliasSymbol(idx map[string]string, tokens []string) (string, bool) {
	found := ""
	for _, token := range tokens {
		sym, ok := idx[strings.ToLower(strings.TrimSpace(token))]
		if !ok {
			continue
		}
		if found != "" && found != sym {
			return "", false
		}
		found = sym
	}
	return found, found != ""
}
