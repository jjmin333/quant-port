package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/enact/quant-port/be/internal/metrics"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

/*
실시간 시세 구독 — `tickFast`의 REST 호출을 푸시로 대체한다.

# 무엇을 대체하나

`tickFast`는 상세 화면을 보는 종목마다 `/orderbook` + `/trades` 2호출을 낸다.
그 둘은 `/prices`와 **같은 `MARKET_DATA` 버킷**(한도 15)을 쓰므로 상한 40종목이면
초당 80호출이고, 실측(2026-08-24 KRX 개장 중) `rate-limit exceeded`가 **분당 251건**
이었다. 구독이 붙은 종목은 그 2호출이 **0**이 된다.

# 무엇을 더 대체하나

`tickFast`뿐 아니라 **화면이 직접 부르는 요청 경로**도 대체한다. 그쪽은 원래
구독과 무관하게 REST를 그대로 냈다 — 자세한 실측과 방법은
`quotes_stream_terminal.go` 헤더에 있다.

# 슬롯 예산

실측으로 확정한 상한 두 개:

	앱(client_id)당 동시 연결 **2**   — 3·4번째를 열면 오래된 2개가 끊긴다
	연결당 구독 **100**              — `too-many-topics: max 100`

자격증명 2개를 다 쓰므로

	앱 2 × 연결 2 × 구독 100 = **400 슬롯**
	구독 단위는 채널×심볼이라 trade+orderbook를 다 붙이면 종목당 2슬롯 → 200종목

예전에는 SECONDARY만 써서 200슬롯이었다. REST와 자격증명을 공유하면 WS 재연결이
REST 토큰을 죽인다고 봤기 때문인데, **그 전제가 반증됐다**(`toss/stream.go` 헤더).

# LOSSY 대비

스펙이 체결·호가를 LOSSY로 명시한다(백프레셔 시 프레임 드롭). 그래서 프레임이
끊긴 종목을 감지해 **폴링으로 되돌린다** — `IsLive`가 그 판정이고, `tickFast`가
그것을 보고 대상에서 뺄지 정한다. 구독이 조용하면 자동으로 예전 동작이 된다.
*/

const (
	// StreamConnections — 앱(client_id) 하나가 열 수 있는 연결 수.
	// 실측: 3·4번째를 열면 가장 오래된 2개가 끊긴다.
	StreamConnections = 2

	// StreamSlotsPerConn — 연결당 구독 상한(스펙: codes 합산 100).
	StreamSlotsPerConn = 100

	/*
		streamStaleAfter — 이 시간 동안 프레임이 없으면 그 종목을 「구독 중이지만
		죽었다」로 보고 폴링으로 되돌린다.

		거래가 뜸한 종목은 정상적으로도 몇 초씩 조용하다. 그래서 짧게 잡으면
		멀쩡한 구독을 폴링으로 되돌려 이득이 사라진다. 20초는 호가 갱신 주기보다
		충분히 길면서, 프레임이 진짜 끊겼을 때 화면이 멈춰 보이기 전에 잡아낸다.
	*/
	streamStaleAfter = 20 * time.Second
)

// QuoteStreamService owns the websocket connections and feeds decoded frames
// into the same caches and publish paths the polling path uses.
type QuoteStreamService struct {
	quotes  *QuotesService
	clients []*toss.StreamClient

	mu       sync.RWMutex
	lastSeen map[string]time.Time // symbol -> 마지막 프레임 수신

	// termMu / terminal — 차트·호가 화면이 읽을 체결·호가 버퍼.
	// 프레임을 쌓아 두고 플러시 goroutine이 캐시로 내보낸다
	// (quotes_stream_terminal.go 헤더 참조).
	termMu   sync.Mutex
	terminal map[string]*streamSymbolState
	tradeSeq atomic.Int64

	// subMu / confirmed — 서버가 **수락한** 구독(심볼 → 그것을 든 연결).
	// 「조용한 종목」과 「끊긴 구독」을 가르는 근거다(IsFed 참조).
	subMu     sync.RWMutex
	confirmed map[string]confirmedTopic
}

/*
NewQuoteStreamService — 주어진 자격증명들로 구독 연결을 연다.

`auths`는 **REST가 쓰는 것과 같은 토큰 관리자**다(`toss.Client.TokenManagers`).
공유해도 되는 이유는 재연결이 재발급을 부르지 않기 때문이다 — 그 근거와 실측은
`toss/stream.go` 헤더에 있다.

앱마다 연결 2개(Toss가 client_id당 허용하는 상한, 실측 M)를 열므로 앱이 둘이면
연결 4개 · 슬롯 400이 된다.
*/
func NewQuoteStreamService(quotes *QuotesService, auths []*toss.TokenManager) *QuoteStreamService {
	s := &QuoteStreamService{
		quotes:    quotes,
		lastSeen:  map[string]time.Time{},
		terminal:  map[string]*streamSymbolState{},
		confirmed: map[string]confirmedTopic{},
	}
	for ai, auth := range auths {
		if auth == nil {
			continue
		}
		tokenFor := auth // 루프 변수를 클로저에 가두지 않는다.
		for i := 0; i < StreamConnections; i++ {
			name := fmt.Sprintf("quant-port-ws-%d-%d", ai+1, i+1)
			s.clients = append(s.clients, toss.NewStreamClient(
				func(ctx context.Context, stale string) (string, error) {
					return tokenFor.TokenAfter(ctx, stale)
				}, s, name))
		}
	}
	return s
}

// Run keeps every connection alive until ctx ends.
func (s *QuoteStreamService) Run(ctx context.Context) {
	for _, c := range s.clients {
		client := c
		go client.Run(ctx)
	}
	// 프레임을 캐시로 내보내는 쪽. 연결과 수명을 같이 한다.
	go s.runTerminalFlush(ctx)
	<-ctx.Done()
}

/*
SetSymbols — 구독 대상을 갈아 끼운다.

호출자(폴러)는 「지금 상세 화면에서 보고 있는 종목」을 넘긴다. 여기서 슬롯
예산에 맞춰 자르고 연결들에 나눠 싣는다.

**trade를 orderbook보다 우선한다.** 체결가는 현재가이자 형성봉의 재료라
한 슬롯이 주는 값이 가장 크고, 호가창은 상세 화면에서도 실제로 보는 사람이 적다.
*/
func (s *QuoteStreamService) SetSymbols(symbols, book []string) {
	if len(s.clients) == 0 {
		return
	}
	topics := s.planTopics(symbols, book)

	// 연결들에 고르게 나눈다. 같은 심볼의 두 채널이 서로 다른 연결에 가도 무방하다.
	per := make([][]toss.StreamTopic, len(s.clients))
	for i, t := range topics {
		idx := i / StreamSlotsPerConn
		if idx >= len(per) {
			break
		}
		per[idx] = append(per[idx], t)
	}
	for i, c := range s.clients {
		c.Subscribe(per[i])
	}
	metrics.SetStreamSubscriptions(len(topics))

	// 구독에서 빠진 종목의 버퍼를 정리한다 — 안 하면 사용자가 종목을 옮겨
	// 다닌 만큼 맵이 자란다.
	keep := make(map[string]struct{}, len(topics))
	for _, t := range topics {
		keep[strings.ToUpper(t.Symbol)] = struct{}{}
	}
	s.forgetTerminalState(keep)
}

/*
IsLive — 이 종목을 구독이 실제로 채우고 있는가.

`tickFast`가 이것을 보고 폴링 대상에서 뺀다. 프레임이 끊기면(LOSSY 드롭·구독
거부·연결 장애) 자동으로 false가 되어 폴링이 되살아난다 — 구독이 조용해져도
화면이 멈추지 않게 하는 안전장치다.
*/
func (s *QuoteStreamService) IsLive(symbol string) bool {
	if s == nil {
		return false
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	s.mu.RLock()
	at, ok := s.lastSeen[sym]
	s.mu.RUnlock()
	return ok && time.Since(at) < streamStaleAfter
}

// OnTrade turns one execution into the same updates the polling path produces.
func (s *QuoteStreamService) OnTrade(f toss.TradeFrame) {
	sym := strings.ToUpper(f.Topic.Symbol)
	s.mark(sym)
	metrics.StreamFrame("trade")

	price := parseDecimal(f.Price)
	if price <= 0 || s.quotes == nil {
		return
	}

	/*
		체결 목록을 여기서 쌓는다.

		전에는 프레임을 가격 갱신에만 쓰고 버렸다. 그래서 화면의 「체결」 표는
		구독 중에도 `/api/v1/trades`를 다시 받아야 했다 — 같은 데이터를 두 번
		받는 셈이었다(quotes_stream_terminal.go 헤더의 실측).
	*/
	tick := tradeTickFromFrame(sym, s.tradeSeq.Add(1), f.Timestamp, price, parseDecimalInt(f.Volume))
	s.termMu.Lock()
	s.stateLocked(sym).recordTrade(tick)
	s.termMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	/*
		가격 캐시를 갱신하면 목록·상세가 읽는 값이 바로 새것이 된다. 폴러의
		`RefreshPrices`가 쓰던 것과 **같은 키**라 읽기 경로는 아무것도 안 바꿔도 된다.
	*/
	if s.quotes.marketStore != nil {
		s.quotes.marketStore.applyStreamPrice(ctx, sym, f.Price, f.Currency)
	}

	// 헤더(quote_update)와 형성봉은 폴링 경로와 같은 함수를 쓴다 —
	// 화면에 나가는 모양이 경로에 따라 달라지면 안 된다.
	if detail, ok := s.quotes.GetQuote(ctx, sym); ok {
		detail.CurrentPrice = price
		s.quotes.publishQuoteUpdate(detail)
	}
	s.quotes.refreshFormingBars(ctx, sym, price)
}

// OnOrderbook persists one book snapshot the same way the polling path does.
func (s *QuoteStreamService) OnOrderbook(f toss.OrderbookFrame) {
	sym := strings.ToUpper(f.Topic.Symbol)
	s.mark(sym)
	metrics.StreamFrame("orderbook")

	if s.quotes == nil {
		return
	}
	book := model.OrderBook{Symbol: sym}
	if !f.Timestamp.IsZero() {
		ts := f.Timestamp.Format(time.RFC3339)
		book.Timestamp = ts
	}
	for _, a := range f.Asks {
		book.Asks = append(book.Asks, model.OrderBookLevel{
			Price: parseDecimal(a.Price), Quantity: parseDecimalInt(a.Volume),
		})
	}
	for _, b := range f.Bids {
		book.Bids = append(book.Bids, model.OrderBookLevel{
			Price: parseDecimal(b.Price), Quantity: parseDecimalInt(b.Volume),
		})
	}
	if len(book.Asks) == 0 && len(book.Bids) == 0 {
		return
	}

	/*
		캐시·DB에 **바로 쓰지 않는다.**

		실측으로 호가 프레임이 종목당 초당 17건 온다. 전에는 그때마다
		`persistOrderBook`이 Redis SET 1회 + MySQL upsert 1회를 냈다 — 시청자가
		늘면 그대로 DB 쓰기가 비례해 늘고, 이 DB는 1 ECPU다.

		여기서는 메모리에만 반영하고, 플러시 goroutine이 Redis 0.5초 · MySQL 3초로
		내보낸다(quotes_stream_terminal.go 헤더).
	*/
	s.termMu.Lock()
	st := s.stateLocked(sym)
	st.book = &book
	st.bookDirty = true
	s.termMu.Unlock()
}

func (s *QuoteStreamService) mark(symbol string) {
	s.mu.Lock()
	s.lastSeen[symbol] = time.Now()
	s.mu.Unlock()
}

/*
streamMarketFor — 심볼에서 시장 코드를 정한다.

스펙이 `codes`에 **그 시장의 심볼만** 넣으라고 하므로 `trade:kr` / `trade:us`를
갈라야 한다. 국내는 6자리 숫자, 그 외는 미국으로 본다 —
`relatedSymbolCandidates`가 쓰는 것과 같은 판별이다.
*/
func streamMarketFor(symbol string) string {
	for _, r := range symbol {
		if r >= 'A' && r <= 'Z' {
			return "us"
		}
	}
	return "kr"
}

/*
SetStream — 구독 서비스를 연결한다. nil이면 폴링만 돈다.

기동 시 한 번만 부른다(server.go). 이후 교체되지 않으므로 잠금이 필요 없다.
*/
func (s *QuotesService) SetStream(stream *QuoteStreamService) {
	s.stream = stream
}

// filterStreamedSymbols drops symbols the websocket is actively feeding.
//
// 구독이 꺼져 있거나 프레임이 끊긴 종목은 그대로 남는다 — 폴링이 받는다.
func (s *QuotesService) filterStreamedSymbols(symbols []string) []string {
	if s.stream == nil || len(symbols) == 0 {
		return symbols
	}
	out := symbols[:0:0]
	for _, sym := range symbols {
		if s.stream.IsLive(sym) {
			continue
		}
		out = append(out, sym)
	}
	return out
}

// PublishStreamTargets hands the poller's terminal set to the subscription
// manager. Called from the poller so the subscribed set tracks exactly what
// users have open.
// PublishStreamTargets — 체결을 구독할 종목(symbols)과, 그중 **호가창을 실제로
// 열어 둔** 종목(book)을 넘긴다. book에 없는 종목은 체결만 받는다.
func (s *QuotesService) PublishStreamTargets(symbols, book []string) {
	if s.stream != nil {
		s.stream.SetSymbols(symbols, book)
	}
}

/*
planTopics — 심볼 목록을 슬롯 예산에 맞는 구독 목록으로 바꾼다.

`SetSymbols`에서 떼어 낸 것은 이 계산이 **테스트 가능한 순수 함수**여야 하기
때문이다. 예산 초과 시 무엇을 버리는지가 이 파일에서 가장 판단이 들어간 부분이고,
연결·네트워크 없이 그것만 검증할 수 있어야 한다.
*/
func (s *QuoteStreamService) planTopics(symbols, book []string) []toss.StreamTopic {
	totalSlots := StreamConnections * StreamSlotsPerConn
	if len(s.clients) > 0 {
		totalSlots = len(s.clients) * StreamSlotsPerConn
	}

	/*
		**호가는 호가창을 실제로 연 종목에만 붙인다.**

		예전에는 상세 화면에 들어간 모든 종목에 체결+호가 2슬롯을 썼다. 그런데
		호가창은 차트 탭에서만 그려지고(`stock-workspace-client.tsx`), 호가를 보는
		사람은 상세 화면을 여는 사람보다 훨씬 적다. 실측(2026-09-02, 2종목 구독):
		프레임이 **호가 40,667 vs 체결 9,461**로 호가가 4.3배였다 — 대부분 아무도
		안 보는 데이터를 받아 디코딩·버퍼링·캐시 쓰기까지 하고 있었다.

		이제 종목당 기본 1슬롯이라 **같은 예산으로 두 배**의 종목을 커버한다.
		호가창을 닫으면 TTL(`marketActiveTTL`) 뒤 구독에서 빠진다.

		예산이 그래도 모자라면 예전과 같이 호가부터 버린다 — 체결가는 현재가이자
		형성봉의 재료라 한 슬롯이 주는 값이 가장 크다.
	*/
	wantBook := make(map[string]struct{}, len(book))
	for _, sym := range book {
		if up := strings.ToUpper(strings.TrimSpace(sym)); up != "" {
			wantBook[up] = struct{}{}
		}
	}
	// 체결이 먼저 전부 들어가고 남는 슬롯만 호가에 준다.
	bookBudget := max(0, totalSlots-len(symbols))

	topics := make([]toss.StreamTopic, 0, totalSlots)
	for _, sym := range symbols {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			continue
		}
		if len(topics) >= totalSlots {
			break
		}
		market := streamMarketFor(sym)
		topics = append(topics, toss.StreamTopic{Channel: toss.ChannelTrade, Market: market, Symbol: sym})

		if _, want := wantBook[sym]; !want || bookBudget <= 0 {
			continue
		}
		if len(topics) >= totalSlots {
			break
		}
		topics = append(topics, toss.StreamTopic{Channel: toss.ChannelOrderbook, Market: market, Symbol: sym})
		bookBudget--
	}
	return topics
}
