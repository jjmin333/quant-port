package main

/*
4차 — **토큰이 죽는 순간 사용자 요청이 끊기는가.**

24시간 만료는 하루에 한 번뿐이라 그냥 기다려서는 검증할 수 없다. 대신 같은 사건을
**즉시 재현**한다: 밖에서 토큰을 새로 발급하면 client_id당 활성 토큰이 하나이므로
API가 쓰던 토큰이 그 자리에서 죽는다. 만료와 구별할 수 없는 상태다.

그 순간 화면 요청을 계속 두드리면서 몇 건이 실패하는지 센다.

	go run ./cmd/probe-toss-coexist -killtoken [심볼]

방어가 제대로면 실패 0이어야 한다:

	401 → CAS invalidate(형제가 갈아 끼웠으면 그것을 씀) → 재시도
	발급이 실패해도 아직 만료 전인 캐시 토큰으로 물러섬

**운영 자격증명의 토큰을 실제로 죽인다.** API가 스스로 회복하는 것이 이 테스트의
관측 대상이므로 그게 정상이지만, 회복이 안 되면 그 상태로 남는다는 뜻이기도 하다.
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// killTokenRounds — 몇 번 죽여 볼지. 회복 도중의 재사망까지 보려면 1로는 부족하다.
const killTokenRounds = 3

func runKillToken(ctx context.Context, c cred, symbol string) {
	fmt.Println("=== 4차: 토큰 강제 사망 중 사용자 요청 끊김 측정 ===")
	fmt.Printf("시각: %s · 종목: %s\n\n", time.Now().Format(time.RFC3339), symbol)

	// 구독이 붙을 시간을 준다 — 붙은 상태가 평상시 모습이다.
	fmt.Println("구독이 붙을 때까지 대기…")
	for i := 0; i < 30; i++ {
		if snap := fetchSnapshot(ctx, symbol); snap != nil && snap.Data.Source == "toss-ws" {
			fmt.Printf("  %d초에 구독 전환\n", i*2)
			break
		}
		time.Sleep(2 * time.Second)
	}

	var (
		mu        sync.Mutex
		ok, bad   int
		failCodes []string
		stalePx   int
		lastPrice float64
	)

	pollCtx, stop := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for pollCtx.Err() == nil {
			status, price := snapshotStatus(pollCtx, symbol)
			mu.Lock()
			switch {
			case status == 200 && price > 0:
				ok++
				if price == lastPrice {
					stalePx++
				}
				lastPrice = price
			case status != 0:
				bad++
				failCodes = append(failCodes,
					fmt.Sprintf("%s HTTP %d", time.Now().Format("15:04:05.000"), status))
			}
			mu.Unlock()
			time.Sleep(200 * time.Millisecond)
		}
	}()

	time.Sleep(3 * time.Second)

	/*
		토큰을 **여러 번** 죽인다.

		한 번으로는 「운 좋게 넘어갔다」와 구별되지 않는다. 연달아 죽이면 회복 도중에
		또 죽는 경우까지 들어가므로, 재시도 횟수(`authRetryAttempts`)가 실제로
		모자라지 않는지 드러난다.

		**발급이 실제로 성공했는지 확인한다.** 셸로 같은 일을 하다가 자격증명이
		잘려 `invalid_client`로 실패했는데도 「무중단」이라고 읽을 뻔했다 —
		아무것도 안 죽였으니 당연히 안 끊긴 것이었다.
	*/
	for k := 1; k <= killTokenRounds; k++ {
		newToken, _, err := issue(ctx, c)
		if err != nil {
			stop()
			wg.Wait()
			fmt.Printf("발급 실패 — 아무것도 죽이지 못했다: %v\n", err)
			return
		}
		fmt.Printf("\n토큰 사망 #%d (%s) — 새 토큰 %s…\n",
			k, time.Now().Format("15:04:05.000"), newToken[:12])

		// 정말 죽었는지 확인: 새 토큰이 200이면 이전 것은 무효화된 것이다.
		if st, _ := restStatus(ctx, newToken); st != 200 {
			fmt.Printf("  ! 새 토큰이 HTTP %d — 발급은 됐으나 쓸 수 없다\n", st)
		}
		time.Sleep(8 * time.Second)
	}

	// 회복을 관찰할 시간.
	time.Sleep(10 * time.Second)
	stop()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("\n스냅샷 요청: 성공 %d · 실패 %d\n", ok, bad)
	for _, f := range failCodes {
		fmt.Printf("  실패 %s\n", f)
	}
	if bad == 0 {
		fmt.Println("⇒ ✓ 토큰이 죽는 순간에도 사용자 요청이 한 건도 안 끊겼다")
	} else {
		fmt.Printf("⇒ ✗ %d건이 끊겼다 — 위 시각을 API 로그와 대조할 것\n", bad)
	}
}

// snapshotStatus returns the HTTP status and the price the screen would show.
func snapshotStatus(ctx context.Context, sym string) (int, float64) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		localAPI+"/api/v1/quotes/"+sym+"/terminal-snapshot", nil)
	if err != nil {
		return 0, 0
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return resp.StatusCode, 0
	}
	var out snapshotResponse
	if decodeJSON(resp, &out) != nil {
		return resp.StatusCode, 0
	}
	return resp.StatusCode, out.Data.Detail.CurrentPrice
}

func decodeJSON(resp *http.Response, dest any) error {
	return json.NewDecoder(resp.Body).Decode(dest)
}
