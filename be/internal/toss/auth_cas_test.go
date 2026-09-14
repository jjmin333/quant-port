package toss

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// memStore is a minimal TokenStore. No locking interface on purpose — the
// lock path is optional and these tests are about the CAS decision itself.
type memStore struct {
	mu   sync.Mutex
	data map[string][]byte
	dels int
}

func newMemStore() *memStore { return &memStore{data: map[string][]byte{}} }

func (m *memStore) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data[key], nil
}

func (m *memStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *memStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dels++
	delete(m.data, key)
	return nil
}

func (m *memStore) put(t *testing.T, key, token string) {
	t.Helper()
	raw, err := json.Marshal(cachedToken{AccessToken: token, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	_ = m.Set(context.Background(), key, raw, time.Hour)
}

func (m *memStore) has(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.data[key]
	return ok
}

/*
401을 맞은 토큰이 **아직 최신일 때만** 버려야 한다.

발급이 파괴적이라(client_id당 활성 토큰 1개) 무조건 버리면 프로세스들이 서로의
토큰을 죽이는 왕복이 시작된다:

	A가 T2 발급 → B가 T1로 401 → B가 T2를 지우고 T3 발급 → A가 T2로 401 → …

REST와 웹소켓이 자격증명을 공유하게 되면서 이 경로가 실제로 열렸다.
*/
func TestInvalidateIfCurrent(t *testing.T) {
	ctx := context.Background()

	t.Run("실패한 토큰이 최신이면 버린다", func(t *testing.T) {
		store := newMemStore()
		tm := NewTokenManagerWithKey("https://x", "id", "secret", store, "k")
		store.put(t, "k", "T1")
		tm.memToken = &cachedToken{AccessToken: "T1", ExpiresAt: time.Now().Add(time.Hour)}

		tm.InvalidateIfCurrent(ctx, "T1")

		if store.has("k") {
			t.Fatal("최신 토큰이 죽었는데 캐시에 남았다 — 다음 호출도 401을 맞는다")
		}
		if tm.memToken != nil {
			t.Fatal("메모리 캐시가 안 지워졌다")
		}
	})

	t.Run("형제가 이미 갈아 끼웠으면 건드리지 않는다", func(t *testing.T) {
		store := newMemStore()
		tm := NewTokenManagerWithKey("https://x", "id", "secret", store, "k")
		store.put(t, "k", "T2") // 형제가 넣어 둔 새 토큰
		tm.memToken = &cachedToken{AccessToken: "T1", ExpiresAt: time.Now().Add(time.Hour)}

		tm.InvalidateIfCurrent(ctx, "T1")

		if !store.has("k") {
			t.Fatal("형제의 새 토큰까지 지웠다 — 재발급 왕복이 시작된다")
		}
		if store.dels != 0 {
			t.Fatalf("Delete가 %d회 불렸다 — 한 번도 부르면 안 된다", store.dels)
		}
		// 내가 들고 있던 죽은 토큰은 버려야 다음 읽기가 저장소의 새 것을 집는다.
		if tm.memToken != nil {
			t.Fatal("죽은 토큰이 메모리에 남았다")
		}
	})

	t.Run("stale이 비면 예전 동작(무조건 버리기)", func(t *testing.T) {
		store := newMemStore()
		tm := NewTokenManagerWithKey("https://x", "id", "secret", store, "k")
		store.put(t, "k", "T9")

		tm.InvalidateIfCurrent(ctx, "")

		if store.has("k") {
			t.Fatal("비교 대상이 없으면 버려야 한다")
		}
	})

	t.Run("저장소가 비어 있어도 죽지 않는다", func(t *testing.T) {
		store := newMemStore()
		tm := NewTokenManagerWithKey("https://x", "id", "secret", store, "k")
		tm.InvalidateIfCurrent(ctx, "T1") // panic 없이 통과해야 한다
	})
}

/*
정상 재연결은 **발급을 일으키지 않아야** 한다.

여기가 이번 변경의 핵심이다. 예전 `IssueFresh`는 재연결마다 새로 발급해서
그 자격증명의 REST 토큰을 죽였고, 그래서 두 용도를 앱 단위로 갈라야 했다.

`issue`가 실제로 불리면 이 테스트는 네트워크로 나가려다 실패한다 — baseURL이
가짜라 에러가 나므로, 에러 없이 캐시된 값이 돌아오는 것 자체가 「발급 안 함」의 증거다.
*/
func TestTokenAfter_reusesCachedTokenWithoutIssuing(t *testing.T) {
	store := newMemStore()
	tm := NewTokenManagerWithKey("https://127.0.0.1:1/never", "id", "secret", store, "k")
	store.put(t, "k", "CACHED")

	got, err := tm.TokenAfter(context.Background(), "")
	if err != nil {
		t.Fatalf("재연결이 발급을 시도했다: %v", err)
	}
	if got != "CACHED" {
		t.Fatalf("token=%q — 캐시된 것을 그대로 써야 한다", got)
	}
}

// 형제가 새 토큰을 넣어 뒀으면 401 뒤에도 발급 없이 그것을 집어야 한다.
func TestTokenAfter_picksUpSiblingToken(t *testing.T) {
	store := newMemStore()
	tm := NewTokenManagerWithKey("https://127.0.0.1:1/never", "id", "secret", store, "k")
	store.put(t, "k", "T2")
	tm.memToken = &cachedToken{AccessToken: "T1", ExpiresAt: time.Now().Add(time.Hour)}

	got, err := tm.TokenAfter(context.Background(), "T1")
	if err != nil {
		t.Fatalf("형제 토큰이 있는데 발급을 시도했다: %v", err)
	}
	if got != "T2" {
		t.Fatalf("token=%q, want T2", got)
	}
}

/*
갱신에 실패해도 **아직 만료 전인 토큰이 있으면 요청이 살아야 한다.**

갱신은 만료 5분 전에 시작한다. 그 5분 사이에 발급이 실패할 이유는 여럿이고
(상류 5xx, 네트워크, 락 경합), 전에는 그때 에러를 그대로 올려 **모든 REST 호출이
같이 죽었다**. 하루 한 번의 회전 구간이 곧 장애 구간이 되는 셈이다.
*/
func TestAccessToken_fallsBackToUnexpiredTokenWhenRefreshFails(t *testing.T) {
	store := newMemStore()
	// 발급은 반드시 실패한다(닿을 수 없는 주소).
	tm := NewTokenManagerWithKey("http://127.0.0.1:1", "id", "secret", store, "k")

	// 갱신 여유(5분)는 지났지만 아직 만료 전인 토큰.
	raw, err := json.Marshal(cachedToken{
		AccessToken: "ALMOST_EXPIRED",
		ExpiresAt:   time.Now().Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Set(context.Background(), "k", raw, time.Hour)

	got, err := tm.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("아직 2분 남은 토큰이 있는데 실패했다: %v", err)
	}
	if got != "ALMOST_EXPIRED" {
		t.Fatalf("token=%q — 만료 전 토큰으로 물러서야 한다", got)
	}
}

// 진짜로 만료된 토큰은 물러설 대상이 아니다 — 어차피 401이다.
func TestAccessToken_expiredTokenIsNotAFallback(t *testing.T) {
	store := newMemStore()
	tm := NewTokenManagerWithKey("http://127.0.0.1:1", "id", "secret", store, "k")

	raw, _ := json.Marshal(cachedToken{
		AccessToken: "DEAD",
		ExpiresAt:   time.Now().Add(-time.Minute),
	})
	_ = store.Set(context.Background(), "k", raw, time.Hour)

	if _, err := tm.AccessToken(context.Background()); err == nil {
		t.Fatal("만료된 토큰으로 물러섰다 — 401만 유발한다")
	}
}

// 여유가 남은 토큰은 갱신을 시도조차 하지 않아야 한다(발급은 파괴적이다).
func TestAccessToken_freshTokenSkipsIssue(t *testing.T) {
	store := newMemStore()
	tm := NewTokenManagerWithKey("http://127.0.0.1:1", "id", "secret", store, "k")

	raw, _ := json.Marshal(cachedToken{
		AccessToken: "FRESH",
		ExpiresAt:   time.Now().Add(6 * time.Hour),
	})
	_ = store.Set(context.Background(), "k", raw, time.Hour)

	got, err := tm.AccessToken(context.Background())
	if err != nil || got != "FRESH" {
		t.Fatalf("token=%q err=%v — 캐시를 그대로 써야 한다", got, err)
	}
}

/*
발급 재시도는 **일시적 실패에만** 걸어야 한다.

`invalid_client` 같은 4xx는 자격증명이 틀렸다는 뜻이라 다시 불러도 같은 답이고,
그 사이 `/oauth2/token` 쿼터만 태운다.
*/
func TestIssueWithRetry_doesNotRetryPermanentFailure(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"invalid_client","error_description":"bad secret"}`)
	}))
	t.Cleanup(srv.Close)

	tm := NewTokenManagerWithKey(srv.URL, "id", "secret", newMemStore(), "k")
	if _, _, err := tm.issueWithRetry(context.Background()); err == nil {
		t.Fatal("자격증명 오류가 성공으로 돌아왔다")
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("발급 호출 %d회 — 영구 실패는 한 번만 시도해야 한다", n)
	}
}

// 일시적 실패는 재시도해서 넘겨야 한다.
func TestIssueWithRetry_recoversFromTransientFailure(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "NEW", "token_type": "Bearer", "expires_in": 3600,
		})
	}))
	t.Cleanup(srv.Close)

	tm := NewTokenManagerWithKey(srv.URL, "id", "secret", newMemStore(), "k")
	token, expiresAt, err := tm.issueWithRetry(context.Background())
	if err != nil {
		t.Fatalf("일시적 503을 못 넘겼다: %v", err)
	}
	if token != "NEW" {
		t.Fatalf("token=%q", token)
	}
	// 만료 시각은 **리드타임을 빼지 않은 실제 값**이어야 한다 — 그래야
	// 「갱신할 때가 됐나」와 「아직 쓸 수 있나」를 따로 물을 수 있다.
	if remaining := time.Until(expiresAt); remaining < 59*time.Minute {
		t.Fatalf("남은 시간 %s — expires_in(3600s)이 그대로 반영돼야 한다", remaining)
	}
	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Fatalf("발급 호출 %d회 — 실패 1 + 성공 1이어야 한다", n)
	}
}

// 갱신 시점과 사용 가능 시점이 리드타임만큼 벌어져 있어야 한다.
func TestTokenNeed_refreshWindowIsTheGraceWindow(t *testing.T) {
	now := time.Now()
	inGrace := now.Add(refreshLeadTime / 2) // 갱신할 때는 지났지만 아직 유효

	if tokenFresh.accepts(inGrace, now) {
		t.Fatal("갱신 여유가 없는데 fresh로 통과했다 — 미리 갱신하는 의미가 없다")
	}
	if !tokenUsable.accepts(inGrace, now) {
		t.Fatal("아직 만료 전인데 usable이 거부했다 — 물러설 곳이 사라진다")
	}

	expired := now.Add(-time.Second)
	if tokenFresh.accepts(expired, now) || tokenUsable.accepts(expired, now) {
		t.Fatal("만료된 토큰이 통과했다")
	}
}

/*
경합이 없을 때 발급은 **기다리지 않고** 나가야 한다.

전에는 락을 잡기 **전에** 형제의 갱신을 50ms × 40회 기다렸다. 아무도 갱신 중이
아니면 그 2초를 그냥 태우고 나서야 발급했고, 그동안 사용자 요청이 잡혀 있었다.
기동 직후와 24시간 회전 — 즉 평상시 경로 — 에 매번 붙던 지연이다.

무리를 막는 것은 Redis 락이므로, 못 잡았을 때만 기다리면 보호는 그대로다.
*/
func TestEnsureFresh_uncontendedIssueDoesNotWait(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "T", "token_type": "Bearer", "expires_in": 3600,
		})
	}))
	t.Cleanup(srv.Close)

	tm := NewTokenManagerWithKey(srv.URL, "id", "secret", newMemStore(), "k")

	start := time.Now()
	if _, err := tm.AccessToken(context.Background()); err != nil {
		t.Fatalf("발급 실패: %v", err)
	}
	// 예전 코드는 여기서 최소 2초를 썼다. 넉넉히 잡아도 1초를 넘으면 회귀다.
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("발급에 %s 걸렸다 — 경합이 없는데 기다리고 있다", elapsed)
	}
}
