package toss

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

/*
Toss 실시간 시세 구독 클라이언트.

# 왜 필요한가

`tickFast`는 종목당 `/orderbook` + `/trades` 2호출을 주기적으로 낸다. 그 둘은
`/prices`와 **같은 `MARKET_DATA` 버킷**(한도 15)을 쓰므로, 상한 40종목이면
초당 80호출이라 한도를 크게 넘긴다 — 실측(2026-08-24 KRX 개장 중)
`rate-limit exceeded` **분당 251건**. 구독으로 옮기면 그 REST 호출이 **0**이 된다.

# 실측으로 확인한 프로토콜

스펙만으로는 구독 선언 형식을 확정할 수 없어 세 번 거부당하며 좁혔다:

	{"type":"subscribe","codes":[…]}   → request must be a json array
	["trade:us:AAPL", …]               → each request must be an object
	[{"type":"subscribe","codes":[…]}] → unknown type: subscribe

정답은 **JSON 배열 안의 객체**이고, `type`은 동작이 아니라 **채널:시장**이다:

	[{"id":"req-1"},{"type":"trade:kr","codes":["005930"]}]

`codes`에는 접두사 없이 **그 시장의 심볼만** 넣는다. 서버는 확인 프레임
(`{"type":"subscriptions","subscribed":[…],"rejected":[…]}`)을 돌려주고,
이후 데이터는 `{"type":"message","topic":"trade:kr:005930","data":{…}}` 형태다.

# ★ 토큰 수명 — 이 파일에서 가장 중요한 제약

한때 여기에는 두 명제가 적혀 있었다.

	① 웹소켓을 닫으면 그 액세스 토큰이 영구 무효화된다
	② client_id당 활성 토큰이 하나다 — 새로 발급하면 이전 것이 즉시 죽는다

**①은 틀렸다.** 2026-08-25 재실측(`cmd/probe-toss-coexist`)에서 정상 종료(E)·
강제 절단(F)·종료 후 60초(J) 어느 쪽에서도 그 토큰의 REST가 **200**이었고,
**같은 토큰으로 재연결도 3/3 성공**했다(I, PRIMARY·SECONDARY 각각).

원래 관측은 혼동이었다. 토큰을 죽인 것은 종료가 아니라 **종료 다음의 재연결이
부른 재발급**(②)이었다. 당시 코드는 재연결마다 `IssueFresh`를 불렀다.

②는 그대로 사실이다. 그래서 이 파일의 규칙은 하나로 줄어든다:

	**재연결에 토큰을 새로 발급하지 않는다.** 들고 있던 것을 그대로 쓴다.
	401이 실제로 났을 때만, 그 실패한 토큰을 관리자에게 알려 주고 다음 것을 받는다.

이 규칙만 지키면 **REST와 자격증명을 공유해도 안전하다** — 실측으로 두 앱 각각
(REST + WS 2연결)을 45초 동시 가동해 REST 30/30 200 · 연결 4/4 생존을 확인했다(G).

부수 확인: 웹소켓 트래픽은 REST의 `MARKET_DATA` 쿼터를 **먹지 않는다**
(프레임 204건에도 remaining 13/15 → 14/15).
*/

const (
	streamURL = "wss://openapi-ws.tossinvest.com/ws/v1"

	// streamPingInterval — 스펙은 클라이언트 송신이 180초간 없으면 서버가 끊는다고
	// 한다. 60초면 그 1/3이라 한 번 놓쳐도 여유가 있다.
	streamPingInterval = 60 * time.Second

	// streamDeclareDebounce — 구독 목록 변경을 모아 한 번에 선언한다.
	// 선언 한도가 5회/초라, 종목이 하나씩 들락거릴 때마다 선언하면 금방 닿는다.
	streamDeclareDebounce = 300 * time.Millisecond

	// streamMaxBackoff — 재연결 지수 백오프 상한.
	streamMaxBackoff = 30 * time.Second
)

// StreamChannel is one subscribable feed.
type StreamChannel string

const (
	ChannelTrade     StreamChannel = "trade"
	ChannelOrderbook StreamChannel = "orderbook"
)

// StreamTopic identifies one channel×symbol subscription — the unit Toss counts
// against the 100-per-connection limit.
type StreamTopic struct {
	Channel StreamChannel
	Market  string // "kr" | "us"
	Symbol  string
}

func (t StreamTopic) String() string {
	return fmt.Sprintf("%s:%s:%s", t.Channel, t.Market, t.Symbol)
}

// TradeFrame is one execution tick.
type TradeFrame struct {
	Topic     StreamTopic
	Price     string
	Volume    string
	Timestamp time.Time
	Currency  Currency
}

// OrderbookFrame is one order-book snapshot.
type OrderbookFrame struct {
	Topic     StreamTopic
	Timestamp time.Time
	Currency  Currency
	Asks      []OrderbookLevel
	Bids      []OrderbookLevel
}

// OrderbookLevel is one price rung.
type OrderbookLevel struct {
	Price  string
	Volume string
}

/*
StreamHandler receives decoded frames. Implementations must not block —
the read loop is what keeps the connection alive.

`OnSubscriptions` / `OnDisconnect`가 있는 이유: 프레임 도착만으로는 「구독이
살아 있다」를 판정할 수 없다. 거래가 뜸한 종목은 정상적으로도 몇 분씩 조용하고,
그때 프레임 부재를 장애로 오인하면 멀쩡한 구독을 두고 REST로 되돌아간다.

서버가 주는 확인 프레임(`subscriptions`)과 연결 종료를 함께 보면 「조용한 것」과
「끊긴 것」이 갈린다.
*/
type StreamHandler interface {
	OnTrade(TradeFrame)
	OnOrderbook(OrderbookFrame)
	// OnSubscriptions reports what the server accepted for this connection.
	OnSubscriptions(conn string, accepted []StreamTopic, rejected []string)
	// OnDisconnect says one connection's subscriptions no longer stand.
	OnDisconnect(conn string)
}

/*
StreamClient — 하나의 웹소켓 연결과 그 구독 목록을 관리한다.

연결 하나당 구독 100건이 상한이라, 슬롯을 더 쓰려면 이 구조체를 여러 개 둔다
(계정당 2연결 × OAuth 앱 수).
*/
type StreamClient struct {
	/*
		token — 연결에 쓸 토큰을 준다.

		`stale`이 비어 있으면 「지금 쓸 것을 달라」, 비어 있지 않으면 「이 토큰으로
		핸드셰이크가 401이었다, 다음 것을 달라」는 뜻이다.

		**실패한 토큰을 넘기는 것이 핵심이다.** client_id당 활성 토큰이 하나라
		발급이 파괴적이므로, 「무조건 새로 발급」은 형제 연결과 REST가 쓰던 토큰까지
		죽인다. 무엇이 죽었는지 알려 줘야 관리자가 **필요할 때만 한 번** 발급한다
		(`TokenManager.TokenAfter`).
	*/
	token   func(ctx context.Context, stale string) (string, error)
	handler StreamHandler
	name    string

	mu      sync.Mutex
	desired map[string]StreamTopic
	dirty   bool
	conn    *websocket.Conn

	declareNow chan struct{}
}

// NewStreamClient builds a client that dials with a caller-managed token.
//
// 그 토큰의 자격증명을 REST와 공유해도 된다 — 재연결이 재발급을 부르지 않는
// 한(파일 헤더 참조).
func NewStreamClient(token func(ctx context.Context, stale string) (string, error), handler StreamHandler, name string) *StreamClient {
	return &StreamClient{
		token:      token,
		handler:    handler,
		name:       name,
		desired:    map[string]StreamTopic{},
		declareNow: make(chan struct{}, 1),
	}
}

// Subscribe replaces the whole desired topic set. The protocol is declarative
// full-replace, so callers hand over the complete list rather than deltas.
func (c *StreamClient) Subscribe(topics []StreamTopic) {
	next := make(map[string]StreamTopic, len(topics))
	for _, t := range topics {
		if t.Symbol == "" || t.Market == "" {
			continue
		}
		next[t.String()] = t
	}

	c.mu.Lock()
	changed := len(next) != len(c.desired)
	if !changed {
		for k := range next {
			if _, ok := c.desired[k]; !ok {
				changed = true
				break
			}
		}
	}
	if changed {
		c.desired = next
		c.dirty = true
	}
	c.mu.Unlock()

	if changed {
		select {
		case c.declareNow <- struct{}{}:
		default: // 이미 예약돼 있다 — 디바운스가 묶는다.
		}
	}
}

// Run keeps one connection alive until ctx ends, reconnecting with backoff.
func (c *StreamClient) Run(ctx context.Context) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := c.session(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			logStream("%s: session ended: %v (retry in %s)", c.name, err, backoff)
		}
		/*
			지수 백오프 + jitter.

			jitter가 없으면 여러 연결이 **같은 순간에** 재연결한다 — 상류 배포로
			한꺼번에 끊겼을 때(`server-shutdown`)가 정확히 그 경우다. 그러면
			재연결 스파이크가 상류를 다시 밀어내 끊김이 반복된다.

			실제 대기는 backoff의 50~100% 구간에서 고른다.
		*/
		wait := backoff/2 + time.Duration(rand.Int63n(int64(backoff/2)+1))
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		backoff = min(backoff*2, streamMaxBackoff)
	}
}

// session runs one connection from handshake to close.
func (c *StreamClient) session(ctx context.Context) error {
	/*
		토큰은 **공유 관리자**에게 받는다.

		재연결에도 같은 토큰을 그대로 쓴다 — 연결을 닫아도 토큰은 살아 있고,
		죽은 토큰으로 재연결도 되지 않는다는 것을 실측으로 확인했다
		(`docs/TOSS_TOKEN_DESIGN.md`의 I·J). 그래서 정상 경로에서는 발급이
		일어나지 않고, REST가 쓰는 토큰도 건드리지 않는다.

		401은 딱 한 경우에 난다: 만료 직전 배경 갱신이 새 토큰을 발급해 우리가
		들고 있던 것이 죽었을 때. 그때만 **실패한 토큰을 알려 주고** 다음 것을 받는다.
	*/
	token, err := c.token(ctx, "")
	if err != nil {
		return fmt.Errorf("token: %w", err)
	}

	conn, status, err := dialStream(ctx, token)
	if status == http.StatusUnauthorized {
		if token, err = c.token(ctx, token); err != nil {
			return fmt.Errorf("token refresh: %w", err)
		}
		conn, status, err = dialStream(ctx, token)
	}
	if err != nil {
		if status > 0 {
			return fmt.Errorf("dial: %w (HTTP %d)", err, status)
		}
		return fmt.Errorf("dial: %w", err)
	}
	// 프레임이 크지 않다 — 호가 10단이 최대다. 기본 한도로 충분하다.
	defer conn.Close(websocket.StatusNormalClosure, "shutting down")

	c.mu.Lock()
	c.conn = conn
	c.dirty = true // 재연결이면 전체를 다시 선언해야 한다.
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.conn = nil
		c.mu.Unlock()
		// 이 연결이 들고 있던 구독은 더 이상 유효하지 않다. 재연결하면 전체를
		// 다시 선언하고 서버가 다시 확인해 준다.
		if c.handler != nil {
			c.handler.OnDisconnect(c.name)
		}
	}()

	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); c.writeLoop(sessionCtx, conn) }()

	var readErr error
	go func() { defer wg.Done(); readErr = c.readLoop(sessionCtx, conn); cancel() }()

	wg.Wait()
	return readErr
}

// writeLoop owns every client→server frame: keepalive and declarations.
//
// 한 goroutine이 쓰기를 독점해야 한다 — websocket 쓰기는 동시 호출이 안전하지 않다.
func (c *StreamClient) writeLoop(ctx context.Context, conn *websocket.Conn) {
	ping := time.NewTicker(streamPingInterval)
	defer ping.Stop()

	// 연결 직후 한 번 선언한다.
	c.declare(ctx, conn)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ping.C:
			wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Write(wctx, websocket.MessageText, []byte("PING"))
			cancel()
			if err != nil {
				return
			}
		case <-c.declareNow:
			// 디바운스 — 여러 변경을 한 선언으로 묶어 5회/초 한도에서 멀어진다.
			select {
			case <-ctx.Done():
				return
			case <-time.After(streamDeclareDebounce):
			}
			c.declare(ctx, conn)
		}
	}
}

// declare sends the full desired list when it changed.
func (c *StreamClient) declare(ctx context.Context, conn *websocket.Conn) {
	c.mu.Lock()
	if !c.dirty {
		c.mu.Unlock()
		return
	}
	byType := map[string][]string{}
	for _, t := range c.desired {
		key := string(t.Channel) + ":" + t.Market
		byType[key] = append(byType[key], t.Symbol)
	}
	c.dirty = false
	c.mu.Unlock()

	// 스펙: JSON 배열 하나. 첫 원소의 id는 응답을 맞춰 보기 위한 것이다.
	payload := []any{map[string]any{"id": c.name}}
	for typ, codes := range byType {
		payload = append(payload, map[string]any{"type": typ, "codes": codes})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}

	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := conn.Write(wctx, websocket.MessageText, raw); err != nil {
		// 실패했으면 다음 기회에 다시 선언해야 한다.
		c.mu.Lock()
		c.dirty = true
		c.mu.Unlock()
	}
}

// readLoop decodes frames until the connection dies.
func (c *StreamClient) readLoop(ctx context.Context, conn *websocket.Conn) error {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		c.handleFrame(data)
	}
}

type streamEnvelope struct {
	Type       string          `json:"type"`
	Topic      string          `json:"topic"`
	Data       json.RawMessage `json:"data"`
	Subscribed []string        `json:"subscribed"`
	Rejected   []string        `json:"rejected"`
	Code       string          `json:"code"`
	Error      *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *StreamClient) handleFrame(data []byte) {
	var env streamEnvelope
	if json.Unmarshal(data, &env) != nil {
		return
	}

	switch env.Type {
	case "message":
		c.handleMessage(env)
	case "subscriptions":
		if len(env.Rejected) > 0 {
			// 거부는 조용히 넘기면 안 된다 — 그 종목만 영영 안 움직인다.
			logStream("%s: %d subscribed, rejected=%v", c.name, len(env.Subscribed), env.Rejected)
		}
		if c.handler != nil {
			accepted := make([]StreamTopic, 0, len(env.Subscribed))
			for _, raw := range env.Subscribed {
				if t, ok := parseStreamTopic(raw); ok {
					accepted = append(accepted, t)
				}
			}
			c.handler.OnSubscriptions(c.name, accepted, env.Rejected)
		}
	case "pong":
		// keepalive 응답. 할 일 없음.
	case "error":
		if env.Error != nil {
			logStream("%s: server error %s: %s", c.name, env.Error.Code, env.Error.Message)
			if env.Error.Code == "server-shutdown" {
				// 상류 배포다. 읽기 루프가 곧 끊기고 Run이 재연결한다.
				return
			}
		}
	}
}

func (c *StreamClient) handleMessage(env streamEnvelope) {
	topic, ok := parseStreamTopic(env.Topic)
	if !ok || c.handler == nil {
		return
	}

	switch topic.Channel {
	case ChannelTrade:
		var d struct {
			Price     string `json:"price"`
			Volume    string `json:"volume"`
			Timestamp string `json:"timestamp"`
			Currency  string `json:"currency"`
		}
		if json.Unmarshal(env.Data, &d) != nil {
			return
		}
		c.handler.OnTrade(TradeFrame{
			Topic:     topic,
			Price:     d.Price,
			Volume:    d.Volume,
			Timestamp: parseStreamTime(d.Timestamp),
			Currency:  Currency(d.Currency),
		})
	case ChannelOrderbook:
		var d struct {
			Timestamp string `json:"timestamp"`
			Currency  string `json:"currency"`
			Asks      []struct {
				Price  string `json:"price"`
				Volume string `json:"volume"`
			} `json:"asks"`
			Bids []struct {
				Price  string `json:"price"`
				Volume string `json:"volume"`
			} `json:"bids"`
		}
		if json.Unmarshal(env.Data, &d) != nil {
			return
		}
		frame := OrderbookFrame{
			Topic:     topic,
			Timestamp: parseStreamTime(d.Timestamp),
			Currency:  Currency(d.Currency),
		}
		for _, a := range d.Asks {
			frame.Asks = append(frame.Asks, OrderbookLevel{Price: a.Price, Volume: a.Volume})
		}
		for _, b := range d.Bids {
			frame.Bids = append(frame.Bids, OrderbookLevel{Price: b.Price, Volume: b.Volume})
		}
		c.handler.OnOrderbook(frame)
	}
}

// parseStreamTopic splits "trade:kr:005930".
func parseStreamTopic(raw string) (StreamTopic, bool) {
	parts := strings.Split(raw, ":")
	if len(parts) != 3 {
		return StreamTopic{}, false
	}
	ch := StreamChannel(parts[0])
	if ch != ChannelTrade && ch != ChannelOrderbook {
		return StreamTopic{}, false
	}
	return StreamTopic{Channel: ch, Market: parts[1], Symbol: parts[2]}, true
}

// parseStreamTime accepts the spec's ISO 8601 with offset.
func parseStreamTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t
	}
	return time.Time{}
}

// logStream keeps the stream log prefix in one place.
func logStream(format string, args ...any) {
	log.Printf("toss stream: "+format, args...)
}

// dialStream opens one connection and reports the handshake status so the
// caller can tell "token died" (401) from every other failure.
func dialStream(ctx context.Context, token string) (*websocket.Conn, int, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	conn, resp, err := websocket.Dial(dialCtx, streamURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + token}},
	})
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	return conn, status, err
}
