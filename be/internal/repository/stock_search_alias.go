package repository

import (
	"sort"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

/*
한글 발음 → 영문 상장명 별칭 표 — **자료는 이 파일에 없다.**

값과 그 근거는 `stock_alias_seed.go` 머리글에 있고, 운영 중에 쓰이는 최신본은
`stock_aliases` 표다(migration 070). 여기 남은 것은 **그 표를 어떻게 쓰는가**,
즉 질의를 별칭으로 넓히는 규칙과 점수뿐이다.
*/

// searchAliasCap 은 한 질의가 쓰는 별칭 수 상한이다. 「에스」처럼 짧은 접두는 여러
// 브랜드에 걸리는데, 전부 OR로 붙이면 후보 질의만 길어지고 순위는 달라지지 않는다.
const searchAliasCap = 4

/*
SearchAliasTokens 는 한글 질의에 대응하는 영문 표기를 돌려준다.

정확히 같은 읽기뿐 아니라 **입력 중인 접두**도 받는다(「네이」→ naver) — 검색창은
글자마다 질의를 보내므로, 다 쳐야 결과가 나오면 중간에 빈 화면이 보인다. 접두는
2글자부터다(1글자는 거의 모든 브랜드에 걸린다).

여러 개가 걸리면 **읽기가 짧은 것부터** `searchAliasCap`개까지 — 같은 입력에 같은
결과를 주기 위해 길이·사전순으로 확정한다.
*/
func SearchAliasTokens(q string) []string {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" || !ContainsHangul(q) {
		return nil
	}
	// 스냅샷을 한 번만 집는다 — 이 함수가 도는 중에 캐시가 교체돼도 한 질의는
	// 한 벌의 표만 본다(정확 일치와 접두 훑기가 다른 표를 보면 답이 갈린다).
	aliases := currentAliases().brand
	if hit, ok := aliases[q]; ok {
		return append([]string(nil), hit...)
	}
	if len([]rune(q)) < 2 {
		return nil
	}

	keys := make([]string, 0, 4)
	for key := range aliases {
		if strings.HasPrefix(key, q) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) < len(keys[j])
		}
		return keys[i] < keys[j]
	})

	seen := map[string]struct{}{}
	out := make([]string, 0, searchAliasCap)
	for _, key := range keys {
		for _, tok := range aliases[key] {
			if _, dup := seen[tok]; dup {
				continue
			}
			seen[tok] = struct{}{}
			out = append(out, tok)
			if len(out) >= searchAliasCap {
				return out
			}
		}
	}
	return out
}

/*
searchAliasScore 는 별칭으로 찾은 매치의 점수다.

이름 매치의 **절반**으로 눌러 둔다. 별칭은 우리가 적어 넣은 추측이고 이름은 DB에 있는
사실이라, 같은 값을 주면 「에스엠」 검색에서 SM으로 시작하는 영문 종목이 상장명이
한글인 에스엠 계열보다 위로 올라간다. 절반이면 한글 이름 부분 일치(4,500)와 같은 칸에
서고, 그 다음은 본주 우선·시가총액이 판단한다.
*/
func searchAliasScore(q string, st model.Stock) int {
	best := 0
	for _, raw := range SearchAliasTokens(q) {
		// 표에는 상장명 표기 그대로 적혀 있다(`kt&g`). 점수는 기호를 지우고 비교한다.
		alias := condenseLatin(raw)
		score := latinNameScore(alias, st.Name)
		if en := latinNameScore(alias, st.EnglishName); en > score {
			score = en
		}
		if score > best {
			best = score
		}
	}
	return best / 2
}

/*
SearchAliasSymbols 는 한글 질의가 **직접 가리키는 종목 심볼**을 돌려준다.

브랜드 별칭(`SearchAliasTokens`)과 달리 이쪽은 이름을 거치지 않는다 — 「구글」의
상장명은 「알파벳 A」라 어떤 LIKE로도 안 이어지기 때문이다.

접두 입력을 받는 규칙은 브랜드 쪽과 같다(검색창이 글자마다 질의를 보낸다). 다만
접두가 여러 종목에 걸릴 수 있으므로 `searchAliasCap`으로 자르고, 같은 입력에 같은
결과를 주도록 별칭 길이·사전순으로 확정한다.

# 한 별칭이 여러 종목을 가리킨다 (2026-09-14 사용자 요청 1)

「구글」에 GOOGL과 GOOG을 함께 달아 두면 **둘 다** 나와야 한다. 예전에는 정확히
일치하는 별칭에서 심볼 하나만 돌려줬는데, 그 하나는 표를 읽는 순서가 정하는
값이었다 — 사용자는 둘을 적어 두고 GOOG만 보고 있었다. 여기서 잘리면 뒤의
점수 계산이 아무리 옳아도 그 종목은 후보에조차 못 든다.
*/
func SearchAliasSymbols(q string) []string {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" || !ContainsHangul(q) {
		return nil
	}
	aliases := currentAliases().symbol
	if syms, ok := aliases[q]; ok && len(syms) > 0 {
		return capSymbols(syms)
	}
	if len([]rune(q)) < 2 {
		return nil
	}

	keys := make([]string, 0, 4)
	for key := range aliases {
		if strings.HasPrefix(key, q) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) < len(keys[j])
		}
		return keys[i] < keys[j]
	})

	seen := map[string]struct{}{}
	out := make([]string, 0, searchAliasCap)
	for _, key := range keys {
		for _, sym := range aliases[key] {
			if _, dup := seen[sym]; dup {
				continue
			}
			seen[sym] = struct{}{}
			out = append(out, sym)
			if len(out) >= searchAliasCap {
				return out
			}
		}
	}
	return out
}

// capSymbols — 중복을 걸러 `searchAliasCap`개까지. 표에 같은 대상이 두 번 적혀
// 있어도(다른 갈래로 들어온 행 등) 후보 질의가 그만큼 길어지지 않는다.
func capSymbols(syms []string) []string {
	seen := make(map[string]struct{}, len(syms))
	out := make([]string, 0, min(len(syms), searchAliasCap))
	for _, sym := range syms {
		if _, dup := seen[sym]; dup {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
		if len(out) >= searchAliasCap {
			break
		}
	}
	return out
}

/*
searchAliasSymbolScore 는 심볼 별칭으로 지목된 종목의 점수다.

이름 부분 일치(4,500)와 **같은 칸**에 세운다 — 근거는 `searchRelevanceScore`의
호출 지점 주석에 있다. 정확히 같은 별칭이면 접두로 걸린 것보다 위다.
*/
func searchAliasSymbolScore(q string, st model.Stock) int {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" || !ContainsHangul(q) {
		return 0
	}
	aliases := currentAliases().symbol
	// 별칭이 여러 종목을 가리키면 **그 전부가 정확 일치**다 (2026-09-14). 첫 칸만
	// 9,000점을 주면 「구글」 검색에서 GOOG이 GOOGL보다 네 칸 아래로 밀린다 —
	// 대표를 고르는 일은 태깅의 몫이지 검색 순위의 몫이 아니다.
	for _, sym := range aliases[q] {
		if strings.EqualFold(sym, st.Symbol) {
			return 9000
		}
	}
	for _, sym := range SearchAliasSymbols(q) {
		if strings.EqualFold(sym, st.Symbol) {
			return 4500
		}
	}
	return 0
}
