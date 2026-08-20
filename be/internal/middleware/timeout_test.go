package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeoutExemptPaths(t *testing.T) {
	exempt := []string{
		"/api/v1/quotes/005930/stream",
		"/api/v1/quotes/005930/quote-stream",
		"/api/v1/import/holdings/screenshots",
		"/api/v1/import/holdings/jobs/abc/commit",
		"/api/v1/media/images",
		"/api/v1/admin/score/recompute",
		"/api/v1/admin/batch/run",
	}
	for _, p := range exempt {
		if !timeoutExempt(p) {
			t.Errorf("%s should be exempt from the request deadline", p)
		}
	}

	limited := []string{
		"/api/v1/quotes/005930",
		"/api/v1/quotes/005930/terminal-snapshot",
		"/api/v1/news/us",
		"/api/v1/rankings",
		// 업로드의 GET 짝은 제한 대상이다 — 이미지를 읽는 것은 빨라야 한다.
		"/api/v1/media/abc123.png",
	}
	for _, p := range limited {
		if timeoutExempt(p) {
			t.Errorf("%s should carry the request deadline", p)
		}
	}
}

// 마감이 실제로 핸들러의 컨텍스트에 도달하는지 — 이게 안 되면 미들웨어가
// 아무 일도 하지 않는다.
func TestTimeoutSetsDeadlineOnContext(t *testing.T) {
	stop := make(chan struct{})
	defer close(stop)

	var hasDeadline bool
	h := RequestTimeout(50 * time.Millisecond)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, hasDeadline = r.Context().Deadline()
	}))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/005930", nil)
	h.ServeHTTP(httptest.NewRecorder(), r)

	if !hasDeadline {
		t.Fatal("handler context carries no deadline")
	}
}

func TestExemptPathHasNoDeadline(t *testing.T) {
	var hasDeadline bool
	h := RequestTimeout(50 * time.Millisecond)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, hasDeadline = r.Context().Deadline()
	}))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/005930/quote-stream", nil)
	h.ServeHTTP(httptest.NewRecorder(), r)

	if hasDeadline {
		t.Fatal("SSE route must not carry a deadline")
	}
}

// 멈춘 핸들러가 마감 시점에 풀려나는지. Redis가 죽었을 때 관측한 상황을
// 컨텍스트 대기로 흉내낸다.
func TestStalledHandlerIsReleasedByDeadline(t *testing.T) {
	released := make(chan struct{})
	h := RequestTimeout(80 * time.Millisecond)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			close(released)
		case <-time.After(5 * time.Second):
		}
	}))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/005930", nil)
	go h.ServeHTTP(httptest.NewRecorder(), r)

	select {
	case <-released:
	case <-time.After(2 * time.Second):
		t.Fatal("stalled handler was never released by the deadline")
	}
}

func TestZeroDurationIsPassthrough(t *testing.T) {
	var hasDeadline bool
	h := RequestTimeout(0)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, hasDeadline = r.Context().Deadline()
	}))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/005930", nil)
	h.ServeHTTP(httptest.NewRecorder(), r)
	if hasDeadline {
		t.Fatal("zero duration must disable the deadline entirely")
	}
}

// 이미 마감이 걸린 컨텍스트를 덮어써서 늘리지 않는지 — 상위에서 더 짧은
// 마감을 걸었다면 그쪽이 이겨야 한다.
func TestDoesNotExtendShorterOuterDeadline(t *testing.T) {
	var got time.Duration
	h := RequestTimeout(10 * time.Second)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if dl, ok := r.Context().Deadline(); ok {
			got = time.Until(dl)
		}
	}))

	outer, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/005930", nil).WithContext(outer)
	h.ServeHTTP(httptest.NewRecorder(), r)

	if got > time.Second {
		t.Fatalf("inner deadline %v overrode the shorter outer one", got)
	}
}
