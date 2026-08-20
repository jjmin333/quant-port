package toss

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

const ErrCodeInvalidToken = "invalid-token"

type Error struct {
	StatusCode int
	Code       string
	Message    string
	RequestID  string
	// RetryAfter is how long the server said to wait before retrying (from
	// Retry-After / X-RateLimit-Reset), set only for 429 responses. Zero
	// means the response carried no usable rate-limit headers — callers
	// should fall back to their own default.
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("toss api %d %s: %s (requestId=%s)", e.StatusCode, e.Code, e.Message, e.RequestID)
	}
	return fmt.Sprintf("toss api %d %s: %s", e.StatusCode, e.Code, e.Message)
}

func newErrorFromAPI(status int, body ErrorBody, rl RateLimitStatus) *Error {
	e := &Error{
		StatusCode: status,
		Code:       body.Error.Code,
		Message:    body.Error.Message,
		RequestID:  body.Error.RequestID,
	}
	if status == http.StatusTooManyRequests {
		switch {
		case rl.RetryAfter > 0:
			e.RetryAfter = time.Duration(rl.RetryAfter) * time.Second
		case rl.ResetSec > 0:
			e.RetryAfter = time.Duration(rl.ResetSec) * time.Second
		}
	}
	return e
}

func IsInvalidTokenError(err error) bool {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusUnauthorized || apiErr.Code == ErrCodeInvalidToken
	}
	return false
}
