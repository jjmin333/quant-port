package toss

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/metrics"
)

const (
	tokenCacheKey          = "toss:oauth:access_token"
	tokenCacheKeySecondary = "toss:oauth:access_token:secondary"
	refreshLeadTime        = 5 * time.Minute
	tokenLockTTL           = 15 * time.Second
	tokenLockWait          = 50 * time.Millisecond
	tokenLockTries         = 40 // ~2s total wait for peer refresh

	// tokenIssueAttempts / tokenIssueBackoff — 일시적 발급 실패를 흡수한다.
	// 짧게 세 번만 시도한다: 이 경로는 사용자 요청을 붙잡고 있으므로 길게 끌면
	// 「느린 실패」가 되고, 그보다는 아직 유효한 캐시 토큰으로 물러서는 편이 낫다.
	tokenIssueAttempts = 3
)

var tokenIssueBackoff = [tokenIssueAttempts - 1]time.Duration{200 * time.Millisecond, 600 * time.Millisecond}

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

	mu          sync.Mutex
	refreshing  bool
	refreshWait sync.Cond
	memToken    *cachedToken
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

/*
AccessToken — 지금 쓸 토큰. 만료 전에 미리 갱신하고, **갱신에 실패해도 아직 쓸 수
있는 토큰이 있으면 그것을 준다.**

# 왜 마지막 줄이 필요한가

갱신은 만료 `refreshLeadTime`(5분) 전에 시작한다. 그 5분 동안 발급이 실패할 수
있는 이유는 여럿이다 — Toss의 일시적 5xx, 네트워크 단절, `/oauth2/token` 자체의
rate limit, Redis 락을 잡은 형제 프로세스의 지연.

전에는 그때 **에러를 그대로 올렸다.** 그러면 손에 아직 5분 남은 멀쩡한 토큰을
들고도 모든 REST 호출이 실패한다 — 사용자에게는 시세가 통째로 멈춘 것으로 보인다.
하루 한 번 있는 회전 구간이 곧 장애 구간이 되는 셈이다.

그래서 순서를 이렇게 둔다:

 1. 갱신 여유가 남은 캐시 토큰   → 그대로
 2. 갱신 시도                    → 성공하면 새 토큰
 3. **아직 만료 전인 캐시 토큰**  → 최후 수단. 다음 호출이 다시 갱신을 시도한다
 4. 셋 다 없으면 그때 에러

3번은 「만료됐는데도 쓴다」가 아니다. 만료된 토큰은 어차피 401이므로 여기서
걸러진다 — 아직 유효한 토큰을 **버리지 않을 뿐**이다.
*/
func (tm *TokenManager) AccessToken(ctx context.Context) (string, error) {
	tm.mu.Lock()
	if token, ok := tm.loadCachedLocked(ctx, tokenFresh); ok {
		tm.mu.Unlock()
		return token, nil
	}
	tm.mu.Unlock()

	token, err := tm.ensureFresh(ctx)
	if err == nil {
		return token, nil
	}

	tm.mu.Lock()
	fallback, ok := tm.loadCachedLocked(ctx, tokenUsable)
	tm.mu.Unlock()
	if ok {
		metrics.TossTokenEvent("fallback_unexpired")
		log.Printf("toss oauth: refresh failed (%v) — 아직 만료 전인 토큰으로 계속한다", err)
		return fallback, nil
	}
	return "", err
}

func (tm *TokenManager) ensureFresh(ctx context.Context) (string, error) {
	tm.mu.Lock()
	for {
		if token, ok := tm.loadCachedLocked(ctx, tokenFresh); ok {
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

	/*
		**락을 먼저 잡는다.**

		전에는 여기서 `waitForPeerRefresh`를 먼저 불렀다. 그 함수는 형제가 새 토큰을
		올려 줄 때까지 50ms × 40회를 도는데, **아무도 갱신 중이 아니면 그 2초를
		그냥 태운 뒤** 락을 잡고 발급했다. 즉 경합이 없는 평상시 경로 —
		기동 직후와 24시간 회전 — 에 매번 2초가 붙었고, 그동안 사용자 요청이 잡혀 있었다.

		무리(herd)를 막는 것은 락이지 이 대기가 아니다. 락을 못 잡았을 때만
		기다리면 같은 보호를 받으면서 평상시 지연이 사라진다.
	*/
	acquired, err := tm.tryAcquireLock(ctx)
	if err != nil {
		return "", err
	}
	if !acquired {
		// 형제가 갱신 중이다 — 그쪽이 올려 줄 때까지 기다린다.
		if token, ok := tm.waitForPeerRefresh(ctx); ok {
			return token, nil
		}
		return "", fmt.Errorf("oauth token refresh lock timeout")
	}
	defer tm.releaseLock(ctx)

	tm.mu.Lock()
	if token, ok := tm.loadCachedLocked(ctx, tokenFresh); ok {
		tm.mu.Unlock()
		return token, nil
	}
	tm.mu.Unlock()

	token, expiresAt, err := tm.issueWithRetry(ctx)
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
		token, ok := tm.loadCachedLocked(ctx, tokenFresh)
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

/*
tokenNeed — 캐시된 토큰을 얼마나 까다롭게 볼지.

`ExpiresAt`은 **Toss가 알려 준 실제 만료 시각**이다(리드타임을 미리 빼 두지
않는다). 그래야 「갱신할 때가 됐나」와 「아직 쓸 수 있나」를 같은 값 하나로 나눠
물을 수 있다 — 미리 빼 두면 두 질문이 섞여 5분짜리 여유가 어디로 갔는지 알 수 없다.
*/
type tokenNeed int

const (
	// tokenFresh — 갱신 여유(refreshLeadTime)가 남아 있어야 한다. 평상시 경로.
	tokenFresh tokenNeed = iota
	// tokenUsable — 아직 만료 전이면 된다. **갱신이 실패했을 때만** 쓴다.
	tokenUsable
)

func (n tokenNeed) accepts(expiresAt, now time.Time) bool {
	if !now.Before(expiresAt) {
		return false // 만료된 것은 어느 쪽이든 안 된다 — 어차피 401이다
	}
	if n == tokenUsable {
		return true
	}
	return !needsRefresh(expiresAt, now)
}

func (tm *TokenManager) loadCachedLocked(ctx context.Context, need tokenNeed) (string, bool) {
	now := time.Now()

	if tm.memToken != nil {
		switch {
		case need.accepts(tm.memToken.ExpiresAt, now):
			return tm.memToken.AccessToken, true
		case !now.Before(tm.memToken.ExpiresAt):
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
	if need.accepts(cached.ExpiresAt, now) {
		tm.memToken = &cached
		return cached.AccessToken, true
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

/*
InvalidateIfCurrent — 401을 맞은 **그 토큰이 아직 최신일 때만** 캐시를 버린다.

# 왜 무조건 버리면 안 되는가

이 자격증명을 쓰는 프로세스가 여럿이다(`api`, `worker-fetch`, 그리고 웹소켓).
새 발급은 **파괴적**이라 이전 토큰을 즉시 죽인다. 그래서 무조건 버리는
`Invalidate`는 다음 왕복을 만든다:

	A가 T2 발급 → B가 들고 있던 T1로 401 → B가 캐시(=T2)를 지우고 T3 발급
	→ A가 T2로 401 → A가 캐시(=T3)를 지우고 T4 발급 → …

한쪽이 우연히 먼저 자리 잡을 때까지 발급이 멈추지 않는다. 지금은 REST와 웹소켓이
자격증명을 나눠 써서 드러나지 않았을 뿐, 합치면 바로 노출된다.

# 무엇이 달라지나

내가 실패한 토큰과 저장소의 현재 값이 **다르면** 형제가 이미 갈아 끼운 것이다.
그때는 아무것도 지우지 않고 그 새 값을 쓰면 된다 — 발급이 자격증명당 1회로
수렴한다.

`stale`이 비어 있으면 비교할 대상이 없으므로 예전 동작(무조건 버리기)을 한다.
*/
func (tm *TokenManager) InvalidateIfCurrent(ctx context.Context, stale string) {
	if strings.TrimSpace(stale) == "" {
		tm.Invalidate(ctx)
		return
	}

	tm.mu.Lock()
	if tm.memToken != nil && tm.memToken.AccessToken == stale {
		tm.memToken = nil
	}
	tm.mu.Unlock()

	if tm.store == nil {
		return
	}
	raw, err := tm.store.Get(ctx, tm.tokenCacheKey)
	if err != nil || len(raw) == 0 {
		return
	}
	var cached cachedToken
	if json.Unmarshal(raw, &cached) != nil {
		return
	}
	if cached.AccessToken != stale {
		// 형제가 이미 새 토큰을 넣어 뒀다 — 지우면 그것까지 죽인다.
		return
	}
	_ = tm.store.Delete(ctx, tm.tokenCacheKey)
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

	// 발급은 **파괴적**이다 — client_id당 활성 토큰이 하나라 새 발급이 이전
	// 토큰을 즉시 죽인다. 정상값은 자격증명당 하루 한 번(만료 5분 전 갱신)이고,
	// 이 카운터가 그보다 크게 늘면 재발급 왕복이 돌고 있다는 뜻이다.
	metrics.TossRESTCall("OAUTH_TOKEN")

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
	/*
		**실제 만료 시각을 그대로 저장한다.**

		전에는 여기서 `refreshLeadTime`을 미리 빼 뒀다. 그런데 `needsRefresh`가
		읽을 때 또 리드타임을 적용하므로 실질 갱신 시점이 만료 10분 전이 됐고,
		더 나쁘게는 **「아직 쓸 수 있는가」를 물을 방법이 사라졌다** — 저장값이
		이미 5분 당겨져 있어 그 5분이 여유인지 만료인지 구분되지 않았다.

		리드타임은 읽는 쪽(`tokenNeed`) 한 곳에서만 적용한다.
	*/
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
	if expiresAt.Before(time.Now().Add(time.Minute)) {
		expiresAt = time.Now().Add(time.Minute)
	}
	metrics.TossTokenEvent("issued")
	return token.AccessToken, expiresAt, nil
}

/*
issueWithRetry — 일시적 실패를 흡수하고 발급한다.

발급이 실패하는 이유는 대개 **곧 사라지는 것들**이다: 네트워크 순간 단절, Toss의
5xx, `/oauth2/token`의 순간 rate limit. 그런데 실패가 그대로 올라가면 그 순간의
모든 REST 호출이 같이 죽는다 — 하루 한 번의 회전 구간에서 특히 뼈아프다.

**영구 실패는 재시도하지 않는다.** `invalid_client` 같은 4xx는 자격증명이 틀렸다는
뜻이라 다시 불러 봐야 같은 답이고, 그 사이 쿼터만 태운다.

호출자에게 남는 방어선이 하나 더 있다 — 여기서 끝내 실패해도 `AccessToken`이
아직 만료 전인 캐시 토큰으로 물러선다.
*/
func (tm *TokenManager) issueWithRetry(ctx context.Context) (string, time.Time, error) {
	var lastErr error
	for attempt := 0; attempt < tokenIssueAttempts; attempt++ {
		if attempt > 0 {
			metrics.TossTokenEvent("issue_retry")
			select {
			case <-ctx.Done():
				return "", time.Time{}, ctx.Err()
			case <-time.After(tokenIssueBackoff[attempt-1]):
			}
		}
		token, expiresAt, err := tm.issue(ctx)
		if err == nil {
			return token, expiresAt, nil
		}
		lastErr = err
		if !isRetryableTokenError(err) {
			break
		}
	}
	metrics.TossTokenEvent("issue_failed")
	return "", time.Time{}, lastErr
}

/*
isRetryableTokenError — 다시 불러 볼 가치가 있는 실패인가.

`issue`는 4xx일 때 `oauth token %d: ...` 형태로 상태 코드를 문자열에 남긴다.
타입을 새로 만드는 대신 그 코드만 보는 이유는, 판단 기준이 **「자격증명이 틀렸나」**
하나뿐이라 타입 하나를 더 들일 만큼 무겁지 않기 때문이다.
*/
func isRetryableTokenError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, permanent := range []string{
		"oauth token 400", "oauth token 401", "oauth token 403", "oauth token 404",
	} {
		if strings.Contains(msg, permanent) {
			return false
		}
	}
	return true
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
				_, ok := tm.loadCachedLocked(bgCtx, tokenFresh)
				tm.mu.Unlock()
				if !ok {
					_, _ = tm.ensureFresh(bgCtx)
				}
			}
		}
	}()
}

/*
TokenAfter — `stale`이 죽었다고 판단됐을 때 **다음에 써야 할 토큰**을 준다.

웹소켓 재연결 경로가 쓴다.

# 여기 `IssueFresh`가 있었다

전에는 웹소켓이 재연결할 때마다 캐시를 무시하고 새로 발급했다. 근거는
「연결을 닫으면 그 토큰이 무효화된다」였는데, **그 전제가 실측으로 반증됐다**
(`docs/TOSS_TOKEN_DESIGN.md`). 정상 종료·강제 절단·종료 후 60초 어느 쪽에서도
그 토큰의 REST가 200이었고, **같은 토큰으로 재연결도 됐다**(3/3).

토큰을 죽인 진범은 종료가 아니라 그 재발급 자체였다. `IssueFresh`는 프로세스가
뜰 때마다 한 번씩 그 자격증명의 토큰을 갈아 치웠고, 그래서 REST와 자격증명을
공유할 수 없었다.

# 지금 하는 일

 1. 실패한 토큰이 아직 최신이면 그때만 캐시에서 버린다(CAS)
 2. 공유 경로로 다시 읽는다 — 형제가 이미 갈아 끼웠으면 그것을 쓰고,
    아무도 안 했으면 락을 잡고 **한 번만** 발급한다

정상 동작에서는 재연결이 캐시된 토큰을 그대로 재사용하므로 발급이 아예 일어나지
않는다. 발급은 하루 한 번(만료 5분 전 배경 갱신)으로 수렴한다.
*/
func (tm *TokenManager) TokenAfter(ctx context.Context, stale string) (string, error) {
	if strings.TrimSpace(stale) == "" {
		// 첫 연결·정상 재연결 — 캐시된 것을 그대로 쓴다. 발급 없음.
		return tm.AccessToken(ctx)
	}
	tm.InvalidateIfCurrent(ctx, stale)
	return tm.ensureFresh(ctx)
}
