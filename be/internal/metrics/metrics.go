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
