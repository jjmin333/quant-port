package toss

import (
	"net/http"
	"testing"
	"time"
)

func TestNewErrorFromAPI_retryAfterOnRateLimit(t *testing.T) {
	err := newErrorFromAPI(http.StatusTooManyRequests, ErrorBody{}, RateLimitStatus{
		Present:    true,
		RetryAfter: 1,
		ResetSec:   3,
	})
	if err.RetryAfter != time.Second {
		t.Fatalf("RetryAfter=%v want 1s (prefers Retry-After over Reset)", err.RetryAfter)
	}
}

func TestNewErrorFromAPI_fallsBackToResetSecWhenNoRetryAfter(t *testing.T) {
	err := newErrorFromAPI(http.StatusTooManyRequests, ErrorBody{}, RateLimitStatus{
		Present:  true,
		ResetSec: 3,
	})
	if err.RetryAfter != 3*time.Second {
		t.Fatalf("RetryAfter=%v want 3s", err.RetryAfter)
	}
}

func TestNewErrorFromAPI_zeroWhenHeadersAbsent(t *testing.T) {
	err := newErrorFromAPI(http.StatusTooManyRequests, ErrorBody{}, RateLimitStatus{})
	if err.RetryAfter != 0 {
		t.Fatalf("RetryAfter=%v want 0 (caller falls back to its own default)", err.RetryAfter)
	}
}

func TestNewErrorFromAPI_ignoresRateLimitHeadersOnNonRateLimitStatus(t *testing.T) {
	err := newErrorFromAPI(http.StatusBadRequest, ErrorBody{}, RateLimitStatus{
		Present:    true,
		RetryAfter: 5,
	})
	if err.RetryAfter != 0 {
		t.Fatalf("RetryAfter=%v want 0 on a non-429 status", err.RetryAfter)
	}
}
