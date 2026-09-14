/*
Command probe-toss-coexist — 「REST 앱 1개 + WS 앱 1개」로 갈라 둔 현재 구성을
「두 앱 모두 REST+WS 공존」으로 바꿀 수 있는지 실측한다.

분리의 근거는 `toss/stream.go` 헤더의 두 실측이다:

	① 웹소켓을 닫으면 그 액세스 토큰이 무효화된다.
	② client_id당 활성 토큰이 하나다 — 새로 발급하면 이전 것이 즉시 죽는다.

둘 다 사실이어도 공존이 불가능한 것은 아니다. ①이 죽이는 것이 **그 연결에
묶인 토큰**뿐이라면, 새 토큰으로 먼저 붙고 나중에 옛 연결을 닫는
make-before-break로 REST 무중단 회전이 가능하다. 이 프로브는 그 가정을
성립시키는 최소 조건들을 하나씩 갈라 본다.

	A 토큰 다중성        — 새로 발급하면 이전 토큰이 정말 죽는가
	B 연결의 토큰 내성    — 열려 있는 WS가 자기 토큰의 무효화를 견디는가
	C 죽은 연결의 무해성  — 이미 죽은 토큰의 WS를 닫아도 현재 토큰이 사는가
	D 공존              — WS를 열어 둔 채 같은 토큰으로 REST가 되는가
	E 종료의 파괴성      — 살아 있는 토큰의 WS를 닫으면 그 토큰이 죽는가(전제 재확인)
	F 강제 종료          — 정상 종료가 아니라 TCP를 끊어도 토큰이 죽는가
	G 목표 형태          — 앱마다 WS 2연결 + REST 동시 가동이 유지되는가
	H 무중단 회전        — make-before-break 절차에서 REST가 한 번도 안 끊기는가

**운영 코드를 건드리지 않는다.** OAuth·REST·WS를 직접 부르는 자족형이다.
읽기 전용이다 — 조회와 구독만 한다.

	go run ./cmd/probe-toss-coexist

주의: 토큰을 여러 번 재발급하므로 **API 컨테이너를 멈춘 뒤** 돌려야 한다.
같은 자격증명을 쓰는 프로세스가 살아 있으면 서로 토큰을 죽여 판정이 오염된다.
*/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/joho/godotenv"
)

const (
	defaultBaseURL = "https://openapi.tossinvest.com"
	wsURL          = "wss://openapi-ws.tossinvest.com/ws/v1"
)

// probeSymbols — KST 새벽에도 프레임이 흐르도록 미국 종목을 쓴다.
var probeSymbols = []string{"AAPL", "TSLA", "NVDA"}

type cred struct {
	name    string
	id      string
	secret  string
	baseURL string
}

var (
	issueCount int
	baseURL    string
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() { <-stop; cancel() }()

	_ = godotenv.Load(".env")
	baseURL = envOr("TOSS_API_BASE_URL", defaultBaseURL)

	primary := cred{name: "PRIMARY", baseURL: baseURL,
		id:     firstEnv("TOSS_PRIMARY_CLIENT_ID", "TOSS_CLIENT_ID"),
		secret: firstEnv("TOSS_PRIMARY_CLIENT_SECRET", "TOSS_CLIENT_SECRET")}
	secondary := cred{name: "SECONDARY", baseURL: baseURL,
		id:     os.Getenv("TOSS_SECONDARY_CLIENT_ID"),
		secret: os.Getenv("TOSS_SECONDARY_CLIENT_SECRET")}

	if primary.id == "" || primary.secret == "" {
		fail("PRIMARY 자격증명이 없다")
	}
	hasSecondary := secondary.id != "" && secondary.secret != ""

	fmt.Println("=== Toss REST+WS 공존 프로브 ===")
	fmt.Printf("시각: %s\n", time.Now().Format(time.RFC3339))
	fmt.Printf("SECONDARY 설정됨: %t\n\n", hasSecondary)

	mode := ""
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	switch mode {
	case "-round2":
		runRound2(ctx, primary, secondary, hasSecondary)
	case "-killtoken":
		// 토큰이 죽는 순간 사용자 요청이 끊기는지 (kill_token.go).
		runKillToken(ctx, primary, firstArg(compareSymbolsArg()))
	case "-compare":
		// 구독이 만든 화면 데이터가 REST 기준값과 맞는지 (compare.go).
		// 이 모드는 **API가 떠 있는 상태**에서 돌린다 — 검증 대상이 그 API다.
		runCompare(ctx, primary, compareSymbolsArg())
	default:
		probeA(ctx, primary)
		probeBC(ctx, primary)
		probeDE(ctx, primary)
		probeF(ctx, primary)
		probeG(ctx, primary, secondary, hasSecondary)
		probeH(ctx, primary)
	}

	fmt.Printf("\n총 토큰 발급 횟수 = %d\n", issueCount)
}

// ── A. 토큰 다중성 ───────────────────────────────────────────────────────

func probeA(ctx context.Context, c cred) {
	fmt.Println("[A] client_id당 활성 토큰이 정말 하나인가")

	t1, exp1, err := issue(ctx, c)
	if err != nil {
		fail(fmt.Sprintf("T1 발급 실패: %v", err))
	}
	fmt.Printf("  T1 발급 — expires_in=%ds (%s)\n", exp1, (time.Duration(exp1) * time.Second).String())

	st, _ := restStatus(ctx, t1)
	fmt.Printf("  T1 REST 즉시            → HTTP %d\n", st)

	t2, _, err := issue(ctx, c)
	if err != nil {
		fail(fmt.Sprintf("T2 발급 실패: %v", err))
	}
	same := t1 == t2
	fmt.Printf("  T2 발급 — T1과 동일 문자열: %t\n", same)

	st1, _ := restStatus(ctx, t1)
	st2, _ := restStatus(ctx, t2)
	fmt.Printf("  T2 발급 후 T1 REST      → HTTP %d\n", st1)
	fmt.Printf("  T2 발급 후 T2 REST      → HTTP %d\n", st2)

	switch {
	case same:
		fmt.Println("  ⇒ 서버가 같은 토큰을 재사용한다 — 발급이 파괴적이지 않다")
	case st1 == 200 && st2 == 200:
		fmt.Println("  ⇒ ✓ 토큰 다중 활성 — ②가 더 이상 사실이 아니다. 용도별 토큰 분리로 끝난다")
	case st1 == 401 && st2 == 200:
		fmt.Println("  ⇒ ✗ 단일 활성 확인 — 새 발급이 이전 토큰을 죽인다(②는 사실)")
	default:
		fmt.Println("  ⇒ ? 예상 밖 — 위 코드로 판단할 것")
	}
	fmt.Println()
}

// ── B/C. 연결의 토큰 내성 · 죽은 연결의 무해성 ────────────────────────────

func probeBC(ctx context.Context, c cred) {
	fmt.Println("[B] 열려 있는 WS가 자기 토큰의 무효화를 견디는가")

	tA, _, err := issue(ctx, c)
	if err != nil {
		fail(fmt.Sprintf("TA 발급 실패: %v", err))
	}
	conn, err := dial(ctx, tA)
	if err != nil {
		fmt.Printf("  WS 연결 실패: %v — B/C 판정 불가\n\n", err)
		return
	}
	closed := false
	defer func() {
		if !closed {
			_ = conn.Close(websocket.StatusNormalClosure, "probe")
		}
	}()

	rx := startReader(ctx, conn)
	declare(ctx, conn, "probe-B")
	time.Sleep(6 * time.Second)
	fmt.Printf("  발급 전 프레임 = %d\n", rx.count())

	tB, _, err := issue(ctx, c)
	if err != nil {
		fail(fmt.Sprintf("TB 발급 실패: %v", err))
	}
	stA, _ := restStatus(ctx, tA)
	stB, _ := restStatus(ctx, tB)
	fmt.Printf("  새 토큰 발급 후 TA REST → HTTP %d (WS가 물고 있던 토큰)\n", stA)
	fmt.Printf("  새 토큰 발급 후 TB REST → HTTP %d\n", stB)

	beforeHold := rx.count()
	time.Sleep(12 * time.Second)
	afterHold := rx.count()
	alive := rx.err() == nil && ping(ctx, conn) == nil
	fmt.Printf("  이후 12초 프레임 = %d (누적 %d), PING 성공 = %t\n", afterHold-beforeHold, afterHold, alive)
	if e := rx.err(); e != nil {
		fmt.Printf("  읽기 종료 사유: %v\n", e)
	}

	switch {
	case alive && afterHold > beforeHold:
		fmt.Println("  ⇒ ✓ 핸드셰이크 시점에만 인증한다 — 연결은 토큰 무효화를 견딘다")
	case alive:
		fmt.Println("  ⇒ ~ 연결은 살았으나 프레임이 안 온다 — 장 상황일 수 있음. 재실행 권장")
	default:
		fmt.Println("  ⇒ ✗ 새 발급이 기존 연결을 끊는다 — make-before-break 불가")
	}
	fmt.Println()

	fmt.Println("[C] 이미 죽은 토큰의 WS를 닫아도 현재 토큰이 사는가")
	_ = conn.Close(websocket.StatusNormalClosure, "probe C")
	closed = true
	time.Sleep(3 * time.Second)
	stB2, _ := restStatus(ctx, tB)
	fmt.Printf("  옛 연결(TA) 종료 후 TB REST → HTTP %d\n", stB2)
	if stB2 == 200 {
		fmt.Println("  ⇒ ✓ 무효화는 그 연결에 묶인 토큰에만 걸린다 — make-before-break 성립")
	} else {
		fmt.Println("  ⇒ ✗ 연결 종료가 client_id의 현재 토큰까지 죽인다 — 회전 절차가 REST를 끊는다")
	}
	fmt.Println()
}

// ── D/E. 공존 · 종료의 파괴성 ─────────────────────────────────────────────

func probeDE(ctx context.Context, c cred) {
	fmt.Println("[D] WS를 열어 둔 채 같은 토큰으로 REST가 되는가")

	t, _, err := issue(ctx, c)
	if err != nil {
		fail(fmt.Sprintf("TD 발급 실패: %v", err))
	}
	conn, err := dial(ctx, t)
	if err != nil {
		fmt.Printf("  WS 연결 실패: %v — D/E 판정 불가\n\n", err)
		return
	}
	closed := false
	defer func() {
		if !closed {
			_ = conn.Close(websocket.StatusNormalClosure, "probe")
		}
	}()

	rx := startReader(ctx, conn)
	declare(ctx, conn, "probe-D")

	remBefore := restRemaining(ctx, t)
	ok, bad := 0, 0
	var codes []int
	for i := 0; i < 12; i++ {
		st, _ := restStatus(ctx, t)
		if st == 200 {
			ok++
		} else {
			bad++
			codes = append(codes, st)
		}
		time.Sleep(2 * time.Second)
	}
	remAfter := restRemaining(ctx, t)
	frames := rx.count()

	fmt.Printf("  24초간 REST 200 = %d/12, 실패 = %d %v\n", ok, bad, codes)
	fmt.Printf("  같은 구간 WS 프레임 = %d\n", frames)
	fmt.Printf("  MARKET_DATA remaining: %s → %s\n", remBefore, remAfter)
	if ok == 12 {
		fmt.Println("  ⇒ ✓ 연결이 살아 있는 동안은 같은 토큰으로 REST가 정상")
	} else {
		fmt.Println("  ⇒ ✗ WS가 열려 있는 것만으로 REST가 깨진다 — 공존 불가")
	}
	fmt.Println()

	fmt.Println("[E] 살아 있는 토큰의 WS를 닫으면 그 토큰이 죽는가 (전제 ① 재확인)")
	_ = conn.Close(websocket.StatusNormalClosure, "probe E")
	closed = true
	time.Sleep(3 * time.Second)
	st, _ := restStatus(ctx, t)
	fmt.Printf("  정상 종료 후 같은 토큰 REST → HTTP %d\n", st)
	switch st {
	case 401:
		fmt.Println("  ⇒ ✗ 전제 ① 유효 — 연결 종료가 토큰을 죽인다")
	case 200:
		fmt.Println("  ⇒ ✓ 전제 ①이 더 이상 성립하지 않는다 — 분리 이유가 사라진다")
	}
	fmt.Println()
}

// ── F. 강제 종료 ─────────────────────────────────────────────────────────

func probeF(ctx context.Context, c cred) {
	fmt.Println("[F] 정상 종료가 아니라 TCP를 끊어도 토큰이 죽는가")

	t, _, err := issue(ctx, c)
	if err != nil {
		fail(fmt.Sprintf("TF 발급 실패: %v", err))
	}
	conn, err := dial(ctx, t)
	if err != nil {
		fmt.Printf("  WS 연결 실패: %v — F 판정 불가\n\n", err)
		return
	}
	declare(ctx, conn, "probe-F")
	time.Sleep(4 * time.Second)

	_ = conn.CloseNow() // close 프레임 없이 소켓만 끊는다
	time.Sleep(3 * time.Second)
	st1, _ := restStatus(ctx, t)
	fmt.Printf("  CloseNow 직후 REST      → HTTP %d\n", st1)
	time.Sleep(10 * time.Second)
	st2, _ := restStatus(ctx, t)
	fmt.Printf("  10초 더 기다린 뒤 REST  → HTTP %d\n", st2)

	switch {
	case st1 == 200 && st2 == 200:
		fmt.Println("  ⇒ 강제 종료는 토큰을 죽이지 않는다 — 무효화는 close 핸드셰이크에 걸려 있다")
	case st1 == 200 && st2 == 401:
		fmt.Println("  ⇒ 지연 무효화 — 서버가 세션 정리 후 죽인다")
	default:
		fmt.Println("  ⇒ 강제 종료도 즉시 토큰을 죽인다")
	}
	fmt.Println()
}

// ── G. 목표 형태 동시 가동 ────────────────────────────────────────────────

type heldApp struct {
	c     cred
	token string
	conns []*websocket.Conn
	rx    []*reader
}

func probeG(ctx context.Context, primary, secondary cred, hasSecondary bool) {
	fmt.Println("[G] 앱마다 WS 2연결 + REST 동시 가동 (목표 형태)")

	creds := []cred{primary}
	if hasSecondary {
		creds = append(creds, secondary)
	}

	var all []heldApp
	defer func() {
		for _, h := range all {
			for _, cn := range h.conns {
				_ = cn.Close(websocket.StatusNormalClosure, "probe G")
			}
		}
	}()

	for _, c := range creds {
		t, _, err := issue(ctx, c)
		if err != nil {
			fmt.Printf("  %s 토큰 발급 실패: %v\n", c.name, err)
			continue
		}
		h := heldApp{c: c, token: t}
		for i := 0; i < 2; i++ {
			cn, err := dial(ctx, t)
			if err != nil {
				fmt.Printf("  %s WS#%d 연결 실패: %v\n", c.name, i+1, err)
				continue
			}
			h.conns = append(h.conns, cn)
			h.rx = append(h.rx, startReader(ctx, cn))
			declare(ctx, cn, fmt.Sprintf("probe-G-%s-%d", strings.ToLower(c.name), i+1))
		}
		fmt.Printf("  %s: WS 연결 %d/2\n", c.name, len(h.conns))
		all = append(all, h)
	}

	// 45초간 두 앱 모두 자기 토큰으로 REST를 계속 부른다.
	type stat struct{ ok, bad int }
	stats := make([]stat, len(all))
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		for i, h := range all {
			st, _ := restStatus(ctx, h.token)
			if st == 200 {
				stats[i].ok++
			} else {
				stats[i].bad++
				fmt.Printf("    %s REST 실패 HTTP %d\n", h.c.name, st)
			}
		}
		time.Sleep(3 * time.Second)
	}

	allGood := len(all) > 0
	for i, h := range all {
		frames := 0
		aliveConns := 0
		for _, r := range h.rx {
			frames += r.count()
		}
		for _, cn := range h.conns {
			if ping(ctx, cn) == nil {
				aliveConns++
			}
		}
		fmt.Printf("  %s: REST 200=%d 실패=%d · WS 프레임=%d · 45초 뒤 살아있는 연결=%d/%d\n",
			h.c.name, stats[i].ok, stats[i].bad, frames, aliveConns, len(h.conns))
		if stats[i].bad > 0 || aliveConns < len(h.conns) {
			allGood = false
		}
	}
	if allGood {
		fmt.Println("  ⇒ ✓ 두 앱 모두 REST+WS 공존이 정상 유지된다")
	} else {
		fmt.Println("  ⇒ ✗ 공존 중 깨지는 지점이 있다 — 위 수치 확인")
	}
	fmt.Println()
}

// ── H. 무중단 회전 ───────────────────────────────────────────────────────

func probeH(ctx context.Context, c cred) {
	fmt.Println("[H] make-before-break 회전에서 REST가 한 번도 안 끊기는가")

	tOld, _, err := issue(ctx, c)
	if err != nil {
		fail(fmt.Sprintf("TH-old 발급 실패: %v", err))
	}
	oldConn, err := dial(ctx, tOld)
	if err != nil {
		fmt.Printf("  옛 연결 실패: %v — H 판정 불가\n\n", err)
		return
	}
	declare(ctx, oldConn, "probe-H-old")
	oldRx := startReader(ctx, oldConn)
	time.Sleep(5 * time.Second)

	// REST를 200ms 간격으로 계속 두드리면서 회전을 수행한다.
	var (
		mu       sync.Mutex
		curToken = tOld
		failLog  []string
		ok, bad  int
	)
	pollCtx, stopPoll := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for pollCtx.Err() == nil {
			mu.Lock()
			tok := curToken
			mu.Unlock()
			st, _ := restStatus(pollCtx, tok)
			mu.Lock()
			switch {
			case st == 200:
				ok++
			case st != 0:
				bad++
				failLog = append(failLog, fmt.Sprintf("%s HTTP %d", time.Now().Format("15:04:05.000"), st))
			}
			mu.Unlock()
			time.Sleep(200 * time.Millisecond)
		}
	}()

	time.Sleep(3 * time.Second)

	// 1) 새 토큰 발급 — 이 순간 tOld가 죽는다.
	tNew, _, err := issue(ctx, c)
	if err != nil {
		stopPoll()
		wg.Wait()
		fmt.Printf("  새 토큰 발급 실패: %v\n\n", err)
		return
	}
	// 2) REST를 즉시 새 토큰으로 갈아탄다 (운영에서는 Redis 저장 → 재읽기).
	mu.Lock()
	curToken = tNew
	mu.Unlock()
	fmt.Println("  새 토큰 발급 + REST 전환 완료")

	// 3) 새 토큰으로 새 연결을 **먼저** 연다.
	newConn, err := dial(ctx, tNew)
	var newRx *reader
	if err != nil {
		fmt.Printf("  새 연결 실패: %v\n", err)
	} else {
		defer newConn.Close(websocket.StatusNormalClosure, "probe H")
		newRx = startReader(ctx, newConn)
		declare(ctx, newConn, "probe-H-new")
		fmt.Println("  새 연결 확립")
	}
	time.Sleep(5 * time.Second)

	// 4) 그다음에 옛 연결을 닫는다 — 이미 죽은 토큰이므로 무해해야 한다.
	_ = oldConn.Close(websocket.StatusNormalClosure, "rotated")
	fmt.Println("  옛 연결 종료")
	time.Sleep(8 * time.Second)

	stopPoll()
	wg.Wait()

	mu.Lock()
	gotOK, gotBad, logs := ok, bad, append([]string(nil), failLog...)
	mu.Unlock()

	fmt.Printf("  회전 구간 REST: 200=%d 실패=%d\n", gotOK, gotBad)
	for _, l := range logs {
		fmt.Printf("    실패 시각 %s\n", l)
	}
	if newRx != nil {
		fmt.Printf("  옛 연결 프레임=%d · 새 연결 프레임=%d · 새 연결 PING=%v\n",
			oldRx.count(), newRx.count(), ping(ctx, newConn) == nil)
	}
	if gotBad == 0 {
		fmt.Println("  ⇒ ✓ 회전 중 REST 무중단 — 두 앱 공존 설계가 성립한다")
	} else {
		fmt.Printf("  ⇒ ~ REST가 %d회 끊겼다 — 전환 타이밍/재시도로 흡수 가능한지 위 시각으로 확인\n", gotBad)
	}
	fmt.Println()
}

// ── helpers ─────────────────────────────────────────────────────────────

type reader struct {
	mu sync.Mutex
	n  int
	e  error
}

func (r *reader) count() int { r.mu.Lock(); defer r.mu.Unlock(); return r.n }
func (r *reader) err() error { r.mu.Lock(); defer r.mu.Unlock(); return r.e }

// startReader drains the connection — 읽지 않으면 백프레셔로 연결이 죽는다.
func startReader(ctx context.Context, conn *websocket.Conn) *reader {
	r := &reader{}
	go func() {
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				r.mu.Lock()
				r.e = err
				r.mu.Unlock()
				return
			}
			if strings.Contains(string(data), "\"type\":\"message\"") {
				r.mu.Lock()
				r.n++
				r.mu.Unlock()
			}
		}
	}()
	return r
}

func declare(ctx context.Context, conn *websocket.Conn, id string) {
	payload, _ := json.Marshal([]any{
		map[string]any{"id": id},
		map[string]any{"type": "trade:us", "codes": probeSymbols},
	})
	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_ = conn.Write(wctx, websocket.MessageText, payload)
}

func ping(ctx context.Context, conn *websocket.Conn) error {
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return conn.Write(wctx, websocket.MessageText, []byte("PING"))
}

func dial(ctx context.Context, token string) (*websocket.Conn, error) {
	dctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	conn, resp, err := websocket.Dial(dctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + token}},
	})
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("%w (HTTP %d)", err, resp.StatusCode)
		}
		return nil, err
	}
	return conn, nil
}

// restStatus calls a cheap MARKET_DATA endpoint and returns its HTTP status.
func restStatus(ctx context.Context, token string) (int, http.Header) {
	if token == "" {
		return 0, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(baseURL, "/")+"/api/v1/prices?symbols=005930", nil)
	if err != nil {
		return 0, nil
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()
	return resp.StatusCode, resp.Header
}

func restRemaining(ctx context.Context, token string) string {
	_, h := restStatus(ctx, token)
	if h == nil {
		return "?"
	}
	rem := h.Get("X-RateLimit-Remaining")
	lim := h.Get("X-RateLimit-Limit")
	if rem == "" {
		return "헤더없음"
	}
	return rem + "/" + lim
}

func issue(ctx context.Context, c cred) (string, int, error) {
	issueCount++
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.id)
	form.Set("client_secret", c.secret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
		Desc        string `json:"error_description"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode >= 400 || body.AccessToken == "" {
		return "", 0, fmt.Errorf("HTTP %d %s %s", resp.StatusCode, body.Error, body.Desc)
	}
	return body.AccessToken, body.ExpiresIn, nil
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func fail(msg string) {
	fmt.Println("FATAL:", msg)
	os.Exit(1)
}
