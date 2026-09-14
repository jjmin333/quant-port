package service

import "testing"

/*
세 틱이 **모두** 세션 필터를 쓴다 (2026-09-12 실측).

`tickFast`와 `tickSlow`는 각각 「마감 중에도 계속 받고 있었다」를 실측하고 필터를
붙였는데 `tickPrice`만 빠져 있었다. 실측(토요일 · 양 시장 마감 · 인바운드 0건):
`/prices` 분당 29회가 나갔고 스택 추적으로 전부 이 경로였다.

같은 누락이 다시 생기지 않게, **소스에 세 호출이 다 있는지**를 시험이 센다.
런타임 판정은 거래소 달력이 필요해 단위 시험에서 재현할 수 없으므로, 여기서는
「필터를 부르는 자리가 셋」이라는 구조적 사실을 잠근다.
*/
func TestAllPollerTicksFilterByLiveSession(t *testing.T) {
	src := readServiceSource(t, "market_poller.go")
	const call = "filterLiveSessionSymbols"

	if n := countOccurrences(src, call); n < 3 {
		t.Fatalf(
			"%s 호출이 %d곳뿐이다 — tickPrice·tickFast·tickSlow 셋 다 써야 한다",
			call, n)
	}

	// tickPrice 본문 안에 있는지 따로 본다 — 다른 두 곳에만 있으면 위 개수는 통과한다.
	body, ok := funcBody(src, "func (p *marketPoller) tickPrice(")
	if !ok {
		t.Fatal("tickPrice 를 찾지 못했다")
	}
	if !containsStr(body, call) {
		t.Fatal("tickPrice 가 세션 필터를 쓰지 않는다 — 마감 중에도 가격을 받는다")
	}
}
