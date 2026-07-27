package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
	"github.com/redis/go-redis/v9"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
)

const (
	usReaderCachePrefix = "news:reader:v1:"
	usReaderCacheTTL    = time.Hour
)

type NewsReaderService struct {
	httpClient *http.Client
	redis      *redis.Client
}

func NewNewsReaderService(redisClient *redis.Client) *NewsReaderService {
	return &NewsReaderService{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		redis:      redisClient,
	}
}

func (s *NewsReaderService) Extract(ctx context.Context, rawURL string) (*model.NewsReaderContent, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("url is required")
	}
	if err := validateReaderURL(rawURL); err != nil {
		return nil, err
	}

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, readerCacheKey(rawURL)).Bytes(); err == nil && len(cached) > 0 {
			var out model.NewsReaderContent
			if json.Unmarshal(cached, &out) == nil {
				return &out, nil
			}
		}
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	if isKoreanNewsURL(rawURL) {
		req.Header.Set("Accept-Language", "ko-KR,ko;q=0.9,en;q=0.8")
	} else {
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	}

	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch status %d", res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}

	article, err := readability.FromReader(strings.NewReader(string(body)), parsed)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}

	out := &model.NewsReaderContent{
		URL:         rawURL,
		Title:       strings.TrimSpace(article.Title),
		Byline:      strings.TrimSpace(article.Byline),
		Excerpt:     strings.TrimSpace(article.Excerpt),
		SiteName:    strings.TrimSpace(article.SiteName),
		TextContent: strings.TrimSpace(article.TextContent),
		ContentHTML: strings.TrimSpace(article.Content),
		FetchedAt:   time.Now(),
	}
	if article.PublishedTime != nil && !article.PublishedTime.IsZero() {
		out.PublishedAt = *article.PublishedTime
	}
	if out.Title == "" && out.TextContent == "" {
		return nil, fmt.Errorf("no readable content extracted")
	}

	if s.redis != nil {
		if payload, err := json.Marshal(out); err == nil {
			_ = s.redis.Set(ctx, readerCacheKey(rawURL), payload, usReaderCacheTTL).Err()
		}
	}
	return out, nil
}

func validateReaderURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("invalid url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported url scheme")
	}
	if !marketdata.ReaderAllowedHost(rawURL) {
		return fmt.Errorf("reader host not allowed: %s", parsed.Hostname())
	}
	return nil
}

func isKoreanNewsURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return strings.HasSuffix(host, ".kr") ||
		strings.Contains(host, "hankyung") ||
		strings.Contains(host, "mk.co.kr")
}

func readerCacheKey(rawURL string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(rawURL))))
	return usReaderCachePrefix + hex.EncodeToString(sum[:16])
}
