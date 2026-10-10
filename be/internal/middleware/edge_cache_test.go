package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
)

func runEdge(t *testing.T, method string, h http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	EdgeCache(30*time.Second, 5*time.Minute)(h).ServeHTTP(rec, httptest.NewRequest(method, "/x", nil))
	return rec
}

func TestEdgeCache_publicWhenIdentityNeverConsulted(t *testing.T) {
	rec := runEdge(t, http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})
	want := "public, max-age=0, s-maxage=30, stale-while-revalidate=300"
	if got := rec.Header().Get("Cache-Control"); got != want {
		t.Fatalf("Cache-Control = %q, want %q", got, want)
	}
}

// 비로그인이라 답이 「없음」이어도 물어본 순간 요청자에 따라 달라지는 경로다.
func TestEdgeCache_privateWhenIdentityConsulted_evenAnonymous(t *testing.T) {
	rec := runEdge(t, http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authctx.UserID(r.Context()); ok {
			t.Fatal("비로그인 요청인데 사용자가 있다")
		}
		_, _ = w.Write([]byte(`{}`))
	})
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

// 서비스가 고루틴으로 퍼져 거기서 물어도 잡힌다.
func TestEdgeCache_privateWhenConsultedFromGoroutine(t *testing.T) {
	rec := runEdge(t, http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = authctx.RequireUserID(r.Context()) }()
		wg.Wait()
		_, _ = w.Write([]byte(`{}`))
	})
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

// 핸들러가 정한 값은 존중하되, 신원을 봤으면 덮는다.
func TestEdgeCache_handlerHeaderKeptUnlessPersonalized(t *testing.T) {
	rec := runEdge(t, http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=30")
		_, _ = w.Write([]byte(`{}`))
	})
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=30" {
		t.Fatalf("핸들러 값이 바뀌었다: %q", got)
	}
	rec = runEdge(t, http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=30")
		authctx.UserID(r.Context())
		_, _ = w.Write([]byte(`{}`))
	})
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("개인화인데 공개로 나갔다: %q", got)
	}
}

func TestEdgeCache_errorsAreNotShared(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		rec := runEdge(t, http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		})
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("%d → Cache-Control %q", status, got)
		}
	}
}

func TestEdgeCache_nonGetUntouched(t *testing.T) {
	rec := runEdge(t, http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})
	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("POST에 캐시 헤더: %q", got)
	}
}
