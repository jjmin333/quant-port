package toss

import (
	"log"
	"net/http"
	"strconv"
	"strings"
)

// RateLimitStatus is the budget Toss reports for the rate-limit bucket a
// request drew from (orderbook/prices/trades/price-limits all share the
// MARKET_DATA bucket) — the API never publishes a fixed number for any
// group, so response headers are the only way to see it.
type RateLimitStatus struct {
	Limit      int
	Remaining  int
	ResetSec   int
	RetryAfter int
	Present    bool
}

func parseRateLimitStatus(h http.Header) RateLimitStatus {
	limit, hasLimit := parseIntHeader(h, "X-RateLimit-Limit")
	remaining, hasRemaining := parseIntHeader(h, "X-RateLimit-Remaining")
	reset, _ := parseIntHeader(h, "X-RateLimit-Reset")
	retryAfter, _ := parseIntHeader(h, "Retry-After")
	return RateLimitStatus{
		Limit:      limit,
		Remaining:  remaining,
		ResetSec:   reset,
		RetryAfter: retryAfter,
		Present:    hasLimit || hasRemaining,
	}
}

func parseIntHeader(h http.Header, key string) (int, bool) {
	v := strings.TrimSpace(h.Get(key))
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return n, true
}

func (rl RateLimitStatus) isLow() bool {
	if !rl.Present {
		return false
	}
	if rl.Remaining == 0 {
		return true
	}
	return rl.Limit > 0 && rl.Remaining*5 <= rl.Limit
}

// logRateLimitStatus surfaces bucket pressure before it turns into a 429: a
// low-remaining warning on successful calls, or the full budget (including
// the server's own Retry-After) on an actual rate-limit response. key is
// "primary" or "secondary" so logs show which credential paid for the call.
func logRateLimitStatus(method, path, key string, status int, rl RateLimitStatus) {
	if !rl.Present {
		return
	}
	if status == http.StatusTooManyRequests {
		log.Printf(
			"toss api rate-limit exceeded: %s %s key=%s limit=%d remaining=%d resetSec=%d retryAfterSec=%d",
			method, path, key, rl.Limit, rl.Remaining, rl.ResetSec, rl.RetryAfter,
		)
		return
	}
	if rl.isLow() {
		log.Printf(
			"toss api rate-limit low: %s %s key=%s limit=%d remaining=%d resetSec=%d",
			method, path, key, rl.Limit, rl.Remaining, rl.ResetSec,
		)
	}
}
