package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
)

func testCfg() RateLimitConfig {
	return RateLimitConfig{Enabled: true, UserRate: 10, UserBurst: 20, IPRate: 30, IPBurst: 60}
}

func TestBucketAllowsBurstThenBlocks(t *testing.T) {
	l := newRateLimiter(testCfg())
	now := time.Now()

	// A fresh bucket starts full, so burst requests in the same instant pass.
	for i := 0; i < 20; i++ {
		if ok, _ := l.allow("u:1", 10, 20, now); !ok {
			t.Fatalf("request %d within burst was blocked", i+1)
		}
	}
	ok, retry := l.allow("u:1", 10, 20, now)
	if ok {
		t.Fatal("request past burst should be blocked")
	}
	if retry <= 0 {
		t.Fatalf("blocked request should report a positive wait, got %v", retry)
	}
}

func TestBucketRefillsOverTime(t *testing.T) {
	l := newRateLimiter(testCfg())
	now := time.Now()

	for i := 0; i < 20; i++ {
		l.allow("u:1", 10, 20, now)
	}
	if ok, _ := l.allow("u:1", 10, 20, now); ok {
		t.Fatal("bucket should be empty")
	}

	// 10 tokens/sec → half a second buys 5.
	later := now.Add(500 * time.Millisecond)
	for i := 0; i < 5; i++ {
		if ok, _ := l.allow("u:1", 10, 20, later); !ok {
			t.Fatalf("refilled token %d should be available", i+1)
		}
	}
	if ok, _ := l.allow("u:1", 10, 20, later); ok {
		t.Fatal("only 5 tokens should have refilled in 500ms")
	}
}

func TestBucketDoesNotExceedBurstAfterLongIdle(t *testing.T) {
	l := newRateLimiter(testCfg())
	now := time.Now()
	l.allow("u:1", 10, 20, now)

	// An hour idle must not bank 36,000 tokens.
	later := now.Add(time.Hour)
	for i := 0; i < 20; i++ {
		if ok, _ := l.allow("u:1", 10, 20, later); !ok {
			t.Fatalf("token %d within burst should be available", i+1)
		}
	}
	if ok, _ := l.allow("u:1", 10, 20, later); ok {
		t.Fatal("tokens accumulated beyond burst")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l := newRateLimiter(testCfg())
	now := time.Now()

	for i := 0; i < 20; i++ {
		l.allow("u:1", 10, 20, now)
	}
	if ok, _ := l.allow("u:1", 10, 20, now); ok {
		t.Fatal("u:1 should be exhausted")
	}
	if ok, _ := l.allow("u:2", 10, 20, now); !ok {
		t.Fatal("u:2 must not be affected by u:1")
	}
}

// A logged-in user must get their own bucket rather than sharing the IP one —
// this is the whole point of the mixed keying, so it gets a test.
func TestLimitKeyPrefersUserOverIP(t *testing.T) {
	cfg := testCfg()

	anon := httptest.NewRequest(http.MethodGet, "/api/v1/quotes", nil)
	anon.RemoteAddr = "203.0.113.9:5555"
	key, rate, burst := limitKeyFor(anon, cfg)
	if key != "ip:203.0.113.9" {
		t.Fatalf("anonymous key = %q, want ip:203.0.113.9", key)
	}
	if rate != cfg.IPRate || burst != cfg.IPBurst {
		t.Fatalf("anonymous limits = %v/%v, want %v/%v", rate, burst, cfg.IPRate, cfg.IPBurst)
	}

	authed := anon.WithContext(authctx.WithUserID(anon.Context(), 42))
	key, rate, burst = limitKeyFor(authed, cfg)
	if key != "u:42" {
		t.Fatalf("authenticated key = %q, want u:42", key)
	}
	if rate != cfg.UserRate || burst != cfg.UserBurst {
		t.Fatalf("authenticated limits = %v/%v, want %v/%v", rate, burst, cfg.UserRate, cfg.UserBurst)
	}
}

// Two users behind one NAT address must not consume each other's allowance.
func TestUsersBehindSameIPDoNotShareBucket(t *testing.T) {
	l := newRateLimiter(testCfg())
	cfg := testCfg()
	now := time.Now()

	base := httptest.NewRequest(http.MethodGet, "/api/v1/quotes", nil)
	base.RemoteAddr = "203.0.113.9:5555"

	spend := func(userID int64, n int) int {
		r := base.WithContext(authctx.WithUserID(base.Context(), userID))
		key, rate, burst := limitKeyFor(r, cfg)
		passed := 0
		for i := 0; i < n; i++ {
			if ok, _ := l.allow(key, rate, burst, now); ok {
				passed++
			}
		}
		return passed
	}

	if got := spend(1, 25); got != 20 {
		t.Fatalf("user 1 passed %d requests, want 20 (burst)", got)
	}
	if got := spend(2, 20); got != 20 {
		t.Fatalf("user 2 passed %d requests, want 20 — same IP must not matter", got)
	}
}

func TestDisabledConfigIsPassthrough(t *testing.T) {
	cfg := testCfg()
	cfg.Enabled = false
	stop := make(chan struct{})
	defer close(stop)

	var served int
	h := RateLimit(cfg, stop)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served++ }))

	for i := 0; i < 500; i++ {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/quotes", nil)
		r.RemoteAddr = "203.0.113.9:5555"
		h.ServeHTTP(httptest.NewRecorder(), r)
	}
	if served != 500 {
		t.Fatalf("disabled limiter served %d/500", served)
	}
}

func TestMiddlewareReturns429WithRetryAfter(t *testing.T) {
	cfg := testCfg()
	cfg.IPRate = 1
	cfg.IPBurst = 1
	stop := make(chan struct{})
	defer close(stop)

	h := RateLimit(cfg, stop)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	call := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/quotes", nil)
		r.RemoteAddr = "203.0.113.9:5555"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}

	if got := call().Code; got != http.StatusOK {
		t.Fatalf("first request = %d, want 200", got)
	}
	rec := call()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("429 must carry Retry-After")
	}
}

func TestSweepDropsIdleBuckets(t *testing.T) {
	l := newRateLimiter(testCfg())
	now := time.Now()
	l.allow("ip:1.1.1.1", 30, 60, now)
	l.allow("ip:2.2.2.2", 30, 60, now)

	l.sweep(now.Add(bucketIdleTTL / 2))
	if len(l.buckets) != 2 {
		t.Fatalf("buckets pruned too early: %d", len(l.buckets))
	}

	l.sweep(now.Add(bucketIdleTTL + time.Second))
	if len(l.buckets) != 0 {
		t.Fatalf("idle buckets survived sweep: %d", len(l.buckets))
	}
}
