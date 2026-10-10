package apibudget

import (
	"context"
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

/*
일일 카운터의 **하루 경계**는 provider마다 다르다.

이 표는 우리가 센 값이 아니라 **상대가 세는 값**을 흉내 내야 뜻이 있다. 국내 API는
KST 자정에 한도가 풀리므로 KST로 세는 게 맞지만, LLM(Gemini 무료 티어)의 일일 한도는
**태평양 자정**에 풀린다. 둘을 같은 KST 키로 세면 창이 8~9시간 어긋나, 우리 헤드룸이
상대의 잔여와 다른 숫자를 가리킨다.

실측(2026-09-08)으로 어긋남의 방향이 시장마다 갈렸다:

	국내 배치 16:30 KST — 태평양 자정(16:00/17:00 KST) 직후라 상대는 거의 0인데
	                     우리는 그날 브리핑 31콜을 이미 쓴 것으로 본다(과소 예약, 안전)
	해외 배치 07:00 KST — 상대는 전날 16:00 KST부터 15시간치를 이미 세는데
	                     우리 카운터는 00:00 KST에 리셋돼 있다(과대 예약, 위험)

배치 상한을 250에서 350으로 올리면 이 오차가 실제 초과로 이어질 수 있어, LLM 버킷만
태평양 시각으로 센다.

컨테이너에 tzdata가 없을 때를 대비한 대체값은 PST(-8)다 — 서머타임이면 경계가 한 시간
빨라질 뿐이고, 그래도 KST로 세는 것보다 상대에 가깝다.
*/
var pacific = func() *time.Location {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return time.FixedZone("PST", -8*3600)
	}
	return loc
}()

// quotaZone은 그 provider의 한도가 풀리는 시간대를 돌려준다.
func quotaZone(provider Provider) *time.Location {
	if CatalogFor(provider).Category == CategoryLLM {
		return pacific
	}
	return kst
}

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
	// exhausted는 하루·한 달 한도가 비었을 때 부르는 훅이다. nil이면 아무 일도
	// 하지 않는다 — exhausted_hook.go 머리글 참조.
	exhausted ExhaustedHook
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
	// MonthTokens는 달 단위 허용량의 잔여. -1은 「그 한도가 없다」다(Calls와 같은 규약).
	MonthTokens int64 `json:"monthTokens"`
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
	out := Headroom{Calls: -1, DayTokens: -1, MinuteTokens: -1, MonthTokens: -1}
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
	if lim.TokensPerMonth > 0 {
		used, _ := m.counter(ctx, provider, "tokens_month")
		out.MonthTokens = remaining(lim.TokensPerMonth, used)
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
			return Exhaustedf("%s daily token budget would be exceeded (%d + ~%d > %d)", provider, used, estimate, lim.TokensPerDay)
		}
	}
	/*
		월 허용량을 남은 날로 나눈 「오늘치」. 고정 하루 한도와 **둘 다** 본다 —
		둘 중 먼저 걸리는 쪽이 실효 한도다(Limits.SpreadMonthlyTokens 머리글).
	*/
	if lim.SpreadMonthlyTokens && lim.TokensPerMonth > 0 {
		day, _ := m.counter(ctx, provider, "tokens_day")
		month, _ := m.counter(ctx, provider, "tokens_month")
		if share := dailyShare(lim.TokensPerMonth, month, day, time.Now().In(quotaZone(provider))); day+estimate > share {
			return Exhaustedf("%s daily share of the monthly budget would be exceeded (%d + ~%d > %d)", provider, day, estimate, share)
		}
	}
	if lim.TokensPerMinute > 0 {
		used, _ := m.counter(ctx, provider, "tokens_minute")
		if used+estimate > int64(lim.TokensPerMinute) {
			return Exhaustedf("%s per-minute token budget would be exceeded (%d + ~%d > %d)", provider, used, estimate, lim.TokensPerMinute)
		}
	}
	// 월 한도는 **하루·분 한도와 나란히** 본다. 셋 중 하나만 있는 상대가 흔하다
	// (Azure Translator F0은 월 200만 자뿐, 일일 한도가 없다).
	if lim.TokensPerMonth > 0 {
		used, _ := m.counter(ctx, provider, "tokens_month")
		if used+estimate > lim.TokensPerMonth {
			return Exhaustedf("%s monthly token budget would be exceeded (%d + ~%d > %d)", provider, used, estimate, lim.TokensPerMonth)
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
		return Exhaustedf("%s blocked until %s", provider, until.Format(time.RFC3339))
	}

	if lim.MinInterval > 0 {
		if last, ok := m.lastCall(ctx, provider); ok && time.Since(last) < lim.MinInterval {
			return Pacingf("%s min interval %s not elapsed", provider, lim.MinInterval)
		}
	}

	if lim.MonthlyQuota > 0 {
		if n, _ := m.counter(ctx, provider, "month"); n >= int64(lim.MonthlyQuota) {
			m.reportExhausted(provider, "month", n, int64(lim.MonthlyQuota))
			return Exhaustedf("%s monthly quota exhausted (%d)", provider, lim.MonthlyQuota)
		}
	}
	if lim.DailyQuota > 0 {
		if n, _ := m.counter(ctx, provider, "day"); n >= int64(lim.DailyQuota) {
			m.reportExhausted(provider, "day", n, int64(lim.DailyQuota))
			return Exhaustedf("%s daily quota exhausted (%d)", provider, lim.DailyQuota)
		}
	}
	if lim.PerMinute > 0 {
		if n, _ := m.counter(ctx, provider, "minute"); n >= int64(lim.PerMinute) {
			return Exhaustedf("%s per-minute quota exhausted (%d)", provider, lim.PerMinute)
		}
	}
	if lim.TokensPerMinute > 0 {
		// Post-hoc protection: blocks once this minute's already-recorded
		// token usage meets the limit. Callers that can estimate a prompt's
		// cost should use CheckTokens, which also rejects the request that
		// would cross the line rather than only the one after it.
		if n, _ := m.counter(ctx, provider, "tokens_minute"); n >= int64(lim.TokensPerMinute) {
			return Exhaustedf("%s tokens-per-minute quota exhausted (%d)", provider, lim.TokensPerMinute)
		}
	}
	if lim.TokensPerDay > 0 {
		if n, _ := m.counter(ctx, provider, "tokens_day"); n >= lim.TokensPerDay {
			return Exhaustedf("%s tokens-per-day quota exhausted (%d)", provider, lim.TokensPerDay)
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

/*
RecordN — 한 번에 n회 호출했다고 기록한다.

한 논리적 작업이 상류 요청 **여러 개**로 이뤄지는 경우가 있다. FRED 배치가 그렇다 —
`QuoteBatch`는 Check·Record 를 한 번만 하면서 안에서 시리즈마다 2회씩 HTTP를 낸다.
그러면 우리 카운터가 실제 사용량을 **N분의 1로** 말하고, 그 상태에서는 한도를 올릴
수도 내릴 수도 없다(quotaZone 머리글의 같은 규칙 — 「우리 헤드룸이 상대의 잔여와
다른 숫자를 가리키면 상한을 올릴 수 없다」).

`Record`를 n번 부르는 것과 같지만 Redis 왕복이 한 번이다.
*/
func (m *Manager) RecordN(ctx context.Context, provider Provider, n int) {
	if m == nil || n <= 0 {
		return
	}
	if n == 1 {
		m.Record(ctx, provider)
		return
	}
	now := time.Now()
	// 메모리 폴백도 n회만큼 올린다 — Record 의 memIncr 와 같은 자리를 쓴다.
	for i := 0; i < n; i++ {
		m.memIncr(provider, now)
	}
	if m.redis == nil {
		return
	}
	delta := int64(n)
	pipe := m.redis.Pipeline()
	pipe.IncrBy(ctx, m.counterKey(provider, "month", now), delta)
	pipe.IncrBy(ctx, m.counterKey(provider, "day", now), delta)
	pipe.IncrBy(ctx, m.counterKey(provider, "minute", now), delta)
	pipe.Set(ctx, m.lastKey(provider), now.UTC().Format(time.RFC3339), 45*24*time.Hour)
	pipe.Expire(ctx, m.counterKey(provider, "month", now), 45*24*time.Hour)
	pipe.Expire(ctx, m.counterKey(provider, "day", now), 48*time.Hour)
	pipe.Expire(ctx, m.counterKey(provider, "minute", now), 2*time.Minute)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("apibudget record %s x%d: %v (using memory fallback)", provider, n, err)
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
	monthKey := m.counterKey(provider, "tokens_month", now)
	m.memAddToKey(minuteKey, tokens)
	m.memAddToKey(dayKey, tokens)
	m.memAddToKey(monthKey, tokens)
	if m.redis != nil {
		pipe := m.redis.Pipeline()
		pipe.IncrBy(ctx, minuteKey, tokens)
		pipe.Expire(ctx, minuteKey, 2*time.Minute)
		pipe.IncrBy(ctx, dayKey, tokens)
		pipe.Expire(ctx, dayKey, 48*time.Hour)
		pipe.IncrBy(ctx, monthKey, tokens)
		// 달 경계를 넘어 한 달 더 살려 둔다 — 관리자 화면이 지난달 사용량을 읽는다.
		pipe.Expire(ctx, monthKey, 62*24*time.Hour)
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

/*
AllProviders — 관리자 화면과 운영 점검이 도는 순서.

**목록을 여기 한 곳에 둔다.** 예전에는 `AllUsage` 안에만 있었고, 그래서 새 제공자를
추가한 사람이 이 목록을 잊으면 그 제공자는 **관리자 콘솔에서 통째로 안 보였다** —
실측으로 `finnhub_backfill`과 `azure_translate` 둘이 그렇게 빠져 있었다. 한도를 지키는
장치가 화면에 안 보이면 그 한도는 관리되지 않는 것과 같다.

`catalog`를 순회하지 않는 이유는 **순서**다. map 순회는 매 요청 순서가 달라 화면이
흔들린다.
*/
func AllProviders() []Provider {
	return []Provider{
		ProviderGoldPrice, ProviderFRED, ProviderFinnhub, ProviderFinnhubMigrate, ProviderFinnhubNews,
		ProviderFinnhubNewsCompany, ProviderKRX, ProviderFinlife,
		ProviderMolitRTMS, ProviderRone, ProviderEcos, ProviderOnbid, ProviderMoisRegion, ProviderUpbitBoard,
		ProviderUpbit, ProviderDART, ProviderEdgar, ProviderEdgarBulk,
		ProviderFinnhubEarnings, ProviderFinnhubBackfill, ProviderFinnhubMetrics, ProviderFnGuide,
		ProviderNaverStock, ProviderApeWisdom, ProviderRedditRSS,
		ProviderNaver, ProviderKakao, ProviderAzureTranslate,
		ProviderOpenAI, ProviderLLMKR, ProviderLLMUS,
		ProviderLLMFeatEarnings, ProviderLLMFeatResearch, ProviderLLMFeatSocial, ProviderLLMSub, ProviderLLMFeatAsset,
	}
}

func (m *Manager) AllUsage(ctx context.Context) []Usage {
	order := AllProviders()
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
	now = now.In(quotaZone(provider))
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
	case "tokens_month":
		return fmt.Sprintf("%s%s:tokens:month:%04d-%02d", keyPrefix, provider, now.Year(), int(now.Month()))
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

/*
dailyShare — 월 허용량 중 **오늘 쓸 수 있는 몫**.

	오늘치 = (월 한도 − 어제까지 쓴 양) / 오늘 포함 남은 날 수

「어제까지」를 월 누계에서 오늘치를 빼서 구한다 — 어제까지만 따로 세는 카운터를
새로 두면 월·일 둘과 어긋날 자리가 하나 더 생긴다.

나누는 수는 **오늘을 포함한** 남은 날이다(말일이면 1). 오늘을 빼면 말일에 0으로
나눈다.

달이 이미 한도를 넘었으면 0을 준다 — 음수를 그대로 두면 「day + estimate > 음수」가
언제나 참이라 결과는 같지만, 0이 읽는 사람에게 정직하다.
*/
func dailyShare(monthLimit, monthUsed, dayUsed int64, now time.Time) int64 {
	if monthLimit <= 0 {
		return 0
	}
	before := monthUsed - dayUsed
	if before < 0 {
		// 월 카운터가 만료됐는데 일 카운터가 남은 경계에서 나온다(TTL이 다르다).
		before = 0
	}
	remaining := monthLimit - before
	if remaining <= 0 {
		return 0
	}
	daysInMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).
		AddDate(0, 1, -1).Day()
	daysLeft := int64(daysInMonth - now.Day() + 1)
	if daysLeft < 1 {
		daysLeft = 1
	}
	return remaining / daysLeft
}
