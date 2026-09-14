package toss

import (
	"errors"
	"sync"
	"time"
)

// defaultFailoverCool is used when a 429 carried no usable Reset / Retry-After.
// Matches the caller's tossRateLimitDefaultWait so the two layers agree.
const defaultFailoverCool = 2 * time.Second

// groupFailover tracks, per Toss rate-limit group, whether primary is cooling
// and secondary should take the next call. State is process-local: each API /
// worker process observes its own 429s. That is enough — a shared Redis flag
// would make every process dump onto secondary at once.
type groupFailover struct {
	mu        sync.Mutex
	coolUntil map[string]time.Time
	now       func() time.Time
}

func newGroupFailover() *groupFailover {
	return &groupFailover{
		coolUntil: map[string]time.Time{},
		now:       time.Now,
	}
}

func (f *groupFailover) currentTime() time.Time {
	if f == nil || f.now == nil {
		return time.Now()
	}
	return f.now()
}

func (f *groupFailover) useSecondary(group string) bool {
	if f == nil {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	until, ok := f.coolUntil[group]
	return ok && f.currentTime().Before(until)
}

func (f *groupFailover) markPrimaryCool(group string, d time.Duration) {
	if f == nil || group == "" {
		return
	}
	if d <= 0 {
		d = defaultFailoverCool
	}
	until := f.currentTime().Add(d)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.coolUntil == nil {
		f.coolUntil = map[string]time.Time{}
	}
	if prev, ok := f.coolUntil[group]; ok && prev.After(until) {
		return
	}
	f.coolUntil[group] = until
}

func coolDuration(rl RateLimitStatus, err error) time.Duration {
	var apiErr *Error
	if errors.As(err, &apiErr) && apiErr.RetryAfter > 0 {
		return apiErr.RetryAfter
	}
	if rl.RetryAfter > 0 {
		return time.Duration(rl.RetryAfter) * time.Second
	}
	if rl.ResetSec > 0 {
		return time.Duration(rl.ResetSec) * time.Second
	}
	if err != nil {
		return defaultFailoverCool
	}
	return time.Second
}
