package service

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestCapActiveTargets_boundsToLimit(t *testing.T) {
	p := &marketPoller{}
	active := map[string]struct{}{
		"AAA": {}, "BBB": {}, "CCC": {}, "DDD": {}, "EEE": {},
	}
	got := p.capActiveTargets(active, 3, &p.fastCursor)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	seen := map[string]struct{}{}
	for _, sym := range got {
		if _, dup := seen[sym]; dup {
			t.Fatalf("duplicate symbol %s in result", sym)
		}
		seen[sym] = struct{}{}
		if _, inActive := active[sym]; !inActive {
			t.Fatalf("result contains %s, not in the active set", sym)
		}
	}
}

func TestCapActiveTargets_returnsAllWhenUnderLimit(t *testing.T) {
	p := &marketPoller{}
	active := map[string]struct{}{"AAA": {}, "BBB": {}}
	got := p.capActiveTargets(active, 40, &p.fastCursor)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
}

func TestCapActiveTargets_emptyActive(t *testing.T) {
	p := &marketPoller{}
	got := p.capActiveTargets(map[string]struct{}{}, 40, &p.fastCursor)
	if len(got) != 0 {
		t.Fatalf("len=%d want 0", len(got))
	}
}

/*
동시 시청자가 상한을 넘으면 누군가는 그 틱에서 밀린다 — 상류 한도라 어쩔 수 없다.
막아야 하는 것은 **같은 사람이 계속 밀리는 것**이다.

전에는 Go 맵을 그냥 순회해 잘랐다. 맵 순회는 매번 무작위라 통계적으로만 고르고,
특정 종목이 연속 여러 틱을 굶을 수 있었다. 커서 방식이면 몇 틱 안에 전원이 정확히
한 번씩 돌아온다.
*/
func TestCapActiveTargets_roundRobinCoversEveryoneInOneCycle(t *testing.T) {
	const (
		total = 100
		limit = 40
	)
	active := make(map[string]struct{}, total)
	for i := 0; i < total; i++ {
		active[fmt.Sprintf("SYM%03d", i)] = struct{}{}
	}

	p := &marketPoller{}
	seen := map[string]int{}
	// 100종목 / 40 = 2.5틱이면 한 바퀴. 3틱이면 전원이 최소 한 번 나와야 한다.
	for tick := 0; tick < 3; tick++ {
		got := p.capActiveTargets(active, limit, &p.fastCursor)
		if len(got) != limit {
			t.Fatalf("tick %d: len=%d want %d", tick, len(got), limit)
		}
		for _, sym := range got {
			seen[sym]++
		}
	}

	if len(seen) != total {
		var missing []string
		for sym := range active {
			if seen[sym] == 0 {
				missing = append(missing, sym)
			}
		}
		t.Fatalf("3틱에 %d/%d 종목만 갱신됨 — %d개가 굶었다 (예: %v)",
			len(seen), total, len(missing), missing[:min(3, len(missing))])
	}

	// 같은 종목이 한 바퀴 안에 두 번 이상 나오면 다른 종목의 차례를 뺏은 것이다.
	for sym, n := range seen {
		if n > 2 {
			t.Fatalf("%s가 3틱에 %d번 — 순회가 편향됐다", sym, n)
		}
	}
}

// 시청자 수가 상한 아래로 내려오면 커서를 되돌려, 다음에 다시 넘칠 때
// 처음부터 공평하게 돈다.
func TestCapActiveTargets_cursorResetsWhenUnderLimit(t *testing.T) {
	p := &marketPoller{}
	big := make(map[string]struct{}, 10)
	for i := 0; i < 10; i++ {
		big[fmt.Sprintf("S%02d", i)] = struct{}{}
	}

	p.capActiveTargets(big, 4, &p.fastCursor) // 커서 전진
	if p.fastCursor == 0 {
		t.Fatal("상한을 넘겼는데 커서가 안 움직였다")
	}

	small := map[string]struct{}{"AAA": {}, "BBB": {}}
	p.capActiveTargets(small, 4, &p.fastCursor)
	if p.fastCursor != 0 {
		t.Fatalf("상한 아래인데 커서가 %d — 0으로 돌아가야 한다", p.fastCursor)
	}
}

/*
fast 틱과 slow 틱은 **커서를 공유하지 않는다.**

공유하면 1초 틱이 커서를 빠르게 밀어 30초 틱이 매번 엉뚱한 구간에서 시작하고,
두 틱의 상한이 40과 120으로 달라 보폭까지 어긋난다 — 어떤 종목은 캔들이 계속
밀리게 된다. 라운드로빈을 넣은 이유(공평한 순회) 자체가 무너진다.
*/
func TestCapActiveTargets_cursorsAreIndependent(t *testing.T) {
	p := &marketPoller{}
	active := make(map[string]struct{}, 10)
	for i := 0; i < 10; i++ {
		active[fmt.Sprintf("S%02d", i)] = struct{}{}
	}

	p.capActiveTargets(active, 4, &p.fastCursor)
	if p.slowCursor != 0 {
		t.Fatalf("fast 틱이 slowCursor를 %d로 움직였다 — 커서는 틱마다 독립이어야 한다", p.slowCursor)
	}

	p.capActiveTargets(active, 3, &p.slowCursor)
	if p.fastCursor != 4 {
		t.Fatalf("slow 틱이 fastCursor를 %d로 바꿨다 — 4로 유지돼야 한다", p.fastCursor)
	}
	if p.slowCursor != 3 {
		t.Fatalf("slowCursor가 %d — 자기 보폭(3)만큼만 움직여야 한다", p.slowCursor)
	}
}

/*
tickSlow 대상은 **지금 보고 있는 종목(active)뿐**이다.

예전 테스트는 `cared`(관심 ∪ 보유)를 구조체에 **직접 넣어** 검증했다. 그래서
`cared`가 운영에서 항상 비어 있다는 사실을 잡아내지 못했다 — 그것을 채우는
`watchlistSymbols`/`heldSymbols`가 컨텍스트의 로그인 사용자를 요구하는데 폴러는
`context.Background()` 파생으로 돌기 때문이다. **테스트는 통과하는데 운영은
다르게 동작하고 있었다.**

그래서 이제 배선을 우회하지 않고, tickSlow가 실제로 쓰는 것과 같은 경로
(`capActiveTargets(active, …)`)를 검증한다. pinned에만 있는 종목(랭킹 추적분
실측 3,652개)이 절대 섞이지 않는 것이 여기서 지켜야 할 성질이다 — 섞이면
`/candles` 한도를 상시 소진하고 사용자의 차트 첫 열람을 밀어낸다.
*/
func TestTickSlowTargets_onlyActiveNeverPinned(t *testing.T) {
	p := &marketPoller{
		pinned: []string{"RANK1", "RANK2", "RANK3", "WATCH1", "HELD1"},
	}
	active := map[string]struct{}{"VIEWING1": {}, "VIEWING2": {}}

	got := p.capActiveTargets(active, marketSlowSymbolCap, &p.slowCursor)

	if len(got) != 2 {
		t.Fatalf("대상 %v — active 2개여야 한다", got)
	}
	for _, sym := range got {
		if _, ok := active[sym]; !ok {
			t.Fatalf("%s가 대상에 들어갔다 — active가 아닌 종목은 /candles를 태운다", sym)
		}
	}
}

/*
tickSlow에는 **상한이 있어야 한다.**

없으면 동시 시청 종목이 늘어난 만큼 `/candles`를 그대로 태운다. 그 버킷은
사용자가 차트를 처음 열 때 쓰는 것과 같은 `MARKET_DATA_CHART`라, 배경 갱신이
전경 요청을 밀어낸다(실측 2026-08-23: 30분간 exceeded 343건).
*/
func TestTickSlowTargets_capped(t *testing.T) {
	p := &marketPoller{}
	active := make(map[string]struct{}, marketSlowSymbolCap*2)
	for i := 0; i < marketSlowSymbolCap*2; i++ {
		active[fmt.Sprintf("S%04d", i)] = struct{}{}
	}

	got := p.capActiveTargets(active, marketSlowSymbolCap, &p.slowCursor)

	if len(got) != marketSlowSymbolCap {
		t.Fatalf("대상 %d개 — 상한 %d로 잘려야 한다", len(got), marketSlowSymbolCap)
	}

	// 다음 틱은 이어받아야 한다 — 같은 구간만 반복하면 나머지가 영영 굶는다.
	next := p.capActiveTargets(active, marketSlowSymbolCap, &p.slowCursor)
	if next[0] == got[0] {
		t.Fatal("다음 틱이 같은 지점에서 시작했다 — 라운드로빈이 돌지 않는다")
	}
}

/*
목록 경로가 스친 종목은 **터미널 대상이 되면 안 된다.**

이 성질이 깨져서 실제 사고가 났다. 뉴스 브리핑 배치가 기사에서 언급 종목을 뽑아
`BatchQuoteSummaries`로 시세를 붙이는데, 그 함수가 종목마다 `Touch`를 불렀고
당시에는 Touch가 곧 「지금 보고 있는 종목」이었다. 결과: 인바운드 요청 0건인데
19종목의 호가·체결을 2초마다 받았다(실측 2026-08-24: `/trades` 분당 123건).

비용 차이가 이 구분의 근거다 — 가격은 200종목/호출이고 터미널은 종목당 2호출이다.
*/
func TestTouch_priceInterestDoesNotBecomeTerminalTarget(t *testing.T) {
	p := newMarketPoller(nil)
	p.Touch("GOLD") // 뉴스가 언급한 종목 (목록 수준)
	p.Touch("BOND")

	now := time.Now()
	p.mu.Lock()
	priced, terminal := len(p.active), len(p.terminal)
	p.mu.Unlock()
	_ = now

	if priced != 2 {
		t.Fatalf("가격 관심 %d개 — Touch는 가격 대상에는 넣어야 한다", priced)
	}
	if terminal != 0 {
		t.Fatalf("터미널 관심 %d개 — 목록 경로는 호가·체결 대상이 되면 안 된다", terminal)
	}
}

// 상세 화면 경로는 둘 다 표시한다 — 터미널을 보는 종목이 가격만 낡을 이유가 없다.
func TestTouchTerminal_impliesPriceInterest(t *testing.T) {
	p := newMarketPoller(nil)
	p.TouchTerminal("005930")

	p.mu.Lock()
	_, inPrice := p.active["005930"]
	_, inTerminal := p.terminal["005930"]
	p.mu.Unlock()

	if !inPrice {
		t.Fatal("TouchTerminal이 가격 관심에 안 넣었다 — 보고 있는 종목의 가격이 낡는다")
	}
	if !inTerminal {
		t.Fatal("TouchTerminal이 터미널 관심에 안 넣었다")
	}
}

// 만료는 두 집합에서 각각 일어나야 한다.
func TestPollSymbols_expiresBothSets(t *testing.T) {
	p := newMarketPoller(nil)
	stale := time.Now().Add(-marketActiveTTL - time.Minute)
	p.active["OLD"] = stale
	p.terminal["OLD"] = stale
	p.TouchTerminal("FRESH")
	p.pinnedAt = time.Now() // refreshPinned 스킵

	priced, terminal, _ := p.pollSymbols(context.Background())

	for _, sym := range priced {
		if sym == "OLD" {
			t.Fatal("만료된 종목이 가격 대상에 남았다")
		}
	}
	if _, ok := terminal["OLD"]; ok {
		t.Fatal("만료된 종목이 터미널 대상에 남았다")
	}
	if _, ok := terminal["FRESH"]; !ok {
		t.Fatal("살아 있는 터미널 관심이 빠졌다")
	}
}
