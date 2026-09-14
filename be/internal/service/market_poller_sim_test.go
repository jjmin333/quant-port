package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

/*
장중 시뮬레이션.

폴러의 문제는 「어느 종목을 얼마나 자주 부르는가」이고, 그건 단위 테스트로는 잘 안
드러난다 — 실제로 상류를 몇 번 때리는지를 세어야 한다. 그래서 Toss를 흉내내는
httptest 서버를 세우고 **엔드포인트별 호출 수**를 센다.

여기서 고정하는 것은 세 가지다.

 1. 차트 예열(tickSlow)이 랭킹 추적분을 부르지 않는다 — `/candles` 한도(20/s)를
    상시 소진하던 원인.
 2. 가격 배치가 **한 묶음이 실패해도 나머지를 계속 받는다** — 전에는 첫 실패에
    return해서 뒤쪽 종목이 통째로 빠졌다(실측 커버리지 pinned의 4%).
 3. 터미널 갱신이 오래 걸려도 가격 갱신이 자기 주기를 지킨다 — 전에는 한 함수에
    묶여 있어 가격도 8초에 한 번씩만 돌았다.
*/

// fakeToss counts calls per endpoint and can be told to fail specific ones.
type fakeToss struct {
	mu     sync.Mutex
	calls  map[string]int
	symbol map[string]int // /prices 가 실제로 받은 종목 수(누적)

	// failPricesNth — n번째 /prices 호출을 실패시킨다(1-based). 0이면 안 함.
	// failPricesSymbol — 이 종목이 든 묶음은 **매번** 실패시킨다. 재시도로도
	// 복구되지 않는 묶음을 만들 때 쓴다(회차 지정만으로는 재시도가 통과해 버린다).
	failPricesNth    int
	failPricesSymbol string

	// rejectPricesSymbol — 이 종목이 든 묶음에 **400 invalid-request**를 준다.
	// 실제 Toss가 상류 목록에 없는 심볼에 하는 응답이다(2026-09-08 실측, QXO).
	// 500(failPricesSymbol)과 갈라 두는 이유: 400은 재시도로 낫지 않으므로
	// 호출부가 원인 심볼을 골라내는 다른 경로를 타야 한다.
	rejectPricesSymbol string
	pricesSeen       int
}

func newFakeToss() *fakeToss {
	return &fakeToss{calls: map[string]int{}, symbol: map[string]int{}}
}

func (f *fakeToss) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[path]
}

func (f *fakeToss) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-token", "token_type": "Bearer", "expires_in": 3600,
		})
	})

	record := func(path string) {
		f.mu.Lock()
		f.calls[path]++
		f.mu.Unlock()
	}

	mux.HandleFunc("/api/v1/prices", func(w http.ResponseWriter, r *http.Request) {
		record("prices")
		syms := strings.Split(r.URL.Query().Get("symbols"), ",")

		f.mu.Lock()
		f.pricesSeen++
		nth := f.pricesSeen
		shouldFail := f.failPricesNth > 0 && nth == f.failPricesNth
		if f.failPricesSymbol != "" {
			for _, s := range syms {
				if s == f.failPricesSymbol {
					shouldFail = true
				}
			}
		}
		shouldReject := false
		if f.rejectPricesSymbol != "" {
			for _, s := range syms {
				if s == f.rejectPricesSymbol {
					shouldReject = true
				}
			}
		}
		if !shouldFail && !shouldReject {
			for _, s := range syms {
				if s != "" {
					f.symbol[s]++
				}
			}
		}
		f.mu.Unlock()

		if shouldReject {
			// 상류가 **요청 자체를 거절**한다 — 묶음에 든 심볼 하나 때문에 전부 400.
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
				"code": "invalid-request", "message": "요청 필드가 올바르지 않습니다.",
			}})
			return
		}

		if shouldFail {
			// 레이트리밋이 **아닌** 일시적 오류 — 이 묶음만 건너뛰고 계속해야 한다.
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "boom"}})
			return
		}

		out := make([]toss.PriceResponse, 0, len(syms))
		for _, s := range syms {
			if s == "" {
				continue
			}
			out = append(out, toss.PriceResponse{Symbol: s, LastPrice: "1000", Currency: toss.CurrencyKRW})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": out})
	})

	for _, p := range []string{"candles", "orderbook", "trades", "stocks"} {
		path := p
		mux.HandleFunc("/api/v1/"+path, func(w http.ResponseWriter, r *http.Request) {
			record(path)
			_ = json.NewEncoder(w).Encode(map[string]any{"result": []any{}})
		})
	}

	// 그 밖의 경로도 빈 응답으로 받아 준다 — 테스트가 라우팅 누락으로 죽지 않게.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"result": []any{}})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// memTokenStore는 Redis 없이 토큰을 들고 있는다.
type memTokenStore struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (s *memTokenStore) Get(_ context.Context, k string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.m[k]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("miss")
}

func (s *memTokenStore) Set(_ context.Context, k string, v []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string][]byte{}
	}
	s.m[k] = v
	return nil
}

func (s *memTokenStore) Delete(_ context.Context, k string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, k)
	return nil
}

func newFakeTossClient(t *testing.T, srv *httptest.Server) *toss.Client {
	t.Helper()
	c, err := toss.NewClientForCredentials(srv.URL, "id", "secret", &memTokenStore{}, "test:token")
	if err != nil {
		t.Fatalf("fake toss client: %v", err)
	}
	return c
}

/*
가격 배치는 한 묶음이 실패해도 나머지를 계속 받아야 한다.

전에는 첫 실패에 `return`해서, 200종목씩 나눈 묶음 중 두 번째가 실패하면 뒤쪽
종목이 그 틱에서 가격을 아예 못 받았다. 실측(2026-08-23) 커버리지가 pinned의
4%였던 원인 중 하나다.
*/
func TestRefreshPrices_continuesAfterFailedBatch(t *testing.T) {
	f := newFakeToss()
	// **계속** 실패하는 묶음을 만든다. 회차로만 지정하면 클라이언트의 일시적
	// 재시도가 통과해 버려, 여기서 보려는 「한 묶음이 죽어도 나머지는 간다」가
	// 검증되지 않는다.
	f.failPricesSymbol = "S0250" // 두 번째 묶음(200~399)에 든다
	srv := f.server(t)

	store := &MarketDataStore{toss: newFakeTossClient(t, srv)}

	// 200종목/호출이므로 500종목이면 3묶음(200+200+100)이다.
	symbols := make([]string, 0, 500)
	for i := 0; i < 500; i++ {
		symbols = append(symbols, fmt.Sprintf("S%04d", i))
	}

	store.RefreshPrices(context.Background(), symbols)

	f.mu.Lock()
	covered := len(f.symbol)
	f.mu.Unlock()

	// 실패한 두 번째 묶음(200종목)만 빠지고 나머지 300종목은 받아야 한다.
	if covered != 300 {
		t.Fatalf("가격을 받은 종목 %d개 — 실패 묶음만 빠진 300개여야 한다", covered)
	}
}

/*
일시적으로 실패한 묶음은 **재시도로 복구돼야** 한다.

위 테스트가 보장하는 것은 「죽은 묶음이 나머지를 끌고 내려가지 않는다」이고,
이쪽은 「잠깐 흔들린 묶음은 아예 안 빠진다」이다. 전에는 500 한 번에 그 묶음의
200종목이 그 틱에서 통째로 가격을 잃었다.
*/
func TestRefreshPrices_transientBatchFailureRecovers(t *testing.T) {
	f := newFakeToss()
	f.failPricesNth = 2 // 두 번째 호출만 실패 — 재시도는 성공한다
	srv := f.server(t)

	store := &MarketDataStore{toss: newFakeTossClient(t, srv)}

	symbols := make([]string, 0, 500)
	for i := 0; i < 500; i++ {
		symbols = append(symbols, fmt.Sprintf("S%04d", i))
	}

	store.RefreshPrices(context.Background(), symbols)

	f.mu.Lock()
	covered := len(f.symbol)
	f.mu.Unlock()

	if covered != 500 {
		t.Fatalf("가격을 받은 종목 %d개 — 일시적 실패는 재시도로 복구돼 500개여야 한다", covered)
	}
}

/*
캔들 갱신은 **지금 보고 있는 종목**만 부른다.

랭킹 추적분(실측 3,652종목)까지 부르면 `/candles`를 상시 소진하고, 그 버킷은
사용자가 차트를 처음 열 때 쓰는 것과 같다 — 배경이 전경을 밀어낸다.

아무도 안 보고 있으면 대상은 **0개**여야 한다. 예전 구현은 이 자리에서
`cared`(관심 ∪ 보유)를 더했는데, 그건 운영에서 항상 비어 있었으므로
(폴러 컨텍스트에 사용자가 없다) 실제 동작과 테스트가 어긋나 있었다.
*/
func TestTickSlow_doesNotFetchRankingTrackedSymbols(t *testing.T) {
	p := &marketPoller{
		pinned: []string{"RANK1", "RANK2", "RANK3", "WATCH1", "HELD1"},
	}

	targets := p.capActiveTargets(map[string]struct{}{}, marketSlowSymbolCap, &p.slowCursor)

	if len(targets) != 0 {
		t.Fatalf("아무도 안 보는데 대상이 %v — pinned는 캔들 대상이 아니다", targets)
	}
	for _, sym := range targets {
		if strings.HasPrefix(sym, "RANK") {
			t.Fatalf("랭킹 추적분 %s가 캔들 대상에 들어갔다 — /candles 한도를 태운다", sym)
		}
	}

	// 랭킹 추적분도 pinned에는 남아 있어야 한다 — 가격은 배치로 계속 받는다.
	found := false
	for _, sym := range p.pinned {
		if sym == "RANK1" {
			found = true
		}
	}
	if !found {
		t.Fatal("랭킹 추적분이 pinned에서도 빠졌다 — 목록 화면의 가격이 멈춘다")
	}
}

/*
터미널 갱신이 길어져도 가격 갱신은 자기 주기를 지켜야 한다.

전에는 둘이 한 함수(tickFast)에 있어 `fastBusy` 하나를 공유했다. 터미널 갱신이
8초 걸리면 가격도 8초에 한 번씩만 돌았다.
*/
func TestPriceTick_notBlockedByBusyTerminalTick(t *testing.T) {
	p := &marketPoller{}

	// 터미널 틱이 오래 붙잡고 있는 상황.
	if !p.fastBusy.CompareAndSwap(false, true) {
		t.Fatal("fastBusy 초기 상태가 이미 true")
	}
	defer p.fastBusy.Store(false)

	// 가격 틱은 자기 플래그를 쓰므로 진입할 수 있어야 한다.
	if !p.priceBusy.CompareAndSwap(false, true) {
		t.Fatal("터미널 틱이 도는 동안 가격 틱이 막혔다 — 두 플래그가 갈라져 있어야 한다")
	}
	p.priceBusy.Store(false)

	// 반대로 가격 틱끼리는 겹치지 않아야 한다(중복 요청 방지).
	p.priceBusy.Store(true)
	if p.priceBusy.CompareAndSwap(false, true) {
		t.Fatal("가격 틱이 자기 자신과 겹쳤다")
	}
	p.priceBusy.Store(false)
}
