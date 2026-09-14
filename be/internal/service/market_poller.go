package service

import (
	"context"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/enact/quant-port/be/internal/metrics"
	"github.com/enact/quant-port/be/internal/safego"
)

const (
	/*
		marketFastPollInterval — 터미널 스냅샷(호가·체결) 갱신 주기.

		**1초에서 2초로 낮췄다(2026-08-24).** 화면이 그보다 빨리 읽지 않기 때문이다 —
		종목 상세는 `use-quote-detail.ts`가 2초, 형성봉은 `use-quote-candles.ts`가
		2초 주기로 읽는다. 1초로 쏘면 두 번 중 한 번은 아무도 안 읽고 버려졌다.

		그런데 버려지기만 한 게 아니라 **다른 갱신을 실패시켰다.** 이 틱은 종목당
		`/orderbook` + `/trades` 2호출이고 둘 다 `MARKET_DATA` **한 버킷**을
		`/prices`와 공유한다(Toss OpenAPI 스펙). 상한 40종목이면 초당 80호출이라
		한도를 크게 넘겨, 실측(2026-08-24 미국장 중) `rate-limit exceeded`가
		`/orderbook` 25 + `/trades` 22 = **분당 약 47건**이었다.

		즉 1초 주기는 신선도를 사는 게 아니라 **429를 사고 있었다.** 주기를 2초로
		두면 호출이 절반이 되어 남은 호출의 성공률이 오르므로, **실효 갱신 간격은
		오히려 좋아진다.** 화면 기준으로는 변화가 없다.

		근본 해결은 웹소켓 구독이다(체결·호가를 푸시로 받으면 이 틱의 REST 호출이
		0이 된다). 이 상수는 그때까지의 완화책이다.
	*/
	marketFastPollInterval = 2 * time.Second
	marketSlowPollInterval = 30 * time.Second
	/*
		marketPricePollInterval — 배치 가격 갱신 주기. 터미널 갱신과 독립이다(tickPrice 헤더).

		이 값은 화면이 볼 수 있는 가격의 **신선도 하한**을 정한다 — 서버가 새 값을
		만드는 주기보다 화면이 자주 물어 봐야 같은 답을 다시 받을 뿐이다. 그래서 이
		상수와 화면의 박자(`fe/src/lib/price-cadence.ts`)는 언제나 같이 움직인다.

		# 2초 → **1초** (2026-09-04 사용자 요청 5)

		09-04 오전에 흩어져 있던 화면별 주기(1·2·5초·없음)를 2초로 모았고, 오후에
		사용자가 실시간성을 위해 1초를 요청했다. **되돌리는 게 아니라 조건이 달라졌다** —
		그 사이에 가격이 폴링에서 SSE로 옮겨 갔기 때문이다.

		2초를 고를 때의 근거는 「1초로 만든 값의 절반을 아무도 안 읽는다」였다. 폴링
		시절에는 그게 맞았다(화면이 2초마다 물었다). 지금은 값을 만드는 즉시
		`cacheMarketPrice`가 구독자 전원에게 밀어내므로, 만든 값은 **전부 읽힌다.**

		# 상류가 견디는가

		`/prices`는 벌크다 — 200종목/호출. 추적 종목이 수백 개라 한 틱이 2~3호출이고,
		1초 주기면 초당 2~3호출이다. `MARKET_DATA` 버킷 한도는 15/s.

		같은 버킷을 쓰는 `tickFast`(종목당 `/orderbook`+`/trades`)가 그 한도의 대부분을
		쓰지만, 그쪽은 `marketFastSymbolCap`(40)·`marketRefreshConcurrency`(4)·
		`fastBusy` 스킵으로 스스로 눌려 있다. 여기서 더하는 것은 **초당 1~1.5호출**,
		한도의 10% 미만이다.

		# 상류가 그만큼 새 값을 주는가

		실측(`market:price:v1`의 updatedAt 간격, 0.2초 표본): 일반 종목 median 999ms.
		즉 상류 자체가 1초 박자라 1초 폴링이 정확히 그 박자와 맞는다. 2초는 그 절반을
		**건너뛰고 있었다.**

		# 이벤트가 두 배가 되지 않는가

		안 된다. 같은 날 `publishListPrice`에 「값이 그대로면 안 보낸다」가 들어갔다
		(사용자 지적 3). 틱은 두 배가 되지만 **나가는 프레임은 값이 실제로 움직인
		횟수**이고, 그 수는 주기와 무관하다 — 오히려 2초 틱이 두 번의 변화를 하나로
		합치던 것이 풀려서 화면이 보는 움직임이 더 정확해진다.
	*/
	marketPricePollInterval = 1 * time.Second
	marketActiveTTL         = 5 * time.Minute

	/*
		marketBookTTL — 호가 관심이 살아 있는 시간. **터미널 TTL보다 훨씬 짧다.**

		화면은 호가창을 보고 있는 동안 2초마다 스냅샷을 다시 받으므로(`use-quote-detail`)
		실제로 보고 있으면 이 표시가 계속 갱신된다. 반대로 탭을 옮기면 갱신이 끊기고,
		그때 5분(marketActiveTTL)이나 붙잡고 있을 이유가 없다 — 그 5분 동안 아무도 안 보는
		호가 프레임이 계속 들어온다(실측: 호가가 체결의 4.3배).

		30초는 화면 주기(2초)의 15배라 일시적인 요청 실패나 탭 전환 왕복에는 끄떡없고,
		진짜로 떠났을 때는 빠르게 정리된다.
	*/
	marketBookTTL = 30 * time.Second
	marketPinnedRefresh     = 1 * time.Minute

	// marketRefreshConcurrency bounds in-flight upstream refreshes per tick so a
	// tick completes inside its interval without bursting the Toss rate limit.
	marketRefreshConcurrency = 4

	// marketFastSymbolCap bounds how many symbols a single fast tick may refresh.
	marketFastSymbolCap = 40

	/*
		marketSlowSymbolCap — 한 slow 틱이 캔들을 다시 받아 올 종목 수의 상한.

		tickSlow에는 상한이 **없었다.** 동시 시청 종목이 늘면 그만큼 `/candles`를
		그대로 태웠고, 그 버킷(MARKET_DATA_CHART)은 **사용자가 차트를 처음 열 때
		쓰는 것과 같은 버킷**이다 — 배경 갱신이 전경 요청을 밀어냈다
		(실측 2026-08-23: 30분간 rate-limit exceeded 343건).

		120인 근거: 30초 주기에 종목당 1분봉 1호출이므로 120/30 = **초당 4호출**.
		일봉은 dailyRefetchInterval(5분)로 묶여 있어 평균 0.4호출/s가 더해진다.
		합계 약 4.4/s로 `/candles` 한도(20/s)의 22%다 — 나머지는 사용자의 차트
		요청 몫으로 남는다.

		상한을 넘으면 라운드로빈으로 밀린다. 밀린 종목도 **형성봉은 계속 움직인다**
		— refreshFormingBars가 tickFast의 가격으로 따로 갱신하기 때문이다.
	*/
	marketSlowSymbolCap = 120
)

type marketPoller struct {
	quotes *QuotesService

	mu sync.Mutex
	// active — 가격 관심(목록 화면 수준). tickPrice의 대상.
	active map[string]time.Time
	// terminal — 터미널 관심(상세 화면). tickFast·tickSlow의 대상.
	// active의 부분집합이다 (Touch 헤더 참조).
	terminal map[string]time.Time
	/*
		book — **호가창을 실제로 열어 둔** 종목.

		`terminal`과 갈라 둔 이유: 웹소켓은 종목당 체결 1 + 호가 1 = 2슬롯을 쓰는데,
		호가를 보는 사람은 상세 화면을 여는 사람보다 훨씬 적다. 그런데 예전에는
		상세 화면에 들어가기만 하면 호가까지 구독했다.

		실측(2026-09-02, 2종목 구독 중): 프레임 수가 **호가 40,667 vs 체결 9,461**로
		호가가 4.3배다. 아무도 안 보는 호가를 받아서 디코딩·버퍼링·캐시 쓰기까지
		하고 있었다는 뜻이다.

		이 집합에 든 종목만 호가를 구독한다. 나머지는 체결만 받으므로 같은 슬롯
		예산으로 **두 배**의 종목을 커버한다.
	*/
	book map[string]time.Time
	// pinned — 가격을 따뜻하게 유지할 집합.
	// = MarketDataStore.activeSymbols()(최근 5분 내 가격을 요청받은 종목).
	// RefreshPrices는 200종목/호출이라 이 크기를 감당한다.
	pinned []string
	// fastCursor / slowCursor — capActiveTargets의 라운드로빈 위치.
	//
	// **틱마다 따로 둔다.** 하나를 공유하면 tickFast(1초)가 커서를 빠르게 밀어
	// tickSlow(30초)가 매번 엉뚱한 구간에서 시작한다 — 두 틱의 상한이 40과 120으로
	// 다르기 때문에 보폭도 다르고, 결과적으로 어떤 종목은 캔들이 계속 밀린다.
	fastCursor int
	slowCursor int
	// sizesLoggedAt — logSizes의 분당 1회 제한.
	sizesLoggedAt time.Time
	pinnedAt      time.Time

	fastBusy  atomic.Bool
	priceBusy atomic.Bool
	slowBusy  atomic.Bool
}

/*
logSizes — 폴러가 매 틱 무엇을 대상으로 삼는지 분당 한 줄로 남긴다.

**이게 없으면 「사용자가 없는데 왜 상류 호출이 나는가」에 답할 수 없다.** 지금까지는
rate-limit 로그로 결과만 보였지 그 원인이 pinned인지 active인지 구분할 수 없었다:

	priced   — tickPrice 대상(가격). 보유 ∪ 관심 ∪ 최근 조회분. 배치 200종목/호출이라 싸다.
	terminal — tickFast·tickSlow 대상(호가·체결·캔들). 종목당 2호출이라 비싸다.
	           **상세 화면을 연 사람이 없으면 0이어야 한다.**
	targets  — 상한(marketFastSymbolCap)을 적용한 뒤 실제로 갱신할 수.

terminal이 0이 아닌데 인바운드 요청이 0이면, 그것을 채우는 배경 경로가 있다는 뜻이고
그 경로가 곧 유휴 시 상류 호출의 원인이다 — 실제로 그렇게 뉴스 브리핑을 찾아냈다.
*/
func (p *marketPoller) logSizes(pinned, active, targets int, sample []string) {
	p.mu.Lock()
	due := time.Since(p.sizesLoggedAt) >= time.Minute
	if due {
		p.sizesLoggedAt = time.Now()
	}
	p.mu.Unlock()
	if !due {
		return
	}
	metrics.SetPollerSizes(pinned, active, targets)
	log.Printf("market poller sizes: priced=%d terminal=%d fastTargets=%d terminalSyms=%v", pinned, active, targets, sample)
}

func newMarketPoller(quotes *QuotesService) *marketPoller {
	return &marketPoller{
		quotes:   quotes,
		active:   make(map[string]time.Time),
		terminal: make(map[string]time.Time),
		book:     make(map[string]time.Time),
	}
}

/*
Touch — 이 종목의 **가격**이 따뜻해야 한다고 표시한다(목록 화면 수준).

`TouchTerminal`과 갈라 둔 이유가 이 파일에서 가장 중요한 구분이다.

두 관심사는 비용이 **10배 이상** 다르다:

	가격   — /prices 배치 200종목당 1호출. 목록 화면이 읽는 것.
	터미널 — 종목당 /orderbook + /trades 2호출. 상세 화면의 호가창·체결이 읽는 것.

전에는 둘이 같은 `active` 맵을 썼다. 그래서 **목록을 만들려고 종목을 스친 것**만으로
그 종목이 「지금 보고 있는 종목」이 되어 2초마다 호가·체결을 받았다.

실측(2026-08-24, 인바운드 0건 · 사용자 0명):

	market poller sizes: pinned=221 active=19 fastTargets=19
	activeSyms=[005930 000660 … BOND CRWD FIVE GOLD HERE HONG NVDA OPEN TGT TSLA]

아무도 접속하지 않았는데 19종목의 터미널을 갱신하고 있었다 — `/trades` 분당 123건,
`/orderbook` 53건. 범인은 **뉴스 브리핑 배치**였다. 기사에서 언급 종목을 뽑아
`BatchQuoteSummaries`로 시세를 붙이는데, 그 함수가 종목마다 이 `Touch`를 불렀다.

**태그 품질 문제가 아니다.** GOLD·BOND·HONG 같은 오탐이 섞여 있긴 했지만, 태깅이
완벽했어도 「기사가 언급한 종목」이 「사용자가 보고 있는 종목」으로 승격되는 구조는
그대로였다. 그리고 그 비용은 사용자 수가 아니라 **기사 수와 태그 다양성**에
비례해서 늘어난다 — 아무도 안 쓰는 새벽에 가장 많이 늘어난다는 뜻이다.
*/
func (p *marketPoller) Touch(symbol string) {
	if symbol == "" {
		return
	}
	p.mu.Lock()
	p.active[strings.ToUpper(symbol)] = time.Now()
	p.mu.Unlock()
}

/*
TouchTerminal — 이 종목을 **지금 상세 화면에서 보고 있다**고 표시한다.

호가창·체결·차트를 실제로 읽는 경로만 부른다. 가격 관심도 당연히 포함하므로
`active`에도 함께 넣는다 — 터미널을 보는 종목이 가격만 낡을 이유는 없다.

여기 들어온 종목만 `tickFast`(호가·체결)와 `tickSlow`(캔들)의 대상이 된다.
*/
func (p *marketPoller) TouchTerminal(symbol string) {
	if symbol == "" {
		return
	}
	sym := strings.ToUpper(symbol)
	now := time.Now()
	p.mu.Lock()
	p.active[sym] = now
	p.terminal[sym] = now
	p.mu.Unlock()
}

/*
TouchOrderBook — 이 종목의 **호가창을 지금 보고 있다**고 표시한다.

화면이 호가를 실제로 그릴 때만 불린다(터미널 스냅샷 요청의 `book=1`). 상세 화면에
들어간 것만으로는 불리지 않는다 — 그 구분이 곧 웹소켓 슬롯 절반을 돌려받는 근거다
(`marketPoller.book` 주석의 프레임 실측 참조).

호가를 보는 것은 상세 화면을 보는 것을 포함하므로 terminal·active에도 함께 넣는다.
*/
func (p *marketPoller) TouchOrderBook(symbol string) {
	if symbol == "" {
		return
	}
	sym := strings.ToUpper(symbol)
	now := time.Now()
	p.mu.Lock()
	p.active[sym] = now
	p.terminal[sym] = now
	p.book[sym] = now
	p.mu.Unlock()
}

func (p *marketPoller) Run(ctx context.Context) {
	fast := time.NewTicker(marketFastPollInterval)
	slow := time.NewTicker(marketSlowPollInterval)
	price := time.NewTicker(marketPricePollInterval)
	defer fast.Stop()
	defer slow.Stop()
	defer price.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-price.C:
			p.tickPrice(ctx)
		case <-fast.C:
			p.tickFast(ctx)
		case <-slow.C:
			p.tickSlow(ctx)
		}
	}
}

func (p *marketPoller) refreshPinned(ctx context.Context) {
	p.mu.Lock()
	fresh := time.Since(p.pinnedAt) < marketPinnedRefresh
	p.mu.Unlock()
	if fresh {
		return
	}

	seen := map[string]struct{}{}
	var out []string
	add := func(sym string) {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			return
		}
		if _, ok := seen[sym]; ok {
			return
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}

	if p.quotes.marketStore != nil {
		for _, sym := range p.quotes.marketStore.WarmPriceSymbols() {
			add(sym)
		}
	}
	/*
		**관심·보유는 여기 넣지 않는다 — 아무도 안 보는 종목이다.**

		예전에는 전 사용자의 관심·보유를 pinned에 넣어 1초마다 데웠다. 그런데 그 값을
		쓰는 곳은 **사용자가 그 화면을 열었을 때뿐**이고(`AllTrackedSymbols`의 소비자는
		이 함수 하나다), 화면은 열리는 순간 어차피 새로 받아 1초 주기로 갱신한다.
		즉 아무도 안 보는 동안의 갱신이 사는 것은 **첫 조회 한 번의 지연**뿐이었다.

		비용은 그 반대로 컸다: 실측(2026-09-02) 사용자 4,652명의 관심·보유 distinct
		종목이 **209개**로, pinned 305개 중 대부분이 이것이었다. 사용자가 늘면 이 축이
		그대로 상류 호출로 이어진다.

		콜드 오픈 비용은 `RefreshPrices`의 단일 비행이 받아 준다 — 동시에 몇 명이 열든
		상류 호출은 1회다(그 함수 헤더의 실측: 동시 30명 → 2회).

		남는 것은 `activeSymbols()`, 즉 **최근에 실제로 조회된 종목**이다.
	*/

	p.mu.Lock()
	p.pinned = out
	p.pinnedAt = time.Now()
	p.mu.Unlock()
}

/*
tickPrice — 배치 가격 갱신. **tickFast에서 떼어 낸 이유가 있다.**

전에는 tickFast가 한 함수 안에서 ① 배치 가격 ② 터미널 갱신(40종목 × 3호출)을
차례로 했다. `/orderbook`·`/trades`가 각 15/s라 ②만으로 최소 8초가 걸리는데,
`fastBusy`가 그 동안 다음 틱을 통째로 스킵한다 — 그래서 **①도 8초에 한 번씩만**
불렸다. 1초 티커를 달아 놓고 실제로는 8초 주기로 돌던 셈이다.

가격은 200종목/호출이고 `/prices` 한도가 15/s라 이론상 3,000종목/초다. 여유가
있는데 못 쓰고 있었다(실측 60초 커버리지 146종목).

자기 busy 플래그를 따로 두어 터미널 갱신이 길어져도 가격은 제 주기를 지킨다.
*/
func (p *marketPoller) tickPrice(ctx context.Context) {
	if !p.priceBusy.CompareAndSwap(false, true) {
		return
	}
	defer p.priceBusy.Store(false)

	if p.quotes.marketStore == nil {
		return
	}
	all, _, _ := p.pollSymbols(ctx)
	if len(all) == 0 {
		return
	}

	/*
		**장이 닫힌 종목은 가격이 바뀌지 않는다** (2026-09-12 실측).

		`tickFast`와 `tickSlow`는 이미 이 필터를 쓴다 — 둘 다 「마감 중에도 계속
		받고 있었다」를 실측으로 확인하고 붙인 것이다(그 두 함수의 머리글). **이 틱만
		빠져 있었다.**

		실측(2026-09-12 00:37~00:43, 토요일 · 양 시장 마감 · 인바운드 요청 0건):
		`/prices` 146호출 / 5분 = **분당 29회**. 스택 추적으로 전부 이 경로였다
		(`marketPoller.Run → tickPrice → RefreshPrices`). 아무도 안 보고 시장도 닫힌
		시간에 분당 29회가 `MARKET_DATA` 버킷(관측 15/s)을 쓰고 있었고, 그 버킷은
		사용자의 호가·체결과 같은 창이다.

		마감 중 가격이 필요한 곳은 화면인데, 그 값은 **이미 캐시와 DB에 있다**
		(`symbol_last_prices` — AGENT_STATE 「Quotes 데이터 정책」). 요청 경로의
		`EnsurePrices`가 캐시가 비면 그때 한 번 받으므로, 배경에서 1초마다 미리
		받아 둘 이유가 없다 — 받아 봐야 이미 가진 것과 같은 값이다.
	*/
	live := p.quotes.filterLiveSessionSymbols(ctx, all)
	if len(live) == 0 {
		return
	}
	p.quotes.marketStore.RefreshPrices(ctx, live)
}

/*
tickFast — **지금 보고 있는 종목**의 터미널 스냅샷(호가·체결)과 형성봉을 갱신한다.

가격은 tickPrice가 따로 맡는다. 여기 남은 것은 종목당 Toss 3호출이 드는 작업이고,
`/orderbook`·`/trades`가 각 15/s라 상한이 필요하다.

관심·보유 종목은 이 틱의 대상이 **아니다** — 목록 화면은 배치 가격을 읽지 per-symbol
터미널 스냅샷을 읽지 않는다. `Touch`가 승격한 종목만 들어온다.
*/
func (p *marketPoller) tickFast(ctx context.Context) {
	if !p.fastBusy.CompareAndSwap(false, true) {
		// Previous fast tick is still running — skip instead of piling up.
		return
	}
	defer p.fastBusy.Store(false)

	// 가격은 tickPrice가 맡는다 — 여기서 부르면 터미널 갱신에 묶여 주기를 잃는다.
	priced, terminal, book := p.pollSymbols(ctx)

	targets := p.capActiveTargets(terminal, marketFastSymbolCap, &p.fastCursor)
	p.logSizes(len(priced), len(terminal), len(targets), targets)

	// 구독 대상은 상한을 적용하기 **전**의 집합이다 — 슬롯이 폴링 상한(40)보다
	// 훨씬 크므로(200) 여기서 미리 자를 이유가 없다.
	p.quotes.PublishStreamTargets(keysOf(terminal), keysOf(book))

	// 장이 닫힌 종목은 체결·호가가 더 이상 바뀌지 않는다. 그런데도 이 틱은
	// 1초마다 대상 전부의 터미널 스냅샷을 다시 받아 왔다 — 대상 40개면
	// 체결 40 + 호가 40 = 초당 80 호출이고, Toss의 해당 한도는 각 15/s다.
	//
	// 실측(사용자 요청 없음, 국내장 마감): `rate-limit exceeded`가 분당 약 600건,
	// 분포는 /trades 145 · /orderbook 139. 그 잡음이 실제 시세 갱신과 같은
	// 한도를 다투고 있었다.
	//
	// 마감 종목을 빼도 화면은 그대로다 — 마감 중 상세는 이미 DB의 last-good을
	// 읽는다(loadClosedMarketQuoteDetail). 가격과 캔들은 아래 slow 틱과 배치
	// RefreshPrices가 계속 갱신한다.
	targets = p.quotes.filterLiveSessionSymbols(ctx, targets)

	/*
		구독이 채우고 있는 종목은 폴링에서 뺀다 — 그게 웹소켓의 이득 전부다.

		`IsLive`는 「구독을 선언했다」가 아니라 **「최근 프레임이 실제로 왔다」**를
		본다. 체결·호가는 LOSSY라 드롭될 수 있고, 구독이 거부되거나 연결이 끊길
		수도 있다. 그때는 자동으로 false가 되어 **폴링이 되살아난다** —
		구독이 조용해져도 화면이 멈추지 않는다.

		구독이 꺼져 있으면(`stream == nil`) 이 줄은 아무 일도 하지 않는다.
	*/
	targets = p.quotes.filterStreamedSymbols(targets)
	if len(targets) == 0 {
		return
	}

	// One batched GetStocks call for every symbol this tick will refresh,
	// instead of each symbol's refreshTerminalSnapshot making its own —
	// /api/v1/stocks and GetStock share Toss's STOCK rate-limit bucket.
	p.quotes.prefetchStockInfo(ctx, targets)

	p.forEachSymbol(ctx, targets, func(symbol string) {
		// Share one resolved price between the header (quote_update) and the
		// daily chart's forming bar (bar_update) — see refreshFormingBars.
		snap, ok := p.quotes.refreshTerminalSnapshot(ctx, symbol)
		var price float64
		if ok {
			price = snap.Detail.CurrentPrice
		}
		p.quotes.refreshFormingBars(ctx, symbol, price)
	})
}

/*
capActiveTargets — 한 틱이 완전 갱신할 종목 수를 상한으로 자르되, **매 틱 다른
구간을 돌게** 한다.

상한 자체는 Toss 한도에서 나온다(호가·체결 각 15/s). 시청자가 몰려 상한을 넘으면
누군가는 그 틱에서 밀릴 수밖에 없다 — 그건 상류 제약이라 어떤 설계로도 못 없앤다.

없앨 수 있는 것은 **누가 밀릴지가 매번 임의로 정해지는 것**이다. 전에는 Go 맵을
그냥 순회해 잘랐다. 맵 순회 순서는 매번 무작위라 통계적으로는 고르지만 보장이
없어, 특정 사용자의 종목이 연속으로 여러 틱을 밀릴 수 있었다.

정렬 + 커서로 바꾸면 순회가 결정적이 된다. active가 100개이고 상한이 40이면 각
종목이 **2.5틱마다 정확히 한 번** 갱신된다 — 주기가 늘어나되 예측 가능해지고,
특정 사용자만 굶는 일이 사라진다.
*/
func (p *marketPoller) capActiveTargets(active map[string]struct{}, limit int, cursor *int) []string {
	if len(active) == 0 || limit <= 0 {
		return nil
	}

	all := make([]string, 0, len(active))
	for sym := range active {
		all = append(all, sym)
	}
	// 정렬해야 커서가 의미를 갖는다 — 맵 순회 순서는 틱마다 달라진다.
	sort.Strings(all)

	if len(all) <= limit {
		p.mu.Lock()
		*cursor = 0
		p.mu.Unlock()
		return all
	}

	p.mu.Lock()
	start := *cursor % len(all)
	*cursor = (start + limit) % len(all)
	p.mu.Unlock()

	targets := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		targets = append(targets, all[(start+i)%len(all)])
	}
	return targets
}

// forEachSymbol runs fn over symbols with bounded concurrency, aborting early
// when ctx is cancelled.
func (p *marketPoller) forEachSymbol(ctx context.Context, symbols []string, fn func(string)) {
	concurrency := marketRefreshConcurrency
	if len(symbols) < concurrency {
		concurrency = len(symbols)
	}
	if concurrency <= 1 {
		for _, sym := range symbols {
			if ctx.Err() != nil {
				return
			}
			fn(sym)
		}
		return
	}

	work := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for sym := range work {
				if ctx.Err() != nil {
					return
				}
				sym := sym
				safego.Call("market-poller-symbol", func() {
					fn(sym)
				})
			}
		}()
	}
	for _, sym := range symbols {
		if ctx.Err() != nil {
			break
		}
		work <- sym
	}
	close(work)
	wg.Wait()
}

// pollSymbols returns every symbol worth a price refresh (out), the subset a user
// has open on a **detail** screen (terminal), and the subset whose **호가창** is
// actually open (book). Expired entries are pruned.
//
// 둘을 갈라 돌려주는 이유는 Touch/TouchTerminal 헤더에 있다 — 목록 화면이 스친
// 종목까지 호가·체결을 받으면 사용자 없이도 상류 한도를 태운다.
func (p *marketPoller) pollSymbols(ctx context.Context) ([]string, map[string]struct{}, map[string]struct{}) {
	p.refreshPinned(ctx)

	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()

	seen := map[string]struct{}{}
	var out []string
	add := func(sym string) string {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			return ""
		}
		if _, ok := seen[sym]; ok {
			return sym
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
		return sym
	}

	// 만료된 가격 관심을 걷어내면서 out(=tickPrice 대상)에 담는다.
	for sym, seenAt := range p.active {
		if now.Sub(seenAt) > marketActiveTTL {
			delete(p.active, sym)
			continue
		}
		add(sym)
	}

	// 터미널 관심은 따로 돌려준다 — 이것만 tickFast·tickSlow의 대상이다.
	terminal := map[string]struct{}{}
	for sym, seenAt := range p.terminal {
		if now.Sub(seenAt) > marketActiveTTL {
			delete(p.terminal, sym)
			continue
		}
		if norm := add(sym); norm != "" {
			terminal[norm] = struct{}{}
		}
	}
	// 호가 관심은 더 짧은 TTL로 만료시킨다 — 호가창을 닫으면 30초 안에 구독에서 빠진다.
	book := map[string]struct{}{}
	for sym, seenAt := range p.book {
		if now.Sub(seenAt) > marketBookTTL {
			delete(p.book, sym)
			continue
		}
		book[strings.ToUpper(strings.TrimSpace(sym))] = struct{}{}
	}

	for _, sym := range p.pinned {
		add(sym)
	}
	return out, terminal, book
}

func (p *marketPoller) tickSlow(ctx context.Context) {
	if !p.quotes.TossEnabled() {
		return
	}
	if !p.slowBusy.CompareAndSwap(false, true) {
		return
	}
	defer p.slowBusy.Store(false)

	/*
		이 틱의 대상은 **지금 보고 있는 종목**이다 — 예열이 아니다.

		주석이 오래 「관심 ∪ 보유 ∪ 조회중」이라고 적혀 있었지만 실제로는
		「조회중」뿐이었다. cared를 채우던 `loadWatchlistSymbols`가 컨텍스트의
		사용자를 요구하는데 폴러는 `context.Background()` 파생으로 돌아 **항상
		nil을 돌려줬다**(구 P2-15). 그래서 이 틱은 처음부터 `active`만 훑었다.

		**그래서 이 호출을 없애면 안 된다.** 이건 사용자가 지금 열어 둔 차트에
		새 봉을 넣는 경로다. 없애면 보고 있는 차트가 멈춘다.

		진짜 문제는 **상한이 없다**는 것이었다. tickFast는 marketFastSymbolCap으로
		자르는데 여기만 무제한이라, 동시 시청 종목이 늘면 `/candles`(20/s)를
		그대로 태웠다 — 실측(2026-08-23) 30분간 rate-limit exceeded 343건.
		그 소진이 **사용자의 차트 첫 열람까지 밀어냈다**(같은 버킷).

		캡 + 라운드로빈으로 바꾼다. 상한을 넘으면 누군가는 그 틱에서 밀리지만,
		커서가 돌기 때문에 다음 틱에 이어받는다 — tickFast가 쓰는 것과 같은
		`capActiveTargets`다. 밀린 종목도 형성봉은 계속 움직인다
		(refreshFormingBars가 가격으로 따로 갱신한다).
	*/
	_, terminal, _ := p.pollSymbols(ctx)
	targets := p.capActiveTargets(terminal, marketSlowSymbolCap, &p.slowCursor)

	/*
		장이 닫힌 종목은 새 봉이 생기지 않는다 — 받아 봐야 이미 가진 것과 같다.

		그런데 이 틱은 마감 중에도 계속 받고 있었고, 심지어 **마감 중이 더
		비쌌다.** `intraday1mFetchTarget`이 「마지막 저장 봉과의 간격」으로 크기를
		정하는데, 마감 중에는 그 간격이 벽시계로 계속 벌어지기 때문이다 —
		월요일 아침이면 금요일 종가 이후 이틀 반이라 상한 720봉(4호출)에 붙는다.
		종목당 30초마다 4호출이고, 그 호출은 사용자의 차트 첫 열람과 같은
		`MARKET_DATA_CHART` 버킷을 쓴다.

		실측(2026-08-24 07:45 KST, 양 시장 마감): `/candles` rate-limit exceeded가
		분당 22건이었다 — 아무도 안 보는 시간에 배경 작업만으로 한도를 넘고 있었다.

		tickFast는 이미 같은 필터를 쓴다(`filterLiveSessionSymbols`). 여기만
		빠져 있었다. 마감 중 차트는 DB의 저장분을 그대로 읽으므로 화면은 변하지
		않는다 — 애초에 바뀔 봉이 없다.
	*/
	targets = p.quotes.filterLiveSessionSymbols(ctx, targets)
	if len(targets) == 0 {
		return
	}

	p.forEachSymbol(ctx, targets, func(symbol string) {
		if err := p.quotes.refreshRawCandles(ctx, symbol); err != nil {
			log.Printf("market poller: refresh candles %s: %v", symbol, err)
		}
		p.quotes.triggerQuoteStatsRefresh(symbol)
	})
}

// keysOf flattens a symbol set for callers that want a slice.
func keysOf(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for sym := range set {
		out = append(out, sym)
	}
	sort.Strings(out)
	return out
}
