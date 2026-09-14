package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/metrics"
	"github.com/enact/quant-port/be/internal/model"
)

/*
목록 화면의 가격 스트림 — 마켓 랭킹·AI 랭킹·보유·관심이 함께 쓴다.

# 왜 폴링을 이걸로 바꾸나 (2026-09-04 사용자 요청 1)

REST 폴링은 **뷰어 한 명이 늘 때마다 서버 작업이 한 벌 더** 생긴다. 값을 만드는
일은 이미 폴러가 2초마다 한 번 하고 있는데, 그 값을 꺼내 주는 왕복이 뷰어 수에
비례해 늘어난다. 실측한 요청당 비용(2026-09-04):

	/rankings/prices      100종목    4 SELECT ·  80ms
	/quotes/batch          20종목   20 SELECT ·  89ms   (종목당 1)
	/stocks/scores/top     20종목  100 SELECT · 194ms   (종목당 5, 기준가 해석)

마지막 줄이 AI 랭킹이다. 그 화면을 2초 폴링으로 「실시간처럼」 만들면 뷰어당
초당 50 SELECT다 — 운영 DB가 1 ECPU라 몇 명만 열어도 끝난다.

스트림은 그 곱셈을 없앤다. 서버는 2초에 한 번 가격을 만들고(그건 원래 하던 일이다),
구독자 수만큼 **이미 만들어 둔 값을 복사**한다. 뷰어가 100명이어도 DB 작업은 0이다.

# 왜 WebSocket이 아닌가 (2026-09-04 사용자 선택)

가격은 서버→클라이언트 단방향이라 WS의 양방향이 사는 자리가 없다. 반대로 SSE는
이 저장소에 이미 돌고 있다 — `/quotes/{symbol}/stream`·`quote-stream`, 허브,
하트비트, 오버플로 처리, 메트릭까지. nginx도 이미 통과시키고 있고(`app_routes.conf`의
`proxy_read_timeout 3600s`), HTTP/2라 브라우저의 오리진당 6연결 제한도 없다.

WS로 가면 nginx Upgrade 프록시·인증 핸드셰이크·재연결 백오프·ping/pong을 전부
새로 써야 하는데, 그렇게 얻는 것이 없다. EventSource는 재연결을 브라우저가 한다.

# 마감이면 연결하지 않는다 (2026-09-04 사용자 추가 요청)

프리·정규·애프터·데이마켓이 **모두** 닫힌 시각에는 가격이 바뀔 수 없다. 그때
연결을 쥐고 있으면 하트비트만 오가는 빈 파이프이고, 서버 쪽에서는 그 종목들이
「보고 있는 종목」으로 세어져 상류 구독까지 따라붙는다.

그래서 이 핸들러는 **열기 전에 시계를 보고**, 닫혀 있으면 `session` 이벤트 하나만
주고 끊는다. 그 이벤트에 다음 개장 시각(`nextSessionAt`)이 들어 있어 화면이 그때
타이머 하나로 다시 붙는다. 열려 있는 동안에도 주기적으로 시계를 다시 봐서, 스트림을
열어 둔 채 장이 닫히면 그 자리에서 같은 이벤트를 주고 끊는다.

「모른다」를 「닫혔다」로 굳히지 않는다 — 달력을 못 받아 세션이 빈 값이면 열어 둔다.
그 반대로 하면 달력 한 번 실패에 화면이 영영 멈춘다.
*/

// listStreamSymbolMax bounds one connection. 목록 화면이 100줄을 넘게 그리지
// 않으므로, 이 상한은 「잘못 만든 요청이 전 종목을 구독하는 것」만 막는다.
const listStreamSymbolMax = 200

/*
listStreamSessionRecheck — 열려 있는 동안 시계를 다시 보는 주기.

30초인 이유는 이 값이 **마감을 알아채기까지의 지연**이기 때문이다. 장이 닫히고
최대 30초 동안은 연결이 남지만, 그 사이에 흐르는 것은 하트비트뿐이라 비용이 없다.
더 자주 보면 달력 캐시 조회가 그만큼 늘고, 더 뜸하면 마감 후 빈 연결이 오래 남는다.
*/
const listStreamSessionRecheck = 30 * time.Second

// ListPriceStreamParams is one client's subscription.
type ListPriceStreamParams struct {
	Symbols []string
	/*
		AssetType — 이 목록이 어느 달력을 쓰는가("domestic" / "us" / "etf").

		목록은 시장 탭 하나를 그리므로 행들이 달력을 공유한다. 종목마다 세션을
		풀면 `EnsureCalendars`를 목록 길이만큼 다시 부르게 되고, 그건 목록 화면이
		피하려던 바로 그 종류의 비용이다.

		빈 값이면 국내로 본다 — `detectMarketSession`의 기본과 같다.
	*/
	AssetType string
}

/*
ParseListStreamSymbols — `?symbols=A,B,C`를 정규화한다.

대문자·중복 제거·상한. 빈 목록이면 구독할 것이 없다는 뜻이라 핸들러가 400을 준다
(빈 스트림을 열어 두면 클라이언트는 「연결됐다」고 믿고 폴링을 끈다).
*/
func ParseListStreamSymbols(raw string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, 16)
	for _, part := range strings.Split(raw, ",") {
		sym := strings.ToUpper(strings.TrimSpace(part))
		if sym == "" {
			continue
		}
		if _, dup := seen[sym]; dup {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
		if len(out) >= listStreamSymbolMax {
			break
		}
	}
	return out
}

/*
listStreamClock — 이 목록이 따르는 시계.

달력을 못 구했을 때(`Session == ""`)를 **열린 것으로** 다룬다. 위 머리글의
「모른다를 닫혔다로 굳히지 않는다」가 여기다.
*/
func (s *QuotesService) listStreamClock(ctx context.Context, assetType string) MarketClock {
	at := strings.ToLower(strings.TrimSpace(assetType))
	if at == "" {
		at = "domestic"
	}
	return s.MarketClockFor(ctx, at)
}

// ServeListPriceStream pushes list-row prices over SSE until the client leaves
// or the market closes.
func (s *QuotesService) ServeListPriceStream(
	w http.ResponseWriter,
	r *http.Request,
	params ListPriceStreamParams,
) {
	if len(params.Symbols) == 0 {
		http.Error(w, "symbols is required", http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	if s.listPriceHub == nil {
		http.Error(w, "stream unavailable", http.StatusServiceUnavailable)
		return
	}

	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// nginx가 SSE를 버퍼링하면 이벤트가 한참 뒤에 몰려 나간다. 이 헤더는 그 경로에서
	// 무시돼도 손해가 없고, 켜져 있으면 프록시가 바로 흘려보낸다.
	w.Header().Set("X-Accel-Buffering", "no")

	ctx := r.Context()

	/*
		구독을 **먼저** 건다.

		스냅샷을 보내고 나서 구독하면 그 사이에 발행된 갱신을 놓친다. 반대 순서면
		같은 값을 두 번 볼 수는 있어도 잃지는 않는다 — 목록 행은 마지막 값만
		그리므로 중복은 눈에 띄지 않는다.
	*/
	events, cancel := s.listPriceHub.SubscribeMany(params.Symbols)
	defer cancel()

	metrics.SSEOpened()
	defer metrics.SSEClosed()

	clock := s.listStreamClock(ctx, params.AssetType)
	if clock.Closed() {
		// 열자마자 닫는다. 화면은 이 한 이벤트로 다음 개장 시각을 안다.
		writeListSSE(w, flusher, "session", clock)
		return
	}

	s.writeListPriceSnapshot(ctx, w, flusher, params.Symbols)

	beat := time.NewTicker(sseHeartbeatInterval)
	defer beat.Stop()
	session := time.NewTicker(listStreamSessionRecheck)
	defer session.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-beat.C:
			if !writeSSEHeartbeat(w, flusher) {
				return
			}
		case <-session.C:
			if now := s.listStreamClock(ctx, params.AssetType); now.Closed() {
				writeListSSE(w, flusher, "session", now)
				return
			}
		case ev, open := <-events:
			if !open {
				return
			}
			if !writeListSSE(w, flusher, "price", ev) {
				return
			}
		}
	}
}

/*
writeListPriceSnapshot — 붙자마자 지금 값을 한 번 보낸다.

**이게 없으면 첫 화면이 비어 있다.** 스트림은 「다음에 바뀔 때」만 말하므로, 값이
한동안 안 움직이는 종목은 구독만 해서는 영영 안 온다. 목록이 REST로 받은 값을
이미 들고 있더라도, 그 요청과 구독 사이의 틈에 바뀐 값은 이 스냅샷이 메운다.

읽기는 `GetPrices` 한 번(Redis MGET)이다 — 종목마다 묻지 않는다.
*/
func (s *QuotesService) writeListPriceSnapshot(
	ctx context.Context,
	w http.ResponseWriter,
	flusher http.Flusher,
	symbols []string,
) {
	if s.marketStore == nil {
		return
	}
	prices := s.marketStore.GetPrices(ctx, symbols)
	if len(prices) == 0 {
		return
	}
	/*
		스냅샷의 시각은 **지금**이다.

		`GetPrices`가 돌려주는 것은 토스 응답 모양(`toss.PriceResponse`)이라 캐시에
		언제 써졌는지가 들어 있지 않다. 여기서 굳이 되짚지 않는 이유는 이 값의 쓰임이
		하나뿐이기 때문이다 — 화면이 「스트림이 준 값이 REST가 준 값보다 새것인가」를
		비교하는 데만 쓴다. 방금 읽은 캐시 값은 그 비교에서 언제나 새것이 맞다.
	*/
	now := time.Now().UTC().Format(time.RFC3339Nano)
	batch := make([]model.ListPriceEvent, 0, len(prices))
	for _, sym := range symbols {
		p, ok := prices[sym]
		if !ok {
			continue
		}
		v := parseDecimal(p.LastPrice)
		if v <= 0 {
			continue
		}
		batch = append(batch, model.ListPriceEvent{
			Type:      "list_price",
			Symbol:    sym,
			Price:     v,
			Currency:  string(modelCurrency(p.Currency)),
			UpdatedAt: now,
		})
	}
	if len(batch) == 0 {
		return
	}
	// 한 프레임에 묶어 보낸다 — 스무 줄이면 SSE 프레임 스무 개가 아니라 하나다.
	writeListSSE(w, flusher, "snapshot", batch)
}

// writeListSSE writes one named event and reports whether the client is still
// reachable.
func writeListSSE(w http.ResponseWriter, flusher http.Flusher, event string, payload any) bool {
	body, err := json.Marshal(payload)
	if err != nil {
		return true
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body); err != nil {
		return false
	}
	flusher.Flush()
	return true
}
