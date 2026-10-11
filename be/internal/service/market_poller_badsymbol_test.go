package service

import (
	"context"
	"fmt"
	"testing"
	"time"
)

/*
불량 심볼 하나가 묶음 전체의 가격을 떨어뜨리면 안 된다.

2026-09-08 실측: 사용자가 해외 종목 QXO를 열자 폴러 묶음(40종목)이 통째로
`400 invalid-request`를 받아 **국내 39종목의 시세가 그 틱마다 사라졌다.** 400은
재시도로 낫지 않으므로 같은 요청이 초당 2회씩 계속 나갔고 Toss `MARKET_DATA`
버킷을 태웠다. 원인 심볼을 골라 격리하는 것이 유일한 출구다.
*/
func TestRefreshPrices_isolatesRejectedSymbol(t *testing.T) {
	f := newFakeToss()
	f.rejectPricesSymbol = "QXO"
	srv := f.server(t)

	store := &MarketDataStore{toss: newFakeTossClient(t, srv)}

	symbols := make([]string, 0, 40)
	for i := 0; i < 39; i++ {
		symbols = append(symbols, fmt.Sprintf("S%04d", i))
	}
	symbols = append(symbols, "QXO")

	store.RefreshPrices(context.Background(), symbols)

	f.mu.Lock()
	covered := len(f.symbol)
	_, gotBad := f.symbol["QXO"]
	f.mu.Unlock()

	if gotBad {
		t.Fatalf("거절당한 QXO의 가격이 채워졌다 — 그럴 수 없다")
	}
	if covered != 39 {
		t.Fatalf("가격을 받은 종목 %d개 — QXO만 빠진 39개여야 한다", covered)
	}
}

/*
격리된 심볼은 **다음 틱부터 호출에 실려 나가지 않는다.**

이게 이번 사고의 핵심이다. 원인을 골라내기만 하고 기억하지 않으면 매 틱마다
이등분 탐색을 다시 치르고, 상류 호출 수는 오히려 늘어난다.
*/
func TestRefreshPrices_quarantinedSymbolIsNotSentAgain(t *testing.T) {
	f := newFakeToss()
	f.rejectPricesSymbol = "QXO"
	srv := f.server(t)

	store := &MarketDataStore{toss: newFakeTossClient(t, srv)}

	symbols := make([]string, 0, 40)
	for i := 0; i < 39; i++ {
		symbols = append(symbols, fmt.Sprintf("S%04d", i))
	}
	symbols = append(symbols, "QXO")

	// 첫 틱 — 이등분 탐색으로 QXO를 찾아 격리한다.
	store.RefreshPrices(context.Background(), symbols)
	firstTickCalls := f.count("prices")

	// 둘째 틱 — 격리가 앞에서 걸러 주므로 호출은 묶음 하나뿐이어야 한다.
	store.RefreshPrices(context.Background(), symbols)
	secondTickCalls := f.count("prices") - firstTickCalls

	if secondTickCalls != 1 {
		t.Fatalf("둘째 틱의 /prices 호출 %d회 — 격리가 걸러 1회여야 한다 (첫 틱 %d회)",
			secondTickCalls, firstTickCalls)
	}

	f.mu.Lock()
	covered := len(f.symbol)
	f.mu.Unlock()
	if covered != 39 {
		t.Fatalf("가격을 받은 종목 %d개 — 39개여야 한다", covered)
	}
}

/*
이등분 탐색의 비용은 **한 번만** 치른다.

40종목에 불량 1개면 대략 log2(40)×2 수준이고, 상한을 넉넉히 잡아도 20회를 넘지
않아야 한다. 지금 이 자리에 상한을 박아 두는 이유는, 탐색이 선형으로 퇴화하면
(심볼마다 한 번씩) 200종목 묶음에서 상류 한도를 그대로 태우기 때문이다.
*/
func TestRefreshPrices_isolationCostIsBounded(t *testing.T) {
	f := newFakeToss()
	f.rejectPricesSymbol = "QXO"
	srv := f.server(t)

	store := &MarketDataStore{toss: newFakeTossClient(t, srv)}

	symbols := make([]string, 0, 40)
	for i := 0; i < 39; i++ {
		symbols = append(symbols, fmt.Sprintf("S%04d", i))
	}
	symbols = append(symbols, "QXO")

	store.RefreshPrices(context.Background(), symbols)

	if got := f.count("prices"); got > 20 {
		t.Fatalf("불량 1개를 골라내는 데 /prices %d회 — 20회 이하여야 한다", got)
	}
}

/*
격리는 **만료된다.**

상장·거래재개처럼 나중에 유효해지는 심볼이 있어 영구 제외는 틀린다. 만료 시각이
지난 항목은 다시 실려 나가야 한다.
*/
func TestRefreshPrices_quarantineExpires(t *testing.T) {
	f := newFakeToss()
	srv := f.server(t)

	store := &MarketDataStore{toss: newFakeTossClient(t, srv)}

	// 이미 만료된 격리를 심어 둔다.
	store.badPriceSymbols = map[string]time.Time{
		"QXO": time.Now().Add(-time.Minute),
	}

	kept := store.dropQuarantinedSymbols([]string{"S0001", "QXO"})
	if len(kept) != 2 {
		t.Fatalf("만료된 격리가 남아 심볼을 걸렀다: %v", kept)
	}

	// 아직 유효한 격리는 걸러야 한다.
	store.badPriceSymbols["QXO"] = time.Now().Add(time.Hour)
	kept = store.dropQuarantinedSymbols([]string{"S0001", "QXO"})
	if len(kept) != 1 || kept[0] != "S0001" {
		t.Fatalf("유효한 격리가 안 걸러졌다: %v", kept)
	}
}

// dropQuarantinedSymbols는 호출부의 슬라이스를 건드리면 안 된다 — 폴러는 같은
// 목록을 여러 곳에서 돌려 쓴다.
func TestDropQuarantinedSymbols_doesNotMutateInput(t *testing.T) {
	store := &MarketDataStore{
		badPriceSymbols: map[string]time.Time{"QXO": time.Now().Add(time.Hour)},
	}

	in := []string{"S0001", "QXO", "S0002"}
	_ = store.dropQuarantinedSymbols(in)

	want := []string{"S0001", "QXO", "S0002"}
	for i := range want {
		if in[i] != want[i] {
			t.Fatalf("입력 슬라이스가 바뀌었다: %v (원본 %v)", in, want)
		}
	}
}

// 404 stock-not-found도 400처럼 격리한다 — 전에는 그 묶음이 통째로 실패로 넘어갔다.
func TestRefreshPrices_isolatesNotFoundSymbol(t *testing.T) {
	f := newFakeToss()
	f.rejectPricesSymbol = "ZZZZ"
	f.rejectAs404 = true
	store := &MarketDataStore{toss: newFakeTossClient(t, f.server(t))}

	store.RefreshPrices(context.Background(), []string{"S0001", "S0002", "ZZZZ"})

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.symbol) != 2 {
		t.Fatalf("가격을 받은 종목 %d개 — ZZZZ만 빠진 2개여야 한다", len(f.symbol))
	}
}

// 스펙 밖 심볼(BAC^B·BRK/B)은 상류에 보내지 않는다 — 보내면 400으로 이등분 탐색을 치른다.
func TestRefreshPrices_skipsCaretSymbols(t *testing.T) {
	f := newFakeToss()
	store := &MarketDataStore{toss: newFakeTossClient(t, f.server(t))}

	store.RefreshPrices(context.Background(), []string{"BAC", "BAC^B", "BRK/B"})

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.symbol) != 1 || f.symbol["BAC"] != 1 {
		t.Fatalf("BAC만 보내야 한다: %v", f.symbol)
	}
}
