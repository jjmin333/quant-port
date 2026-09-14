package service

import (
	"context"
	"sync"
	"testing"
	"time"
)

/*
같은 심볼 집합에 대한 동시 갱신이 하나로 합쳐지는지 본다.

이 테스트가 지키는 것은 **사용자 수에 비례하던 유일한 상류 경로**다. 캐시가 식은
목록을 여러 사용자가 동시에 열면 예전에는 각자 Toss를 불렀다.

상류를 실제로 부르지 않고 조정자만 검증한다 — `RefreshPrices`의 나머지는 Toss
클라이언트가 필요하고, 여기서 확인할 것은 「몇 번 실행되는가」뿐이다.
*/
func TestPriceRefreshCoordinator_collapsesSameSet(t *testing.T) {
	c := newQuoteRefreshCoordinator()

	var mu sync.Mutex
	runs := 0
	release := make(chan struct{})

	key := priceRefreshKey([]string{"005930", "000660"})

	var wg sync.WaitGroup

	/*
		**소유자를 먼저 세운 뒤에 나머지를 붙인다.**

		20개를 한꺼번에 띄우면 소유자가 일을 끝내고 키를 비운 뒤에야 도착하는
		goroutine이 생겨서, 그것이 다시 소유자가 된다 — 합쳐지지 않은 것이 아니라
		**애초에 동시가 아니었던 것**이다(그렇게 짰다가 20건 중 18건이 실행됐다).
		소유자가 확실히 자리를 잡은 뒤에 나머지를 보내야 「동시」를 시험하게 된다.
	*/
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.doSync(context.Background(), key, func(context.Context) {
			mu.Lock()
			runs++
			mu.Unlock()
			<-release // 나머지가 다 도착할 때까지 키를 붙잡고 있는다
		})
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		started := runs
		mu.Unlock()
		if started >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("소유자가 시작되지 않았다")
		}
		time.Sleep(time.Millisecond)
	}

	entered := make(chan struct{}, 19)
	for i := 0; i < 19; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			entered <- struct{}{}
			c.doSync(context.Background(), key, func(context.Context) {
				mu.Lock()
				runs++
				mu.Unlock()
			})
		}()
	}
	for i := 0; i < 19; i++ {
		<-entered
	}
	// 19개가 전부 doSync에 들어간 뒤에 소유자를 풀어 준다.
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if runs != 1 {
		t.Fatalf("동시 20건이 %d번 실행됐다 — 1번이어야 한다(상류 호출이 사용자 수에 비례한다)", runs)
	}
}

// 심볼 순서가 달라도 같은 집합이면 같은 키여야 한다 — 목록마다 순위 순서가
// 미묘하게 다를 수 있는데, 정렬하지 않으면 같은 집합이 안 합쳐진다.
func TestPriceRefreshKey_orderIndependent(t *testing.T) {
	a := priceRefreshKey([]string{"AAPL", "TSLA", "NVDA"})
	b := priceRefreshKey([]string{"NVDA", "AAPL", "TSLA"})
	if a != b {
		t.Fatalf("순서만 다른 같은 집합의 키가 다르다: %s vs %s", a, b)
	}

	c := priceRefreshKey([]string{"AAPL", "TSLA"})
	if a == c {
		t.Fatal("다른 집합이 같은 키를 받았다 — 서로를 막는다")
	}
}
