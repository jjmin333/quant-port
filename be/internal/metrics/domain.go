package metrics

import (
	"sort"
	"sync"
	"sync/atomic"
)

/*
도메인 지표 — 이번 성능 작업에서 **몰라서 헤맸던 것**만 넣는다.

metrics.go의 선정 기준("실제로 몰라서 막혔던 것")을 그대로 따른다. 각 항목이
어떤 질문에 답하는지 적어 두지 않으면 다음 사람이 지울지 남길지 판단할 수 없다.

	폴러 대상 수 — 「사용자가 없는데 왜 상류 호출이 나는가」. 로그로 한 번
	               답했지만(뉴스 브리핑이 범인이었다) 지표가 없으면 재발을
	               알아채지 못한다. terminal이 0이 아닌데 트래픽이 없으면 그것이
	               곧 경보다.
	캐시 히트율  — Redis 전체 히트율이 59%인데 **어느 접두사에서** 미스가 나는지
	               몰라 TTL을 감으로 올릴 뻔했다. 접두사별로 세야 근거가 생긴다.
	영구 키 비중 — TTL 없는 키(md:rev:*)가 심볼 수만큼 무한히 자란다. maxmemory에
	               닿으면 volatile-lru가 **TTL 있는 캐시만** 버리므로, 영구 키
	               비중이 곧 실질 캐시 용량의 잠식률이다.
	SSE 스트림   — 재기동 시 전부 끊기고, 죽은 클라이언트가 TCP 타임아웃까지
	               goroutine과 소켓을 잡는다. 2 OCPU 박스에서 상한을 정하려면
	               현재 수를 알아야 한다.
	차트 폴백    — 「트래픽이 늘어도 차트는 뜬다」가 계약이다. 계약이 지켜지는지는
	               nodata 폴백이 0인지로만 확인할 수 있다.

카디널리티는 의도적으로 낮게 잡았다 — 접두사와 사유는 코드가 정하는 유한 집합이지
사용자 입력이 아니다.
*/

var (
	pollerPriced   atomic.Int64
	pollerTerminal atomic.Int64
	pollerTargets  atomic.Int64

	sseStreams atomic.Int64

	labeledMu sync.Mutex
	cacheHit  = map[string]*atomic.Int64{}
	cacheMiss = map[string]*atomic.Int64{}
	chartFall = map[string]*atomic.Int64{}

	redisKeysTotal     atomic.Int64
	redisKeysPersisted atomic.Int64

	// 배치 감시. watchdogAge는 -1로 시작한다 — 「한 번도 안 돎」이 「방금 돎」으로
	// 보이면 안 된다.
	batchUnhealthy   atomic.Int64
	batchWatchdogAge = func() *atomic.Int64 {
		var v atomic.Int64
		v.Store(-1)
		return &v
	}()
)

// SetPollerSizes records what the market poller targeted on its last tick.
func SetPollerSizes(priced, terminal, targets int) {
	pollerPriced.Store(int64(priced))
	pollerTerminal.Store(int64(terminal))
	pollerTargets.Store(int64(targets))
}

// SSEOpened / SSEClosed track live server-sent-event streams.
func SSEOpened() { sseStreams.Add(1) }
func SSEClosed() { sseStreams.Add(-1) }

// CacheHit / CacheMiss count Redis lookups by key prefix (e.g. "market:price").
func CacheHit(prefix string)  { bump(cacheHit, prefix) }
func CacheMiss(prefix string) { bump(cacheMiss, prefix) }

// ChartFallback counts chart requests that could not be served from the
// intended source. `reason` is one of: db, budget, ratelimit, timeout, nodata.
func ChartFallback(reason string) { bump(chartFall, reason) }

// SetRedisKeyCounts records the last observed key census.
func SetRedisKeyCounts(total, persisted int64) {
	redisKeysTotal.Store(total)
	redisKeysPersisted.Store(persisted)
}

/*
배치 감시 — 화면 밖으로 나가는 유일한 신호.

관리자가 콘솔을 열어야만 아는 상태는 아무도 안 볼 수 있다. `/metrics`는 이미 있는
엔드포인트라, 외부 모니터가 여기 두 값만 긁어도 「배치가 늦었다」와 「감시가 멈췄다」를
알 수 있다.

`watchdogAge`가 -1이면 워치독이 한 번도 안 돈 것이다 — 0(방금 돎)과 구분해야 한다.
*/
func SetBatchHealth(unhealthy int, watchdogAgeSeconds int64) {
	batchUnhealthy.Store(int64(unhealthy))
	batchWatchdogAge.Store(watchdogAgeSeconds)
}

func bump(m map[string]*atomic.Int64, key string) {
	if key == "" {
		return
	}
	labeledMu.Lock()
	c, ok := m[key]
	if !ok {
		c = &atomic.Int64{}
		m[key] = c
	}
	labeledMu.Unlock()
	c.Add(1)
}

// snapshotLabeled copies a labeled counter map so the handler never holds the
// mutex while writing to the response.
func snapshotLabeled(m map[string]*atomic.Int64) map[string]int64 {
	labeledMu.Lock()
	out := make(map[string]int64, len(m))
	for k, v := range m {
		out[k] = v.Load()
	}
	labeledMu.Unlock()
	return out
}

// sortedKeys keeps the exposition order stable — a diffable /metrics output is
// far easier to eyeball during a change than a randomly ordered one.
func sortedKeys(m map[string]int64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var (
	streamSubs   atomic.Int64
	streamFrames = map[string]*atomic.Int64{}
)

// SetStreamSubscriptions records how many websocket topics are declared.
func SetStreamSubscriptions(n int) { streamSubs.Store(int64(n)) }

// StreamFrame counts decoded websocket frames by channel.
func StreamFrame(channel string) { bump(streamFrames, channel) }

var (
	tossRESTCalls  = map[string]*atomic.Int64{}
	tossRESTByPath = map[string]*atomic.Int64{}
)

/*
TossRESTCall — Toss로 **실제로 나간** REST 요청을 rate-limit 그룹별로 센다.

「웹소켓으로 옮겼더니 REST가 얼마나 줄었나」를 숫자로 답하려면 이것이 필요하다.
`rate-limit exceeded` 로그는 **한도에 닿았을 때만** 찍히므로, 한도 아래에서
낭비되는 호출은 보이지 않는다 — 그 구간이 정확히 최적화 대상이다.

그룹은 `toss.rateLimitGroup`이 정하는 유한 집합(MARKET_DATA·STOCK·…)이라
카디널리티가 고정이다.
*/
func TossRESTCall(group string) { bump(tossRESTCalls, group) }

/*
TossRESTPath — 같은 호출을 **엔드포인트별로** 센다.

그룹만으로는 최적화 대상을 못 고른다. `MARKET_DATA`는 `/prices`·`/orderbook`·
`/trades`가 함께 쓰는 버킷이라, 어느 쪽이 예산을 먹는지는 경로로만 갈린다.
경로는 `toss.metricPath`가 심볼을 `{symbol}`로 접어 카디널리티를 고정한다.
*/
func TossRESTPath(path string) { bump(tossRESTByPath, path) }

var streamRejected atomic.Int64

/*
StreamSubscriptionRejected — 서버가 **거부한** 구독 선언 건수.

거부는 조용한 고장이다. 그 종목만 프레임이 안 오는데 연결은 멀쩡하므로 로그를
보지 않으면 「저 종목만 값이 안 바뀐다」로만 드러난다. 0이 아니면 그 자체가 경보다.
*/
func StreamSubscriptionRejected(n int) {
	if n > 0 {
		streamRejected.Add(int64(n))
	}
}

var tokenEvents = map[string]*atomic.Int64{}

/*
TossTokenEvent — OAuth 토큰 수명에서 일어난 일.

이 경로는 **하루 한 번**만 움직이므로 로그로는 사실상 안 보인다. 그런데 그 한 번이
어긋나면 모든 REST 호출이 같이 죽는다. 그래서 카운터로 남긴다.

	issued              발급 성공. 자격증명당 하루 1이 정상값이고, 그보다 크면
	                    어딘가에서 불필요한 재발급이 돌고 있다는 뜻이다
	issue_retry         일시적 실패 후 재시도
	issue_failed        재시도까지 실패
	fallback_unexpired  갱신에 실패해 **아직 만료 전인** 토큰으로 계속했다.
	                    0이 아니면 갱신 경로가 흔들리고 있다는 조기 경보다
	retry_401           만료·경합으로 401을 맞고 새 토큰으로 재시도했다
*/
func TossTokenEvent(kind string) { bump(tokenEvents, kind) }
