package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrCacheKeyForbidden = errors.New("cache key is not accessible")
	ErrCacheKeyNotFound  = errors.New("cache key not found")
	ErrCacheKeyReadOnly  = errors.New("cache key type is read-only in admin")
)

type CacheNamespace struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Pattern     string `json:"pattern"`
	Description string `json:"description"`
}

type CacheEntrySummary struct {
	Key        string `json:"key"`
	Type       string `json:"type"`
	TTLSeconds int64  `json:"ttlSeconds"`
	SizeBytes  int    `json:"sizeBytes"`
}

type CacheListResult struct {
	Pattern string              `json:"pattern"`
	Cursor  uint64              `json:"cursor"`
	Entries []CacheEntrySummary `json:"entries"`
}

type CacheEntryDetail struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	TTLSeconds  int64  `json:"ttlSeconds"`
	Value       string `json:"value"`
	ValueFormat string `json:"valueFormat"`
	ReadOnly    bool   `json:"readOnly"`
}

type CacheAdminService struct {
	redis *redis.Client
}

func NewCacheAdminService(redisClient *redis.Client) *CacheAdminService {
	return &CacheAdminService{redis: redisClient}
}

func (s *CacheAdminService) Namespaces() []CacheNamespace {
	return []CacheNamespace{
		{ID: "quotes", Label: "시세 · 터미널", Pattern: "quotes:*", Description: "quote detail, terminal, trades, orderbook, stats"},
		{ID: "market", Label: "마켓 · 랭킹", Pattern: "market:*", Description: "rankings, prices, ref price, FX, calendars"},
		{ID: "md", Label: "캔들 스냅샷", Pattern: "md:*", Description: "candle snapshots, raw tails, revision counters"},
		{ID: "news", Label: "뉴스", Pattern: "news:*", Description: "news snapshots, reader cache, cursors, OG preview"},
		{ID: "holdings", Label: "보유종목", Pattern: "holdings:*", Description: "manual holdings per user"},
		{ID: "import", Label: "임포트", Pattern: "import:*", Description: "holdings import job snapshots"},
		{ID: "apibudget", Label: "API 할당량", Pattern: "apibudget:*", Description: "provider rate-limit counters (수정 시 주의)"},
		{ID: "all", Label: "전체 (주의)", Pattern: "*", Description: "모든 Redis 키 — 민감 키는 자동 제외"},
	}
}

var cacheAdminBlockedPrefixes = []string{
	"toss:oauth:",
}

var cacheAdminReadOnlyPrefixes = []string{
	"apibudget:",
}

func cacheAdminKeyBlocked(key string) bool {
	k := strings.TrimSpace(key)
	if k == "" {
		return true
	}
	for _, p := range cacheAdminBlockedPrefixes {
		if strings.HasPrefix(k, p) {
			return true
		}
	}
	return false
}

func cacheAdminKeyReadOnly(key string) bool {
	k := strings.TrimSpace(key)
	for _, p := range cacheAdminReadOnlyPrefixes {
		if strings.HasPrefix(k, p) {
			return true
		}
	}
	return false
}

// cacheAdminScanPattern turns plain keywords into Redis SCAN globs (*substring*).
// Patterns that already contain glob metacharacters are passed through unchanged.
func cacheAdminScanPattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return "*"
	}
	if strings.ContainsAny(pattern, "*?[") {
		return pattern
	}
	return "*" + pattern + "*"
}

func (s *CacheAdminService) List(ctx context.Context, pattern string, cursor uint64, limit int) (CacheListResult, error) {
	if s.redis == nil {
		return CacheListResult{}, fmt.Errorf("redis unavailable")
	}
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		pattern = "*"
	}
	pattern = cacheAdminScanPattern(pattern)
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	out := CacheListResult{Pattern: pattern, Cursor: cursor, Entries: []CacheEntrySummary{}}
	for len(out.Entries) < limit {
		keys, next, err := s.redis.Scan(ctx, out.Cursor, pattern, int64(limit*4)).Result()
		if err != nil {
			return CacheListResult{}, err
		}
		for _, key := range keys {
			if cacheAdminKeyBlocked(key) {
				continue
			}
			entry, ok, err := s.summary(ctx, key)
			if err != nil {
				return CacheListResult{}, err
			}
			if ok {
				out.Entries = append(out.Entries, entry)
			}
			if len(out.Entries) >= limit {
				break
			}
		}
		out.Cursor = next
		if next == 0 || len(out.Entries) >= limit {
			break
		}
	}
	return out, nil
}

func (s *CacheAdminService) summary(ctx context.Context, key string) (CacheEntrySummary, bool, error) {
	exists, err := s.redis.Exists(ctx, key).Result()
	if err != nil {
		return CacheEntrySummary{}, false, err
	}
	if exists == 0 {
		return CacheEntrySummary{}, false, nil
	}
	typ, err := s.redis.Type(ctx, key).Result()
	if err != nil {
		return CacheEntrySummary{}, false, err
	}
	ttl, err := s.redis.TTL(ctx, key).Result()
	if err != nil {
		return CacheEntrySummary{}, false, err
	}
	size := 0
	if typ == "string" {
		if n, err := s.redis.StrLen(ctx, key).Result(); err == nil {
			size = int(n)
		}
	}
	return CacheEntrySummary{
		Key:        key,
		Type:       typ,
		TTLSeconds: ttlSeconds(ttl),
		SizeBytes:  size,
	}, true, nil
}

func ttlSeconds(d time.Duration) int64 {
	if d == -1 {
		return -1
	}
	if d == -2 {
		return -2
	}
	return int64(d.Seconds())
}

func (s *CacheAdminService) Get(ctx context.Context, key string) (CacheEntryDetail, error) {
	if s.redis == nil {
		return CacheEntryDetail{}, fmt.Errorf("redis unavailable")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return CacheEntryDetail{}, fmt.Errorf("key required")
	}
	if cacheAdminKeyBlocked(key) {
		return CacheEntryDetail{}, ErrCacheKeyForbidden
	}

	exists, err := s.redis.Exists(ctx, key).Result()
	if err != nil {
		return CacheEntryDetail{}, err
	}
	if exists == 0 {
		return CacheEntryDetail{}, ErrCacheKeyNotFound
	}

	typ, err := s.redis.Type(ctx, key).Result()
	if err != nil {
		return CacheEntryDetail{}, err
	}
	ttl, err := s.redis.TTL(ctx, key).Result()
	if err != nil {
		return CacheEntryDetail{}, err
	}

	readOnly := cacheAdminKeyReadOnly(key) || typ != "string"
	detail := CacheEntryDetail{
		Key:        key,
		Type:       typ,
		TTLSeconds: ttlSeconds(ttl),
		ReadOnly:   readOnly,
	}

	switch typ {
	case "string":
		raw, err := s.redis.Get(ctx, key).Result()
		if err != nil {
			return CacheEntryDetail{}, err
		}
		detail.Value, detail.ValueFormat = formatCacheValue(raw)
	default:
		detail.Value = fmt.Sprintf("(type %s — admin에서 문자열 키만 수정 가능)", typ)
		detail.ValueFormat = "text"
	}
	return detail, nil
}

func formatCacheValue(raw string) (string, string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw, "text"
	}
	if json.Valid([]byte(trimmed)) {
		var v any
		if err := json.Unmarshal([]byte(trimmed), &v); err == nil {
			pretty, err := json.MarshalIndent(v, "", "  ")
			if err == nil {
				return string(pretty), "json"
			}
		}
		return raw, "json"
	}
	return raw, "text"
}

func (s *CacheAdminService) Set(ctx context.Context, key, value string, ttlSecondsPtr *int64) error {
	if s.redis == nil {
		return fmt.Errorf("redis unavailable")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("key required")
	}
	if cacheAdminKeyBlocked(key) || cacheAdminKeyReadOnly(key) {
		return ErrCacheKeyForbidden
	}

	typ, err := s.redis.Type(ctx, key).Result()
	if err != nil {
		return err
	}
	if typ != "none" && typ != "string" {
		return ErrCacheKeyReadOnly
	}

	compact := compactJSONIfPossible(value)
	var ttl time.Duration
	if ttlSecondsPtr == nil {
		if existing, err := s.redis.TTL(ctx, key).Result(); err == nil && existing > 0 {
			ttl = existing
		}
	} else if *ttlSecondsPtr < 0 {
		ttl = 0
	} else {
		ttl = time.Duration(*ttlSecondsPtr) * time.Second
	}
	return s.redis.Set(ctx, key, compact, ttl).Err()
}

func compactJSONIfPossible(value string) string {
	trimmed := strings.TrimSpace(value)
	if !json.Valid([]byte(trimmed)) {
		return value
	}
	var v any
	if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
		return value
	}
	out, err := json.Marshal(v)
	if err != nil {
		return value
	}
	return string(out)
}

func (s *CacheAdminService) Delete(ctx context.Context, key string) error {
	if s.redis == nil {
		return fmt.Errorf("redis unavailable")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("key required")
	}
	if cacheAdminKeyBlocked(key) || cacheAdminKeyReadOnly(key) {
		return ErrCacheKeyForbidden
	}
	n, err := s.redis.Del(ctx, key).Result()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrCacheKeyNotFound
	}
	return nil
}
