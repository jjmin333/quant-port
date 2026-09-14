package main

/*
2차 프로브 — 1차에서 **전제 ①이 깨진 것**(WS를 닫아도 토큰이 산다)을 재현성 있게
확인하고, 그 위에서 용량 설계에 필요한 상한을 잰다.

1차 결과 요약:

	A 새 발급이 이전 토큰을 죽인다              → ② 유효
	B 열린 연결은 자기 토큰의 무효화를 견딘다    → 핸드셰이크 시점 인증
	C 죽은 토큰의 연결을 닫아도 현재 토큰이 산다 → make-before-break 성립
	D WS를 열어 둔 채 같은 토큰 REST 12/12 200
	E **정상 종료 후에도 같은 토큰 REST 200**   → ① 반증
	F 강제 드롭 후에도 200

E가 사실이면 「연결이 끊기면 REST가 죽는다」가 아니라 「**재연결할 때 토큰을
새로 발급하면** REST가 죽는다」가 진짜 인과다. 그렇다면 재연결에 같은 토큰을
재사용하기만 하면 발급 자체가 사라진다 — 이 파일의 I가 그것을 확인한다.

	I 종료 후 같은 토큰으로 재연결이 되는가 (3회 반복)
	J 종료 후 토큰 유효성 — 대기 시간별 (+2s/+20s/+60s)
	K 구독 중이던 연결을 강제 드롭한 뒤 같은 토큰으로 재연결
	L 2연결 중 하나만 닫았을 때 나머지 연결과 토큰
	M 앱당 동시 연결 상한이 정말 2인가 (3·4번째 시도)
	N 연결당 구독 슬롯 상한이 100인가 (120건 선언)

	go run ./cmd/probe-toss-coexist -round2
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// manySymbols — N 테스트용. 실재 여부와 무관하게 슬롯 수락/거부만 본다.
func manySymbols(n int) []string {
	base := []string{
		"AAPL", "TSLA", "NVDA", "MSFT", "AMZN", "GOOGL", "META", "NFLX", "AMD", "INTC",
		"BA", "DIS", "KO", "PEP", "WMT", "JPM", "V", "MA", "CRM", "ORCL",
		"ADBE", "AVGO", "QCOM", "TXN", "CSCO", "PFE", "MRK", "ABBV", "T", "VZ",
		"XOM", "CVX", "NKE", "MCD", "SBUX", "COST", "HD", "LOW", "UNH", "CAT",
		"GE", "F", "GM", "UBER", "LYFT", "PYPL", "SQ", "SHOP", "SNAP", "PINS",
		"RBLX", "COIN", "PLTR", "SOFI", "HOOD", "DAL", "UAL", "AAL", "LUV", "CCL",
		"RCL", "NCLH", "MAR", "ABNB", "DASH", "ZM", "DOCU", "TWLO", "NET", "DDOG",
		"SNOW", "CRWD", "ZS", "OKTA", "MDB", "TEAM", "WDAY", "NOW", "PANW", "FTNT",
		"MU", "LRCX", "AMAT", "KLAC", "ASML", "TSM", "ARM", "SMCI", "DELL", "HPQ",
		"IBM", "SAP", "SPOT", "TTD", "ROKU", "EA", "TTWO", "ATVI", "U", "MTCH",
		"LULU", "TGT", "DG", "ROST", "TJX", "BBY", "EBAY", "ETSY", "W", "CHWY",
		"YUM", "CMG", "DPZ", "WEN", "QSR", "DRI", "BLMN", "TXRH", "PZZA", "SHAK",
	}
	if n > len(base) {
		n = len(base)
	}
	return base[:n]
}

func runRound2(ctx context.Context, primary, secondary cred, hasSecondary bool) {
	fmt.Println("=== 2차: ① 반증의 재현성과 용량 상한 ===")
	fmt.Printf("시각: %s\n\n", time.Now().Format(time.RFC3339))

	probeI(ctx, primary)
	probeJ(ctx, primary)
	probeK(ctx, primary)
	probeL(ctx, primary)
	probeM(ctx, primary)
	probeN(ctx, primary)

	if hasSecondary {
		fmt.Println("[I-2] SECONDARY에서도 같은 결과인가 (I 축약 재실행)")
		probeI(ctx, secondary)
	}
}

// ── I. 종료 후 같은 토큰으로 재연결 ───────────────────────────────────────

func probeI(ctx context.Context, c cred) {
	fmt.Printf("[I] %s — 종료 후 **같은 토큰**으로 재연결이 되는가 (3회)\n", c.name)

	t, _, err := issue(ctx, c)
	if err != nil {
		fmt.Printf("  토큰 발급 실패: %v\n\n", err)
		return
	}

	reconnects, restOK := 0, 0
	for i := 1; i <= 3; i++ {
		conn, err := dial(ctx, t)
		if err != nil {
			fmt.Printf("  #%d 연결 실패: %v\n", i, err)
			break
		}
		rx := startReader(ctx, conn)
		declare(ctx, conn, fmt.Sprintf("probe-I-%d", i))
		time.Sleep(5 * time.Second)
		frames := rx.count()
		_ = conn.Close(websocket.StatusNormalClosure, "probe I")
		time.Sleep(2 * time.Second)

		st, _ := restStatus(ctx, t)
		if st == 200 {
			restOK++
		}
		reconnects++
		fmt.Printf("  #%d 연결·구독 프레임=%d · 종료 후 REST → HTTP %d\n", i, frames, st)
	}

	switch {
	case reconnects == 3 && restOK == 3:
		fmt.Println("  ⇒ ✓ 재연결에 토큰 재발급이 전혀 필요 없다 — REST를 건드릴 일 자체가 없다")
	case reconnects == 3:
		fmt.Println("  ⇒ ~ 재연결은 되나 REST가 중간에 깨진다 — 위 코드 확인")
	default:
		fmt.Println("  ⇒ ✗ 같은 토큰 재연결이 막힌다 — 재연결마다 새 발급이 필요하다")
	}
	fmt.Println()
}

// ── J. 종료 후 대기 시간별 토큰 유효성 ────────────────────────────────────

func probeJ(ctx context.Context, c cred) {
	fmt.Println("[J] 종료 후 토큰이 나중에 죽지는 않는가 (지연 무효화)")

	t, _, err := issue(ctx, c)
	if err != nil {
		fmt.Printf("  토큰 발급 실패: %v\n\n", err)
		return
	}
	conn, err := dial(ctx, t)
	if err != nil {
		fmt.Printf("  연결 실패: %v\n\n", err)
		return
	}
	rx := startReader(ctx, conn)
	declare(ctx, conn, "probe-J")
	time.Sleep(6 * time.Second)
	fmt.Printf("  종료 전 프레임 = %d\n", rx.count())
	_ = conn.Close(websocket.StatusNormalClosure, "probe J")

	for _, wait := range []time.Duration{2 * time.Second, 20 * time.Second, 60 * time.Second} {
		time.Sleep(wait)
		st, _ := restStatus(ctx, t)
		fmt.Printf("  누적 대기 후 REST (+%s) → HTTP %d\n", wait, st)
		if st != 200 {
			fmt.Println("  ⇒ 지연 무효화가 있다 — 위 시각이 임계")
			fmt.Println()
			return
		}
	}
	fmt.Println("  ⇒ ✓ 종료 후 1분 이상 지나도 토큰이 산다 — 지연 무효화 없음")
	fmt.Println()
}

// ── K. 강제 드롭 후 재연결 ────────────────────────────────────────────────

func probeK(ctx context.Context, c cred) {
	fmt.Println("[K] 구독 중이던 연결을 강제 드롭한 뒤 같은 토큰으로 재연결")

	t, _, err := issue(ctx, c)
	if err != nil {
		fmt.Printf("  토큰 발급 실패: %v\n\n", err)
		return
	}
	conn, err := dial(ctx, t)
	if err != nil {
		fmt.Printf("  연결 실패: %v\n\n", err)
		return
	}
	rx := startReader(ctx, conn)
	declare(ctx, conn, "probe-K")
	time.Sleep(6 * time.Second)
	fmt.Printf("  드롭 전 프레임 = %d\n", rx.count())

	_ = conn.CloseNow()
	time.Sleep(3 * time.Second)

	conn2, err := dial(ctx, t)
	if err != nil {
		fmt.Printf("  ⇒ ✗ 재연결 실패: %v\n\n", err)
		return
	}
	defer conn2.Close(websocket.StatusNormalClosure, "probe K")
	rx2 := startReader(ctx, conn2)
	declare(ctx, conn2, "probe-K2")
	time.Sleep(8 * time.Second)
	st, _ := restStatus(ctx, t)
	fmt.Printf("  재연결 후 프레임 = %d · REST → HTTP %d\n", rx2.count(), st)
	if rx2.count() > 0 && st == 200 {
		fmt.Println("  ⇒ ✓ 비정상 절단도 같은 토큰으로 그대로 복구된다")
	} else {
		fmt.Println("  ⇒ ~ 위 수치 확인")
	}
	fmt.Println()
}

// ── L. 2연결 중 하나만 종료 ───────────────────────────────────────────────

func probeL(ctx context.Context, c cred) {
	fmt.Println("[L] 같은 토큰의 2연결 중 하나만 닫으면 나머지와 토큰은")

	t, _, err := issue(ctx, c)
	if err != nil {
		fmt.Printf("  토큰 발급 실패: %v\n\n", err)
		return
	}
	c1, err1 := dial(ctx, t)
	c2, err2 := dial(ctx, t)
	if err1 != nil || err2 != nil {
		fmt.Printf("  연결 실패: %v / %v\n\n", err1, err2)
		return
	}
	defer c2.Close(websocket.StatusNormalClosure, "probe L")
	r1, r2 := startReader(ctx, c1), startReader(ctx, c2)
	declare(ctx, c1, "probe-L1")
	declare(ctx, c2, "probe-L2")
	time.Sleep(6 * time.Second)
	fmt.Printf("  종료 전 프레임: #1=%d #2=%d\n", r1.count(), r2.count())

	_ = c1.Close(websocket.StatusNormalClosure, "probe L close one")
	before2 := r2.count()
	time.Sleep(8 * time.Second)
	st, _ := restStatus(ctx, t)
	alive2 := r2.err() == nil && ping(ctx, c2) == nil
	fmt.Printf("  #1 종료 후 → #2 추가 프레임=%d · #2 PING=%t · REST HTTP %d\n",
		r2.count()-before2, alive2, st)
	if alive2 && st == 200 {
		fmt.Println("  ⇒ ✓ 연결 종료는 형제 연결·토큰 어느 쪽에도 영향이 없다")
	} else {
		fmt.Println("  ⇒ ✗ 한 연결의 종료가 다른 것을 끌고 내려간다")
	}
	fmt.Println()
}

// ── M. 앱당 동시 연결 상한 ────────────────────────────────────────────────

func probeM(ctx context.Context, c cred) {
	fmt.Println("[M] 앱당 동시 연결 상한 — 4개까지 열어 본다")

	t, _, err := issue(ctx, c)
	if err != nil {
		fmt.Printf("  토큰 발급 실패: %v\n\n", err)
		return
	}
	var conns []*websocket.Conn
	var rxs []*reader
	defer func() {
		for _, cn := range conns {
			_ = cn.Close(websocket.StatusNormalClosure, "probe M")
		}
	}()
	for i := 1; i <= 4; i++ {
		cn, err := dial(ctx, t)
		if err != nil {
			fmt.Printf("  #%d 연결 실패: %v\n", i, err)
			continue
		}
		conns = append(conns, cn)
		rxs = append(rxs, startReader(ctx, cn))
		declare(ctx, cn, fmt.Sprintf("probe-M-%d", i))
		fmt.Printf("  #%d 연결 성공\n", i)
		time.Sleep(time.Second)
	}

	// 상한을 넘으면 스펙상 **가장 오래된 연결이 끊긴다**. 15초 뒤 생존을 센다.
	time.Sleep(15 * time.Second)
	alive := 0
	for i, cn := range conns {
		ok := rxs[i].err() == nil && ping(ctx, cn) == nil
		if ok {
			alive++
		}
		fmt.Printf("  #%d 생존=%t 프레임=%d\n", i+1, ok, rxs[i].count())
	}
	fmt.Printf("  ⇒ 동시 유지 = %d개\n", alive)
	fmt.Println()
}

// ── N. 연결당 구독 슬롯 상한 ──────────────────────────────────────────────

func probeN(ctx context.Context, c cred) {
	fmt.Println("[N] 연결당 구독 슬롯 상한 — trade 120건 선언")

	t, _, err := issue(ctx, c)
	if err != nil {
		fmt.Printf("  토큰 발급 실패: %v\n\n", err)
		return
	}
	conn, err := dial(ctx, t)
	if err != nil {
		fmt.Printf("  연결 실패: %v\n\n", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "probe N")

	syms := manySymbols(120)
	payload, _ := json.Marshal([]any{
		map[string]any{"id": "probe-N"},
		map[string]any{"type": "trade:us", "codes": syms},
	})
	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = conn.Write(wctx, websocket.MessageText, payload)
	cancel()
	if err != nil {
		fmt.Printf("  선언 실패: %v\n\n", err)
		return
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		rctx, rcancel := context.WithDeadline(ctx, deadline)
		_, data, err := conn.Read(rctx)
		rcancel()
		if err != nil {
			break
		}
		s := string(data)
		if strings.Contains(s, "\"subscriptions\"") || strings.Contains(s, "\"type\":\"error\"") {
			var env struct {
				Type       string   `json:"type"`
				Subscribed []string `json:"subscribed"`
				Rejected   []string `json:"rejected"`
			}
			if json.Unmarshal(data, &env) == nil && env.Type == "subscriptions" {
				fmt.Printf("  선언 %d건 → subscribed=%d rejected=%d\n",
					len(syms), len(env.Subscribed), len(env.Rejected))
				if len(env.Rejected) > 0 {
					fmt.Printf("  거부 샘플: %v\n", env.Rejected[:min(5, len(env.Rejected))])
				}
			} else {
				fmt.Printf("  응답: %s\n", truncate2(s, 300))
			}
			fmt.Println()
			return
		}
	}
	fmt.Println("  확인 프레임을 못 받았다 — 판정 불가")
	fmt.Println()
}

func truncate2(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
