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
		redis:  redisClient,
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
	MinuteTokens int64     `json:"minuteTokens,omitempty"`
	DayTokens    int64     `json:"dayTokens,omitempty"`
	LastCallAt   time.Time `json:"lastCallAt,omitempty"`
	BlockedUntil time.Time `json:"blockedUntil,omitempty"`
	Limits       Limits    `json:"limits"`
}

// Headroom is what a batch planner needs before it starts: how many more
// calls and tokens today's quota still allows. A negative value is clamped to
// zero, and an unset limit reports -1 ("unbounded") so callers can tell
// "no limit" apart from "nothing left".
type Headroom struct {
	Calls        int64 `json:"calls"`
	DayTokens    int64 `json:"dayTokens"`
	MinuteTokens int64 `json:"minuteTokens"`
}

// SetLimits overrides one provider's quotas at wiring time (see
// config.LLMConfig's env overrides). Must be called before the manager is
// shared with request handlers — it is not safe against concurrent Check.
func (m *Manager) SetLimits(provider Provider, lim Limits) {
	if m == nil {
		return
	}
	m.limits[provider] = lim
}

// LimitsFor reports the currently enforced quotas for provider.
func (m *Manager) LimitsFor(provider Provider) Limits {
	if m == nil {
		return Limits{}
	}
	return m.limits[provider]
}

// Headroom reports the remaining daily call and token allowance. Batch jobs
// call this once up front to size their work list instead of discovering the
// ceiling one rejected call at a time.
func (m *Manager) Headroom(ctx context.Context, provider Provider) Headroom {
	out := Headroom{Calls: -1, DayTokens: -1, MinuteTokens: -1}
	if m == nil {
		return out
	}
	lim, ok := m.limits[provider]
	if !ok {
		return out
	}
	if lim.DailyQuota > 0 {
		used, _ := m.counter(ctx, provider, "day")
		out.Calls = remaining(int64(lim.DailyQuota), used)
	}
	if lim.TokensPerDay > 0 {
		used, _ := m.counter(ctx, provider, "tokens_day")
		out.DayTokens = remaining(lim.TokensPerDay, used)
	}
	if lim.TokensPerMinute > 0 {
		used, _ := m.counter(ctx, provider, "tokens_minute")
		out.MinuteTokens = remaining(int64(lim.TokensPerMinute), used)
	}
	return out
}

func remaining(limit, used int64) int64 {
	if used >= limit {
		return 0
	}
	return limit - used
}

// CheckTokens is Check plus a pre-flight token test: it rejects a call whose
// *estimated* token cost would push this minute or this day over budget.
// Check alone can only react to tokens already spent, which lets a single
// large request overshoot; batch callers that know roughly what a prompt
// costs should use this instead.
func (m *Manager) CheckTokens(ctx context.Context, provider Provider, estimate int64) error {
	if m == nil {
		return nil
	}
	if err := m.Check(ctx, provider); err != nil {
		return err
	}
	if estimate <= 0 {
		return nil
	}
	lim, ok := m.limits[provider]
	if !ok {
		return nil
	}
	if lim.TokensPerDay > 0 {
		used, _ := m.counter(ctx, provider, "tokens_day")
		if used+estimate > lim.TokensPerDay {
			return fmt.Errorf("%s daily token budget would be exceeded (%d + ~%d > %d)", provider, used, estimate, lim.TokensPerDay)
		}
	}
	if lim.TokensPerMinute > 0 {
		used, _ := m.counter(ctx, provider, "tokens_minute")
		if used+estimate > int64(lim.TokensPerMinute) {
			return fmt.Errorf("%s per-minute token budget would be exceeded (%d + ~%d > %d)", provider, used, estimate, lim.TokensPerMinute)
		}
	}
	return nil
}

// MinInterval reports a provider's configured spacing, so a multi-call batch
// can *pace itself* instead of failing on the second call.
//
// Check는 "지금 부를 수 있나"만 답하므로, 여러 번 불러야 하는 배치는 첫 호출
// 뒤에 반드시 실패한다 — 국내 실적 백필이 15개월 중 14개월을 그렇게 잃었다.
// 기다리면 해결되는 제약이라면 기다리는 게 맞고, 그러려면 얼마나 기다려야
// 하는지를 알아야 한다.
func (m *Manager) MinInterval(provider Provider) time.Duration {
	if m == nil {
		return 0
	}
	lim, ok := m.limits[provider]
	if !ok {
		return 0
	}
	return lim.MinInterval
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
	if lim.TokensPerMinute > 0 {
		// Post-hoc protection: blocks once this minute's already-recorded
		// token usage meets the limit. Callers that can estimate a prompt's
		// cost should use CheckTokens, which also rejects the request that
		// would cross the line rather than only the one after it.
		if n, _ := m.counter(ctx, provider, "tokens_minute"); n >= int64(lim.TokensPerMinute) {
			return fmt.Errorf("%s tokens-per-minute quota exhausted (%d)", provider, lim.TokensPerMinute)
		}
	}
	if lim.TokensPerDay > 0 {
		if n, _ := m.counter(ctx, provider, "tokens_day"); n >= lim.TokensPerDay {
			return fmt.Errorf("%s tokens-per-day quota exhausted (%d)", provider, lim.TokensPerDay)
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

// RecordTokens adds to this minute's token usage for provider — call after
// a response reports actual usage (e.g. an OpenAI-compatible chat
// completion's `usage.total_tokens`), not as a pre-flight estimate.
func (m *Manager) RecordTokens(ctx context.Context, provider Provider, tokens int64) {
	if m == nil || tokens <= 0 {
		return
	}
	now := time.Now()
	minuteKey := m.counterKey(provider, "tokens_minute", now)
	dayKey := m.counterKey(provider, "tokens_day", now)
	m.memAddToKey(minuteKey, tokens)
	m.memAddToKey(dayKey, tokens)
	if m.redis != nil {
		pipe := m.redis.Pipeline()
		pipe.IncrBy(ctx, minuteKey, tokens)
		pipe.Expire(ctx, minuteKey, 2*time.Minute)
		pipe.IncrBy(ctx, dayKey, tokens)
		pipe.Expire(ctx, dayKey, 48*time.Hour)
		if _, err := pipe.Exec(ctx); err != nil {
			log.Printf("apibudget record tokens %s: %v (using memory fallback)", provider, err)
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
	out.MinuteTokens, _ = m.counter(ctx, provider, "tokens_minute")
	out.DayTokens, _ = m.counter(ctx, provider, "tokens_day")
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
		ProviderGoldPrice, ProviderFRED, ProviderFinnhub, ProviderFinnhubMigrate, ProviderFinnhubNews, ProviderKRX,
		ProviderUpbit, ProviderDART, ProviderEdgar, ProviderEdgarBulk, ProviderFinnhubEarnings, ProviderFnGuide,
		ProviderNaver, ProviderKakao,
		ProviderPapagoText, ProviderPapagoWeb, ProviderOpenAI,
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

func (m *Manager) memAddToKey(key string, delta int64) {
	m.mem.mu.Lock()
	defer m.mem.mu.Unlock()
	m.mem.counters[key] += delta
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
	case "tokens_minute":
		return fmt.Sprintf("%s%s:tokens:min:%04d-%02d-%02dT%02d:%02d", keyPrefix, provider, now.Year(), int(now.Month()), now.Day(), now.Hour(), now.Minute())
	case "tokens_day":
		return fmt.Sprintf("%s%s:tokens:day:%04d-%02d-%02d", keyPrefix, provider, now.Year(), int(now.Month()), now.Day())
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
