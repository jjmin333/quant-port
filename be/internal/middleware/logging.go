package middleware

import (
	"log"
	"net/http"
	"strconv"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/enact/quant-port/be/internal/metrics"
)

// Logger writes one line per request:
//
//	<reqID> GET /api/v1/news/us 200 27.4kB 663.2ms
//
// 상태 코드와 응답 크기가 없으면 로그로 알 수 있는 것이 "얼마나 걸렸는가"뿐이다.
// 200과 500이 구분되지 않아 에러율을 셀 수 없고, 크기가 없어 gzip이 실제로
// 먹었는지도 알 수 없다. 요청 ID는 Recoverer가 남기는 패닉 로그와 같은 값이라
// 스택 트레이스를 어느 요청의 것인지 짚을 수 있다.
//
// 쿼리 문자열은 일부러 남기지 않는다 — 뉴스·종목 검색의 `q=`가 사용자가 친 값이라
// 그대로 로그 수집기에 쌓인다. 어떤 엔드포인트였는지는 경로만으로 충분하다.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// chi의 래퍼를 쓰는 이유: 직접 감싸면 SSE가 죽는다. 스트리밍 핸들러는
		// w.(http.Flusher)와 http.NewResponseController(w).SetWriteDeadline에
		// 의존하는데, 평범한 struct로 감싸면 Flusher 단언이 실패하고
		// ResponseController는 밑단의 SetWriteDeadline을 못 찾는다. 후자가
		// 조용히 실패하면 WriteTimeout 30초가 SSE 연결을 끊는다.
		// chi의 WrapResponseWriter는 Flush/Hijack/ReadFrom을 위임하고
		// Unwrap()을 구현해 두 경로를 모두 보존한다.
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

		metrics.RequestStarted()
		start := time.Now()
		next.ServeHTTP(ww, r)
		elapsed := time.Since(start)
		metrics.RequestFinished()
		metrics.ObserveRequest(ww.Status())

		if reqID := chimw.GetReqID(r.Context()); reqID != "" {
			log.Printf("%s %s %s %d %s %s",
				reqID, r.Method, r.URL.Path, ww.Status(), formatBytes(ww.BytesWritten()), elapsed)
			return
		}
		log.Printf("%s %s %d %s %s",
			r.Method, r.URL.Path, ww.Status(), formatBytes(ww.BytesWritten()), elapsed)
	})
}

// formatBytes keeps the size column narrow — raw byte counts on a 98kB news
// payload push the duration off to an unpredictable column.
func formatBytes(n int) string {
	switch {
	case n < 1024:
		return strconv.Itoa(n) + "B"
	case n < 1024*1024:
		return strconv.FormatFloat(float64(n)/1024, 'f', 1, 64) + "kB"
	default:
		return strconv.FormatFloat(float64(n)/(1024*1024), 'f', 1, 64) + "MB"
	}
}
