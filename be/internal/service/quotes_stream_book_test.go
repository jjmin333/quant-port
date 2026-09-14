package service

import (
	"testing"
	"time"
)

/*
호가는 **호가창을 실제로 연 종목에만** 붙는다.

이 규칙이 웹소켓 슬롯 절반을 돌려준다. 예전에는 상세 화면에 들어가기만 하면
호가까지 구독해서 종목당 2슬롯을 썼는데, 실측(2026-09-02) 프레임 수가 호가
40,667 대 체결 9,461로 **호가가 4.3배**였다 — 대부분 아무도 안 보는 데이터였다.
*/
func TestPlanTopics_bookOnlyForOpenOrderBooks(t *testing.T) {
	s := &QuoteStreamService{lastSeen: map[string]time.Time{}}

	symbols := []string{"005930", "000660", "AAPL"}
	book := []string{"000660"} // 호가창을 연 것은 하나뿐

	trades, books := countTopics(s.planTopics(symbols, book))
	if trades != 3 {
		t.Fatalf("체결 구독 %d개 — 전 종목(3)이어야 한다", trades)
	}
	if books != 1 {
		t.Fatalf("호가 구독 %d개 — 호가창을 연 1개여야 한다", books)
	}
}

// 아무도 호가창을 안 열었으면 호가 구독은 0이고, 같은 예산으로 종목 수는 두 배가 된다.
func TestPlanTopics_noBookInterestDoublesCapacity(t *testing.T) {
	s := &QuoteStreamService{lastSeen: map[string]time.Time{}}
	total := StreamConnections * StreamSlotsPerConn

	symbols := make([]string, 0, total)
	for i := 0; i < total; i++ {
		symbols = append(symbols, "SYM"+string(rune('A'+i%26))+string(rune('A'+i/26)))
	}

	trades, books := countTopics(s.planTopics(symbols, nil))
	if books != 0 {
		t.Fatalf("호가창을 연 종목이 없는데 호가 구독 %d개", books)
	}
	if trades != total {
		t.Fatalf("체결 구독 %d개 — 예산 %d를 다 써서 종목 %d개를 담아야 한다", trades, total, total)
	}
}

/*
호가창을 연 종목이 예산보다 많으면 **체결을 먼저 채우고** 남는 슬롯만 호가에 준다.

체결가는 현재가이자 형성봉의 재료라 한 슬롯이 주는 값이 가장 크다 — 예산이
모자랄 때 버릴 것은 호가다.
*/
func TestPlanTopics_tradeWinsWhenBudgetTight(t *testing.T) {
	s := &QuoteStreamService{lastSeen: map[string]time.Time{}}
	total := StreamConnections * StreamSlotsPerConn

	symbols := make([]string, 0, total)
	for i := 0; i < total; i++ {
		symbols = append(symbols, "SYM"+string(rune('A'+i%26))+string(rune('A'+i/26)))
	}

	// 전원이 호가창을 열었다고 해도 예산이 꽉 차 있으면 호가는 못 받는다.
	trades, books := countTopics(s.planTopics(symbols, symbols))
	if trades+books > total {
		t.Fatalf("슬롯 %d개 사용 — 상한 %d를 넘었다", trades+books, total)
	}
	if books != 0 {
		t.Fatalf("예산이 꽉 찼는데 호가 %d개 — 체결을 우선해야 한다", books)
	}
}
