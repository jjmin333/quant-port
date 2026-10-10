package marketdata

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const embedPolicyCachePrefix = "news:embed:v1:"

// EmbedPolicy describes whether an article URL can be shown inside an iframe.
// ok       — no blocking headers detected (iframe likely works)
// blocked  — X-Frame-Options / CSP frame-ancestors deny embedding
// unknown  — could not determine (network error, paywall, JS frame-buster possible)
type EmbedPolicy string

const (
	EmbedPolicyOK      EmbedPolicy = "ok"
	EmbedPolicyBlocked EmbedPolicy = "blocked"
	EmbedPolicyUnknown EmbedPolicy = "unknown"
)

var metaCSPRe = regexp.MustCompile(`(?i)content=["']?([^"'>]*frame-ancestors[^"'>]*)["']?`)

/*
프로세스 안 호스트 캐시 — 목록 응답이 Redis를 100번 두드리던 자리다.

이 함수는 뉴스 목록을 만들 때 **기사마다 한 번씩** 불린다. 값은 호스트 단위인데
호출은 기사 단위라, 기사 100건이 서른 남짓한 호스트를 놓고 Redis GET을 100번
순차로 날렸다 (실측: `/news/us?limit=100` 한 요청에 Redis 명령 342개, 응답 1.35초).
로컬에서 GET 하나가 4ms면 그것만으로 1.3초다 — VM에서 Redis가 다른 컨테이너에
있으면 더 벌어진다.

호스트 종류는 수십 개 수준이라 프로세스 메모리에 통째로 들어간다. Redis는 여전히
프로세스 사이에서 공유하는 7일짜리 저장소로 남고, 이 맵은 그 앞의 짧은 캐시다.
*/
const (
	embedPolicyMemoTTL = 10 * time.Minute
	// unknown은 대개 네트워크 실패라 짧게 잡는다. 알아낸 값과 같은 TTL을 주면
	// 한 번 실패한 호스트가 10분 동안 실패한 채로 굳는다.
	embedPolicyMemoUnknownTTL = 60 * time.Second
)

var embedPolicyMemo sync.Map // host -> embedPolicyMemoEntry

type embedPolicyMemoEntry struct {
	policy    EmbedPolicy
	expiresAt time.Time
}

func embedPolicyFromMemo(host string) (EmbedPolicy, bool) {
	v, ok := embedPolicyMemo.Load(host)
	if !ok {
		return "", false
	}
	entry, ok := v.(embedPolicyMemoEntry)
	if !ok || time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.policy, true
}

func rememberEmbedPolicy(host string, policy EmbedPolicy) {
	ttl := embedPolicyMemoTTL
	if policy == EmbedPolicyUnknown {
		ttl = embedPolicyMemoUnknownTTL
	}
	embedPolicyMemo.Store(host, embedPolicyMemoEntry{policy: policy, expiresAt: time.Now().Add(ttl)})
}

// embedPolicyProbing tracks hosts with a background probe already in flight, so
// a list of 100 articles from one new outlet starts one probe, not 100.
var embedPolicyProbing sync.Map // host -> struct{}

func embedPolicyHost(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	return strings.TrimPrefix(host, "m.")
}

// ResolveEmbedPolicy checks HTTP (and lightweight HTML meta) signals for iframe embedding.
//
// 조회 경로에서는 절대 외부 HTTP를 치지 않는다. 예전에는 캐시가 비면 이 자리에서
// 기사 원문을 최대 64KB까지 받아 왔는데(타임아웃 8초), 그 말은 어느 매체의 7일
// TTL이 만료되는 순간 그걸 처음 밟은 **사용자 요청이** 남의 사이트 응답을 기다렸다는
// 뜻이다. 지금은 조사(probe)를 배경으로 띄우고 이번 응답은 unknown으로 답한다 —
// 다음 요청부터 제대로 나온다.
func ResolveEmbedPolicy(ctx context.Context, client *http.Client, redis *redis.Client, rawURL string) EmbedPolicy {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || rawURL == "#" {
		return EmbedPolicyUnknown
	}

	host := embedPolicyHost(rawURL)
	if host == "" {
		return EmbedPolicyUnknown
	}

	if policy, ok := embedPolicyFromMemo(host); ok {
		return policy
	}

	if redis != nil {
		if cached, err := redis.Get(ctx, embedPolicyCachePrefix+host).Result(); err == nil && cached != "" {
			policy := EmbedPolicy(cached)
			rememberEmbedPolicy(host, policy)
			return policy
		}
	}

	startEmbedPolicyProbe(client, redis, host, rawURL)
	return EmbedPolicyUnknown
}

// startEmbedPolicyProbe fetches one host's policy off the request path and
// stores it in both caches. At most one probe per host is in flight.
func startEmbedPolicyProbe(client *http.Client, rdb *redis.Client, host, rawURL string) {
	if _, busy := embedPolicyProbing.LoadOrStore(host, struct{}{}); busy {
		return
	}
	go func() {
		defer embedPolicyProbing.Delete(host)
		// 요청 ctx를 물려받지 않는다. 물려받으면 사용자가 응답을 받는 순간 ctx가
		// 취소되어 조사가 매번 중도에 죽고, 그 호스트는 영영 unknown으로 남는다.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		policy := probeEmbedPolicy(ctx, client, rawURL)
		rememberEmbedPolicy(host, policy)
		if rdb != nil && policy != EmbedPolicyUnknown {
			_ = rdb.Set(ctx, embedPolicyCachePrefix+host, string(policy), 7*24*time.Hour).Err()
		}
	}()
}

func probeEmbedPolicy(ctx context.Context, client *http.Client, rawURL string) EmbedPolicy {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return EmbedPolicyUnknown
	}
	req.Header.Set("User-Agent", "quant-port-news/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	res, err := client.Do(req)
	if err != nil {
		return EmbedPolicyUnknown
	}
	defer res.Body.Close()

	if policy := policyFromHeaders(res.Header); policy != EmbedPolicyUnknown {
		return policy
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if err != nil {
		return EmbedPolicyUnknown
	}
	if policy := policyFromHTML(string(body)); policy != EmbedPolicyUnknown {
		return policy
	}

	// No explicit deny — most domestic outlets omit XFO (matches current UX).
	return EmbedPolicyOK
}

func policyFromHeaders(h http.Header) EmbedPolicy {
	xfo := strings.ToLower(strings.TrimSpace(h.Get("X-Frame-Options")))
	switch xfo {
	case "deny", "sameorigin":
		return EmbedPolicyBlocked
	}

	csp := strings.ToLower(h.Get("Content-Security-Policy"))
	if strings.Contains(csp, "frame-ancestors") {
		if strings.Contains(csp, "frame-ancestors 'none'") {
			return EmbedPolicyBlocked
		}
		if strings.Contains(csp, "frame-ancestors 'self'") {
			return EmbedPolicyBlocked
		}
	}
	return EmbedPolicyUnknown
}

func policyFromHTML(html string) EmbedPolicy {
	lower := strings.ToLower(html)
	if strings.Contains(lower, "frame-ancestors 'none'") || strings.Contains(lower, "frame-ancestors 'self'") {
		return EmbedPolicyBlocked
	}
	if m := metaCSPRe.FindStringSubmatch(html); len(m) > 1 {
		val := strings.ToLower(m[1])
		if strings.Contains(val, "frame-ancestors 'none'") || strings.Contains(val, "frame-ancestors 'self'") {
			return EmbedPolicyBlocked
		}
	}
	return EmbedPolicyUnknown
}
