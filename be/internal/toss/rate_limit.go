package toss

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
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
	logFirstSeenLimit(rateLimitGroup(path), key, rl.Limit)
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

/*
logFirstSeenLimit — 그룹마다 한도를 **처음 본 순간 한 번** 남긴다.

Toss는 어떤 그룹에도 한도 숫자를 공표하지 않는다(헤더에만 있다). 새 버킷을 쓰기 시작할
때(2026-09-24 STOCK_TRADING_TREND) 「그 버킷의 한도가 얼마인가」를 로그 한 줄로 알 수 있게
한다 — 같은 그룹에 처음 보는 한도 값이 오면 그때 한 번 더 찍힌다.

**값마다 한 번이다.** 한 그룹 안에서도 경로마다 한도가 다르다(MARKET_INDICATOR는 10과
5가 번갈아 온다). 「마지막 값과 다르면 찍기」였을 때는 그 둘이 오갈 때마다 찍혀 하루
400줄이 됐다(10-15).
*/
var seenLimits sync.Map // group|key|limit → struct{}

func logFirstSeenLimit(group, key string, limit int) {
	if limit <= 0 {
		return
	}
	if _, loaded := seenLimits.LoadOrStore(group+"|"+key+"|"+strconv.Itoa(limit), struct{}{}); loaded {
		return
	}
	log.Printf("toss api rate-limit observed: group=%s key=%s limit=%d", group, key, limit)
}
