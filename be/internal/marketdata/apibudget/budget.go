package apibudget

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "apibudget:v1:"

var kst = time.FixedZone("KST", 9*3600)

type memState struct {
	mu       sync.RWMutex
	counters map[string]int64
	lastCall map[Provider]time.Time
	blocked  map[Provider]time.Time
}

// Manager tracks external API usage in Redis with in-memory fallback when Redis is unavailable.
type Manager struct {
	redis  *redis.Client
	limits map[Provider]Limits
	mem    memState
}

func NewManager(redisClient *redis.Client) *Manager {
	limits := make(map[Provider]Limits, len(DefaultLimits))
	for p, l := range DefaultLimits {
		limits[p] = l
	}
	return &Manager{
		redis: redisClient,
		limits: limits,
		mem: memState{
			counters: make(map[string]int64),
			lastCall: make(map[Provider]time.Time),
			blocked:  make(map[Provider]time.Time),
		},
	}
}

type Usage struct {
	Provider     Provider  `json:"provider"`
	MonthCount   int64     `json:"monthCount"`
	DayCount     int64     `json:"dayCount"`
	MinuteCount  int64     `json:"minuteCount"`
	LastCallAt   time.Time `json:"lastCallAt,omitempty"`
	BlockedUntil time.Time `json:"blockedUntil,omitempty"`
	Limits       Limits    `json:"limits"`
}

// Check reports whether a live API call is allowed. Cache reads do not need Check.
func (m *Manager) Check(ctx context.Context, provider Provider) error {
	if m == nil {
		return nil
	}
	lim, ok := m.limits[provider]
	if !ok {
		return nil
	}

	if until, ok := m.blockedUntil(ctx, provider); ok && time.Now().Before(until) {
		return fmt.Errorf("%s blocked until %s", provider, until.Format(time.RFC3339))
	}

	if lim.MinInterval > 0 {
		if last, ok := m.lastCall(ctx, provider); ok && time.Since(last) < lim.MinInterval {
			return fmt.Errorf("%s min interval %s not elapsed", provider, lim.MinInterval)
		}
	}

	if lim.MonthlyQuota > 0 {
		if n, _ := m.counter(ctx, provider, "month"); n >= int64(lim.MonthlyQuota) {
			return fmt.Errorf("%s monthly quota exhausted (%d)", provider, lim.MonthlyQuota)
		}
	}
	if lim.DailyQuota > 0 {
		if n, _ := m.counter(ctx, provider, "day"); n >= int64(lim.DailyQuota) {
			return fmt.Errorf("%s daily quota exhausted (%d)", provider, lim.DailyQuota)
		}
	}
	if lim.PerMinute > 0 {
		if n, _ := m.counter(ctx, provider, "minute"); n >= int64(lim.PerMinute) {
			return fmt.Errorf("%s per-minute quota exhausted (%d)", provider, lim.PerMinute)
		}
	}
	return nil
}

func (m *Manager) Record(ctx context.Context, provider Provider) {
	if m == nil {
		return
	}
	now := time.Now()
	m.memIncr(provider, now)

	if m.redis != nil {
		pipe := m.redis.Pipeline()
		pipe.Incr(ctx, m.counterKey(provider, "month", now))
		pipe.Incr(ctx, m.counterKey(provider, "day", now))
		pipe.Incr(ctx, m.counterKey(provider, "minute", now))
		pipe.Set(ctx, m.lastKey(provider), now.UTC().Format(time.RFC3339), 45*24*time.Hour)
		pipe.Expire(ctx, m.counterKey(provider, "month", now), 45*24*time.Hour)
		pipe.Expire(ctx, m.counterKey(provider, "day", now), 48*time.Hour)
		pipe.Expire(ctx, m.counterKey(provider, "minute", now), 2*time.Minute)
		if _, err := pipe.Exec(ctx); err != nil {
			log.Printf("apibudget record %s: %v (using memory fallback)", provider, err)
		}
	}
}

func (m *Manager) Block(ctx context.Context, provider Provider, until time.Time) {
	if m == nil {
		return
	}
	m.memBlock(provider, until)
	if m.redis != nil {
		ttl := time.Until(until)
		if ttl < time.Minute {
			ttl = time.Minute
		}
		if err := m.redis.Set(ctx, m.blockKey(provider), until.UTC().Format(time.RFC3339), ttl).Err(); err != nil {
			log.Printf("apibudget block %s: %v (using memory fallback)", provider, err)
		}
	}
	log.Printf("apibudget: %s blocked until %s", provider, until.Format(time.RFC3339))
}

func (m *Manager) BlockFor(ctx context.Context, provider Provider, d time.Duration) {
	m.Block(ctx, provider, time.Now().Add(d))
}

func (m *Manager) Usage(ctx context.Context, provider Provider) Usage {
	out := Usage{Provider: provider, Limits: m.limits[provider]}
	if m == nil {
		return out
	}
	out.MonthCount, _ = m.counter(ctx, provider, "month")
	out.DayCount, _ = m.counter(ctx, provider, "day")
	out.MinuteCount, _ = m.counter(ctx, provider, "minute")
	if last, ok := m.lastCall(ctx, provider); ok {
		out.LastCallAt = last
	}
	if until, ok := m.blockedUntil(ctx, provider); ok {
		out.BlockedUntil = until
	}
	return out
}

func (m *Manager) AllUsage(ctx context.Context) []Usage {
	order := []Provider{
		ProviderGoldPrice, ProviderFRED, ProviderFinnhub, ProviderUpbit, ProviderDART, ProviderEdgar,
		ProviderNaver, ProviderKakao,
	}
	out := make([]Usage, 0, len(order))
	for _, p := range order {
		out = append(out, m.Usage(ctx, p))
	}
	return out
}

func (m *Manager) counter(ctx context.Context, provider Provider, window string) (int64, error) {
	key := m.counterKey(provider, window, time.Now())
	if m.redis != nil {
		n, err := m.redis.Get(ctx, key).Int64()
		if err == nil {
			return n, nil
		}
		if err != redis.Nil {
			return m.memCounter(key), nil
		}
	}
	return m.memCounter(key), nil
}

func (m *Manager) lastCall(ctx context.Context, provider Provider) (time.Time, bool) {
	if m.redis != nil {
		raw, err := m.redis.Get(ctx, m.lastKey(provider)).Result()
		if err == nil && raw != "" {
			if t, err := time.Parse(time.RFC3339, raw); err == nil {
				return t, true
			}
		}
	}
	return m.memLastCall(provider)
}

func (m *Manager) blockedUntil(ctx context.Context, provider Provider) (time.Time, bool) {
	if m.redis != nil {
		raw, err := m.redis.Get(ctx, m.blockKey(provider)).Result()
		if err == nil && raw != "" {
			if until, err := time.Parse(time.RFC3339, raw); err == nil {
				return until, true
			}
		}
	}
	return m.memBlockedUntil(provider)
}

func (m *Manager) memCounter(key string) int64 {
	m.mem.mu.RLock()
	defer m.mem.mu.RUnlock()
	return m.mem.counters[key]
}

func (m *Manager) memIncr(provider Provider, now time.Time) {
	m.mem.mu.Lock()
	defer m.mem.mu.Unlock()
	m.mem.counters[m.counterKey(provider, "month", now)]++
	m.mem.counters[m.counterKey(provider, "day", now)]++
	m.mem.counters[m.counterKey(provider, "minute", now)]++
	m.mem.lastCall[provider] = now
}

func (m *Manager) memLastCall(provider Provider) (time.Time, bool) {
	m.mem.mu.RLock()
	defer m.mem.mu.RUnlock()
	t, ok := m.mem.lastCall[provider]
	return t, ok && !t.IsZero()
}

func (m *Manager) memBlock(provider Provider, until time.Time) {
	m.mem.mu.Lock()
	defer m.mem.mu.Unlock()
	m.mem.blocked[provider] = until
}

func (m *Manager) memBlockedUntil(provider Provider) (time.Time, bool) {
	m.mem.mu.RLock()
	defer m.mem.mu.RUnlock()
	t, ok := m.mem.blocked[provider]
	return t, ok && !t.IsZero()
}

func (m *Manager) counterKey(provider Provider, window string, now time.Time) string {
	now = now.In(kst)
	switch window {
	case "month":
		return fmt.Sprintf("%s%s:count:month:%04d-%02d", keyPrefix, provider, now.Year(), int(now.Month()))
	case "day":
		return fmt.Sprintf("%s%s:count:day:%04d-%02d-%02d", keyPrefix, provider, now.Year(), int(now.Month()), now.Day())
	case "minute":
		return fmt.Sprintf("%s%s:count:min:%04d-%02d-%02dT%02d:%02d", keyPrefix, provider, now.Year(), int(now.Month()), now.Day(), now.Hour(), now.Minute())
	default:
		return fmt.Sprintf("%s%s:count:%s", keyPrefix, provider, window)
	}
}

func (m *Manager) lastKey(provider Provider) string {
	return fmt.Sprintf("%s%s:last", keyPrefix, provider)
}

func (m *Manager) blockKey(provider Provider) string {
	return fmt.Sprintf("%s%s:blocked_until", keyPrefix, provider)
}

// ParseRetryAfter maps HTTP Retry-After header values to duration.
func ParseRetryAfter(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Minute
	}
	if sec, err := strconv.Atoi(raw); err == nil && sec > 0 {
		return time.Duration(sec) * time.Second
	}
	if t, err := time.Parse(time.RFC1123, raw); err == nil {
		return time.Until(t)
	}
	return time.Minute
}

func MarshalUsage(items []Usage) ([]byte, error) {
	return json.Marshal(items)
}
