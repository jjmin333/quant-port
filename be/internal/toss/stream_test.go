package toss

import (
	"encoding/json"
	"testing"
	"time"
)

/*
구독 선언 형식은 **실측으로 확정한 것**이라 테스트로 고정한다.

스펙만으로는 확정할 수 없어 세 번 거부당하며 좁혔다(stream.go 헤더 참조).
누가 나중에 "더 자연스러운" 형태로 바꾸면 서버가 조용히 거부하고 그 종목만
안 움직인다 — 그 회귀를 여기서 잡는다.
*/
func TestStreamTopic_String(t *testing.T) {
	got := StreamTopic{Channel: ChannelTrade, Market: "kr", Symbol: "005930"}.String()
	if got != "trade:kr:005930" {
		t.Fatalf("topic=%q want trade:kr:005930", got)
	}
}

func TestParseStreamTopic(t *testing.T) {
	topic, ok := parseStreamTopic("orderbook:us:AAPL")
	if !ok {
		t.Fatal("parse 실패")
	}
	if topic.Channel != ChannelOrderbook || topic.Market != "us" || topic.Symbol != "AAPL" {
		t.Fatalf("topic=%+v", topic)
	}

	// 모르는 채널은 거른다 — personal:order 같은 것이 시세 경로로 새면 안 된다.
	if _, ok := parseStreamTopic("personal:order:123"); ok {
		t.Fatal("알 수 없는 채널이 통과했다")
	}
	if _, ok := parseStreamTopic("trade:kr"); ok {
		t.Fatal("형식이 어긋난 topic이 통과했다")
	}
}

/*
선언 payload는 **JSON 배열 안의 객체**여야 하고, `type`은 채널:시장이며
`codes`에는 접두사 없이 심볼만 들어간다.

실측으로 확인한 거부 사유들:

	객체로 보냄        → request must be a json array
	문자열 배열로 보냄 → each request must be an object
	type:"subscribe"   → unknown type: subscribe
*/
func TestStreamClient_declarePayloadShape(t *testing.T) {
	c := NewStreamClient(nil, nil, "probe")
	c.Subscribe([]StreamTopic{
		{Channel: ChannelTrade, Market: "kr", Symbol: "005930"},
		{Channel: ChannelTrade, Market: "kr", Symbol: "000660"},
		{Channel: ChannelOrderbook, Market: "us", Symbol: "AAPL"},
	})

	c.mu.Lock()
	byType := map[string][]string{}
	for _, tp := range c.desired {
		key := string(tp.Channel) + ":" + tp.Market
		byType[key] = append(byType[key], tp.Symbol)
	}
	c.mu.Unlock()

	payload := []any{map[string]any{"id": "probe"}}
	for typ, codes := range byType {
		payload = append(payload, map[string]any{"type": typ, "codes": codes})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded []map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("최상위가 JSON 배열이 아니다: %v", err)
	}
	if len(decoded) != 3 { // id + trade:kr + orderbook:us
		t.Fatalf("원소 %d개 — id 1 + 채널 2여야 한다: %s", len(decoded), raw)
	}
	for _, obj := range decoded[1:] {
		typ, _ := obj["type"].(string)
		if typ == "subscribe" {
			t.Fatal(`type이 "subscribe"다 — 서버가 unknown type으로 거부한다`)
		}
		if typ != "trade:kr" && typ != "orderbook:us" {
			t.Fatalf("type=%q — 채널:시장이어야 한다", typ)
		}
		codes, _ := obj["codes"].([]any)
		for _, code := range codes {
			if s, _ := code.(string); len(s) == 0 || s == "trade:kr:005930" {
				t.Fatalf("codes에 %q — 접두사 없이 심볼만 넣어야 한다", s)
			}
		}
	}
}

// 같은 목록을 다시 넣으면 재선언하지 않는다 — 선언 한도가 5회/초다.
func TestStreamClient_subscribeIsIdempotent(t *testing.T) {
	c := NewStreamClient(nil, nil, "probe")
	topics := []StreamTopic{{Channel: ChannelTrade, Market: "kr", Symbol: "005930"}}

	c.Subscribe(topics)
	c.mu.Lock()
	c.dirty = false // 선언이 나간 것으로 친다
	c.mu.Unlock()

	c.Subscribe(topics)
	c.mu.Lock()
	dirty := c.dirty
	c.mu.Unlock()
	if dirty {
		t.Fatal("같은 목록인데 재선언 예약이 걸렸다 — 5회/초 한도를 낭비한다")
	}
}

func TestParseStreamTime(t *testing.T) {
	got := parseStreamTime("2026-08-24T12:09:08.000+09:00")
	if got.IsZero() {
		t.Fatal("스펙 형식(오프셋 포함 ISO 8601)을 못 읽었다")
	}
	if got.UTC().Hour() != 3 {
		t.Fatalf("시각=%v — +09:00 오프셋이 반영돼야 한다", got.UTC())
	}
	if !parseStreamTime("").IsZero() {
		t.Fatal("빈 문자열은 zero time이어야 한다")
	}
}

// 프레임 디코딩 — 실측한 실제 응답 형태를 그대로 넣는다.
func TestStreamClient_handleTradeFrame(t *testing.T) {
	got := make(chan TradeFrame, 1)
	c := NewStreamClient(nil, tradeSink{got}, "probe")

	c.handleFrame([]byte(`{"type":"message","topic":"trade:us:NVDA","data":{"price":"216.38","volume":"5","timestamp":"2026-08-24T12:10:34.000+09:00","currency":"USD"}}`))

	select {
	case f := <-got:
		if f.Price != "216.38" || f.Volume != "5" {
			t.Fatalf("frame=%+v", f)
		}
		if f.Topic.Symbol != "NVDA" || f.Currency != CurrencyUSD {
			t.Fatalf("frame=%+v", f)
		}
		if f.Timestamp.IsZero() {
			t.Fatal("timestamp를 못 읽었다")
		}
	case <-time.After(time.Second):
		t.Fatal("체결 프레임이 핸들러에 안 왔다")
	}
}

// 구독 확인·에러 프레임은 데이터로 오인하면 안 된다.
func TestStreamClient_ignoresNonMessageFrames(t *testing.T) {
	got := make(chan TradeFrame, 1)
	c := NewStreamClient(nil, tradeSink{got}, "probe")

	c.handleFrame([]byte(`{"type":"subscriptions","id":"probe","subscribed":["trade:us:NVDA"],"rejected":[]}`))
	c.handleFrame([]byte(`{"type":"error","error":{"code":"wrong-format","message":"nope"}}`))
	c.handleFrame([]byte(`{"type":"pong"}`))
	c.handleFrame([]byte(`not json`))

	select {
	case f := <-got:
		t.Fatalf("데이터가 아닌 프레임이 체결로 전달됐다: %+v", f)
	default:
	}
}

type tradeSink struct{ ch chan TradeFrame }

func (s tradeSink) OnTrade(f TradeFrame)       { s.ch <- f }
func (s tradeSink) OnOrderbook(OrderbookFrame) {}

func (s tradeSink) OnSubscriptions(string, []StreamTopic, []string) {}
func (s tradeSink) OnDisconnect(string)                             {}
