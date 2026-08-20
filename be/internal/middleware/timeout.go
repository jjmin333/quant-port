package middleware

import (
	"net/http"
	"strings"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

/*
RequestTimeout — 요청 하나가 붙잡을 수 있는 시간의 상한.

**왜 필요한가.** `http.Server{WriteTimeout}`은 이름과 달리 이 일을 못 한다. 응답을
**쓰기 시작한 뒤**부터 재기 때문에, 핸들러가 아무것도 쓰지 않고 멈춰 있으면 영원히
붙잡힌다. 실측으로 확인했다 — Redis를 정지시킨 상태에서 `/quotes/{symbol}`과
`/quotes/{symbol}/terminal-snapshot`이 **120초를 넘겨도 응답하지 않았다**(curl의
--max-time에 걸려 끊긴 것이지 서버가 끊은 게 아니다). FE가 2초마다 폴링하는
경로라, Redis가 잠깐 흔들리면 연결과 goroutine이 그대로 쌓인다.

chi의 Timeout은 컨텍스트에 마감을 건다. go-redis와 database/sql이 모두 컨텍스트를
존중하므로, 위의 멈춤이 마감 시점에 에러로 바뀌어 핸들러가 정상적으로 빠져나온다.

**제외 경로가 있는 이유.** 마감을 무조건 걸면 정상 기능이 깨진다:

  - SSE(`/stream`, `/quote-stream`)는 응답이 끝나지 않는 것이 정상이다.
  - 스크린샷 임포트와 이미지 업로드는 본문 수신 자체가 오래 걸릴 수 있다
    (OCR은 비동기라 요청은 접수만 하지만, 업로드 대역폭은 사용자 회선에 달렸다).
  - `/admin/score/recompute`는 전 종목 점수를 **동기로** 재계산한다. 배치 트리거
    (`/admin/batch/run`)는 큐에 넣고 202로 곧장 반환하지만 이쪽은 아니다.
    관리자 화면 전체를 빼는 쪽이 하나씩 세는 것보다 안전하다.
*/
func RequestTimeout(d time.Duration) func(http.Handler) http.Handler {
	if d <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	timeout := chimw.Timeout(d)

	return func(next http.Handler) http.Handler {
		limited := timeout(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if timeoutExempt(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			limited.ServeHTTP(w, r)
		})
	}
}

// timeoutExempt reports paths that must not carry a request deadline.
func timeoutExempt(path string) bool {
	// SSE — 끝나지 않는 것이 정상이다.
	if strings.HasSuffix(path, "/stream") || strings.HasSuffix(path, "/quote-stream") {
		return true
	}
	// 업로드 — 본문 수신 시간이 사용자 회선에 달렸다.
	if strings.HasPrefix(path, "/api/v1/import/") || path == "/api/v1/media/images" {
		return true
	}
	// 관리자 — 동기로 오래 도는 작업이 섞여 있다(score/recompute).
	if strings.HasPrefix(path, "/api/v1/admin/") {
		return true
	}
	return false
}
