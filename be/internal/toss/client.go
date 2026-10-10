package toss

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/metrics"
)

const (
	accountHeader  = "X-Tossinvest-Account"
	defaultTimeout = 20 * time.Second

	keyPrimary   = "primary"
	keySecondary = "secondary"

	/*
		authRetryAttempts — 401을 맞았을 때 새 토큰으로 다시 시도하는 횟수(첫 시도 포함).

		자격증명을 REST·웹소켓·워커가 공유하고 발급이 파괴적이라(client_id당 활성
		토큰 1개), 회전 순간에는 **연속으로** 밀릴 수 있다. 3이면 두 번의 연속 회전을
		견딘다. 더 늘리지 않는 것은 그 이상이면 토큰 문제가 아니라 자격증명 문제이고,
		그때는 빨리 실패해 로그에 남는 편이 낫기 때문이다.
	*/
	authRetryAttempts = 3

	// transientRetryAttempts — 전송 실패·5xx에서 조회를 다시 보내는 횟수(첫 시도 포함).
	transientRetryAttempts = 3
)

var (
	authRetryBackoff      = [authRetryAttempts - 1]time.Duration{100 * time.Millisecond, 300 * time.Millisecond}
	transientRetryBackoff = [transientRetryAttempts - 1]time.Duration{150 * time.Millisecond, 450 * time.Millisecond}
)

// Client is the reusable Toss Securities Open API HTTP client.
// Domain methods are grouped in separate files (market_data.go, asset.go, ...).
//
// When a secondary OAuth app is configured, every call goes to primary first.
// A 429 (or remaining ≤ 20% of the group's limit) on primary cools that
// rate-limit group until Toss's reset window; later calls in the same group
// use secondary until it expires. Other groups stay on primary. Call sites
// keep passing a single *Client — routing is internal.
type Client struct {
	baseURL    string
	httpClient *http.Client
	primary    *TokenManager
	secondary  *TokenManager
	failover   *groupFailover
}

func NewClient(cfg config.TossConfig, tokenStore TokenStore) (*Client, error) {
	c, err := NewClientForCredentials(cfg.BaseURL, cfg.ClientID, cfg.ClientSecret, tokenStore, tokenCacheKey)
	if err != nil {
		return nil, err
	}
	if !cfg.SecondaryEnabled() {
		return c, nil
	}
	c.secondary = NewTokenManagerWithKey(
		c.baseURL,
		cfg.SecondaryClientID,
		cfg.SecondaryClientSecret,
		tokenStore,
		tokenCacheKeySecondary,
	)
	c.failover = newGroupFailover()
	return c, nil
}

func NewClientForCredentials(baseURL, clientID, clientSecret string, tokenStore TokenStore, redisTokenKey string) (*Client, error) {
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("toss credentials are not configured")
	}

	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		primary: NewTokenManagerWithKey(baseURL, clientID, clientSecret, tokenStore, redisTokenKey),
	}, nil
}

func (c *Client) Enabled() bool {
	return c != nil && c.primary != nil
}

func (c *Client) HasSecondary() bool {
	return c != nil && c.secondary != nil
}

/*
TokenManagers — 이 클라이언트가 쓰는 자격증명들의 토큰 관리자.

웹소켓이 **REST와 같은 관리자**를 쓰게 하려고 연다. 자기 것을 따로 만들면
같은 client_id에 대해 발급 주체가 둘이 되고, 발급이 파괴적이라 서로의 토큰을
죽인다 — 예전에 두 용도를 앱 단위로 갈라 놓아야 했던 이유가 정확히 그것이었다.

순서는 primary, secondary다. 호출자(구독 서비스)가 앱마다 연결을 나눠 연다.
*/
func (c *Client) TokenManagers() []*TokenManager {
	if c == nil {
		return nil
	}
	out := make([]*TokenManager, 0, 2)
	if c.primary != nil {
		out = append(out, c.primary)
	}
	if c.secondary != nil {
		out = append(out, c.secondary)
	}
	return out
}

// StartTokenRefresh runs proactive background OAuth token renewal for every
// configured key so the first failover is not blocked on a token round-trip.
func (c *Client) StartTokenRefresh(ctx context.Context) {
	if c == nil {
		return
	}
	if c.primary != nil {
		c.primary.StartBackgroundRefresh(ctx)
	}
	if c.secondary != nil {
		c.secondary.StartBackgroundRefresh(ctx)
	}
}

func (c *Client) get(ctx context.Context, path string, query url.Values, accountSeq *int64, dest any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, accountSeq, dest)
}

func (c *Client) pickAuth(group string) *TokenManager {
	if c.secondary != nil && c.failover.useSecondary(group) {
		return c.secondary
	}
	return c.primary
}

func (c *Client) keyName(auth *TokenManager) string {
	if c.secondary != nil && auth == c.secondary {
		return keySecondary
	}
	return keyPrimary
}

func (c *Client) do(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	accountSeq *int64,
	dest any,
) error {
	group := rateLimitGroup(path)
	auth := c.pickAuth(group)
	err := c.doWithAuth(ctx, auth, method, path, query, body, accountSeq, dest)
	if err == nil {
		return nil
	}
	if auth == c.primary && c.secondary != nil && IsRateLimitError(err) {
		c.failover.markPrimaryCool(group, coolDuration(RateLimitStatus{}, err))
		log.Printf("toss api failover: %s %s group=%s primary exhausted, retrying on secondary", method, path, group)
		return c.doWithAuth(ctx, c.secondary, method, path, query, body, accountSeq, dest)
	}
	return err
}

func (c *Client) doWithAuth(
	ctx context.Context,
	auth *TokenManager,
	method, path string,
	query url.Values,
	body any,
	accountSeq *int64,
	dest any,
) error {
	if auth == nil {
		return fmt.Errorf("toss credentials are not configured")
	}
	key := c.keyName(auth)
	group := rateLimitGroup(path)

	for attempt := 0; attempt < authRetryAttempts; attempt++ {
		token, err := auth.AccessToken(ctx)
		if err != nil {
			return err
		}

		raw, status, rl, err := c.doResilient(ctx, method, path, query, body, accountSeq, token, key)
		if err != nil {
			return err
		}

		if status == http.StatusNoContent {
			return nil
		}

		if status >= 400 {
			apiErr := errorFromStatus(status, raw, rl)
			if IsInvalidTokenError(apiErr) && attempt < authRetryAttempts-1 {
				/*
					401을 맞았다. 토큰이 죽었다는 뜻이고, 이유는 둘 중 하나다:
					만료됐거나, **형제 프로세스가 갈아 끼웠거나**.

					무조건 버리지 않는다 — 형제가 이미 새 토큰을 넣어 뒀다면 그것까지
					죽여 재발급 왕복이 시작된다(`InvalidateIfCurrent` 헤더).

					재시도를 한 번이 아니라 여러 번 두는 이유: 자격증명을 REST와
					웹소켓이 공유하고 프로세스도 여럿이라, 회전 순간에 **연속으로**
					두 번 밀릴 수 있다. 그때 사용자 요청이 실패하면 화면이 빈다.
					각 시도는 토큰을 다시 읽으므로 대개 두 번째에 붙는다.
				*/
				metrics.TossTokenEvent("retry_401")
				auth.InvalidateIfCurrent(ctx, token)
				if !sleepCtx(ctx, authRetryBackoff[attempt]) {
					return apiErr
				}
				continue
			}
			if auth == c.primary && IsRateLimitError(apiErr) {
				c.failover.markPrimaryCool(group, coolDuration(rl, apiErr))
			}
			return apiErr
		}

		if auth == c.primary && rl.isLow() {
			c.failover.markPrimaryCool(group, coolDuration(rl, nil))
		}

		if dest == nil {
			return nil
		}
		if err := json.Unmarshal(raw, dest); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return nil
	}
	return fmt.Errorf("toss api: unauthorized after token refresh")
}

func errorFromStatus(status int, raw []byte, rl RateLimitStatus) error {
	var errBody ErrorBody
	if json.Unmarshal(raw, &errBody) == nil && errBody.Error.Code != "" {
		return newErrorFromAPI(status, errBody, rl)
	}
	if status == http.StatusTooManyRequests {
		return newErrorFromAPI(status, ErrorBody{Error: APIError{
			Code:    errCodeRateLimitExceeded,
			Message: strings.TrimSpace(string(raw)),
		}}, rl)
	}
	if status == http.StatusUnauthorized {
		return newErrorFromAPI(status, ErrorBody{Error: APIError{
			Code:    ErrCodeInvalidToken,
			Message: strings.TrimSpace(string(raw)),
		}}, rl)
	}
	return fmt.Errorf("toss api %d: %s", status, strings.TrimSpace(string(raw)))
}

/*
doResilient — 일시적 실패를 흡수하고 한 요청을 보낸다.

# 무엇을 일시적으로 보나

	전송 실패(연결 리셋·타임아웃·DNS 순간 실패)
	5xx

둘 다 **상류가 잠깐 흔들린 것**이고, 200ms 뒤에는 대개 붙는다. 전에는 그대로
에러를 올렸고, 그러면 시세 한 칸이 통째로 비거나 화면이 last-good으로 물러났다.

# 왜 GET만 재시도하나

전송이 실패했다는 것은 **요청이 도달했는지 알 수 없다**는 뜻이지 도달하지
않았다는 뜻이 아니다. 조회는 몇 번을 더 보내도 같으므로 안전하지만, 주문 같은
POST/DELETE는 두 번 적용될 수 있다. 그래서 조회 외에는 예전대로 한 번만 보낸다.

# 429는 여기서 다루지 않는다

rate limit은 「잠깐 흔들린 것」이 아니라 **예산을 다 쓴 것**이다. 여기서 되쏘면
같은 버킷을 더 깎을 뿐이라, 상위의 그룹별 failover(다른 자격증명으로 전환)와
`Retry-After` 존중이 맡는다.
*/
func (c *Client) doResilient(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	accountSeq *int64,
	token string,
	key string,
) ([]byte, int, RateLimitStatus, error) {
	attempts := 1
	if method == http.MethodGet {
		attempts = transientRetryAttempts
	}

	var (
		raw    []byte
		status int
		rl     RateLimitStatus
		err    error
	)
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			metrics.TossRESTCall("TRANSIENT_RETRY")
			if !sleepCtx(ctx, transientRetryBackoff[attempt-1]) {
				break
			}
		}
		raw, status, rl, err = c.doOnce(ctx, method, path, query, body, accountSeq, token, key)
		if err == nil && status < 500 {
			return raw, status, rl, nil
		}
		if attempt == attempts-1 {
			break
		}
		if err != nil {
			log.Printf("toss api transient failure: %s %s key=%s: %v (재시도)", method, path, key, err)
		} else {
			log.Printf("toss api transient failure: %s %s key=%s: HTTP %d (재시도)", method, path, key, status)
		}
	}
	return raw, status, rl, err
}

// sleepCtx waits unless ctx ends first. false means the caller should give up.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

func (c *Client) doOnce(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	accountSeq *int64,
	token string,
	key string,
) ([]byte, int, RateLimitStatus, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, 0, RateLimitStatus{}, fmt.Errorf("parse url: %w", err)
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, 0, RateLimitStatus{}, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, 0, RateLimitStatus{}, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accountSeq != nil {
		req.Header.Set(accountHeader, strconv.FormatInt(*accountSeq, 10))
	}

	// 실제로 나간 요청만 센다 — 캐시·단일비행으로 막힌 것은 여기 오지 않는다.
	metrics.TossRESTCall(rateLimitGroup(path))
	metrics.TossRESTPath(metricPath(path))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, RateLimitStatus{}, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	rl := parseRateLimitStatus(resp.Header)
	logRateLimitStatus(method, path, key, resp.StatusCode, rl)

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, rl, fmt.Errorf("read body: %w", err)
	}
	return raw, resp.StatusCode, rl, nil
}
