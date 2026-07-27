package marketdata

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
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
	EmbedPolicyOK       EmbedPolicy = "ok"
	EmbedPolicyBlocked  EmbedPolicy = "blocked"
	EmbedPolicyUnknown  EmbedPolicy = "unknown"
)

var metaCSPRe = regexp.MustCompile(`(?i)content=["']?([^"'>]*frame-ancestors[^"'>]*)["']?`)

// ResolveEmbedPolicy checks HTTP (and lightweight HTML meta) signals for iframe embedding.
func ResolveEmbedPolicy(ctx context.Context, client *http.Client, redis *redis.Client, rawURL string) EmbedPolicy {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || rawURL == "#" {
		return EmbedPolicyUnknown
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return EmbedPolicyUnknown
	}
	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")

	if redis != nil {
		if cached, err := redis.Get(ctx, embedPolicyCachePrefix+host).Result(); err == nil && cached != "" {
			return EmbedPolicy(cached)
		}
	}

	policy := probeEmbedPolicy(ctx, client, rawURL)
	if redis != nil {
		_ = redis.Set(ctx, embedPolicyCachePrefix+host, string(policy), 7*24*time.Hour).Err()
	}
	return policy
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

func EmbedPolicyLabel(policy EmbedPolicy, publisherName string) string {
	switch policy {
	case EmbedPolicyOK:
		return "사이트 내 미리보기 가능"
	case EmbedPolicyBlocked:
		if publisherName != "" {
			return fmt.Sprintf("%s는 사이트 내 미리보기 제한", publisherName)
		}
		return "사이트 내 미리보기 제한"
	default:
		return "미리보기 미확인"
	}
}
