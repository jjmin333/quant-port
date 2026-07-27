package toss

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	tokenCacheKey   = "toss:oauth:access_token"
	refreshLeadTime = 5 * time.Minute
	tokenLockTTL    = 15 * time.Second
	tokenLockWait   = 50 * time.Millisecond
	tokenLockTries  = 40 // ~2s total wait for peer refresh
)

type cachedToken struct {
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

// TokenStore persists OAuth tokens (Redis implementation provided).
type TokenStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

type tokenLockStore interface {
	TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Unlock(ctx context.Context, key string) error
}

type TokenManager struct {
	baseURL       string
	clientID      string
	clientSecret  string
	store         TokenStore
	tokenCacheKey string

	mu         sync.Mutex
	refreshing bool
	refreshWait sync.Cond
	memToken   *cachedToken
}

func NewTokenManager(baseURL, clientID, clientSecret string, store TokenStore) *TokenManager {
	return NewTokenManagerWithKey(baseURL, clientID, clientSecret, store, tokenCacheKey)
}

func NewTokenManagerWithKey(baseURL, clientID, clientSecret string, store TokenStore, cacheKey string) *TokenManager {
	if strings.TrimSpace(cacheKey) == "" {
		cacheKey = tokenCacheKey
	}
	tm := &TokenManager{
		baseURL:       baseURL,
		clientID:      clientID,
		clientSecret:  clientSecret,
		store:         store,
		tokenCacheKey: cacheKey,
	}
	tm.refreshWait.L = &tm.mu
	return tm
}

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

type oauthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// AccessToken returns a valid token, proactively refreshing before expiry.
func (tm *TokenManager) AccessToken(ctx context.Context) (string, error) {
	tm.mu.Lock()
	if token, ok := tm.loadCachedLocked(ctx, false); ok {
		tm.mu.Unlock()
		return token, nil
	}
	tm.mu.Unlock()
	return tm.ensureFresh(ctx)
}

func (tm *TokenManager) ensureFresh(ctx context.Context) (string, error) {
	tm.mu.Lock()
	for {
		if token, ok := tm.loadCachedLocked(ctx, false); ok {
			tm.mu.Unlock()
			return token, nil
		}
		if !tm.refreshing {
			break
		}
		tm.refreshWait.Wait()
	}
	tm.refreshing = true
	tm.mu.Unlock()

	defer func() {
		tm.mu.Lock()
		tm.refreshing = false
		tm.refreshWait.Broadcast()
		tm.mu.Unlock()
	}()

	if token, ok := tm.waitForPeerRefresh(ctx); ok {
		return token, nil
	}

	acquired, err := tm.tryAcquireLock(ctx)
	if err != nil {
		return "", err
	}
	if !acquired {
		if token, ok := tm.waitForPeerRefresh(ctx); ok {
			return token, nil
		}
		return "", fmt.Errorf("oauth token refresh lock timeout")
	}
	defer tm.releaseLock(ctx)

	tm.mu.Lock()
	if token, ok := tm.loadCachedLocked(ctx, false); ok {
		tm.mu.Unlock()
		return token, nil
	}
	tm.mu.Unlock()

	token, expiresAt, err := tm.issue(ctx)
	if err != nil {
		return "", err
	}

	cached := &cachedToken{AccessToken: token, ExpiresAt: expiresAt}
	tm.mu.Lock()
	tm.memToken = cached
	tm.mu.Unlock()
	tm.persistCached(ctx, cached)

	return token, nil
}

func (tm *TokenManager) waitForPeerRefresh(ctx context.Context) (string, bool) {
	for i := 0; i < tokenLockTries; i++ {
		tm.mu.Lock()
		token, ok := tm.loadCachedLocked(ctx, false)
		tm.mu.Unlock()
		if ok {
			return token, true
		}
		select {
		case <-ctx.Done():
			return "", false
		case <-time.After(tokenLockWait):
		}
	}
	return "", false
}

func (tm *TokenManager) lockKey() string {
	return tm.tokenCacheKey + ":lock"
}

func (tm *TokenManager) tryAcquireLock(ctx context.Context) (bool, error) {
	lockStore, ok := tm.store.(tokenLockStore)
	if !ok || lockStore == nil {
		return true, nil
	}
	return lockStore.TryLock(ctx, tm.lockKey(), tokenLockTTL)
}

func (tm *TokenManager) releaseLock(ctx context.Context) {
	lockStore, ok := tm.store.(tokenLockStore)
	if !ok || lockStore == nil {
		return
	}
	_ = lockStore.Unlock(ctx, tm.lockKey())
}

func (tm *TokenManager) loadCachedLocked(ctx context.Context, allowStale bool) (string, bool) {
	now := time.Now()

	if tm.memToken != nil {
		if allowStale || now.Before(tm.memToken.ExpiresAt) {
			if allowStale || !needsRefresh(tm.memToken.ExpiresAt, now) {
				return tm.memToken.AccessToken, true
			}
		} else {
			tm.memToken = nil
		}
	}

	if tm.store == nil {
		return "", false
	}

	raw, err := tm.store.Get(ctx, tm.tokenCacheKey)
	if err != nil || len(raw) == 0 {
		return "", false
	}

	var cached cachedToken
	if json.Unmarshal(raw, &cached) != nil {
		return "", false
	}
	if allowStale || now.Before(cached.ExpiresAt) {
		if allowStale || !needsRefresh(cached.ExpiresAt, now) {
			tm.memToken = &cached
			return cached.AccessToken, true
		}
	}
	return "", false
}

func needsRefresh(expiresAt, now time.Time) bool {
	return !expiresAt.After(now.Add(refreshLeadTime))
}

func (tm *TokenManager) Invalidate(ctx context.Context) {
	tm.mu.Lock()
	tm.memToken = nil
	tm.mu.Unlock()
	if tm.store != nil {
		_ = tm.store.Delete(ctx, tm.tokenCacheKey)
	}
}

func (tm *TokenManager) persistCached(ctx context.Context, cached *cachedToken) {
	if tm.store == nil {
		return
	}
	raw, err := json.Marshal(cached)
	if err != nil {
		return
	}
	ttl := time.Until(cached.ExpiresAt)
	if ttl <= 0 {
		return
	}
	_ = tm.store.Set(ctx, tm.tokenCacheKey, raw, ttl)
}

func (tm *TokenManager) issue(ctx context.Context) (string, time.Time, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", tm.clientID)
	form.Set("client_secret", tm.clientSecret)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(tm.baseURL, "/")+"/oauth2/token",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("new token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var oauthErr oauthErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&oauthErr)
		if oauthErr.Error != "" {
			msg := oauthErr.Error
			if oauthErr.ErrorDescription != "" {
				msg += ": " + oauthErr.ErrorDescription
			}
			return "", time.Time{}, fmt.Errorf("oauth token %d: %s", resp.StatusCode, msg)
		}
		return "", time.Time{}, fmt.Errorf("oauth token failed: status %d", resp.StatusCode)
	}

	var token oauthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return "", time.Time{}, fmt.Errorf("decode token: %w", err)
	}

	expiresIn := token.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 86400
	}
	// Refresh refreshLeadTime before actual expiry.
	expiresAt := time.Now().Add(time.Duration(expiresIn)*time.Second - refreshLeadTime)
	if expiresAt.Before(time.Now().Add(time.Minute)) {
		expiresAt = time.Now().Add(time.Minute)
	}
	return token.AccessToken, expiresAt, nil
}

// StartBackgroundRefresh proactively renews tokens before they expire.
func (tm *TokenManager) StartBackgroundRefresh(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				bgCtx := context.Background()
				tm.mu.Lock()
				_, ok := tm.loadCachedLocked(bgCtx, false)
				tm.mu.Unlock()
				if !ok {
					_, _ = tm.ensureFresh(bgCtx)
				}
			}
		}
	}()
}
