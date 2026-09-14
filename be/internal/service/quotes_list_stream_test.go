package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func TestParseListStreamSymbolsNormalizesAndDedupes(t *testing.T) {
	got := ParseListStreamSymbols(" aapl , MSFT,aapl ,, 005930 ")
	want := []string{"AAPL", "MSFT", "005930"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// 상한이 없으면 잘못 만든 요청 하나가 전 종목을 구독한다.
func TestParseListStreamSymbolsCapsLength(t *testing.T) {
	raw := ""
	for i := 0; i < listStreamSymbolMax+50; i++ {
		if i > 0 {
			raw += ","
		}
		raw += "S" + string(rune('A'+i%26)) + string(rune('a'+i/26))
	}
	if got := len(ParseListStreamSymbols(raw)); got != listStreamSymbolMax {
		t.Errorf("length = %d, want %d", got, listStreamSymbolMax)
	}
}

func TestParseListStreamSymbolsEmptyInput(t *testing.T) {
	if got := ParseListStreamSymbols("  , ,"); len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

/*
한 채널이 심볼 여러 개를 받는다 — 이것이 이 허브의 존재 이유다. 심볼마다 채널을
만들어 합치면 뷰어당 고루틴이 심볼 수만큼 늘어난다.
*/
func TestListPriceHubOneChannelServesEverySymbol(t *testing.T) {
	hub := newListPriceHub()
	ch, cancel := hub.SubscribeMany([]string{"AAPL", "MSFT"})
	defer cancel()

	hub.Publish(model.ListPriceEvent{Symbol: "AAPL", Price: 1})
	hub.Publish(model.ListPriceEvent{Symbol: "MSFT", Price: 2})

	got := map[string]float64{}
	for i := 0; i < 2; i++ {
		select {
		case ev := <-ch:
			got[ev.Symbol] = ev.Price
		case <-time.After(time.Second):
			t.Fatalf("이벤트 %d개만 받았다: %v", i, got)
		}
	}
	if got["AAPL"] != 1 || got["MSFT"] != 2 {
		t.Errorf("got %v", got)
	}
}

// 구독하지 않은 종목은 오지 않는다 — 목록마다 자기 종목만 받아야 대역폭이 산다.
func TestListPriceHubIgnoresUnsubscribedSymbols(t *testing.T) {
	hub := newListPriceHub()
	ch, cancel := hub.SubscribeMany([]string{"AAPL"})
	defer cancel()

	hub.Publish(model.ListPriceEvent{Symbol: "TSLA", Price: 9})
	hub.Publish(model.ListPriceEvent{Symbol: "AAPL", Price: 1})

	select {
	case ev := <-ch:
		if ev.Symbol != "AAPL" {
			t.Errorf("받은 심볼 = %q, want AAPL", ev.Symbol)
		}
	case <-time.After(time.Second):
		t.Fatal("아무것도 못 받았다")
	}
}

func TestListPriceHubIsCaseInsensitive(t *testing.T) {
	hub := newListPriceHub()
	ch, cancel := hub.SubscribeMany([]string{"aapl"})
	defer cancel()

	hub.Publish(model.ListPriceEvent{Symbol: "AAPL", Price: 7})
	select {
	case ev := <-ch:
		if ev.Price != 7 {
			t.Errorf("price = %v", ev.Price)
		}
	case <-time.After(time.Second):
		t.Fatal("대소문자가 다르면 못 받는다 — 구독과 발행의 키가 어긋났다")
	}
}

/*
취소는 모든 키에서 빼고 **한 번만** 닫는다. 두 번 닫으면 패닉이고, 그 패닉은
클라이언트가 연결을 끊는 평범한 순간에 서버에서 터진다.
*/
func TestListPriceHubCancelIsIdempotentAndUnregisters(t *testing.T) {
	hub := newListPriceHub()
	_, cancel := hub.SubscribeMany([]string{"AAPL", "MSFT"})

	if !hub.HasSubscribers("AAPL") || !hub.HasSubscribers("MSFT") {
		t.Fatal("구독 직후인데 구독자가 없다고 한다")
	}
	cancel()
	cancel() // 두 번 불러도 패닉이 나면 안 된다.

	if hub.HasSubscribers("AAPL") || hub.HasSubscribers("MSFT") {
		t.Error("취소했는데 구독자가 남아 있다")
	}
	// 남은 키가 없으면 맵에서 통째로 지워야 한다 — 안 그러면 종목 수만큼 빈
	// 맵이 프로세스 수명 내내 쌓인다.
	if len(hub.subs) != 0 {
		t.Errorf("subs = %v, want 빈 맵", hub.subs)
	}
}

/*
HasSubscribers가 있는 이유: 가격 캐시 쓰기는 추적 종목 **전체**에 2초마다 돈다.
아무도 안 보는 종목의 이벤트를 만들어 빈 맵에 던지면 할당만 남는다.
*/
func TestListPriceHubHasSubscribersGatesPublishing(t *testing.T) {
	hub := newListPriceHub()
	if hub.HasSubscribers("AAPL") {
		t.Error("아무도 구독 안 했는데 true")
	}
	_, cancel := hub.SubscribeMany([]string{"AAPL"})
	defer cancel()
	if !hub.HasSubscribers("aapl") {
		t.Error("소문자로 물으면 못 찾는다 — 발행자가 정규화 전에 물을 수 있다")
	}
}

/*
느린 구독자가 발행자를 멈추면 안 된다. 버퍼가 차면 **그 이벤트를 버린다** —
가격은 다음 틱에 어차피 다시 온다.
*/
func TestListPriceHubDropsInsteadOfBlocking(t *testing.T) {
	hub := newListPriceHub()
	_, cancel := hub.SubscribeMany([]string{"AAPL"})
	defer cancel()

	done := make(chan struct{})
	go func() {
		for i := 0; i < listPriceChanBuffer*3; i++ {
			hub.Publish(model.ListPriceEvent{Symbol: "AAPL", Price: float64(i)})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("발행이 막혔다 — 느린 구독자 하나가 폴러를 세운다")
	}
}

/*
마감이면 **열자마자 끊는다** (2026-09-04 사용자 추가 요청).

`marketStore`가 없으면 달력이 비고, `detectMarketSession`은 그때 "closed"를 준다 —
마감 상태를 만드는 가장 짧은 길이라 여기서 그걸 쓴다.

검사하는 것 셋: ① 응답이 곧바로 끝난다(연결을 붙잡지 않는다) ② `session` 이벤트가
하나 나간다 ③ 가격 이벤트는 하나도 없다.
*/
func TestListPriceStreamClosesImmediatelyWhenMarketIsClosed(t *testing.T) {
	svc := &QuotesService{listPriceHub: newListPriceHub()}

	req := httptest.NewRequest(http.MethodGet, "/quotes/prices/stream?symbols=005930", nil)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		svc.ServeListPriceStream(rec, req, ListPriceStreamParams{
			Symbols:   []string{"005930"},
			AssetType: "domestic",
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("마감인데 연결을 붙잡고 있다 — 다음 개장까지 빈 파이프가 남는다")
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: session") {
		t.Errorf("session 이벤트가 없다 — 화면이 언제 다시 붙을지 모른다: %q", body)
	}
	if strings.Contains(body, "event: price") || strings.Contains(body, "event: snapshot") {
		t.Errorf("마감인데 가격을 보냈다: %q", body)
	}
	if !strings.Contains(body, `"marketSession":"closed"`) {
		t.Errorf("세션이 closed로 안 적혔다: %q", body)
	}
}

// 구독은 끝나면서 반드시 풀린다 — 안 그러면 마감 때마다 허브에 유령 채널이 쌓인다.
func TestListPriceStreamUnsubscribesOnClose(t *testing.T) {
	svc := &QuotesService{listPriceHub: newListPriceHub()}
	req := httptest.NewRequest(http.MethodGet, "/quotes/prices/stream?symbols=005930", nil)

	svc.ServeListPriceStream(httptest.NewRecorder(), req, ListPriceStreamParams{
		Symbols:   []string{"005930"},
		AssetType: "domestic",
	})

	if svc.listPriceHub.HasSubscribers("005930") {
		t.Error("스트림이 끝났는데 구독이 남아 있다")
	}
}

// 심볼이 없으면 400이다. 빈 스트림을 열어 주면 화면은 「붙었다」고 믿고 폴링을 끈다.
func TestListPriceStreamRejectsEmptySymbols(t *testing.T) {
	svc := &QuotesService{listPriceHub: newListPriceHub()}
	rec := httptest.NewRecorder()
	svc.ServeListPriceStream(rec, httptest.NewRequest(http.MethodGet, "/x", nil),
		ListPriceStreamParams{AssetType: "us"})

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

/*
「모른다」를 「닫혔다」로 굳히지 않는다 — 이건 위 테스트의 반대편이다.

`listStreamClock`은 자산 구분이 비면 국내로 본다. 그 기본값이 사라지면 빈 문자열이
`detectMarketSession`에 그대로 들어가 엉뚱한 갈래를 탄다.
*/
func TestListStreamClockDefaultsToDomestic(t *testing.T) {
	svc := &QuotesService{}
	blank := svc.listStreamClock(context.Background(), "  ")
	domestic := svc.listStreamClock(context.Background(), "domestic")
	if blank.Session != domestic.Session {
		t.Errorf("빈 자산 구분 = %q, 국내 = %q — 같아야 한다", blank.Session, domestic.Session)
	}
}

/*
값이 그대로면 방송하지 않는다 (2026-09-04 사용자 지적 3).

`cacheMarketPrice`는 폴러의 매 틱마다 불린다 — 가격이 움직였는지와 무관하게. 그
호출 하나하나가 이벤트가 되면 화면이 그것을 갱신으로 세어 셀에 플래시를 주고,
사용자에게는 **없는 체결이 있는 것처럼** 보인다.

허브에 직접 넣지 않고 `publishListPrice`를 지나는 이유는, 중복 제거가 붙어야 할
자리가 **허브가 아니라 발행자**이기 때문이다. 허브에서 걸러도 되지만 그러면
구독자마다 마지막 값을 들고 있어야 한다.
*/
func TestPublishListPriceSkipsUnchangedPrice(t *testing.T) {
	hub := newListPriceHub()
	store := &MarketDataStore{}
	store.AttachListPriceHub(hub)

	ch, cancel := hub.SubscribeMany([]string{"005930"})
	defer cancel()

	at := time.Now().UTC()
	store.publishListPrice("005930", cachedMarketPrice{LastPrice: "71000", UpdatedAt: at})
	store.publishListPrice("005930", cachedMarketPrice{LastPrice: "71000", UpdatedAt: at.Add(time.Second)})
	store.publishListPrice("005930", cachedMarketPrice{LastPrice: "71000", UpdatedAt: at.Add(2 * time.Second)})

	select {
	case ev := <-ch:
		if ev.Price != 71000 {
			t.Fatalf("첫 이벤트 가격 = %v, want 71000", ev.Price)
		}
	case <-time.After(time.Second):
		t.Fatal("첫 값이 안 나갔다 — 중복 제거가 새 값까지 먹었다")
	}

	select {
	case ev := <-ch:
		t.Fatalf("값이 그대로인데 이벤트가 또 나갔다: %+v", ev)
	case <-time.After(100 * time.Millisecond):
	}

	// 실제로 움직이면 다시 나간다 — 중복 제거가 스트림을 잠가 버리면 안 된다.
	store.publishListPrice("005930", cachedMarketPrice{LastPrice: "71100", UpdatedAt: at.Add(3 * time.Second)})
	select {
	case ev := <-ch:
		if ev.Price != 71100 {
			t.Fatalf("가격 = %v, want 71100", ev.Price)
		}
	case <-time.After(time.Second):
		t.Fatal("가격이 움직였는데 이벤트가 안 나갔다")
	}
}

// 중복 제거는 **심볼별**이다. 한 종목이 조용하다고 다른 종목이 막히면 안 된다.
func TestPublishListPriceDedupesPerSymbol(t *testing.T) {
	hub := newListPriceHub()
	store := &MarketDataStore{}
	store.AttachListPriceHub(hub)

	ch, cancel := hub.SubscribeMany([]string{"005930", "000660"})
	defer cancel()

	at := time.Now().UTC()
	store.publishListPrice("005930", cachedMarketPrice{LastPrice: "71000", UpdatedAt: at})
	store.publishListPrice("000660", cachedMarketPrice{LastPrice: "250000", UpdatedAt: at})
	store.publishListPrice("005930", cachedMarketPrice{LastPrice: "71000", UpdatedAt: at.Add(time.Second)})
	store.publishListPrice("000660", cachedMarketPrice{LastPrice: "251000", UpdatedAt: at.Add(time.Second)})

	got := map[string]int{}
	for len(got) < 2 {
		select {
		case ev := <-ch:
			got[ev.Symbol]++
		case <-time.After(time.Second):
			t.Fatalf("이벤트가 모자라다: %v", got)
		}
	}
	// 000660은 값이 움직였으므로 한 번 더 온다.
	select {
	case ev := <-ch:
		got[ev.Symbol]++
		if ev.Symbol != "000660" {
			t.Fatalf("움직인 것은 000660인데 %s가 왔다", ev.Symbol)
		}
	case <-time.After(time.Second):
		t.Fatal("000660이 움직였는데 두 번째 이벤트가 없다")
	}
	if got["005930"] != 1 {
		t.Errorf("005930 이벤트 = %d, want 1 (값이 안 움직였다)", got["005930"])
	}
	if got["000660"] != 2 {
		t.Errorf("000660 이벤트 = %d, want 2", got["000660"])
	}
}
