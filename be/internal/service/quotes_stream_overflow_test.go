package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/joho/godotenv"
)

func init() {
	// be/.env — integration tests share probe credentials.
	for _, p := range []string{".env", "../.env", "../../.env"} {
		_ = godotenv.Load(p)
	}
}

func streamWithClientCount(n int) *QuoteStreamService {
	clients := make([]*toss.StreamClient, n)
	return &QuoteStreamService{
		lastSeen: map[string]time.Time{},
		clients:  clients,
	}
}

func testSymbols(n int) []string {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = fmt.Sprintf("T%04d", i)
	}
	return out
}

func tradeSymbols(topics []toss.StreamTopic) map[string]struct{} {
	m := make(map[string]struct{})
	for _, t := range topics {
		if t.Channel == toss.ChannelTrade {
			m[t.Symbol] = struct{}{}
		}
	}
	return m
}

func orderbookSymbols(topics []toss.StreamTopic) map[string]struct{} {
	m := make(map[string]struct{})
	for _, t := range topics {
		if t.Channel == toss.ChannelOrderbook {
			m[t.Symbol] = struct{}{}
		}
	}
	return m
}

/*
201번째 활성 차트 — trade WS는 유지되고 orderbook만 빠진다.

운영 구성(자격증명 2 × 연결 2 × 100슬롯 = 400)에서 trade+orderbook 풀 커버는
200종목까지다. 201종목부터는 planTopics가 orderbook 슬롯을 줄이기 시작한다.
오류는 없다.
*/
func TestPlanTopics_over200_keepsTradeDropsOrderbook(t *testing.T) {
	const productionClients = 4 // PRIMARY 2 + SECONDARY 2
	s := streamWithClientCount(productionClients)
	total := productionClients * StreamSlotsPerConn

	syms := testSymbols(201)
	topics := s.planTopics(syms, syms)
	if len(topics) > total {
		t.Fatalf("topics=%d exceed budget %d", len(topics), total)
	}

	trades := tradeSymbols(topics)
	books := orderbookSymbols(topics)

	if len(trades) != 201 {
		t.Fatalf("trade subscriptions=%d want 201 — 201번째도 가격 WS를 받아야 한다", len(trades))
	}
	wantBooks := total - len(syms) // 400 - 201 = 199
	if len(books) != wantBooks {
		t.Fatalf("orderbook subscriptions=%d want %d", len(books), wantBooks)
	}
	if _, ok := trades["T0200"]; !ok {
		t.Fatal("201번째(T0200) trade 구독이 없다")
	}
	if _, ok := books["T0200"]; ok {
		t.Fatal("201번째(T0200) orderbook이 남아 있다 — REST로 받아야 한다")
	}
}

/*
401번째 활성 차트 — WS 구독 자체가 planTopics에서 잘린다.

슬롯 400을 trade만으로 채우면 그 이후 종목은 선언 목록에 들어가지 않는다.
오류는 없고 tickFast REST 폴링이 받는다(filterStreamedSymbols).
*/
func TestPlanTopics_over400_unsubscribedSymbols(t *testing.T) {
	s := streamWithClientCount(4)
	total := 4 * StreamSlotsPerConn

	syms := testSymbols(450)
	topics := s.planTopics(syms, syms)
	if len(topics) != total {
		t.Fatalf("topics=%d want %d (trade-only fill)", len(topics), total)
	}

	trades := tradeSymbols(topics)
	if len(trades) != total {
		t.Fatalf("trade subscriptions=%d want %d", len(trades), total)
	}
	if len(orderbookSymbols(topics)) != 0 {
		t.Fatal("450종목이면 orderbook 슬롯은 0이어야 한다")
	}
	if _, ok := trades["T0399"]; !ok {
		t.Fatal("400번째(T0399)는 구독되어야 한다")
	}
	if _, ok := trades["T0400"]; ok {
		t.Fatal("401번째(T0400)는 planTopics에서 잘려야 한다")
	}
}

/*
초과 종목은 IsLive/IsFed=false → filterStreamedSymbols가 REST 폴링 대상에 남긴다.

사용자/API에 오류를 반환하는 경로는 없다.
*/
func TestOverflowSymbols_fallBackToRESTPolling(t *testing.T) {
	s := streamWithClientCount(4)
	q := &QuotesService{stream: s}

	all := testSymbols(450)
	topics := s.planTopics(all, all)
	subscribed := tradeSymbols(topics)

	// 첫 종목만 WS 프레임을 받은 것처럼 표시.
	s.mark("T0000")

	for sym := range subscribed {
		if sym == "T0000" {
			continue
		}
		if s.IsLive(sym) {
			t.Fatalf("%s: 구독만 했지 프레임은 없는데 live다", sym)
		}
	}

	overflow := "T0400"
	if _, ok := subscribed[overflow]; ok {
		t.Fatalf("%s는 구독 목록에 없어야 한다", overflow)
	}
	if s.IsLive(overflow) || s.IsFed(overflow) {
		t.Fatalf("%s: WS가 없는데 live/fed다 — REST로 가야 한다", overflow)
	}

	polled := q.filterStreamedSymbols(all)
	if len(polled) != 449 {
		t.Fatalf("REST 폴링 대상=%d want 449 (T0000만 WS live로 제외)", len(polled))
	}
	found := false
	for _, sym := range polled {
		if sym == overflow {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("%s가 REST 폴링 대상에 없다 — 초과 종목이 굶는다", overflow)
	}
}

// forgetTerminalState는 구독에서 빠진 종목 버퍼를 정리한다.
func TestForgetTerminalState_dropsUnsubscribedSymbols(t *testing.T) {
	s := streamWithClientCount(4)
	s.terminal = map[string]*streamSymbolState{
		"T0000": {trades: []model.TradeTick{{Price: 1}}},
		"T0400": {trades: []model.TradeTick{{Price: 2}}},
	}

	topics := s.planTopics(testSymbols(450), testSymbols(450))
	keep := make(map[string]struct{}, len(topics))
	for _, topic := range topics {
		keep[topic.Symbol] = struct{}{}
	}
	s.forgetTerminalState(keep)

	s.termMu.Lock()
	defer s.termMu.Unlock()
	if _, ok := s.terminal["T0000"]; !ok {
		t.Fatal("구독 대상 T0000 버퍼가 지워졌다")
	}
	if _, ok := s.terminal["T0400"]; ok {
		t.Fatal("구독에서 잘린 T0400 버퍼가 남아 있다")
	}
}

// per-connection 분배: 연결당 100슬롯을 넘기지 않는다.
func TestSetSymbols_partitionsAcrossConnections(t *testing.T) {
	s := streamWithClientCount(4)
	topics := s.planTopics(testSymbols(200), testSymbols(200))
	if len(topics) != 400 {
		t.Fatalf("200종목 trade+book = 400 topics, got %d", len(topics))
	}

	per := make([][]toss.StreamTopic, len(s.clients))
	for i, topic := range topics {
		idx := i / StreamSlotsPerConn
		if idx >= len(per) {
			t.Fatalf("topic index %d exceeds connection count", i)
		}
		per[idx] = append(per[idx], topic)
	}
	for i, batch := range per {
		if len(batch) > StreamSlotsPerConn {
			t.Fatalf("connection %d carries %d topics — per-conn cap is %d",
				i, len(batch), StreamSlotsPerConn)
		}
	}
}

// live US tickers for Toss integration (probe-toss-coexist/round2.go 와 동일 풀).
func liveUSSymbols(n int) []string {
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

/*
TestIntegrationToss_rejectsOver100InSingleDeclare — 한 번에 105건 선언하면
Toss는 subscriptions(부분 거부)가 아니라 error 프레임으로 **전체 선언**을 거부한다.

우리 코드는 연결당 100슬롯 이하로 나눠 보내므로 이 오류 경로를 피한다.
*/
func TestIntegrationToss_rejectsOver100InSingleDeclare(t *testing.T) {
	conn, _ := dialTossWS(t)
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	declare := 105
	payload, _ := json.Marshal([]any{
		map[string]any{"id": "overflow-test-over"},
		map[string]any{"type": "trade:us", "codes": liveUSSymbols(declare)},
	})
	ctx := context.Background()
	if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
		t.Fatalf("Write subscribe: %v", err)
	}

	frame, ok := readWSFrameUntil(ctx, conn, 15*time.Second, func(data []byte) bool {
		s := string(data)
		return strings.Contains(s, `"type":"error"`) || strings.Contains(s, `"type":"subscriptions"`)
	})
	if !ok {
		t.Fatal("error/subscriptions 프레임을 못 받았다")
	}
	t.Logf("response: %s", truncateWS(string(frame), 300))

	if strings.Contains(string(frame), `"type":"subscriptions"`) {
		t.Fatal("105건 선언이 subscriptions로 수락됐다 — cap 100 위반")
	}
	if !strings.Contains(string(frame), "too-many-topics") {
		t.Fatalf("expected too-many-topics error, got: %s", truncateWS(string(frame), 200))
	}
}

/*
TestIntegrationToss_acceptsUnder100InSingleDeclare — 100 미만 선언은 error 없이 수락된다.
*/
func TestIntegrationToss_acceptsUnder100InSingleDeclare(t *testing.T) {
	conn, _ := dialTossWS(t)
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	codes := []string{"AAPL", "TSLA", "NVDA"}
	payload, _ := json.Marshal([]any{
		map[string]any{"id": "overflow-test-3"},
		map[string]any{"type": "trade:us", "codes": codes},
	})
	ctx := context.Background()
	if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
		t.Fatalf("Write subscribe: %v", err)
	}

	frame, ok := readWSFrameUntil(ctx, conn, 15*time.Second, func(data []byte) bool {
		s := string(data)
		return strings.Contains(s, `"type":"subscriptions"`) || strings.Contains(s, `"type":"error"`)
	})
	if !ok {
		t.Fatal("subscriptions/error 프레임을 못 받았다")
	}

	var env struct {
		Type       string   `json:"type"`
		Subscribed []string `json:"subscribed"`
	}
	if err := json.Unmarshal(frame, &env); err != nil {
		t.Fatalf("unmarshal: %v raw=%s", err, truncateWS(string(frame), 200))
	}
	if env.Type == "error" || strings.Contains(string(frame), `"type":"error"`) {
		t.Fatalf("선언이 error로 거부됐다: %s", truncateWS(string(frame), 200))
	}
	if len(env.Subscribed) != len(codes) {
		t.Fatalf("subscribed=%d want %d", len(env.Subscribed), len(codes))
	}
}

func dialTossWS(t *testing.T) (*websocket.Conn, string) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	tossCfg := cfg.Toss
	if tossCfg.ClientID == "" || tossCfg.ClientSecret == "" {
		t.Skip("Toss credentials missing — set TOSS_PRIMARY_CLIENT_ID/SECRET in be/.env")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := toss.NewClientForCredentials(tossCfg.BaseURL, tossCfg.ClientID, tossCfg.ClientSecret, nil, "overflow-test:token")
	if err != nil {
		t.Fatalf("NewClientForCredentials: %v", err)
	}
	token, err := client.TokenManagers()[0].AccessToken(ctx)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}

	conn, _, err := websocket.Dial(ctx, "wss://openapi-ws.tossinvest.com/ws/v1", &websocket.DialOptions{
		HTTPHeader: map[string][]string{"Authorization": {"Bearer " + token}},
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	return conn, token
}

func readWSFrameUntil(ctx context.Context, conn *websocket.Conn, wait time.Duration, match func([]byte) bool) ([]byte, bool) {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		rctx, cancel := context.WithDeadline(ctx, deadline)
		_, data, err := conn.Read(rctx)
		cancel()
		if err != nil {
			return nil, false
		}
		if match(data) {
			return data, true
		}
	}
	return nil, false
}

func truncateWS(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
