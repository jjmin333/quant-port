package main

/*
3차 — **구독으로 만든 화면 데이터가 REST로 받은 것과 같은가.**

REST 호출을 없앤 것만으로는 검증이 끝나지 않는다. 호가창과 체결 표는 사용자가
숫자를 그대로 읽는 화면이라, 값이 틀리면 「느리다」가 아니라 **틀린 정보**가 된다.

그래서 같은 순간에 두 경로를 나란히 받아 비교한다:

	우리 API   GET /api/v1/quotes/{sym}/terminal-snapshot   (구독이 채운 것)
	Toss REST  GET /api/v1/orderbook, /api/v1/trades        (기준값)

호가는 **순간 스냅샷**이라 두 요청 사이에 실제로 바뀔 수 있다. 그래서 완전 일치를
요구하지 않고 **구조와 자릿수**를 본다 — 단 수, 가격 정렬, 최우선 호가의 괴리율.
장이 조용할수록 일치도가 올라간다.

	go run ./cmd/probe-toss-coexist -compare [심볼...]
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const localAPI = "http://localhost:8080"

type snapshotResponse struct {
	Data struct {
		Detail struct {
			Symbol       string  `json:"symbol"`
			CurrentPrice float64 `json:"currentPrice"`
		} `json:"detail"`
		OrderBook struct {
			Symbol string `json:"symbol"`
			Asks   []struct {
				Price    float64 `json:"price"`
				Quantity int64   `json:"quantity"`
			} `json:"asks"`
			Bids []struct {
				Price    float64 `json:"price"`
				Quantity int64   `json:"quantity"`
			} `json:"bids"`
		} `json:"orderBook"`
		Trades []struct {
			Time     string  `json:"time"`
			Price    float64 `json:"price"`
			Quantity int64   `json:"quantity"`
			Side     string  `json:"side"`
		} `json:"trades"`
		Source string `json:"source"`
	} `json:"data"`
}

func runCompare(ctx context.Context, c cred, symbols []string) {
	fmt.Println("=== 3차: 구독이 만든 화면 데이터 vs Toss REST ===")
	fmt.Printf("시각: %s\n\n", time.Now().Format(time.RFC3339))

	/*
		기준값을 받을 토큰.

		**되도록 발급하지 않는다.** 이 모드는 API가 떠 있는 상태에서 돌리는데,
		발급은 파괴적이라(client_id당 활성 토큰 1개) 그 순간 API가 쓰던 토큰이
		죽는다. 검증하려는 대상을 검증 도구가 흔드는 셈이다.

		그래서 운영이 쓰는 공유 토큰을 그대로 받아 쓴다:

			TOSS_PROBE_TOKEN=$(docker exec quant-port-redis \
			  redis-cli --raw GET toss:oauth:access_token | jq -r .accessToken)

		없으면 발급으로 물러선다 — 그때는 API가 401 한 번을 맞고 스스로 회복한다
		(CAS invalidate). 그 회복 자체도 볼 만한 것이므로 실패로 치지 않는다.
	*/
	token := strings.TrimSpace(os.Getenv("TOSS_PROBE_TOKEN"))
	if token == "" {
		fmt.Println("(TOSS_PROBE_TOKEN 없음 — 새로 발급한다. API가 401을 한 번 맞고 회복할 것)")
		var err error
		token, _, err = issue(ctx, c)
		if err != nil {
			fail(fmt.Sprintf("토큰 발급 실패: %v", err))
		}
	} else {
		fmt.Println("(운영 공유 토큰을 그대로 사용 — 발급 없음)")
	}

	for _, sym := range symbols {
		compareSymbol(ctx, token, strings.ToUpper(sym))
	}
}

func compareSymbol(ctx context.Context, token, sym string) {
	fmt.Printf("[%s]\n", sym)

	// 구독이 붙을 시간을 준다 — 폴러가 「보고 있는 종목」으로 인식해야 선언한다.
	for i := 0; i < 12; i++ {
		_ = fetchSnapshot(ctx, sym)
		time.Sleep(2 * time.Second)
	}

	snap := fetchSnapshot(ctx, sym)
	if snap == nil {
		fmt.Println("  스냅샷을 못 받았다 — API가 떠 있는지 확인할 것")
		fmt.Println()
		return
	}
	fmt.Printf("  source=%s  현재가=%.4f  호가 asks=%d bids=%d  체결=%d건\n",
		snap.Data.Source, snap.Data.Detail.CurrentPrice,
		len(snap.Data.OrderBook.Asks), len(snap.Data.OrderBook.Bids), len(snap.Data.Trades))

	if snap.Data.Source != "toss-ws" {
		fmt.Println("  ! 구독이 아니라 REST가 채웠다 — 비교의 의미가 없다(장 마감·구독 미확인)")
	}

	book := fetchTossOrderbook(ctx, token, sym)
	trades := fetchTossTrades(ctx, token, sym)

	// ── 구조 ──
	checkBookStructure(snap)

	// ── 최우선 호가 괴리 ──
	if book != nil {
		compareTop(snap, book)
	} else {
		fmt.Println("  기준 호가를 못 받았다 — 괴리 비교 생략")
	}

	// ── 체결 ──
	compareTrades(snap, trades)
	fmt.Println()
}

func checkBookStructure(snap *snapshotResponse) {
	asks := snap.Data.OrderBook.Asks
	bids := snap.Data.OrderBook.Bids
	if len(asks) == 0 && len(bids) == 0 {
		fmt.Println("  ✗ 호가가 비었다 — 화면의 호가창이 빈 채로 남는다")
		return
	}

	// 매도는 낮은 가격부터, 매수는 높은 가격부터 오는 것이 화면이 기대하는 순서다.
	askSorted := sort.SliceIsSorted(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })
	bidSorted := sort.SliceIsSorted(bids, func(i, j int) bool { return bids[i].Price > bids[j].Price })
	fmt.Printf("  정렬: asks 오름차순=%t · bids 내림차순=%t\n", askSorted, bidSorted)

	if len(asks) > 0 && len(bids) > 0 && asks[0].Price <= bids[0].Price {
		fmt.Printf("  ✗ 최우선 매도(%.4f) ≤ 최우선 매수(%.4f) — 스프레드가 뒤집혔다\n",
			asks[0].Price, bids[0].Price)
	}
	for _, lv := range append(append([]struct {
		Price    float64 `json:"price"`
		Quantity int64   `json:"quantity"`
	}{}, asks...), bids...) {
		if lv.Price <= 0 || lv.Quantity < 0 {
			fmt.Printf("  ✗ 이상한 호가 단: price=%v qty=%v\n", lv.Price, lv.Quantity)
			return
		}
	}
	fmt.Println("  ✓ 호가 구조 정상 (가격>0, 수량≥0, 스프레드 정상)")
}

func compareTop(snap *snapshotResponse, book *tossBook) {
	ours := snap.Data.OrderBook
	if len(ours.Asks) == 0 || len(book.Asks) == 0 {
		fmt.Println("  최우선 호가 비교 불가 (한쪽이 비었다)")
		return
	}
	ourAsk, refAsk := ours.Asks[0].Price, parseF(book.Asks[0].Price)
	gap := 0.0
	if refAsk > 0 {
		gap = math.Abs(ourAsk-refAsk) / refAsk * 100
	}
	fmt.Printf("  최우선 매도: 구독 %.4f vs REST %.4f (괴리 %.4f%%)\n", ourAsk, refAsk, gap)
	switch {
	case gap == 0:
		fmt.Println("  ✓ 완전 일치")
	case gap < 0.5:
		fmt.Println("  ✓ 두 요청 사이의 정상 변동 범위")
	default:
		fmt.Println("  ! 괴리가 크다 — 장중이면 정상 변동일 수 있으나 확인 필요")
	}
}

func compareTrades(snap *snapshotResponse, ref []tossTrade) {
	ours := snap.Data.Trades
	if len(ours) == 0 {
		fmt.Println("  체결 목록이 비었다 (구독 직후이거나 거래가 없는 시간)")
		return
	}

	// 화면은 배열 순서를 그대로 그린다 — 최신이 앞이어야 한다.
	newestFirst := true
	for i := 1; i < len(ours); i++ {
		a, _ := time.Parse(time.RFC3339, ours[i-1].Time)
		b, _ := time.Parse(time.RFC3339, ours[i].Time)
		if !a.IsZero() && !b.IsZero() && b.After(a) {
			newestFirst = false
			break
		}
	}
	fmt.Printf("  체결 %d건 · 최신순=%t · side=%q\n", len(ours), newestFirst, ours[0].Side)

	// REST가 주던 것과 같은 모양인지 — side는 양쪽 다 "unknown"이어야 한다.
	if len(ref) > 0 {
		fmt.Printf("  기준 체결 최신가=%s · 구독 최신가=%.4f\n", ref[0].Price, ours[0].Price)
	}
	if !newestFirst {
		fmt.Println("  ✗ 최신순이 아니다 — 화면이 과거 체결을 위에 그린다")
	}
}

// ── HTTP ────────────────────────────────────────────────────────────────

func fetchSnapshot(ctx context.Context, sym string) *snapshotResponse {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		localAPI+"/api/v1/quotes/"+sym+"/terminal-snapshot", nil)
	if err != nil {
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var out snapshotResponse
	if json.NewDecoder(resp.Body).Decode(&out) != nil {
		return nil
	}
	return &out
}

type tossLevel struct {
	Price  string `json:"price"`
	Volume string `json:"volume"`
}

type tossBook struct {
	Asks []tossLevel `json:"asks"`
	Bids []tossLevel `json:"bids"`
}

type tossTrade struct {
	Timestamp string `json:"timestamp"`
	Price     string `json:"price"`
	Volume    string `json:"volume"`
}

// 파라미터는 `symbol`(단수)이고 result도 배열이 아니다 — `toss.GetOrderbook` 참조.
func fetchTossOrderbook(ctx context.Context, token, sym string) *tossBook {
	var env struct {
		Result tossBook `json:"result"`
	}
	if !tossGet(ctx, token, "/api/v1/orderbook?symbol="+sym, &env) {
		return nil
	}
	if len(env.Result.Asks) == 0 && len(env.Result.Bids) == 0 {
		return nil
	}
	return &env.Result
}

func fetchTossTrades(ctx context.Context, token, sym string) []tossTrade {
	var env struct {
		Result []tossTrade `json:"result"`
	}
	if !tossGet(ctx, token, "/api/v1/trades?symbol="+sym+"&size=10", &env) {
		return nil
	}
	return env.Result
}

func tossGet(ctx context.Context, token, path string, dest any) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(baseURL, "/")+path, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		fmt.Printf("    (기준값 %s → HTTP %d)\n", path, resp.StatusCode)
		return false
	}
	return json.NewDecoder(resp.Body).Decode(dest) == nil
}

func parseF(s string) float64 {
	var v float64
	_, _ = fmt.Sscanf(strings.TrimSpace(s), "%g", &v)
	return v
}

func compareSymbolsArg() []string {
	if len(os.Args) > 2 {
		return os.Args[2:]
	}
	return []string{"AAPL"}
}

func firstArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return "AAPL"
}
