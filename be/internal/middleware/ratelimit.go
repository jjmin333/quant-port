package middleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
)

/*
레이트 리밋 — 로그인 사용자는 user_id, 비로그인은 IP로 센다.

**왜 하나의 IP 기준으로 하지 않는가.** 국내 이동통신은 CGNAT를 광범위하게 쓰고
사무실·학교도 NAT 뒤에 있다. IP 하나에 수백 명이 앉아 있을 수 있어서, 1인 기준으로
임계값을 잡으면 정상 사용자가 서로를 밀어낸다. 반대로 NAT를 감안해 넉넉하게 잡으면
한 명이 그 넉넉한 값을 혼자 다 쓸 수 있어 방어가 안 된다.

로그인 사용자는 개인이 특정되므로 촘촘하게(기본 10 req/s) 잡아도 안전하고, 비로그인은
IP를 공유할 수 있으니 느슨하게(기본 30 req/s) 잡되 계정 없이 할 수 있는 일 자체가
읽기뿐이다.

**임계값 근거.** FE 폴링 실측 — 종목을 연 `/quotes` 화면이 상세 2초 + 캔들 2초 +
목록 5초 + 핀 5초로 약 1.4 req/s, 탭 3개면 약 4 req/s다. 기본값 10은 그 2.5배이고,
burst 20이 탭을 한꺼번에 여는 순간의 뭉침을 흡수한다.

**2026-09-04: 10 → 20 req/s, burst 20 → 40** (사용자 지적 11 — 「페이지를 이동하는
경우에 요청한 횟수가 너무 잦습니다가 뜨면서 아무 데이터가 안 뜬다」).

위 근거가 **정상 폴링만 세고 화면 전환을 안 셌다.** 화면 하나를 여는 순간은 폴링과
성격이 다르다 — 그 화면의 모든 위젯이 첫 데이터를 **동시에** 부르고, 앞 화면의
진행 중인 요청이 아직 살아 있다. burst 20은 그 겹침을 못 버텼다.

진짜 원인은 한도가 아니라 요청 수였고 그쪽을 먼저 고쳤다 — `useUpcomingEvents`가
종목당 1요청(최대 12)을 배치 1요청으로 바꿨다. 상향은 그 위에 얹는 **여유분**이고,
아직 못 찾은 다른 뭉침 지점이 사용자를 막지 않게 하는 것이 목적이다.

여전히 남용은 막는다. 20 req/s는 정상 사용(탭 3개 ≈ 4 req/s)의 다섯 배이고,
비로그인 IP 한도(30)를 넘지 않는다 — 로그인이 익명보다 느슨해지면 로그인해서
긁는 것이 이득이 된다.

**한계 (의도적).** 카운터가 프로세스 메모리에 있다. API를 2대로 늘리면 실효 한도가
2배가 된다. Redis로 옮기면 정확해지지만 요청마다 왕복이 붙는데, 지금은 단일
인스턴스라 값을 치르지 않는다. 다중 인스턴스로 갈 때 다시 볼 것.

**끌 수 있어야 한다.** 임계값을 잘못 잡으면 정상 사용자가 429를 맞는다. 그때
재배포를 기다리지 않도록 RATE_LIMIT_ENABLED=false로 즉시 끌 수 있다.
*/

// RateLimitConfig carries the tunables. A zero Rate disables that bucket kind.
type RateLimitConfig struct {
	Enabled   bool
	UserRate  float64 // tokens per second for an authenticated user
	UserBurst float64
	IPRate    float64 // tokens per second for an anonymous client IP
	IPBurst   float64
}

// DefaultRateLimitConfig — see the threshold reasoning above.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:   true,
		UserRate:  20,
		UserBurst: 40,
		IPRate:    30,
		IPBurst:   60,
	}
}

// bucketIdleTTL is how long an untouched bucket is kept. Without eviction the
// map grows once per distinct IP forever — a slow memory leak that a scanner
// could accelerate.
const bucketIdleTTL = 10 * time.Minute

type bucket struct {
	tokens float64
	last   time.Time
}

type rateLimiter struct {
	cfg RateLimitConfig

	mu      sync.Mutex
	buckets map[string]*bucket
}

func newRateLimiter(cfg RateLimitConfig) *rateLimiter {
	return &rateLimiter{cfg: cfg, buckets: make(map[string]*bucket)}
}

// allow spends one token for key. It returns false and the wait until the next
// token when the bucket is empty.
func (l *rateLimiter) allow(key string, rate, burst float64, now time.Time) (bool, time.Duration) {
	if rate <= 0 {
		return true, 0
	}
	if burst < 1 {
		burst = 1
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		// 새 클라이언트는 가득 찬 버킷으로 시작한다 — 첫 요청이 burst를 다 쓰는 게
		// 아니라, 화면 하나가 여는 여러 요청을 한 번에 통과시키기 위해서다.
		l.buckets[key] = &bucket{tokens: burst - 1, last: now}
		return true, 0
	}

	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * rate
		if b.tokens > burst {
			b.tokens = burst
		}
		b.last = now
	}

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}

	need := (1 - b.tokens) / rate
	return false, time.Duration(need * float64(time.Second))
}

// sweep drops buckets nobody has touched for bucketIdleTTL.
func (l *rateLimiter) sweep(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, b := range l.buckets {
		if now.Sub(b.last) > bucketIdleTTL {
			delete(l.buckets, k)
		}
	}
}

// RateLimit returns middleware that must be mounted **after** the auth
// middleware — it reads the user id the auth layer puts in the context, and
// without that every logged-in request would fall back to the shared IP bucket.
//
// stop cancels the background sweeper; pass the server's shutdown channel.
func RateLimit(cfg RateLimitConfig, stop <-chan struct{}) func(http.Handler) http.Handler {
	if !cfg.Enabled {
		return func(next http.Handler) http.Handler { return next }
	}

	l := newRateLimiter(cfg)

	go func() {
		t := time.NewTicker(bucketIdleTTL)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case now := <-t.C:
				l.sweep(now)
			}
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key, rate, burst := limitKeyFor(r, cfg)

			ok, retry := l.allow(key, rate, burst, time.Now())
			if !ok {
				// 최소 1초 — 0을 주면 클라이언트가 곧바로 다시 쳐서 같은 벽을 만난다.
				secs := int(retry.Seconds())
				if secs < 1 {
					secs = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(secs))
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func limitKeyFor(r *http.Request, cfg RateLimitConfig) (key string, rate, burst float64) {
	if userID, ok := authctx.UserID(r.Context()); ok {
		return "u:" + strconv.FormatInt(userID, 10), cfg.UserRate, cfg.UserBurst
	}
	return "ip:" + clientIP(r), cfg.IPRate, cfg.IPBurst
}

// clientIP prefers whatever chi's RealIP middleware resolved (it honours
// X-Forwarded-For / X-Real-IP), falling back to the socket address. Port is
// stripped so the same client does not get a fresh bucket per connection.
func clientIP(r *http.Request) string {
	addr := r.RemoteAddr
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
