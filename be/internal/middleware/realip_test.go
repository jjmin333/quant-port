package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func reqWith(remote string, headers map[string]string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/quotes", nil)
	r.RemoteAddr = remote
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

// The bypass that motivated this file: a client hitting the app directly must
// not be able to name its own IP.
func TestUntrustedPeerCannotSpoofForwardedFor(t *testing.T) {
	r := reqWith("203.0.113.50:4444", map[string]string{
		"X-Forwarded-For": "10.9.9.9",
		"X-Real-IP":       "10.8.8.8",
	})
	if got := resolveClientIP(r, true); got != "203.0.113.50" {
		t.Fatalf("got %q, want the socket address 203.0.113.50", got)
	}
}

// A proxy that appends puts the address it observed on the right; the left is
// whatever the client claimed.
func TestTrustedProxyUsesRightmostHop(t *testing.T) {
	r := reqWith("10.0.0.5:5555", map[string]string{
		"X-Forwarded-For": "10.9.9.9, 203.0.113.77",
	})
	if got := resolveClientIP(r, true); got != "203.0.113.77" {
		t.Fatalf("got %q, want 203.0.113.77 (rightmost, proxy-observed)", got)
	}
}

func TestTrustedProxySkipsChainedProxies(t *testing.T) {
	// client, real client, proxy1, proxy2 — walk right past the private hops.
	r := reqWith("10.0.0.5:5555", map[string]string{
		"X-Forwarded-For": "1.1.1.1, 203.0.113.77, 10.0.0.9, 172.16.0.3",
	})
	if got := resolveClientIP(r, true); got != "203.0.113.77" {
		t.Fatalf("got %q, want 203.0.113.77", got)
	}
}

func TestTrustedProxyFallsBackToXRealIP(t *testing.T) {
	r := reqWith("127.0.0.1:5555", map[string]string{"X-Real-IP": "203.0.113.9"})
	if got := resolveClientIP(r, true); got != "203.0.113.9" {
		t.Fatalf("got %q, want 203.0.113.9", got)
	}
}

func TestAllPrivateChainKeepsLeftmost(t *testing.T) {
	// Internal-only traffic (container to container): no public hop exists, so
	// the leftmost valid entry is the most specific thing available.
	r := reqWith("10.0.0.5:5555", map[string]string{
		"X-Forwarded-For": "192.168.1.20, 10.0.0.9",
	})
	if got := resolveClientIP(r, true); got != "192.168.1.20" {
		t.Fatalf("got %q, want 192.168.1.20", got)
	}
}

func TestTrustDisabledIgnoresHeaders(t *testing.T) {
	r := reqWith("10.0.0.5:5555", map[string]string{"X-Forwarded-For": "203.0.113.77"})
	if got := resolveClientIP(r, false); got != "10.0.0.5" {
		t.Fatalf("got %q, want 10.0.0.5 when proxy headers are not trusted", got)
	}
}

func TestGarbageHeaderFallsBackToPeer(t *testing.T) {
	r := reqWith("10.0.0.5:5555", map[string]string{"X-Forwarded-For": "not-an-ip, also-bad"})
	if got := resolveClientIP(r, true); got != "10.0.0.5" {
		t.Fatalf("got %q, want the peer address", got)
	}
}

func TestIPv6PeerIsParsed(t *testing.T) {
	r := reqWith("[2001:db8::1]:5555", nil)
	if got := resolveClientIP(r, true); got != "2001:db8::1" {
		t.Fatalf("got %q, want 2001:db8::1", got)
	}
}

// End-to-end through the middleware: the spoofing client must land in one
// bucket, not 300 of them.
func TestSpoofedHeadersShareOneBucket(t *testing.T) {
	cfg := RateLimitConfig{Enabled: true, UserRate: 10, UserBurst: 20, IPRate: 30, IPBurst: 60}
	stop := make(chan struct{})
	defer close(stop)

	h := RealIP(true)(RateLimit(cfg, stop)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	blocked := 0
	for i := 0; i < 300; i++ {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/quotes", nil)
		r.RemoteAddr = "203.0.113.50:4444" // public peer → headers ignored
		r.Header.Set("X-Forwarded-For", "10.9.0."+string(rune('0'+i%10)))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code == http.StatusTooManyRequests {
			blocked++
		}
	}
	if blocked == 0 {
		t.Fatal("rotating X-Forwarded-For bypassed the limiter")
	}
}
