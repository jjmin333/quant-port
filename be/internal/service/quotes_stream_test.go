package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

// 국내는 6자리 숫자, 미국은 알파벳 티커. 스펙이 codes에 그 시장 심볼만
// 넣으라고 해서 trade:kr / trade:us를 갈라야 한다.
func TestStreamMarketFor(t *testing.T) {
	for sym, want := range map[string]string{
		"005930": "kr", "000660": "kr",
		"AAPL": "us", "BRK.B": "us",
	} {
		if got := streamMarketFor(sym); got != want {
			t.Fatalf("%s → %s want %s", sym, got, want)
		}
	}
}

/*
슬롯 예산 — 구독은 **채널×심볼**로 세므로 종목당 trade+orderbook이면 2슬롯이다.

예산을 넘기면 trade를 우선한다. 체결가는 현재가이자 형성봉의 재료라 한 슬롯이
주는 값이 가장 크고, 호가창은 상세 화면에서도 보는 사람이 적다.
*/
func TestQuoteStreamService_slotBudget(t *testing.T) {
	s := &QuoteStreamService{lastSeen: map[string]time.Time{}}
	total := StreamConnections * StreamSlotsPerConn

	// 예산의 절반 이하면 전 종목이 trade+orderbook을 받는다.
	few := make([]string, 0, 20)
	for i := 0; i < 20; i++ {
		few = append(few, "00000"+string(rune('A'+i)))
	}
	trades, books := countTopics(s.planTopics(few, few))
	if trades != 20 || books != 20 {
		t.Fatalf("여유 있을 때 trade=%d book=%d — 둘 다 20이어야 한다", trades, books)
	}

	// 종목 수가 예산 전체와 같으면 orderbook은 포기하고 trade만 남는다.
	many := make([]string, 0, total)
	for i := 0; i < total; i++ {
		many = append(many, "SYM"+string(rune('A'+i%26))+string(rune('A'+i/26)))
	}
	trades, books = countTopics(s.planTopics(many, many))
	if trades+books > total {
		t.Fatalf("슬롯 %d개 사용 — 상한 %d를 넘었다", trades+books, total)
	}
	if books != 0 {
		t.Fatalf("예산이 꽉 찼는데 orderbook %d개 — trade를 우선해야 한다", books)
	}
}

// IsLive는 「구독을 선언했다」가 아니라 「최근 프레임이 왔다」를 본다 —
// LOSSY 드롭·구독 거부·연결 장애 때 폴링이 되살아나야 하기 때문이다.
func TestQuoteStreamService_isLiveExpires(t *testing.T) {
	s := &QuoteStreamService{lastSeen: map[string]time.Time{}}

	if s.IsLive("005930") {
		t.Fatal("프레임을 한 번도 못 받았는데 live다 — 폴링이 꺼져 화면이 멈춘다")
	}
	s.mark("005930")
	if !s.IsLive("005930") {
		t.Fatal("방금 프레임을 받았는데 live가 아니다")
	}

	s.mu.Lock()
	s.lastSeen["005930"] = time.Now().Add(-streamStaleAfter - time.Second)
	s.mu.Unlock()
	if s.IsLive("005930") {
		t.Fatal("프레임이 끊겼는데 여전히 live다 — 폴링이 안 되살아난다")
	}
}

// 구독이 꺼져 있으면 폴링 대상이 그대로여야 한다.
func TestFilterStreamedSymbols_noStreamIsPassthrough(t *testing.T) {
	q := &QuotesService{}
	in := []string{"005930", "000660"}
	if got := q.filterStreamedSymbols(in); len(got) != 2 {
		t.Fatalf("구독이 꺼졌는데 %d개로 줄었다 — 폴링이 굶는다", len(got))
	}
}

func countTopics(topics []toss.StreamTopic) (trades, books int) {
	for _, t := range topics {
		switch t.Channel {
		case toss.ChannelTrade:
			trades++
		case toss.ChannelOrderbook:
			books++
		}
	}
	return
}
