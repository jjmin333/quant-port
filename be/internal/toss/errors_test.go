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

func TestIsRateLimitError(t *testing.T) {
	err := newErrorFromAPI(http.StatusTooManyRequests, ErrorBody{Error: APIError{Code: "rate-limit-exceeded"}}, RateLimitStatus{})
	if !IsRateLimitError(err) {
		t.Fatal("429 rate-limit-exceeded should match")
	}
	other := newErrorFromAPI(http.StatusBadRequest, ErrorBody{Error: APIError{Code: "bad-request"}}, RateLimitStatus{})
	if IsRateLimitError(other) {
		t.Fatal("400 must not match")
	}
}
