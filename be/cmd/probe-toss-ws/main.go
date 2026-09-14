/*
Command probe-toss-ws answers the two questions that gate the websocket
migration. Both are things the Toss spec does not state, so they must be
measured against the live API before any design depends on them.

	V2 — 웹소켓 트래픽이 REST 한도(MARKET_DATA)를 소비하는가?
	     소비한다면 「tickFast를 구독으로 바꿔 REST 호출을 0으로」라는 전제가 무너진다.
	V3 — 스펙의 「계정당 2연결」에서 '계정'이 OAuth 앱 단위인가?
	     앱 단위라면 PRIMARY+SECONDARY로 구독 슬롯이 200 → 400이 된다.

**운영 코드를 건드리지 않는다.** OAuth와 REST를 직접 호출하는 자족형이다 —
프로브 하나를 위해 `toss.Client`에 토큰·헤더 접근자를 뚫으면 그 구멍이 남는다.

읽기 전용이다. 구독과 조회만 하고 아무것도 쓰지 않는다.

	go run ./cmd/probe-toss-ws
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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/joho/godotenv"
)

const (
	defaultBaseURL = "https://openapi.tossinvest.com"
	wsURL          = "wss://openapi-ws.tossinvest.com/ws/v1"
	frameWindow    = 45 * time.Second
	holdWindow     = 20 * time.Second
)

// probeSymbols — 체결이 자주 나는 종목이어야 프레임이 실제로 흐른다.
// 국내장이 닫혀 있어도 미국 종목은 KST 야간에 움직인다.
var probeSymbols = []string{"AAPL", "TSLA", "NVDA"}

type credential struct {
	name    string
	id      string
	secret  string
	token   string
	baseURL string
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() { <-stop; cancel() }()

	_ = godotenv.Load(".env")

	base := envOr("TOSS_API_BASE_URL", defaultBaseURL)
	creds := []credential{
		{name: "PRIMARY", id: firstEnv("TOSS_PRIMARY_CLIENT_ID", "TOSS_CLIENT_ID"),
			secret: firstEnv("TOSS_PRIMARY_CLIENT_SECRET", "TOSS_CLIENT_SECRET"), baseURL: base},
		{name: "SECONDARY", id: os.Getenv("TOSS_SECONDARY_CLIENT_ID"),
			secret: os.Getenv("TOSS_SECONDARY_CLIENT_SECRET"), baseURL: base},
	}

	fmt.Println("=== Toss 웹소켓 프로브 ===")
	var usable []credential
	for _, c := range creds {
		ok := c.id != "" && c.secret != ""
		fmt.Printf("  %-9s 설정됨: %t\n", c.name, ok)
		if ok {
			usable = append(usable, c)
		}
	}
	if len(usable) == 0 {
		fail("자격증명이 없다 — TOSS_PRIMARY_CLIENT_ID/SECRET 를 확인할 것")
	}
	fmt.Println()

	for i := range usable {
		tok, err := issueToken(ctx, usable[i])
		if err != nil {
			fail(fmt.Sprintf("%s 토큰 발급 실패: %v", usable[i].name, err))
		}
		usable[i].token = tok
	}

	probeV2(ctx, usable[0])
	probeV3(ctx, usable)
}

// ── V2 ──────────────────────────────────────────────────────────────────

func probeV2(ctx context.Context, c credential) {
	fmt.Println("[V2] 웹소켓 트래픽이 MARKET_DATA REST 한도를 소비하는가")

	before, ok := restRemaining(ctx, c)
	if !ok {
		fmt.Println("  ? X-RateLimit-Remaining 헤더를 못 읽었다 — 판정 불가")
		return
	}
	fmt.Printf("  구독 전  remaining = %d\n", before)

	/*
		구독 선언 형식 — 실측으로 좁힌 뒤 AsyncAPI 스펙에서 확정했다.

		  {"type":"subscribe","codes":[…]}   → "request must be a json array"
		  ["trade:us:AAPL", …]               → "each request must be an object"
		  [{"type":"subscribe","codes":[…]}] → "unknown type: subscribe"

		스펙 확인본: 「연결 후 JSON **배열 1개**를 텍스트 프레임으로 보냅니다」.
		`type`은 동작이 아니라 **채널:시장**(`trade:us`·`orderbook:kr`…)이고,
		`codes`에는 그 시장의 **심볼만** 넣는다(접두사를 반복하지 않는다).
	*/
	shapes := []struct {
		name string
		body any
	}{
		{"[{id},{type:trade:us,codes}]", []any{
			map[string]any{"id": "probe-1"},
			map[string]any{"type": "trade:us", "codes": probeSymbols},
		}},
	}

	var frames int
	var accepted string
	for _, sh := range shapes {
		fmt.Printf("  선언 형식 시험: %s\n", sh.name)
		n, rejected, err := subscribeAndCount(ctx, c.token, sh.body, 12*time.Second)
		if err != nil {
			fmt.Printf("    연결 실패: %v\n", err)
			continue
		}
		if rejected != "" {
			fmt.Printf("    거부: %s\n", rejected)
			continue
		}
		accepted, frames = sh.name, n
		fmt.Printf("    ✓ 수락됨 — 12초간 프레임 %d개\n", n)
		break
	}
	if accepted == "" {
		fmt.Println("  ✗ 모든 후보가 거부됐다 — AsyncAPI 스펙의 정확한 스키마 확인 필요")
		fmt.Println("    (연결·인증 자체는 성공하므로 계약은 살아 있다)")
		return
	}
	fmt.Printf("  %v간 수신 프레임 = %d (형식 %s)\n", frameWindow, frames, accepted)

	after, ok := restRemaining(ctx, c)
	if !ok {
		fmt.Println("  ? 사후 헤더를 못 읽었다 — 판정 불가")
		return
	}
	fmt.Printf("  구독 후  remaining = %d\n", after)

	/*
		판정 기준.

		이 프로브 자신이 REST를 2회 부르므로 잔량은 그만큼은 줄어든다(창이 리셋되면
		오히려 늘 수도 있다). 프레임이 수백 개인데 잔량이 그 규모로 줄었다면 같은
		예산을 쓰는 것이고, 프레임 수와 무관하게 유지됐다면 별도 예산이다.
	*/
	drop := before - after
	switch {
	case frames == 0:
		fmt.Println("  ? 프레임 0 — 장 시간에 다시 돌릴 것 (지금은 판정 불가)")
	case drop >= int64(frames)/2 && drop > 5:
		fmt.Printf("  ✗ 공유로 보인다 (잔량 %d 감소 vs 프레임 %d) — 웹소켓 전환 계획 재검토 필요\n", drop, frames)
	default:
		fmt.Printf("  ✓ 별도 예산으로 보인다 (프레임 %d개에도 잔량 감소 %d)\n", frames, drop)
	}
}

// ── V3 ──────────────────────────────────────────────────────────────────

func probeV3(ctx context.Context, creds []credential) {
	fmt.Println("\n[V3] '계정당 2연결'의 계정 단위")
	if len(creds) < 2 {
		fmt.Println("  - SECONDARY 미설정 — 판정 불가. 슬롯은 200으로 가정할 것")
		return
	}
	fmt.Println("  PRIMARY 2연결 + SECONDARY 2연결 = 4연결 동시 유지 시도")

	// 토큰을 다시 발급한다 — 앞선 V2가 쓴 토큰을 재사용하면 401이 났다(실측).
	for i := range creds {
		if tok, err := issueToken(ctx, creds[i]); err == nil {
			creds[i].token = tok
		} else {
			fmt.Printf("    %s 토큰 재발급 실패: %v\n", creds[i].name, err)
		}
	}

	var conns []*websocket.Conn
	defer func() {
		for _, conn := range conns {
			_ = conn.Close(websocket.StatusNormalClosure, "probe done")
		}
	}()

	for _, c := range creds {
		for i := 0; i < 2; i++ {
			conn, err := dial(ctx, c.token)
			if err != nil {
				fmt.Printf("    %s #%d 연결 실패: %v\n", c.name, i+1, err)
				continue
			}
			conns = append(conns, conn)
		}
	}
	fmt.Printf("  연결 성공 = %d/4\n", len(conns))

	select {
	case <-ctx.Done():
		return
	case <-time.After(holdWindow):
	}

	// 스펙상 상한을 넘으면 **가장 오래된 연결이 끊긴다**. PING이 통하면 살아 있다.
	alive := 0
	for _, conn := range conns {
		wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := conn.Write(wctx, websocket.MessageText, []byte("PING"))
		cancel()
		if err == nil {
			alive++
		}
	}
	fmt.Printf("  %v 뒤 살아 있는 연결 = %d/4\n", holdWindow, alive)

	if alive >= 4 {
		fmt.Println("  ✓ OAuth 앱 단위 — 슬롯 400 (trade 280 + orderbook 120 배분 가능)")
	} else {
		fmt.Println("  ✗ 증권 계좌 단위 — 슬롯 200. SECONDARY는 REST 오버플로 전용으로 둘 것")
	}
}

// ── helpers ─────────────────────────────────────────────────────────────

func subscribeAndCount(ctx context.Context, token string, shape any, window time.Duration) (int, string, error) {
	ctx, cancel := context.WithTimeout(ctx, window+20*time.Second)
	defer cancel()

	conn, err := dial(ctx, token)
	if err != nil {
		return 0, "", err
	}
	defer conn.Close(websocket.StatusNormalClosure, "probe done")

	payload, _ := json.Marshal(shape)

	wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
	err = conn.Write(wctx, websocket.MessageText, payload)
	wcancel()
	if err != nil {
		return 0, "", fmt.Errorf("declare: %w", err)
	}

	deadline := time.Now().Add(window)
	count := 0
	for time.Now().Before(deadline) {
		rctx, rcancel := context.WithDeadline(ctx, deadline)
		_, data, err := conn.Read(rctx)
		rcancel()
		if err != nil {
			break
		}
		if len(data) == 0 {
			continue
		}
		// 서버가 형식을 거부하면 그 자리에서 멈춘다 — 다음 후보를 시험해야 한다.
		if strings.Contains(string(data), `"type":"error"`) {
			return count, truncate(string(data), 220), nil
		}
		count++
		if count <= 2 {
			fmt.Printf("    샘플 프레임: %s\n", truncate(string(data), 220))
		}
	}
	return count, "", nil
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

// restRemaining calls a cheap MARKET_DATA endpoint and reads the bucket header.
func restRemaining(ctx context.Context, c credential) (int64, bool) {
	u := strings.TrimRight(c.baseURL, "/") + "/api/v1/prices?symbols=005930"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, false
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()

	raw := resp.Header.Get("X-RateLimit-Remaining")
	if raw == "" {
		fmt.Printf("    (헤더 없음 · HTTP %d · limit=%q reset=%q)\n",
			resp.StatusCode, resp.Header.Get("X-RateLimit-Limit"), resp.Header.Get("X-RateLimit-Reset"))
		return 0, false
	}
	fmt.Printf("    (HTTP %d · limit=%s)\n", resp.StatusCode, resp.Header.Get("X-RateLimit-Limit"))
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func issueToken(ctx context.Context, c credential) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.id)
	form.Set("client_secret", c.secret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var body struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Desc        string `json:"error_description"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode >= 400 || body.AccessToken == "" {
		return "", fmt.Errorf("HTTP %d %s %s", resp.StatusCode, body.Error, body.Desc)
	}
	return body.AccessToken, nil
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

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func fail(msg string) {
	fmt.Println("FATAL:", msg)
	os.Exit(1)
}

// codesFor builds the channel:market:symbol codes the spec documents.
func codesFor(symbols []string) []string {
	out := make([]string, 0, len(symbols))
	for _, s := range symbols {
		out = append(out, "trade:us:"+s)
	}
	return out
}

func objPerCode(typ string, codes []string) []any {
	out := make([]any, 0, len(codes))
	for _, c := range codes {
		out = append(out, map[string]any{"type": typ, "code": c})
	}
	return out
}

func objOnlyCode(codes []string) []any {
	out := make([]any, 0, len(codes))
	for _, c := range codes {
		out = append(out, map[string]any{"code": c})
	}
	return out
}
