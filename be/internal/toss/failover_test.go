package toss

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/config"
)

type memTokenStore struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (s *memTokenStore) Get(_ context.Context, k string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.m[k]; ok {
		return v, nil
	}
	return nil, nil
}

func (s *memTokenStore) Set(_ context.Context, k string, v []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string][]byte{}
	}
	s.m[k] = v
	return nil
}

func (s *memTokenStore) Delete(_ context.Context, k string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, k)
	return nil
}

type dualTossServer struct {
	mu                 sync.Mutex
	primaryPrices      int
	secondaryPrices    int
	primaryCandles     int
	secondaryCandles   int
	primaryFailPrices  bool
	primaryFailCandles bool
	primaryLowRemain   bool
	primaryStatus      int // non-429 failure, 0 = success

	// primaryFailTimes — primaryStatus를 **처음 N번만** 낸다(0 = 계속).
	// 「잠깐 흔들렸다 복구되는」 상류를 흉내 낸다.
	primaryFailTimes int
	// primaryUnauthorizedTimes — 처음 N번 401을 낸다. 토큰 회전 경합 재현용.
	primaryUnauthorizedTimes int
}

func (s *dualTossServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		id := r.Form.Get("client_id")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "tok-" + id,
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})
	mux.HandleFunc("/api/v1/prices", func(w http.ResponseWriter, r *http.Request) {
		s.serve(w, r, "prices")
	})
	mux.HandleFunc("/api/v1/candles", func(w http.ResponseWriter, r *http.Request) {
		s.serve(w, r, "candles")
	})
	return mux
}

func (s *dualTossServer) serve(w http.ResponseWriter, r *http.Request, kind string) {
	auth := r.Header.Get("Authorization")
	primary := auth == "Bearer tok-primary-id"

	s.mu.Lock()
	switch {
	case kind == "prices" && primary:
		s.primaryPrices++
	case kind == "prices" && !primary:
		s.secondaryPrices++
	case kind == "candles" && primary:
		s.primaryCandles++
	case kind == "candles" && !primary:
		s.secondaryCandles++
	}
	fail429 := primary && ((kind == "prices" && s.primaryFailPrices) || (kind == "candles" && s.primaryFailCandles))
	failOther := primary && s.primaryStatus != 0 && kind == "prices"
	if failOther && s.primaryFailTimes > 0 {
		// 「처음 N번만 실패」 — 그 뒤에는 정상 응답으로 돌아온다.
		failOther = s.primaryPrices <= s.primaryFailTimes
	}
	fail401 := primary && kind == "prices" && s.primaryPrices <= s.primaryUnauthorizedTimes
	low := primary && s.primaryLowRemain
	status := s.primaryStatus
	s.mu.Unlock()

	if fail401 {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"code":"invalid-token","message":"expired"}}`)
		return
	}
	if failOther {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, `{"error":{"code":"boom","message":"boom"}}`)
		return
	}
	if fail429 {
		w.Header().Set("X-RateLimit-Limit", "15")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1")
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"code":"rate-limit-exceeded","message":"rate-limit exceeded"}}`)
		return
	}
	if low {
		w.Header().Set("X-RateLimit-Limit", "15")
		w.Header().Set("X-RateLimit-Remaining", "2")
		w.Header().Set("X-RateLimit-Reset", "1")
	}
	if kind == "prices" {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{{"symbol": "AAPL", "lastPrice": "1", "currency": "USD"}},
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"result": map[string]any{"candles": []any{}, "hasNext": false},
	})
}

func newDualClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := NewClient(config.TossConfig{
		BaseURL:               srv.URL,
		ClientID:              "primary-id",
		ClientSecret:          "primary-secret",
		SecondaryClientID:     "secondary-id",
		SecondaryClientSecret: "secondary-secret",
	}, &memTokenStore{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if !c.HasSecondary() {
		t.Fatal("expected secondary to be attached")
	}
	// Seed tokens so tests don't wait ~2s for the peer-refresh lock loop
	// that only exists to coordinate Redis across processes.
	seedTestToken(c.primary, "tok-primary-id")
	seedTestToken(c.secondary, "tok-secondary-id")
	return c
}

func seedTestToken(tm *TokenManager, token string) {
	if tm == nil {
		return
	}
	tm.mu.Lock()
	tm.memToken = &cachedToken{AccessToken: token, ExpiresAt: time.Now().Add(time.Hour)}
	tm.mu.Unlock()
}

func TestFailover_429OnPrimaryRetriesSecondary(t *testing.T) {
	stub := &dualTossServer{primaryFailPrices: true}
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)

	c := newDualClient(t, srv)
	prices, err := c.GetPrices(context.Background(), []string{"AAPL"})
	if err != nil {
		t.Fatalf("GetPrices: %v", err)
	}
	if len(prices) != 1 || prices[0].Symbol != "AAPL" {
		t.Fatalf("got %+v, want AAPL from secondary", prices)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.primaryPrices != 1 || stub.secondaryPrices != 1 {
		t.Fatalf("calls primary=%d secondary=%d, want 1 and 1", stub.primaryPrices, stub.secondaryPrices)
	}
}

func TestFailover_stickyOnSecondaryUntilCoolExpires(t *testing.T) {
	stub := &dualTossServer{primaryFailPrices: true}
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)

	c := newDualClient(t, srv)
	frozen := time.Now()
	c.failover.now = func() time.Time { return frozen }

	if _, err := c.GetPrices(context.Background(), []string{"AAPL"}); err != nil {
		t.Fatalf("first GetPrices: %v", err)
	}

	stub.mu.Lock()
	stub.primaryFailPrices = false
	stub.mu.Unlock()

	if _, err := c.GetPrices(context.Background(), []string{"AAPL"}); err != nil {
		t.Fatalf("second GetPrices: %v", err)
	}
	stub.mu.Lock()
	p2, s2 := stub.primaryPrices, stub.secondaryPrices
	stub.mu.Unlock()
	if p2 != 1 || s2 != 2 {
		t.Fatalf("after sticky: primary=%d secondary=%d, want 1 and 2 (second call must skip primary)", p2, s2)
	}

	frozen = frozen.Add(2 * time.Second)
	if _, err := c.GetPrices(context.Background(), []string{"AAPL"}); err != nil {
		t.Fatalf("third GetPrices: %v", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.primaryPrices != 2 || stub.secondaryPrices != 2 {
		t.Fatalf("after cool: primary=%d secondary=%d, want 2 and 2 (back on primary)", stub.primaryPrices, stub.secondaryPrices)
	}
}

func TestFailover_groupsAreIndependent(t *testing.T) {
	stub := &dualTossServer{primaryFailCandles: true}
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)

	c := newDualClient(t, srv)
	if _, err := c.GetCandles(context.Background(), GetCandlesParams{Symbol: "AAPL", Interval: CandleInterval1d, Count: 1}); err != nil {
		t.Fatalf("GetCandles: %v", err)
	}
	if _, err := c.GetPrices(context.Background(), []string{"AAPL"}); err != nil {
		t.Fatalf("GetPrices: %v", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.primaryCandles != 1 || stub.secondaryCandles != 1 {
		t.Fatalf("candles primary=%d secondary=%d, want failover 1/1", stub.primaryCandles, stub.secondaryCandles)
	}
	if stub.primaryPrices != 1 || stub.secondaryPrices != 0 {
		t.Fatalf("prices primary=%d secondary=%d, want primary only (different group)", stub.primaryPrices, stub.secondaryPrices)
	}
}

/*
5xx는 **같은 자격증명으로 다시 시도**하되, 다른 자격증명으로 넘기지는 않는다.

둘을 갈라야 하는 이유: 5xx는 상류가 잠깐 흔들린 것이므로 잠시 뒤 같은 키로 붙지만,
failover는 **rate limit 전용**이다. 500에 failover를 걸면 멀쩡한 예비 키의 예산을
상류 장애 때마다 태우게 된다.
*/
func TestFailover_serverErrorRetriesSameKeyAndDoesNotSwitch(t *testing.T) {
	stub := &dualTossServer{primaryStatus: http.StatusInternalServerError}
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)

	c := newDualClient(t, srv)
	_, err := c.GetPrices(context.Background(), []string{"AAPL"})
	if err == nil {
		t.Fatal("expected primary 500 to surface")
	}
	if IsRateLimitError(err) {
		t.Fatal("500 must not look like a rate limit")
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.primaryPrices != transientRetryAttempts {
		t.Fatalf("primary=%d, want %d — 5xx는 같은 키로 재시도해야 한다",
			stub.primaryPrices, transientRetryAttempts)
	}
	if stub.secondaryPrices != 0 {
		t.Fatalf("secondary=%d, want 0 — 500에 failover를 걸면 예비 키 예산을 태운다", stub.secondaryPrices)
	}
}

// 5xx가 한 번만 나고 복구되면 사용자에게는 아무 일도 없어야 한다.
func TestTransientRetry_recoversWithoutSurfacingError(t *testing.T) {
	stub := &dualTossServer{primaryStatus: http.StatusBadGateway, primaryFailTimes: 1}
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)

	c := newDualClient(t, srv)
	if _, err := c.GetPrices(context.Background(), []string{"AAPL"}); err != nil {
		t.Fatalf("일시적 502가 그대로 올라왔다: %v", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.primaryPrices != 2 {
		t.Fatalf("primary=%d, want 2 (실패 1 + 성공 1)", stub.primaryPrices)
	}
	if stub.secondaryPrices != 0 {
		t.Fatalf("secondary=%d, want 0", stub.secondaryPrices)
	}
}

/*
401은 **여러 번** 다시 시도해야 한다.

자격증명을 REST·웹소켓·워커가 공유하고 발급이 파괴적이라, 회전 순간에 연속으로
두 번 밀릴 수 있다. 한 번만 재시도하면 그때 사용자 요청이 실패한다.
*/
func TestAuthRetry_survivesConsecutiveRotations(t *testing.T) {
	stub := &dualTossServer{primaryUnauthorizedTimes: 2}
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)

	c := newDualClient(t, srv)
	if _, err := c.GetPrices(context.Background(), []string{"AAPL"}); err != nil {
		t.Fatalf("연속 401 두 번을 못 견뎠다: %v", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.primaryPrices != 3 {
		t.Fatalf("primary=%d, want 3 (401 두 번 + 성공)", stub.primaryPrices)
	}
}

// 401이 계속되면 결국 실패해야 한다 — 무한 재시도는 자격증명 문제를 감춘다.
func TestAuthRetry_givesUpEventually(t *testing.T) {
	stub := &dualTossServer{primaryUnauthorizedTimes: 99}
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)

	c := newDualClient(t, srv)
	_, err := c.GetPrices(context.Background(), []string{"AAPL"})
	if err == nil {
		t.Fatal("401이 계속되는데 성공으로 돌아왔다")
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.primaryPrices != authRetryAttempts {
		t.Fatalf("primary=%d, want %d", stub.primaryPrices, authRetryAttempts)
	}
}

func TestFailover_lowRemainingSteersNextCall(t *testing.T) {
	stub := &dualTossServer{primaryLowRemain: true}
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)

	c := newDualClient(t, srv)
	frozen := time.Now()
	c.failover.now = func() time.Time { return frozen }

	if _, err := c.GetPrices(context.Background(), []string{"AAPL"}); err != nil {
		t.Fatalf("first GetPrices: %v", err)
	}
	stub.mu.Lock()
	stub.primaryLowRemain = false
	stub.mu.Unlock()

	if _, err := c.GetPrices(context.Background(), []string{"AAPL"}); err != nil {
		t.Fatalf("second GetPrices: %v", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.primaryPrices != 1 || stub.secondaryPrices != 1 {
		t.Fatalf("after low remaining: primary=%d secondary=%d, want 1 and 1", stub.primaryPrices, stub.secondaryPrices)
	}
}

func TestNewClientForCredentials_noSecondary(t *testing.T) {
	stub := &dualTossServer{primaryFailPrices: true}
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)

	c, err := NewClientForCredentials(srv.URL, "primary-id", "primary-secret", &memTokenStore{}, "test:token")
	if err != nil {
		t.Fatalf("NewClientForCredentials: %v", err)
	}
	seedTestToken(c.primary, "tok-primary-id")
	if c.HasSecondary() {
		t.Fatal("single-key constructor must not attach secondary")
	}
	_, err = c.GetPrices(context.Background(), []string{"AAPL"})
	if !IsRateLimitError(err) {
		t.Fatalf("err=%v, want rate limit (no failover)", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.secondaryPrices != 0 {
		t.Fatalf("secondary was called without being configured")
	}
}

func TestNewClient_identicalCredentialsSkipSecondary(t *testing.T) {
	c, err := NewClient(config.TossConfig{
		BaseURL:               "https://example.invalid",
		ClientID:              "same",
		ClientSecret:          "same-secret",
		SecondaryClientID:     "same",
		SecondaryClientSecret: "same-secret",
	}, &memTokenStore{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.HasSecondary() {
		t.Fatal("identical secondary credentials must not attach a second TokenManager")
	}
}

func TestRateLimitGroup(t *testing.T) {
	cases := map[string]string{
		"/api/v1/prices":                          groupMarketData,
		"/api/v1/orderbook":                       groupMarketData,
		"/api/v1/trades":                          groupMarketData,
		"/api/v1/price-limits":                    groupMarketData,
		"/api/v1/candles":                         groupMarketDataChart,
		"/api/v1/stocks":                          groupStock,
		"/api/v1/stocks/005930/warnings":          groupStock,
		"/api/v1/rankings":                        groupRanking,
		"/api/v1/exchange-rate":                   groupMarketInfo,
		"/api/v1/market-calendar/KR":              groupMarketInfo,
		"/api/v1/market-calendar/US":              groupMarketInfo,
		"/api/v1/market-indicators/prices":        groupMarketIndicator,
		"/api/v1/market-indicators/KOSPI/candles": groupMarketIndicator,
		"/api/v1/unknown":                         groupOther,
	}
	for path, want := range cases {
		if got := rateLimitGroup(path); got != want {
			t.Errorf("rateLimitGroup(%q)=%s want %s", path, got, want)
		}
	}
}

func TestRateLimitStatus_isLow(t *testing.T) {
	if (RateLimitStatus{Present: true, Limit: 15, Remaining: 3}).isLow() == false {
		t.Fatal("3/15 is 20% — should be low")
	}
	if (RateLimitStatus{Present: true, Limit: 15, Remaining: 4}).isLow() {
		t.Fatal("4/15 is above 20% — should not be low")
	}
	if (RateLimitStatus{Present: true, Remaining: 0}).isLow() == false {
		t.Fatal("remaining=0 is exhausted even without a limit")
	}
	if (RateLimitStatus{}).isLow() {
		t.Fatal("absent headers are not low")
	}
}
