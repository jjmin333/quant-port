// Package metrics exposes a minimal Prometheus text endpoint.
//
// **왜 client_golang을 쓰지 않았나.** 지금 필요한 것은 몇 개의 카운터와 이미
// 구조체로 들고 있는 풀 통계뿐이다. 그걸 위해 의존성 트리를 늘리는 대신 텍스트
// 포맷을 직접 찍는다 — 포맷이 단순하고(`name{label="v"} 숫자`), 나중에 지표가
// 늘어 라이브러리가 필요해지면 이 파일만 바꾸면 된다.
//
// 노출 지표를 고른 기준은 "이번 출시 점검에서 실제로 몰라서 막혔던 것":
//
//   - MySQL 풀 — `MYSQL_MAX_OPEN_CONNS`를 부하 테스트로 정해야 하는데 대기가
//     생기는지 볼 방법이 없었다. WaitCount/WaitDuration이 그 답이다.
//   - goroutine 수 — Redis 장애 때 요청이 쌓여도 알 길이 없었다.
//   - 상태 코드별 요청 수 — 에러율. 액세스 로그를 세는 것 말고는 방법이 없었다.
package metrics

import (
	"database/sql"
	"fmt"
	"net/http"
	"runtime"

	"github.com/enact/quant-port/be/internal/import/ocr"
	"strconv"
	"sync/atomic"
)

// 상태 코드는 클래스로만 센다. 경로별로 세면 카디널리티가 라우트 수만큼 늘고,
// 그건 이 규모에서 얻는 것보다 잃는 게 크다.
var (
	reqTotal    [6]atomic.Int64 // index = status/100 (0은 미분류)
	reqInFlight atomic.Int64
)

// ObserveRequest records one finished request.
func ObserveRequest(status int) {
	class := status / 100
	if class < 0 || class >= len(reqTotal) {
		class = 0
	}
	reqTotal[class].Add(1)
}

// RequestStarted / RequestFinished track concurrency.
func RequestStarted()  { reqInFlight.Add(1) }
func RequestFinished() { reqInFlight.Add(-1) }

// PoolStater is anything that can report Redis pool counters (go-redis's
// *redis.Client satisfies it). Kept as an interface so this package does not
// import the Redis driver.
type PoolStater interface {
	PoolStats() *RedisPoolStats
}

// RedisPoolStats mirrors the fields we care about from go-redis.
type RedisPoolStats struct {
	TotalConns uint32
	IdleConns  uint32
	StaleConns uint32
}

// Handler renders the metrics. db may be nil.
func Handler(db *sql.DB, redisStats func() RedisPoolStats) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		writeCounter(w, "quantport_http_requests_total",
			"Requests completed, by status class.",
			map[string]int64{
				"1xx":     reqTotal[1].Load(),
				"2xx":     reqTotal[2].Load(),
				"3xx":     reqTotal[3].Load(),
				"4xx":     reqTotal[4].Load(),
				"5xx":     reqTotal[5].Load(),
				"unknown": reqTotal[0].Load(),
			}, "class")

		writeGauge(w, "quantport_http_requests_in_flight",
			"Requests currently being served.", float64(reqInFlight.Load()))

		writeGauge(w, "quantport_goroutines",
			"Live goroutines.", float64(runtime.NumGoroutine()))

		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		writeGauge(w, "quantport_heap_bytes", "Heap in use.", float64(mem.HeapInuse))

		// GOMAXPROCS — 컨테이너 안에서 `nproc`는 **호스트**의 코어 수를 말한다
		// (실측: cgroup 상한 2인 컨테이너에서 24). Go 1.25는 cgroup CPU 상한에서
		// 이 값을 잡으므로, "이 프로세스가 실제로 쓸 수 있는 코어"는 여기서만 보인다.
		// OCR 동시성 상한이 이 값에서 파생되므로 둘을 함께 노출한다.
		writeGauge(w, "quantport_gomaxprocs",
			"Effective GOMAXPROCS (Go derives it from the cgroup CPU limit).",
			float64(runtime.GOMAXPROCS(0)))
		writeGauge(w, "quantport_ocr_max_concurrency",
			"Concurrent tesseract processes allowed. 0 = unlimited.",
			float64(ocr.MaxConcurrency()))

		if db != nil {
			s := db.Stats()
			// WaitCount/WaitDuration이 0이 아니면 풀이 부족하다는 뜻이다 —
			// MYSQL_MAX_OPEN_CONNS를 정하는 근거가 여기 있다.
			writeGauge(w, "quantport_mysql_open_connections", "Open connections.", float64(s.OpenConnections))
			writeGauge(w, "quantport_mysql_in_use", "Connections in use.", float64(s.InUse))
			writeGauge(w, "quantport_mysql_idle", "Idle connections.", float64(s.Idle))
			writeGauge(w, "quantport_mysql_max_open", "Configured ceiling.", float64(s.MaxOpenConnections))
			writeCounterSingle(w, "quantport_mysql_wait_total", "Times a caller waited for a connection.", float64(s.WaitCount))
			writeCounterSingle(w, "quantport_mysql_wait_seconds_total", "Total time spent waiting.", s.WaitDuration.Seconds())
		}

		if redisStats != nil {
			s := redisStats()
			writeGauge(w, "quantport_redis_total_conns", "Redis pool connections.", float64(s.TotalConns))
			writeGauge(w, "quantport_redis_idle_conns", "Redis pool idle connections.", float64(s.IdleConns))
		}

		writeDomain(w, db)
	}
}

// writeDomain renders the counters defined in domain.go. Kept separate so the
// infrastructure gauges above stay readable.
func writeDomain(w http.ResponseWriter, db *sql.DB) {
	writeGauge(w, "quantport_poller_priced_symbols",
		"Symbols the market poller refreshes prices for (cheap, batched).", float64(pollerPriced.Load()))
	writeGauge(w, "quantport_poller_terminal_symbols",
		"Symbols with an open detail screen (expensive: orderbook+trades per symbol). Should be 0 with no traffic.",
		float64(pollerTerminal.Load()))
	writeGauge(w, "quantport_poller_fast_targets",
		"Symbols the last fast tick actually refreshed after the cap.", float64(pollerTargets.Load()))

	writeGauge(w, "quantport_sse_streams",
		"Live server-sent-event streams.", float64(sseStreams.Load()))

	// 배치가 늦었는지, 그리고 그것을 판정하는 워치독이 살아 있는지. 후자가 없으면
	// 전자의 0은 「정상」이 아니라 「아무도 안 보고 있음」이다.
	writeGauge(w, "quantport_batch_unhealthy",
		"Batch jobs judged late, failed or stuck. Should stay 0.", float64(batchUnhealthy.Load()))
	writeGauge(w, "quantport_batch_watchdog_age_seconds",
		"Seconds since the batch watchdog last recorded a pass. -1 means it never has.",
		float64(batchWatchdogAge.Load()))

	writeLabeled(w, "quantport_cache_hit_total",
		"Redis cache hits by key prefix.", snapshotLabeled(cacheHit), "prefix")
	writeLabeled(w, "quantport_cache_miss_total",
		"Redis cache misses by key prefix.", snapshotLabeled(cacheMiss), "prefix")
	writeLabeled(w, "quantport_chart_fallback_total",
		"Chart requests served from a fallback source, by reason. nodata must stay 0.",
		snapshotLabeled(chartFall), "reason")

	// 영구 키 비중. maxmemory에 닿으면 volatile-lru가 TTL 있는 캐시만 버리므로,
	// 이 비율이 곧 실질 캐시 용량의 잠식률이다 (domain.go 헤더 참조).
	if total := redisKeysTotal.Load(); total > 0 {
		writeGauge(w, "quantport_redis_keys_total", "Keys seen in the last census.", float64(total))
		writeGauge(w, "quantport_redis_keys_persisted", "Keys without a TTL in the last census.",
			float64(redisKeysPersisted.Load()))
	}

	writeGauge(w, "quantport_stream_subscriptions",
		"Websocket topics currently declared (channel x symbol).", float64(streamSubs.Load()))
	writeLabeled(w, "quantport_stream_frames_total",
		"Decoded websocket frames by channel.", snapshotLabeled(streamFrames), "channel")
	writeCounterSingle(w, "quantport_stream_subscriptions_rejected_total",
		"Subscription declarations the server rejected. Non-zero means some symbol silently stopped updating.",
		float64(streamRejected.Load()))
	writeLabeled(w, "quantport_toss_token_events_total",
		"OAuth token lifecycle events. issued should be ~1/day per credential; fallback_unexpired and issue_failed must stay 0.",
		snapshotLabeled(tokenEvents), "kind")
	writeLabeled(w, "quantport_toss_rest_calls_total",
		"Outbound Toss REST requests by rate-limit group. Falls as screens move to the websocket.",
		snapshotLabeled(tossRESTCalls), "group")
	writeLabeled(w, "quantport_toss_rest_path_total",
		"Outbound Toss REST requests by endpoint. Picks out which call inside a shared bucket costs.",
		snapshotLabeled(tossRESTByPath), "path")

	writeInnoDBLocks(w, db)
}

/*
writeInnoDBLocks — 행 락 대기. 감사에서 누적 21,974건 · 최대 51.9초가 나왔는데
**개선 전후를 비교할 방법이 없었다**(카운터가 누적값이라 재기동 시 초기화된다).

`SHOW GLOBAL STATUS`는 가볍고 /metrics는 내부 네트워크에서만 닿으므로 스크레이프
때마다 읽어도 된다. 실패하면 조용히 건너뛴다 — 지표 하나 때문에 /metrics 전체가
5xx가 되면 안 된다.
*/
func writeInnoDBLocks(w http.ResponseWriter, db *sql.DB) {
	if db == nil {
		return
	}
	rows, err := db.Query(`SHOW GLOBAL STATUS WHERE Variable_name IN
		('Innodb_row_lock_waits','Innodb_row_lock_time','Innodb_row_lock_time_max','Innodb_row_lock_current_waits')`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return
		}
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			continue
		}
		switch name {
		case "Innodb_row_lock_waits":
			writeCounterSingle(w, "quantport_innodb_row_lock_waits_total", "Row lock waits since server start.", v)
		case "Innodb_row_lock_time":
			writeCounterSingle(w, "quantport_innodb_row_lock_seconds_total", "Total row lock wait time.", v/1000)
		case "Innodb_row_lock_time_max":
			writeGauge(w, "quantport_innodb_row_lock_seconds_max", "Longest single row lock wait.", v/1000)
		case "Innodb_row_lock_current_waits":
			writeGauge(w, "quantport_innodb_row_lock_current_waits", "Transactions waiting on a row lock right now.", v)
		}
	}
}

func writeLabeled(w http.ResponseWriter, name, help string, values map[string]int64, label string) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n", name, help, name)
	for _, k := range sortedKeys(values) {
		fmt.Fprintf(w, "%s{%s=%q} %d\n", name, label, k, values[k])
	}
}

func writeGauge(w http.ResponseWriter, name, help string, v float64) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %s\n", name, help, name, name, fmtFloat(v))
}

func writeCounterSingle(w http.ResponseWriter, name, help string, v float64) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %s\n", name, help, name, name, fmtFloat(v))
}

func writeCounter(w http.ResponseWriter, name, help string, values map[string]int64, label string) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n", name, help, name)
	for k, v := range values {
		fmt.Fprintf(w, "%s{%s=%q} %d\n", name, label, k, v)
	}
}

func fmtFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
