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

const errCodeInvalidRequest = "invalid-request"

/*
IsInvalidRequestError — 상류가 **요청 자체를 거절**했는가(400).

레이트리밋(429)이나 일시적 장애와 갈라야 하는 이유: 400은 기다린다고 낫지 않는다.
같은 요청을 다시 보내면 같은 400이 온다. `/prices`처럼 심볼을 묶어 보내는
엔드포인트에서는 **묶음 안의 심볼 하나**가 이 응답을 부르므로, 호출부는 재시도가
아니라 원인 심볼을 골라내는 쪽으로 가야 한다.
*/
func IsInvalidRequestError(err error) bool {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusBadRequest || apiErr.Code == errCodeInvalidRequest
	}
	return false
}

const errCodeRateLimitExceeded = "rate-limit-exceeded"

// errCodeStockNotFound — 토스가 그 종목을 아예 취급하지 않는다는 응답.
//
// **재시도로 나아지지 않는 종류의 404다.** 우리 카탈로그에는 있지만 상대에게는
// 없는 종목(상장폐지·우선주·일부 ETN 등)이 여기 걸린다.
const errCodeStockNotFound = "stock-not-found"

/*
IsStockNotFound — 「상대가 이 종목을 모른다」인가.

호출부가 이것을 429·일시 장애와 **구분해야** 한다. 구분하지 않으면 상대에게 없는
종목을 매일 다시 물어보게 되는데, 실측(2026-09-11) `symbol_sync_state`의
`kind='warnings'` 17,918행 중 **3,388행(19%)이 이 404**였고 전부 `error`로 기록돼
있었다 — `SelectTargets`의 30일 not_found 쿨다운이 `error`에는 걸리지 않으므로
그 3,388호출이 매일 새로 나갔다.
*/
func IsStockNotFound(err error) bool {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound || apiErr.Code == errCodeStockNotFound
	}
	return false
}

func IsRateLimitError(err error) bool {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusTooManyRequests || apiErr.Code == errCodeRateLimitExceeded
	}
	return false
}
