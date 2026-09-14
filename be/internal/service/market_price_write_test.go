package service

import (
	"sync"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
가격 쓰기 중복 제거(RefreshPrices의 lastWrittenPrices) 회귀 테스트.

이 로직이 틀리는 방향은 둘인데 무게가 다르다.

  - 너무 많이 쓴다 → 예전 상태로 돌아갈 뿐이다(낭비).
  - **너무 적게 쓴다 → 가격이 DB에 영영 저장되지 않는다.**

아래 테스트는 두 번째를 막는 데 초점을 둔다 — 값이 바뀌면 반드시 통과시키고,
쓰기에 실패하면 기록을 되돌려 다음 틱이 다시 시도하게 한다.
*/

func TestLastPriceChanged_writesOncePerDistinctValue(t *testing.T) {
	s := &MarketDataStore{}

	if !s.lastPriceChanged("AAPL", 100, model.CurrencyUSD) {
		t.Fatal("첫 관측은 반드시 써야 한다")
	}
	if s.lastPriceChanged("AAPL", 100, model.CurrencyUSD) {
		t.Fatal("같은 값을 다시 쓰면 안 된다 — 이것이 유휴 쓰기의 원인이었다")
	}
	if !s.lastPriceChanged("AAPL", 101, model.CurrencyUSD) {
		t.Fatal("가격이 바뀌었으면 반드시 써야 한다")
	}
	if !s.lastPriceChanged("AAPL", 101, model.CurrencyKRW) {
		t.Fatal("통화가 바뀌었으면 반드시 써야 한다 — 값만 비교하면 통화 정정이 유실된다")
	}
	if s.lastPriceChanged("AAPL", 101, model.CurrencyKRW) {
		t.Fatal("통화까지 같아지면 다시 쓰지 않아야 한다")
	}
}

// 값이 그대로여도 주기가 지나면 다시 쓴다 — 메모리와 DB가 어긋났을 때
// 스스로 복구하는 성질을 값 비교 때문에 잃지 않도록.
func TestLastPriceChanged_rewritesAfterInterval(t *testing.T) {
	s := &MarketDataStore{}

	if !s.lastPriceChanged("AAPL", 100, model.CurrencyUSD) {
		t.Fatal("첫 관측")
	}
	if s.lastPriceChanged("AAPL", 100, model.CurrencyUSD) {
		t.Fatal("주기 안에서는 같은 값을 다시 쓰지 않는다")
	}

	// 마지막 기록 시각을 주기 밖으로 되돌린다.
	s.mu.Lock()
	prev := s.lastWrittenPrices["AAPL"]
	prev.at = prev.at.Add(-lastPriceRewriteInterval - time.Second)
	s.lastWrittenPrices["AAPL"] = prev
	s.mu.Unlock()

	if !s.lastPriceChanged("AAPL", 100, model.CurrencyUSD) {
		t.Fatalf("주기(%s)가 지나면 같은 값이라도 한 번은 다시 써야 한다 — "+
			"이게 없으면 DB가 틀어져도 재기동 전까지 복구되지 않는다", lastPriceRewriteInterval)
	}
	if s.lastPriceChanged("AAPL", 100, model.CurrencyUSD) {
		t.Fatal("다시 쓴 직후에는 주기가 초기화되어야 한다")
	}
}

func TestLastPriceChanged_isPerSymbol(t *testing.T) {
	s := &MarketDataStore{}

	if !s.lastPriceChanged("AAPL", 100, model.CurrencyUSD) {
		t.Fatal("AAPL 첫 관측")
	}
	if !s.lastPriceChanged("MSFT", 100, model.CurrencyUSD) {
		t.Fatal("다른 심볼이 같은 가격이라고 건너뛰면 안 된다")
	}
}

func TestForgetLastPrices_allowsRetryAfterWriteFailure(t *testing.T) {
	s := &MarketDataStore{}

	if !s.lastPriceChanged("AAPL", 100, model.CurrencyUSD) {
		t.Fatal("첫 관측")
	}
	// DB 쓰기가 실패한 상황을 흉내낸다.
	s.forgetLastPrices([]repository.LastPriceRow{
		{Symbol: "AAPL", LastPrice: 100, Currency: model.CurrencyUSD},
	})

	if !s.lastPriceChanged("AAPL", 100, model.CurrencyUSD) {
		t.Fatal("쓰기 실패 후에는 같은 값이라도 다시 시도해야 한다 — " +
			"되돌리지 않으면 그 종목의 가격이 영영 저장되지 않는다")
	}
}

// 두 틱이 겹쳐도 **정확히 한 번**만 통과해야 한다.
//
// 판단과 기록이 갈라져 있으면 둘 다 "안 바뀌었다"로 보고 아무도 쓰지 않는
// 경우가 생긴다. 그쪽이 위험한 실패라 동시성으로 못 박는다.
func TestLastPriceChanged_concurrentCallersWriteExactlyOnce(t *testing.T) {
	s := &MarketDataStore{}

	const goroutines = 64
	var wg sync.WaitGroup
	var mu sync.Mutex
	writes := 0

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if s.lastPriceChanged("AAPL", 100, model.CurrencyUSD) {
				mu.Lock()
				writes++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if writes != 1 {
		t.Fatalf("동시 호출 %d건 중 %d건이 통과했다 — 정확히 1건이어야 한다", goroutines, writes)
	}
}
