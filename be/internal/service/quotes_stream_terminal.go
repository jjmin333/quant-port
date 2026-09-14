package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/metrics"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

/*
구독 프레임을 **차트·호가 화면이 읽는 캐시**로 적재한다.

# 왜 필요한가

구독을 붙였는데도 그 화면의 REST 호출은 그대로였다. 구독은 폴러의 `tickFast`만
대체했고, 화면이 2초마다 부르는 `/quotes/{symbol}/terminal-snapshot`은 캐시
TTL이 1초라 **매번 만료돼** `refreshTerminalSnapshot`으로 들어가 REST를 다시 냈다.

실측(2026-08-25 미국 정규장, 시청자 1명·30초·요청 15건, 그 종목은 구독 중):

	/api/v1/orderbook  +19    MARKET_DATA (15 req/s)
	/api/v1/trades     +19    MARKET_DATA
	/api/v1/stocks     +19    STOCK (**5 req/s** — 동시 시청자 8명이면 소진)

같은 30초 동안 웹소켓은 그 종목의 프레임을 수백 건 받고 있었다.
**데이터는 이미 와 있었고, 쓰지 않았을 뿐이다.**

# 무엇을 하나

	체결 — 프레임을 링버퍼에 쌓는다(50건, REST `/trades`가 주던 개수와 동일)
	호가 — 마지막 프레임을 그대로 들고 있는다

그리고 **폴링 경로가 쓰던 것과 같은 캐시 키·같은 형식**으로 쓴다. 그래야 읽는
쪽이 데이터 출처를 몰라도 되고, 구독이 끊기면 폴링이 같은 키를 이어서 채운다.

# 왜 프레임마다 쓰지 않는가

프레임은 종목당 초당 수십 건까지 온다(실측: 호가 17/s). 그대로 흘리면

	Redis  — 초당 수십 SET. 값이 작아 감당은 되지만 대부분 덮어쓰기다
	MySQL  — `UpsertOrderBook`이 초당 17회. 이 DB는 **1 ECPU**다

그래서 프레임은 메모리에만 반영하고, **플러시 goroutine**이 Redis 0.5초 ·
MySQL 3초 간격으로 변경된 종목만 내보낸다. 스로틀을 호출 지점에 두지 않고
별도 goroutine으로 뺀 이유는 **마지막 프레임이 유실되지 않게** 하기 위해서다 —
호출 지점 스로틀은 조용해지기 직전의 갱신을 영영 안 쓰게 된다.

화면 갱신 주기가 2초이므로 0.5초 해상도면 사용자가 보는 지연은 늘지 않는다.
*/

const (
	// streamTradeBufferSize — REST `/trades`가 주던 개수와 맞춘다. 화면이 같은
	// 길이를 기대하고 있고, 늘리면 캐시 값만 커진다.
	streamTradeBufferSize = 50

	// streamCacheFlushEvery — Redis 반영 주기.
	streamCacheFlushEvery = 500 * time.Millisecond

	// streamDBFlushEvery — MySQL 반영 주기. 재기동 후 last-good 복구용이라
	// 초 단위 해상도면 충분하다.
	streamDBFlushEvery = 3 * time.Second
)

// streamSymbolState is one symbol's un-flushed frame state.
type streamSymbolState struct {
	trades     []model.TradeTick // newest first
	book       *model.OrderBook
	tradeDirty bool
	bookDirty  bool
	lastDBAt   time.Time
}

// recordTrade appends one tick, keeping the buffer newest-first and capped.
func (st *streamSymbolState) recordTrade(tick model.TradeTick) {
	st.trades = append([]model.TradeTick{tick}, st.trades...)
	if len(st.trades) > streamTradeBufferSize {
		st.trades = st.trades[:streamTradeBufferSize]
	}
	st.tradeDirty = true
}

/*
tradeTickFromFrame — 체결 프레임을 폴링 경로가 만들던 것과 **같은 모양**으로 바꾼다.

`Side`가 "unknown"인 것은 REST `mapTrades`도 같다 — Toss는 어느 경로로도
매수/매도 구분을 주지 않는다. 화면은 그때 등락률 색을 쓰므로 값이 같아야 한다.

`ID`는 화면의 React key로만 쓰인다. 폴링 경로의 형식(타임스탬프-가격-수량-순번)을
따르되 순번 자리에 링버퍼 유입 순서를 넣어, 같은 타임스탬프의 체결이 여러 건
와도 키가 겹치지 않게 한다.
*/
func tradeTickFromFrame(symbol string, seq int64, ts time.Time, price float64, qty int64) model.TradeTick {
	timeStr := ""
	if !ts.IsZero() {
		timeStr = ts.Format(time.RFC3339)
	}
	return model.TradeTick{
		ID:       fmt.Sprintf("%s-%g-%d-%d", timeStr, price, qty, seq),
		Symbol:   symbol,
		Time:     timeStr,
		Price:    price,
		Quantity: qty,
		Side:     "unknown",
	}
}

// stateLocked returns (creating if needed) one symbol's buffer.
// Caller must hold termMu.
func (s *QuoteStreamService) stateLocked(symbol string) *streamSymbolState {
	if s.terminal == nil {
		s.terminal = map[string]*streamSymbolState{}
	}
	st, ok := s.terminal[symbol]
	if !ok {
		st = &streamSymbolState{}
		s.terminal[symbol] = st
	}
	return st
}

/*
runTerminalFlush — 변경된 종목만 캐시로 내보낸다.

`Run`이 연결들과 함께 띄운다. ctx가 끝나면 **마지막으로 한 번 더 플러시**한다 —
종료 직전의 프레임이 캐시에 안 남으면 재기동 직후 화면이 그만큼 과거를 보여 준다.
*/
func (s *QuoteStreamService) runTerminalFlush(ctx context.Context) {
	ticker := time.NewTicker(streamCacheFlushEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.flushTerminal(context.Background())
			return
		case <-ticker.C:
			s.flushTerminal(ctx)
		}
	}
}

func (s *QuoteStreamService) flushTerminal(ctx context.Context) {
	if s.quotes == nil {
		return
	}

	type pending struct {
		symbol string
		trades []model.TradeTick
		book   *model.OrderBook
		toDB   bool
	}

	now := time.Now()
	var work []pending

	s.termMu.Lock()
	for sym, st := range s.terminal {
		if !st.tradeDirty && !st.bookDirty {
			continue
		}
		p := pending{symbol: sym, toDB: now.Sub(st.lastDBAt) >= streamDBFlushEvery}
		if st.tradeDirty {
			p.trades = append([]model.TradeTick(nil), st.trades...)
			st.tradeDirty = false
		}
		if st.bookDirty && st.book != nil {
			book := *st.book
			p.book = &book
			st.bookDirty = false
		}
		if p.toDB {
			st.lastDBAt = now
		}
		work = append(work, p)
	}
	s.termMu.Unlock()

	for _, p := range work {
		if len(p.trades) > 0 {
			s.quotes.cacheTrades(ctx, p.symbol, p.trades, streamDataSource)
			if p.toDB && s.quotes.quoteTerminal != nil {
				_ = s.quotes.quoteTerminal.ReplaceTrades(ctx, p.symbol, p.trades)
			}
		}
		if p.book != nil {
			s.quotes.cacheOrderBook(ctx, p.symbol, *p.book, streamDataSource)
			if p.toDB && s.quotes.quoteTerminal != nil {
				_ = s.quotes.quoteTerminal.UpsertOrderBook(ctx, p.symbol, *p.book)
			}
		}
	}
}

// streamDataSource labels cache envelopes written from the subscription so
// `source` in the API response tells the two paths apart during debugging.
const streamDataSource = "toss-ws"

/*
forgetTerminalState — 더 이상 구독하지 않는 종목의 버퍼를 버린다.

`SetSymbols`가 부른다. 안 버리면 사용자가 종목을 옮겨 다닌 만큼 맵이 자란다 —
프로세스가 며칠 살아 있으면 그것이 곧 누수다.

**구독 목록에서 빠졌다고 즉시 지우지는 않는다.** 화면을 잠깐 벗어났다 돌아오는
경우가 흔하고, 그때 버퍼가 비어 있으면 체결 목록이 한 번 비었다가 다시 찬다.
`IsLive`가 죽었다고 보는 시간(streamStaleAfter)만큼 유예를 준다.
*/
func (s *QuoteStreamService) forgetTerminalState(keep map[string]struct{}) {
	cutoff := time.Now().Add(-streamStaleAfter)

	/*
		`lastSeen`을 **먼저 복사한 뒤** termMu를 잡는다.

		두 잠금을 겹쳐 잡으면 순서가 생긴다. 지금 코드에서 반대 순서로 잡는 곳은
		없지만, `OnTrade`가 `mark`(s.mu) 다음에 termMu를 잡는 모양이라 누군가
		그 둘을 합치는 순간 교착이 된다. 겹치지 않게 두면 그 위험 자체가 없다.
	*/
	s.mu.RLock()
	recent := make(map[string]struct{}, len(s.lastSeen))
	for sym, at := range s.lastSeen {
		if at.After(cutoff) {
			recent[sym] = struct{}{}
		}
	}
	s.mu.RUnlock()

	s.termMu.Lock()
	defer s.termMu.Unlock()
	for sym := range s.terminal {
		if _, ok := keep[sym]; ok {
			continue
		}
		if _, ok := recent[sym]; ok {
			continue
		}
		delete(s.terminal, sym)
	}
}

/*
streamQuietGrace — 프레임이 없어도 구독을 믿어 주는 시간.

「조용하다」와 「끊겼다」는 다르다. 거래가 뜸한 종목은 정상적으로도 몇 분씩
프레임이 없고, 그때 데이터가 안 오는 이유는 **바뀐 게 없어서**다. 그것을 장애로
보고 REST로 되돌아가면 같은 값을 다시 받아 오게 된다 — 실측에서 미국 장 마감 뒤
`source`가 `toss-ws`와 `toss` 사이를 오갔던 것이 이 경우다.

그래서 확인된 구독이 서 있고 연결이 살아 있으면 프레임이 뜸해도 버퍼를 쓴다.
상한을 5분으로 잡은 것은 **이 코드베이스가 이미 쓰던 기준**이다 —
`liveMicrostructureMaxAge`가 같은 데이터(호가·체결)를 그만큼 신선하다고 본다.

연결이 끊기거나 구독이 거부되면 확인이 즉시 취소되므로, 이 유예가 진짜 장애를
덮는 일은 없다.
*/
const streamQuietGrace = liveMicrostructureMaxAge

// confirmedTopic is one accepted subscription and the connection holding it.
type confirmedTopic struct {
	conn string
}

/*
OnSubscriptions — 서버가 수락한 구독을 기록한다.

거부된 것은 기록하지 않는다. 그래야 그 종목은 `IsFed`가 false가 되어 폴링이
계속 받는다 — 거부를 조용히 넘기면 그 종목만 영영 안 움직인다.
*/
func (s *QuoteStreamService) OnSubscriptions(conn string, accepted []toss.StreamTopic, rejected []string) {
	if s == nil {
		return
	}
	s.subMu.Lock()
	if s.confirmed == nil {
		s.confirmed = map[string]confirmedTopic{}
	}
	// 이 연결의 이전 확인은 통째로 대체된다 — 선언이 전체 교체 방식이므로
	// 남겨 두면 구독에서 뺀 종목이 확인된 채로 남는다.
	for sym, c := range s.confirmed {
		if c.conn == conn {
			delete(s.confirmed, sym)
		}
	}
	for _, t := range accepted {
		s.confirmed[strings.ToUpper(t.Symbol)] = confirmedTopic{conn: conn}
	}
	s.subMu.Unlock()

	if len(rejected) > 0 {
		metrics.StreamSubscriptionRejected(len(rejected))
	}
}

// OnDisconnect drops every subscription this connection was holding.
func (s *QuoteStreamService) OnDisconnect(conn string) {
	if s == nil {
		return
	}
	s.subMu.Lock()
	for sym, c := range s.confirmed {
		if c.conn == conn {
			delete(s.confirmed, sym)
		}
	}
	s.subMu.Unlock()
}

func (s *QuoteStreamService) isConfirmed(symbol string) bool {
	s.subMu.RLock()
	_, ok := s.confirmed[symbol]
	s.subMu.RUnlock()
	return ok
}

/*
IsFed — 이 종목의 화면 데이터를 구독이 대고 있는가.

`IsLive`(프레임 도착 20초)보다 **넓다**. 둘을 가른 이유는 쓰는 곳의 질문이 다르기
때문이다:

	IsLive — 「폴링을 꺼도 되나」   (tickFast). 틀리면 화면이 멈추므로 보수적으로.
	IsFed  — 「REST를 안 불러도 되나」. 조용한 것은 바뀐 게 없다는 뜻이므로
	         확인된 구독이 서 있으면 믿는다.

`IsLive`를 넓히지 않은 것은 폴링이 최후의 안전망이기 때문이다. 구독이 조용한
동안 폴링이 도는 것은 비용이지 고장이 아니지만, 그 반대는 고장이다.
*/
func (s *QuoteStreamService) IsFed(symbol string) bool {
	if s == nil {
		return false
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if s.IsLive(sym) {
		return true
	}
	if !s.isConfirmed(sym) {
		return false
	}
	s.mu.RLock()
	at, seen := s.lastSeen[sym]
	s.mu.RUnlock()
	// 확인만 되고 프레임을 한 번도 못 받았으면 아직 못 믿는다 — 구독이 수락됐어도
	// 실제로 데이터가 오는지는 확인 전이다.
	return seen && time.Since(at) < streamQuietGrace
}

/*
StreamedTerminalData — 구독이 들고 있는 이 종목의 체결·호가.

`refreshTerminalSnapshot`이 REST 대신 이것을 쓴다. **캐시가 아니라 메모리에서**
바로 주는 이유는 플러시 주기(0.5초)와 화면 주기(2초) 사이의 어긋남을 없애기
위해서다 — 캐시를 읽으면 최대 0.5초 낡은 것을 보게 된다.

`ok=false`면 호출자는 예전 경로(REST)를 그대로 탄다.
*/
func (s *QuoteStreamService) StreamedTerminalData(symbol string) ([]model.TradeTick, *model.OrderBook, bool) {
	if s == nil {
		return nil, nil, false
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if !s.IsFed(sym) {
		return nil, nil, false
	}

	s.termMu.Lock()
	st, ok := s.terminal[sym]
	if !ok {
		s.termMu.Unlock()
		return nil, nil, false
	}
	trades := append([]model.TradeTick(nil), st.trades...)
	var book *model.OrderBook
	if st.book != nil {
		b := *st.book
		book = &b
	}
	s.termMu.Unlock()

	if len(trades) == 0 && book == nil {
		return nil, nil, false
	}
	return trades, book, true
}

/*
streamTerminalSources — 스냅샷 조립에 쓸 체결·호가를 고른다.

구독이 살아 있으면 그쪽을, 아니면 `ok=false`를 준다. `QuotesService` 쪽에서
`s.stream`이 nil인 경우까지 여기서 흡수해, 호출부에 nil 검사가 흩어지지 않게 한다.
*/
func (s *QuotesService) streamTerminalSources(symbol string) ([]model.TradeTick, *model.OrderBook, bool) {
	if s.stream == nil {
		return nil, nil, false
	}
	return s.stream.StreamedTerminalData(symbol)
}
