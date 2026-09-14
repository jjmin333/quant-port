package service

import (
	"sync"

	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

type streamHub struct {
	mu   sync.RWMutex
	subs map[string]map[chan model.StreamEvent]struct{}
}

func newStreamHub() *streamHub {
	return &streamHub{subs: make(map[string]map[chan model.StreamEvent]struct{})}
}

func streamKey(symbol string, interval model.CandleInterval) string {
	return symbol + "|" + string(interval)
}

func (h *streamHub) Subscribe(symbol string, interval model.CandleInterval) (<-chan model.StreamEvent, func()) {
	ch := make(chan model.StreamEvent, 8)
	key := streamKey(symbol, interval)

	h.mu.Lock()
	if h.subs[key] == nil {
		h.subs[key] = make(map[chan model.StreamEvent]struct{})
	}
	h.subs[key][ch] = struct{}{}
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		delete(h.subs[key], ch)
		if len(h.subs[key]) == 0 {
			delete(h.subs, key)
		}
		h.mu.Unlock()
		close(ch)
	}
	return ch, cancel
}

func (h *streamHub) Publish(ev model.StreamEvent) {
	key := streamKey(ev.Symbol, ev.Interval)
	h.mu.RLock()
	subs := h.subs[key]
	for ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
	h.mu.RUnlock()
}

type quoteStreamHub struct {
	mu   sync.RWMutex
	subs map[string]map[chan model.QuoteStreamEvent]struct{}
}

func newQuoteStreamHub() *quoteStreamHub {
	return &quoteStreamHub{subs: make(map[string]map[chan model.QuoteStreamEvent]struct{})}
}

func (h *quoteStreamHub) Subscribe(symbol string) (<-chan model.QuoteStreamEvent, func()) {
	ch := make(chan model.QuoteStreamEvent, 8)
	key := strings.ToUpper(strings.TrimSpace(symbol))

	h.mu.Lock()
	if h.subs[key] == nil {
		h.subs[key] = make(map[chan model.QuoteStreamEvent]struct{})
	}
	h.subs[key][ch] = struct{}{}
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		delete(h.subs[key], ch)
		if len(h.subs[key]) == 0 {
			delete(h.subs, key)
		}
		h.mu.Unlock()
		close(ch)
	}
	return ch, cancel
}

func (h *quoteStreamHub) Publish(ev model.QuoteStreamEvent) {
	key := strings.ToUpper(strings.TrimSpace(ev.Symbol))
	h.mu.RLock()
	subs := h.subs[key]
	for ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
	h.mu.RUnlock()
}

/*
listPriceHub — 목록 화면들이 나눠 쓰는 가격 방송.

# 왜 quoteStreamHub를 그냥 쓰지 않는가

그쪽은 **상세 화면 한 종목**의 것이다. 발행자가 `refreshTerminalSnapshot`(터미널
대상 종목)과 토스 웹소켓 프레임(구독 중인 몇 종목)뿐이라, 목록에 있는 대부분의
종목에는 이벤트가 아예 흐르지 않는다. 목록이 그 허브를 구독하면 **조용한 연결**을
쥐고 폴링만 계속하게 된다.

이 허브의 발행자는 `MarketDataStore.cacheMarketPrice` — 가격이 캐시에 써지는
**단 하나의 길목**이다. 배치 폴러(`tickPrice`)와 웹소켓 체결(`applyStreamPrice`)이
둘 다 그 함수를 지나므로, 어느 경로로 들어온 값이든 같은 자리에서 나간다.

# 한 채널이 여러 심볼에 등록된다

목록은 종목 스무 개를 한꺼번에 본다. 심볼마다 채널을 만들어 고루틴으로 합치면
뷰어 한 명당 고루틴 스물한 개다. 대신 **같은 채널을 심볼 수만큼의 구독 집합에
넣는다** — 고루틴은 0개이고, 취소는 그 채널을 모든 키에서 빼고 한 번만 닫는다.

버퍼가 32인 이유: 2초 틱에서 스무 종목이 한꺼번에 발행되므로 8이면 한 틱도 못
담는다. 그래도 넘치면 **버린다**(`default:`) — 느린 구독자 하나가 발행자를
멈추게 하는 것이 가장 나쁘고, 가격은 다음 틱에 어차피 다시 온다.
*/
type listPriceHub struct {
	mu   sync.RWMutex
	subs map[string]map[chan model.ListPriceEvent]struct{}
}

func newListPriceHub() *listPriceHub {
	return &listPriceHub{subs: make(map[string]map[chan model.ListPriceEvent]struct{})}
}

// listPriceChanBuffer holds one tick's worth of a typical list (20 rows) with
// room to spare, so a reader that is briefly busy does not drop the batch.
const listPriceChanBuffer = 32

/*
SubscribeMany는 심볼 여러 개를 **채널 하나로** 구독한다.

중복 심볼은 한 번만 등록한다 — 같은 키에 같은 채널이 두 번 들어가는 일은 맵이라
어차피 없지만, `keys` 목록에 중복이 남으면 취소가 같은 자리를 두 번 지운다.
*/
func (h *listPriceHub) SubscribeMany(symbols []string) (<-chan model.ListPriceEvent, func()) {
	ch := make(chan model.ListPriceEvent, listPriceChanBuffer)

	seen := make(map[string]struct{}, len(symbols))
	keys := make([]string, 0, len(symbols))
	for _, s := range symbols {
		key := strings.ToUpper(strings.TrimSpace(s))
		if key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	h.mu.Lock()
	for _, key := range keys {
		if h.subs[key] == nil {
			h.subs[key] = make(map[chan model.ListPriceEvent]struct{})
		}
		h.subs[key][ch] = struct{}{}
	}
	h.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			for _, key := range keys {
				delete(h.subs[key], ch)
				if len(h.subs[key]) == 0 {
					delete(h.subs, key)
				}
			}
			h.mu.Unlock()
			close(ch)
		})
	}
	return ch, cancel
}

func (h *listPriceHub) Publish(ev model.ListPriceEvent) {
	key := strings.ToUpper(strings.TrimSpace(ev.Symbol))
	if key == "" {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs[key] {
		select {
		case ch <- ev:
		default:
			// 느린 구독자는 이 틱을 놓친다. 다음 틱에 같은 종목이 다시 온다.
		}
	}
}

/*
HasSubscribers — 이 종목을 지금 보고 있는 목록이 하나라도 있는가.

발행자가 이벤트 구조체를 만들기 **전에** 묻는다. 가격 캐시 쓰기는 추적 종목
전체(수백 개)에 대해 2초마다 도는데, 그중 실제로 누가 보고 있는 종목은 몇 개뿐이다.
아무도 안 보는 종목의 이벤트를 만들어 빈 맵에 발행하는 것은 순수한 낭비다.
*/
func (h *listPriceHub) HasSubscribers(symbol string) bool {
	key := strings.ToUpper(strings.TrimSpace(symbol))
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs[key]) > 0
}
