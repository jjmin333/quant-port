package service

import (
	"sync"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

func newTerminalStream() *QuoteStreamService {
	return &QuoteStreamService{
		lastSeen: map[string]time.Time{},
		terminal: map[string]*streamSymbolState{},
	}
}

/*
체결 버퍼는 **최신이 앞**이고 상한이 있어야 한다.

화면은 배열을 정렬 없이 그대로 그리므로 순서가 곧 표시 순서다. 상한이 없으면
장이 열려 있는 동안 종목마다 무한히 자라 — 시청자 수 × 종목 수만큼 메모리를 먹는다.
*/
func TestStreamSymbolState_recordTradeIsNewestFirstAndCapped(t *testing.T) {
	st := &streamSymbolState{}
	for i := 1; i <= streamTradeBufferSize+25; i++ {
		st.recordTrade(model.TradeTick{Price: float64(i)})
	}

	if len(st.trades) != streamTradeBufferSize {
		t.Fatalf("버퍼 %d건 — 상한 %d를 지켜야 한다", len(st.trades), streamTradeBufferSize)
	}
	if st.trades[0].Price != float64(streamTradeBufferSize+25) {
		t.Fatalf("맨 앞이 %v — 가장 최근 체결이어야 한다", st.trades[0].Price)
	}
	if st.trades[1].Price >= st.trades[0].Price {
		t.Fatal("최신순이 아니다 — 화면이 과거 체결을 위에 그린다")
	}
	if !st.tradeDirty {
		t.Fatal("dirty가 안 섰다 — 플러시가 이 갱신을 영영 안 내보낸다")
	}
}

/*
구독이 만든 체결 틱은 REST 경로(`mapTrades`)와 **같은 모양**이어야 한다.

화면이 두 경로를 구분하지 않으므로, 모양이 다르면 구독이 붙었다 끊길 때마다
표의 표시가 달라진다.
*/
func TestTradeTickFromFrame_matchesRESTShape(t *testing.T) {
	ts := time.Date(2026, 8, 25, 10, 30, 0, 0, time.UTC)
	tick := tradeTickFromFrame("005930", 7, ts, 71500, 12)

	if tick.Symbol != "005930" || tick.Price != 71500 || tick.Quantity != 12 {
		t.Fatalf("값이 안 맞는다: %+v", tick)
	}
	// REST의 mapTrades도 "unknown"을 넣는다 — Toss가 어느 경로로도 매수/매도를 안 준다.
	if tick.Side != "unknown" {
		t.Fatalf("side=%q — REST 경로와 같아야 화면 색이 안 바뀐다", tick.Side)
	}
	if got, want := tick.Time, ts.Format(time.RFC3339); got != want {
		t.Fatalf("time=%q want %q", got, want)
	}
	// 같은 체결이 연달아 와도 React key가 겹치면 안 된다.
	other := tradeTickFromFrame("005930", 8, ts, 71500, 12)
	if tick.ID == other.ID {
		t.Fatalf("동일 체결 두 건의 ID가 같다: %q", tick.ID)
	}
}

// 타임스탬프가 없는 프레임도 버려서는 안 된다 — 가격·수량은 유효하다.
func TestTradeTickFromFrame_zeroTimestamp(t *testing.T) {
	tick := tradeTickFromFrame("AAPL", 1, time.Time{}, 311.09, 5)
	if tick.Time != "" {
		t.Fatalf("time=%q — 빈 타임스탬프는 빈 문자열이어야 한다", tick.Time)
	}
	if tick.Price != 311.09 {
		t.Fatal("가격이 유실됐다")
	}
}

/*
구독이 살아 있지 않으면 스냅샷 경로가 **REST로 되돌아가야** 한다.

이 판정이 틀리면 프레임이 끊긴 종목의 화면이 조용히 멈춘다 — 사용자에게는
「값이 안 바뀐다」로만 보이므로 가장 잡기 어려운 고장이다.
*/
func TestStreamedTerminalData_requiresLiveFrames(t *testing.T) {
	s := newTerminalStream()
	s.termMu.Lock()
	s.stateLocked("005930").recordTrade(model.TradeTick{Price: 71500})
	s.termMu.Unlock()

	if _, _, ok := s.StreamedTerminalData("005930"); ok {
		t.Fatal("프레임을 한 번도 못 받았는데 구독 데이터를 내줬다")
	}

	s.mark("005930")
	trades, book, ok := s.StreamedTerminalData("005930")
	if !ok || len(trades) != 1 {
		t.Fatalf("live인데 ok=%v trades=%d", ok, len(trades))
	}
	if book != nil {
		t.Fatal("호가 프레임이 없었는데 호가를 내줬다")
	}

	// 프레임이 끊기면 다시 REST로.
	s.mu.Lock()
	s.lastSeen["005930"] = time.Now().Add(-streamStaleAfter - time.Second)
	s.mu.Unlock()
	if _, _, ok := s.StreamedTerminalData("005930"); ok {
		t.Fatal("프레임이 끊겼는데 여전히 구독 데이터를 내준다 — 화면이 멈춘다")
	}
}

// 소문자로 물어봐도 같은 종목이어야 한다 — 라우팅은 심볼을 대문자로 정규화한다.
func TestStreamedTerminalData_symbolIsCaseInsensitive(t *testing.T) {
	s := newTerminalStream()
	s.mark("AAPL")
	s.termMu.Lock()
	s.stateLocked("AAPL").recordTrade(model.TradeTick{Price: 311})
	s.termMu.Unlock()

	if _, _, ok := s.StreamedTerminalData("aapl"); !ok {
		t.Fatal("소문자 심볼로 못 찾는다")
	}
}

// 버퍼가 비어 있으면 ok=false여야 한다 — 빈 체결 목록으로 화면을 덮으면 안 된다.
func TestStreamedTerminalData_emptyBufferFallsBack(t *testing.T) {
	s := newTerminalStream()
	s.mark("005930")
	s.termMu.Lock()
	s.stateLocked("005930")
	s.termMu.Unlock()

	if _, _, ok := s.StreamedTerminalData("005930"); ok {
		t.Fatal("버퍼가 비었는데 ok=true — 화면의 체결 표가 비워진다")
	}
}

/*
반환값은 **복사본**이어야 한다.

호출자가 받은 슬라이스를 그대로 스냅샷에 실어 캐시로 보내는데, 그 사이에 프레임이
더 오면 같은 배열을 두 goroutine이 만지게 된다.
*/
func TestStreamedTerminalData_returnsCopy(t *testing.T) {
	s := newTerminalStream()
	s.mark("005930")
	s.termMu.Lock()
	s.stateLocked("005930").recordTrade(model.TradeTick{Price: 100})
	s.termMu.Unlock()

	trades, _, _ := s.StreamedTerminalData("005930")
	trades[0].Price = 999

	again, _, _ := s.StreamedTerminalData("005930")
	if again[0].Price != 100 {
		t.Fatal("내부 버퍼가 호출자에게 노출됐다")
	}
}

/*
구독에서 빠진 종목의 버퍼는 정리하되, **방금까지 프레임이 오던 것은 남긴다.**

화면을 잠깐 벗어났다 돌아오는 일이 흔하고, 그때 버퍼가 비어 있으면 체결 표가
한 번 비었다가 다시 찬다.
*/
func TestForgetTerminalState(t *testing.T) {
	s := newTerminalStream()
	s.termMu.Lock()
	s.stateLocked("KEEP")
	s.stateLocked("RECENT")
	s.stateLocked("STALE")
	s.stateLocked("NEVERSEEN")
	s.termMu.Unlock()

	s.mu.Lock()
	s.lastSeen["RECENT"] = time.Now()
	s.lastSeen["STALE"] = time.Now().Add(-streamStaleAfter - time.Minute)
	s.mu.Unlock()

	s.forgetTerminalState(map[string]struct{}{"KEEP": {}})

	s.termMu.Lock()
	defer s.termMu.Unlock()
	if _, ok := s.terminal["KEEP"]; !ok {
		t.Fatal("구독 중인 종목을 지웠다")
	}
	if _, ok := s.terminal["RECENT"]; !ok {
		t.Fatal("방금까지 프레임이 오던 종목을 지웠다 — 되돌아오면 표가 비어 보인다")
	}
	if _, ok := s.terminal["STALE"]; ok {
		t.Fatal("오래된 종목이 안 지워졌다 — 맵이 무한히 자란다")
	}
	if _, ok := s.terminal["NEVERSEEN"]; ok {
		t.Fatal("프레임을 한 번도 못 받은 종목이 안 지워졌다")
	}
}

/*
플러시는 **변경된 것만** 내보내고 dirty를 내려야 한다.

안 내리면 조용한 종목까지 0.5초마다 Redis에 다시 쓴다 — 시청자가 늘수록
쓰기가 비례해 늘고, 그러라고 만든 스로틀이 무의미해진다.
*/
func TestFlushTerminal_clearsDirtyAndSkipsQuietSymbols(t *testing.T) {
	s := newTerminalStream()
	s.quotes = &QuotesService{} // redis=nil → 캐시 쓰기는 no-op, 상태 전이만 본다

	s.termMu.Lock()
	st := s.stateLocked("005930")
	st.recordTrade(model.TradeTick{Price: 71500})
	st.book = &model.OrderBook{Symbol: "005930", Bids: []model.OrderBookLevel{{Price: 71400, Quantity: 10}}}
	st.bookDirty = true
	s.termMu.Unlock()

	s.flushTerminal(t.Context())

	s.termMu.Lock()
	st = s.terminal["005930"]
	if st.tradeDirty || st.bookDirty {
		t.Fatalf("플러시 후에도 dirty (trade=%v book=%v)", st.tradeDirty, st.bookDirty)
	}
	if st.lastDBAt.IsZero() {
		t.Fatal("첫 플러시가 DB로 안 나갔다 — 재기동 시 last-good이 비어 있게 된다")
	}
	firstDB := st.lastDBAt
	s.termMu.Unlock()

	// 아무것도 안 바뀐 채로 다시 플러시하면 DB 시각도 그대로여야 한다.
	s.flushTerminal(t.Context())
	s.termMu.Lock()
	if !s.terminal["005930"].lastDBAt.Equal(firstDB) {
		t.Fatal("변경이 없는데 DB로 다시 나갔다")
	}
	s.termMu.Unlock()
}

// DB 쓰기는 스로틀 간격 안에서는 한 번뿐이어야 한다 — 이 DB는 1 ECPU다.
func TestFlushTerminal_throttlesDBWrites(t *testing.T) {
	s := newTerminalStream()
	s.quotes = &QuotesService{}

	mark := func() {
		s.termMu.Lock()
		s.stateLocked("005930").recordTrade(model.TradeTick{Price: 1})
		s.termMu.Unlock()
	}

	mark()
	s.flushTerminal(t.Context())
	s.termMu.Lock()
	first := s.terminal["005930"].lastDBAt
	s.termMu.Unlock()

	mark()
	s.flushTerminal(t.Context())
	s.termMu.Lock()
	second := s.terminal["005930"].lastDBAt
	s.termMu.Unlock()

	if !second.Equal(first) {
		t.Fatalf("스로틀 안에서 DB로 두 번 나갔다 (%s → %s)", first, second)
	}

	// 간격을 넘기면 다시 나가야 한다.
	//
	// `first`와 비교하지 않는 이유: Windows의 시계 해상도가 ~15ms라 테스트가
	// 그보다 빨리 끝나면 두 `time.Now()`가 **같은 값**이 나온다. 직전에 우리가
	// 직접 밀어 넣은 과거 시각을 기준으로 봐야 판정이 시계에 안 흔들린다.
	staleAt := time.Now().Add(-streamDBFlushEvery - time.Second)
	s.termMu.Lock()
	s.terminal["005930"].lastDBAt = staleAt
	s.termMu.Unlock()
	mark()
	s.flushTerminal(t.Context())
	s.termMu.Lock()
	third := s.terminal["005930"].lastDBAt
	s.termMu.Unlock()
	if !third.After(staleAt) {
		t.Fatal("스로틀 간격을 넘겼는데 DB로 안 나갔다")
	}
}

// 구독이 없으면 스냅샷 조립은 예전 경로 그대로여야 한다.
func TestStreamTerminalSources_noStream(t *testing.T) {
	q := &QuotesService{}
	if _, _, ok := q.streamTerminalSources("005930"); ok {
		t.Fatal("구독이 꺼졌는데 구독 데이터가 있다고 답했다")
	}
}

/*
스냅샷의 각 부분을 어디서 가져올지 — 이 변경에서 가장 판단이 들어간 규칙이다.

틀렸을 때의 고장이 둘 다 조용하다: 너무 적극적이면 죽은 구독의 값을 계속 보여
화면이 멈추고, 너무 소극적이면 REST를 그대로 타 최적화가 통째로 무효가 된다.
*/
func TestPlanTerminalSources(t *testing.T) {
	book := &model.OrderBook{
		Symbol: "005930",
		Bids:   []model.OrderBookLevel{{Price: 71400, Quantity: 10}},
	}
	trades := []model.TradeTick{{Price: 71500}}

	t.Run("구독이 없으면 전부 REST", func(t *testing.T) {
		p := planTerminalSources(false, book, trades)
		if p.anyFromStream() {
			t.Fatal("구독이 죽었는데 구독 데이터를 쓴다 — 화면이 조용히 멈춘다")
		}
	})

	t.Run("구독이 살아 있고 둘 다 차 있으면 전부 구독", func(t *testing.T) {
		p := planTerminalSources(true, book, trades)
		if !p.BookFromStream || !p.TradesFromStream {
			t.Fatalf("%+v — 구독이 주는데도 REST를 탄다", p)
		}
	})

	t.Run("trade만 구독된 경우 호가는 REST", func(t *testing.T) {
		// 슬롯이 모자라면 planTopics가 orderbook을 버린다.
		p := planTerminalSources(true, nil, trades)
		if p.BookFromStream {
			t.Fatal("호가 구독이 없는데 구독에서 가져온다고 했다")
		}
		if !p.TradesFromStream {
			t.Fatal("체결은 구독이 주는데 REST를 탄다")
		}
	})

	t.Run("빈 호가는 구독으로 치지 않는다", func(t *testing.T) {
		// 호가 프레임이 왔는데 양쪽이 다 비어 있으면 화면의 호가창이 빈다.
		p := planTerminalSources(true, &model.OrderBook{Symbol: "005930"}, trades)
		if p.BookFromStream {
			t.Fatal("빈 호가를 구독 데이터로 썼다 — 호가창이 빈 채로 남는다")
		}
	})

	t.Run("체결 버퍼가 비면 체결은 REST", func(t *testing.T) {
		p := planTerminalSources(true, book, nil)
		if p.TradesFromStream {
			t.Fatal("빈 체결 목록으로 화면을 덮는다")
		}
		if !p.BookFromStream {
			t.Fatal("호가는 구독이 주는데 REST를 탄다")
		}
	})
}

/*
`IsFed`는 「조용한 것」과 「끊긴 것」을 갈라야 한다.

거래가 뜸한 종목은 정상적으로도 몇 분씩 프레임이 없다. 그때 REST로 되돌아가면
**바뀌지도 않은 값을 다시 받아 오게 된다** — 실측에서 미국 장 마감 뒤 `source`가
`toss-ws`와 `toss` 사이를 오갔던 것이 이 경우다.
*/
func TestIsFed_quietButConfirmedStaysFed(t *testing.T) {
	s := newTerminalStream()
	s.confirmed = map[string]confirmedTopic{}
	s.OnSubscriptions("ws-1", []toss.StreamTopic{
		{Channel: toss.ChannelTrade, Market: "kr", Symbol: "005930"},
	}, nil)
	s.mark("005930")

	// 프레임 도착 기준(IsLive)은 지났지만 유예 안이다.
	s.mu.Lock()
	s.lastSeen["005930"] = time.Now().Add(-streamStaleAfter - time.Second)
	s.mu.Unlock()

	if s.IsLive("005930") {
		t.Fatal("전제가 틀렸다 — IsLive는 만료돼 있어야 한다")
	}
	if !s.IsFed("005930") {
		t.Fatal("확인된 구독이 서 있는데 REST로 되돌아간다 — 안 바뀐 값을 다시 받는다")
	}

	// 유예까지 넘기면 진짜로 못 믿는다.
	s.mu.Lock()
	s.lastSeen["005930"] = time.Now().Add(-streamQuietGrace - time.Second)
	s.mu.Unlock()
	if s.IsFed("005930") {
		t.Fatal("유예를 넘겼는데 여전히 구독을 믿는다 — 화면이 멈춘 채로 남는다")
	}
}

// 연결이 끊기면 그 연결의 확인은 **즉시** 무효여야 한다.
func TestOnDisconnect_dropsThatConnectionsConfirmations(t *testing.T) {
	s := newTerminalStream()
	s.OnSubscriptions("ws-1", []toss.StreamTopic{
		{Channel: toss.ChannelTrade, Market: "kr", Symbol: "005930"},
	}, nil)
	s.OnSubscriptions("ws-2", []toss.StreamTopic{
		{Channel: toss.ChannelTrade, Market: "us", Symbol: "AAPL"},
	}, nil)
	s.mark("005930")
	s.mark("AAPL")
	s.mu.Lock()
	s.lastSeen["005930"] = time.Now().Add(-streamStaleAfter - time.Second)
	s.lastSeen["AAPL"] = time.Now().Add(-streamStaleAfter - time.Second)
	s.mu.Unlock()

	s.OnDisconnect("ws-1")

	if s.IsFed("005930") {
		t.Fatal("끊긴 연결의 구독을 여전히 믿는다")
	}
	if !s.IsFed("AAPL") {
		t.Fatal("멀쩡한 연결의 구독까지 버렸다 — 그쪽이 REST로 되돌아간다")
	}
}

// 거부된 구독은 확인으로 치면 안 된다 — 그 종목만 영영 안 움직인다.
func TestOnSubscriptions_rejectedIsNotConfirmed(t *testing.T) {
	s := newTerminalStream()
	s.OnSubscriptions("ws-1", nil, []string{"trade:kr:005930"})
	s.mark("005930")
	s.mu.Lock()
	s.lastSeen["005930"] = time.Now().Add(-streamStaleAfter - time.Second)
	s.mu.Unlock()

	if s.IsFed("005930") {
		t.Fatal("거부된 구독을 믿는다 — 폴링이 안 되살아나 그 종목이 멈춘다")
	}
}

/*
확인만 되고 **프레임을 한 번도 못 받은** 종목은 아직 믿으면 안 된다.

구독이 수락돼도 실제로 데이터가 흐르는지는 첫 프레임을 봐야 안다. 그전에 믿으면
빈 버퍼로 화면을 덮는다.
*/
func TestIsFed_confirmedButNoFrameYet(t *testing.T) {
	s := newTerminalStream()
	s.OnSubscriptions("ws-1", []toss.StreamTopic{
		{Channel: toss.ChannelTrade, Market: "kr", Symbol: "005930"},
	}, nil)

	if s.IsFed("005930") {
		t.Fatal("프레임을 한 번도 못 받았는데 구독을 믿는다")
	}
}

// 재선언 시 그 연결의 옛 확인은 통째로 교체돼야 한다 — 선언이 전체 교체 방식이다.
func TestOnSubscriptions_replacesPreviousDeclaration(t *testing.T) {
	s := newTerminalStream()
	s.OnSubscriptions("ws-1", []toss.StreamTopic{
		{Channel: toss.ChannelTrade, Market: "kr", Symbol: "OLD"},
	}, nil)
	s.OnSubscriptions("ws-1", []toss.StreamTopic{
		{Channel: toss.ChannelTrade, Market: "kr", Symbol: "NEW"},
	}, nil)

	s.subMu.RLock()
	defer s.subMu.RUnlock()
	if _, ok := s.confirmed["OLD"]; ok {
		t.Fatal("구독에서 뺀 종목이 확인된 채로 남았다")
	}
	if _, ok := s.confirmed["NEW"]; !ok {
		t.Fatal("새로 확인된 종목이 없다")
	}
}

/*
동시성 — 프레임 수신·플러시·조회·정리가 **동시에** 돈다.

실제 배치가 그렇다: 연결 4개가 각자 goroutine에서 프레임을 넣고, 플러시가
0.5초마다 훑고, 요청 경로가 아무 때나 읽고, 폴러 틱이 정리한다. 이 조합에서
자료구조가 깨지면 증상이 「가끔 값이 이상하다」로만 나와 재현이 거의 불가능하다.

`-race`로 돌려야 의미가 있다(이 머신에는 gcc가 없어 빌더 이미지로 돌린다):

	docker run --rm -v <repo>:/src -w /src/be -e CGO_ENABLED=1 \
	  golang:1.25-bookworm go test -race ./internal/service/
*/
func TestStreamTerminal_concurrentAccess(t *testing.T) {
	s := newTerminalStream()
	s.quotes = &QuotesService{} // redis/DB 없이 상태 전이만 본다

	syms := []string{"005930", "000660", "AAPL", "TSLA"}
	stop := make(chan struct{})
	var wg sync.WaitGroup

	// 프레임 수신 — 연결 수만큼.
	for c := 0; c < 4; c++ {
		wg.Add(1)
		go func(c int) {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				sym := syms[(c+i)%len(syms)]
				s.mark(sym)
				s.termMu.Lock()
				st := s.stateLocked(sym)
				st.recordTrade(model.TradeTick{Price: float64(i), Symbol: sym})
				st.book = &model.OrderBook{
					Symbol: sym,
					Bids:   []model.OrderBookLevel{{Price: float64(i), Quantity: int64(i)}},
				}
				st.bookDirty = true
				s.termMu.Unlock()
			}
		}(c)
	}

	// 플러시.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			s.flushTerminal(t.Context())
		}
	}()

	// 요청 경로.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			trades, book, ok := s.StreamedTerminalData(syms[i%len(syms)])
			if ok && len(trades) > streamTradeBufferSize {
				t.Errorf("버퍼 상한을 넘겼다: %d", len(trades))
				return
			}
			_ = book
		}
	}()

	// 구독 갱신 + 정리.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			s.OnSubscriptions("ws-1", []toss.StreamTopic{
				{Channel: toss.ChannelTrade, Market: "kr", Symbol: syms[i%len(syms)]},
			}, nil)
			s.forgetTerminalState(map[string]struct{}{syms[i%len(syms)]: {}})
			if i%50 == 0 {
				s.OnDisconnect("ws-1")
			}
		}
	}()

	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()
}
