package toss

import (
	"net/http"
	"testing"
)

func TestParseRateLimitStatus_allHeadersPresent(t *testing.T) {
	h := http.Header{}
	h.Set("X-RateLimit-Limit", "10")
	h.Set("X-RateLimit-Remaining", "2")
	h.Set("X-RateLimit-Reset", "1")
	h.Set("Retry-After", "1")

	rl := parseRateLimitStatus(h)
	if !rl.Present {
		t.Fatal("Present=false, want true")
	}
	if rl.Limit != 10 || rl.Remaining != 2 || rl.ResetSec != 1 || rl.RetryAfter != 1 {
		t.Fatalf("got %+v", rl)
	}
}

func TestParseRateLimitStatus_absentHeaders(t *testing.T) {
	rl := parseRateLimitStatus(http.Header{})
	if rl.Present {
		t.Fatalf("Present=true for empty headers, want false: %+v", rl)
	}
}

func TestParseRateLimitStatus_nonNumericIgnored(t *testing.T) {
	h := http.Header{}
	h.Set("X-RateLimit-Limit", "not-a-number")
	rl := parseRateLimitStatus(h)
	if rl.Present {
		t.Fatalf("Present=true for unparseable header, want false: %+v", rl)
	}
}

// Header.Get canonicalizes casing, so a lowercase header from a
// case-sensitive upstream is still picked up.
func TestParseRateLimitStatus_headerNameCaseInsensitive(t *testing.T) {
	h := http.Header{}
	h.Set("x-ratelimit-remaining", "0")
	rl := parseRateLimitStatus(h)
	if !rl.Present || rl.Remaining != 0 {
		t.Fatalf("got %+v", rl)
	}
}
