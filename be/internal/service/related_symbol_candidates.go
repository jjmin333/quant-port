package service

import "strings"

// knownRelatedETFOverrides lists leveraged/inverse ETF tickers that do not follow
// the generic prefix heuristic (e.g. SNXX for SNDK).
var knownRelatedETFOverrides = map[string][]string{
	"QQQ":  {"TQQQ", "SQQQ", "QLD", "PSQ"},
	"SNDK": {"SNXX", "SNDQ", "SNDU", "SNDG"},
}

/*
looksLikeUSTicker — 이 파일의 추측 규칙을 적용해도 되는 심볼인가.

아래 후보 생성은 **미국 레버리지·인버스 ETF의 작명 관습**에서 나온 것이다
(TSLA → TSLL·TSLQ, AMD → TAMD·SAMD). 그래서 알파벳 티커에만 의미가 있다.

국내 종목은 6자리 숫자다. 판정 기준을 「알파벳이 하나라도 있는가」로 두면
005930은 걸러지고 AAPL·BRK.B는 통과한다.
*/
func looksLikeUSTicker(sym string) bool {
	for _, r := range sym {
		if r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return false
}

/*
relatedSymbolCandidates returns ETF symbol guesses for a primary underlying ticker.

**국내 종목에는 추측을 돌리지 않는다.** 돌리면 005930에서 005Q·005U·005G·
005X·005D·005L·00XX 같은 **존재할 수 없는 코드 7개**가 나온다.

그게 그냥 헛수고로 끝나지 않았다. 호출자(`ensureRelatedCatalogFromToss`)는
「`stocks`에 없는 후보」를 Toss에서 받아 채우는데, 허구 코드는 Toss에도 없어
저장되지 않는다 — 그래서 `missing`이 **영영 줄지 않고** 국내 종목을 검색할
때마다 Toss 호출이 한 번씩, 수렴 없이 반복됐다.

비용이 큰 이유는 `/stocks/search`가 **비로그인도 부를 수 있는 공개 경로**이고,
그 호출이 `STOCK` 한도를 `tickFast`의 `prefetchStockInfo`와 나눠 쓰기 때문이다
— 검색 트래픽이 시세 갱신을 밀어낼 수 있었다.

수동 매핑(`knownRelatedETFOverrides`)은 그대로 살린다. 국내 종목에 대해 나중에
실제 관련 상품을 등록하면 추측 없이 그것만 쓰인다.
*/
func relatedSymbolCandidates(primary string) []string {
	sym := strings.ToUpper(strings.TrimSpace(primary))
	if sym == "" {
		return nil
	}

	if !looksLikeUSTicker(sym) {
		if manual := knownRelatedETFOverrides[sym]; len(manual) > 0 {
			return append([]string(nil), manual...)
		}
		return nil
	}

	seen := map[string]struct{}{sym: {}}
	out := make([]string, 0, 12)

	add := func(candidate string) {
		candidate = strings.ToUpper(strings.TrimSpace(candidate))
		if candidate == "" || candidate == sym {
			return
		}
		if len(candidate) > 6 {
			return
		}
		if _, dup := seen[candidate]; dup {
			return
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}

	for _, c := range knownRelatedETFOverrides[sym] {
		add(c)
	}

	if len(sym) == 3 {
		add("T" + sym)
		add("S" + sym)
	}

	if len(sym) >= 3 {
		prefix := sym[:3]
		for _, suffix := range []string{"Q", "U", "G", "X", "D", "L"} {
			add(prefix + suffix)
		}
	}

	if len(sym) >= 2 {
		add(sym[:2] + "XX")
	}

	return out
}
