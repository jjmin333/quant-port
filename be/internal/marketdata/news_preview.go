package marketdata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const ogImageCachePrefix = "news:og:v2:"

var (
	ogImageMetaRe  = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:image(?::url)?["'][^>]+content=["']([^"']+)["']`)
	ogImageMetaRe2 = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+property=["']og:image(?::url)?["']`)
)

// FetchOGImage loads og:image from an article URL (2–3s timeout, Redis cache by article URL).
func FetchOGImage(ctx context.Context, client *http.Client, redis *redis.Client, rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return ""
	}
	cacheKey := ogImageCacheKey(rawURL)

	if redis != nil {
		if cached, err := redis.Get(ctx, cacheKey).Result(); err == nil {
			return cached
		}
	}

	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}

	imageURL := probeOGImage(ctx, client, rawURL)
	if redis != nil {
		ttl := 7 * 24 * time.Hour
		if imageURL == "" {
			ttl = 6 * time.Hour
		}
		_ = redis.Set(ctx, cacheKey, imageURL, ttl).Err()
	}
	return imageURL
}

func ogImageCacheKey(rawURL string) string {
	normalized := strings.ToLower(strings.TrimSpace(rawURL))
	sum := sha256.Sum256([]byte(normalized))
	return ogImageCachePrefix + hex.EncodeToString(sum[:])
}

func probeOGImage(ctx context.Context, client *http.Client, rawURL string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; quant-port-news/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	res, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return ""
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 256<<10))
	if err != nil {
		return ""
	}
	html := string(body)
	for _, re := range []*regexp.Regexp{ogImageMetaRe, ogImageMetaRe2} {
		if m := re.FindStringSubmatch(html); len(m) > 1 {
			return normalizeImageURL(strings.TrimSpace(m[1]), rawURL)
		}
	}
	return ""
}

func normalizeImageURL(imageURL, pageURL string) string {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return ""
	}
	if strings.HasPrefix(imageURL, "//") {
		return "https:" + imageURL
	}
	if strings.HasPrefix(imageURL, "http://") || strings.HasPrefix(imageURL, "https://") {
		return imageURL
	}
	base, err := url.Parse(pageURL)
	if err != nil {
		return imageURL
	}
	ref, err := url.Parse(imageURL)
	if err != nil {
		return imageURL
	}
	return base.ResolveReference(ref).String()
}

func normalizeNewsHost(host string) string {
	host = strings.ToLower(host)
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	return host
}
