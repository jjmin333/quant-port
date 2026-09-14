package repository

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type fakeNetErr struct{}

func (fakeNetErr) Error() string   { return "dial tcp: i/o timeout" }
func (fakeNetErr) Timeout() bool   { return true }
func (fakeNetErr) Temporary() bool { return true }

var _ net.Error = fakeNetErr{}

/*
**캐시 미스로 회로가 열리면 안 된다.**

이 테스트가 이 파일에서 가장 중요하다. `redis.Nil`은 평소에 끊임없이 나오는 정상
동작이고, 그것을 실패로 세면 Redis가 멀쩡한데도 회로가 열려 **캐시가 통째로 꺼진다.**
그러면 부하가 전부 MySQL로 가는데, 운영 DB는 1 ECPU다.
*/
func TestBreakerIgnoresCacheMiss(t *testing.T) {
	b := &redisBreaker{}
	for i := 0; i < 50; i++ {
		if countsAsFailure(redis.Nil) {
			t.Fatalf("redis.Nil 을 실패로 셌다 — 캐시 미스는 정상이다")
		}
		b.recordSuccess()
	}
	if b.isOpen() {
		t.Fatalf("캐시 미스만 있었는데 회로가 열렸다")
	}
}

// 사용자가 화면을 닫으면 컨텍스트가 취소된다. 그것은 **부르는 쪽 사정**이라
// Redis 탓이 아니다 — 세면 붐빌 때 회로가 열린다.
func TestBreakerIgnoresCallerCancellation(t *testing.T) {
	if countsAsFailure(context.Canceled) {
		t.Fatalf("context.Canceled 를 Redis 실패로 셌다")
	}
}

// 연결 실패는 임계까지 세고 나서 연다 — 한 번에 열면 순간적인 연결 재활용 실패로도
// 캐시가 꺼진다.
func TestBreakerOpensAfterThreshold(t *testing.T) {
	b := &redisBreaker{}
	netErr := fakeNetErr{}

	if !countsAsFailure(netErr) {
		t.Fatalf("네트워크 오류를 실패로 안 셌다")
	}

	for i := 1; i < redisBreakerThreshold; i++ {
		b.recordFailure(netErr)
		if b.isOpen() {
			t.Fatalf("실패 %d회(임계 %d)에 회로가 열렸다 — 너무 예민하다", i, redisBreakerThreshold)
		}
	}
	b.recordFailure(netErr)
	if !b.isOpen() {
		t.Fatalf("실패 %d회인데 회로가 안 열렸다", redisBreakerThreshold)
	}
}

// 열린 회로는 쿨다운이 지나면 스스로 닫힌다 — Redis가 돌아왔는데 캐시가 계속
// 꺼져 있으면 부하가 MySQL에 남는다.
func TestBreakerClosesAfterCooldown(t *testing.T) {
	b := &redisBreaker{}
	for i := 0; i < redisBreakerThreshold; i++ {
		b.recordFailure(fakeNetErr{})
	}
	if !b.isOpen() {
		t.Fatalf("회로가 안 열렸다")
	}

	// 쿨다운이 지난 것으로 만든다.
	b.mu.Lock()
	b.openUntil = time.Now().Add(-time.Millisecond)
	b.mu.Unlock()

	if b.isOpen() {
		t.Fatalf("쿨다운이 지났는데 여전히 열려 있다")
	}
}

// 성공 한 번이면 실패 카운트가 초기화된다 — 드문드문한 실패가 쌓여서 열리면 안 된다.
func TestBreakerResetsOnSuccess(t *testing.T) {
	b := &redisBreaker{}
	b.recordFailure(fakeNetErr{})
	b.recordSuccess()
	b.recordFailure(fakeNetErr{})
	if b.isOpen() {
		t.Fatalf("사이에 성공이 있었는데 회로가 열렸다")
	}
}

/*
회로가 열려 있으면 명령은 **네트워크를 건드리지 않고 즉시** 실패해야 한다.

이것이 10초를 없애는 실제 동작이다. `next`가 불리면 안 된다.
*/
func TestBreakerShortCircuitsWithoutCallingNext(t *testing.T) {
	b := &redisBreaker{}
	for i := 0; i < redisBreakerThreshold; i++ {
		b.recordFailure(fakeNetErr{})
	}

	called := false
	hook := b.ProcessHook(func(_ context.Context, _ redis.Cmder) error {
		called = true
		return nil
	})

	cmd := redis.NewStringCmd(context.Background(), "get", "k")
	start := time.Now()
	err := hook(context.Background(), cmd)
	elapsed := time.Since(start)

	if called {
		t.Fatalf("회로가 열렸는데 실제 명령이 나갔다")
	}
	if !errors.Is(err, ErrRedisCircuitOpen) {
		t.Fatalf("오류가 ErrRedisCircuitOpen 이 아니다: %v", err)
	}
	if !errors.Is(cmd.Err(), ErrRedisCircuitOpen) {
		t.Fatalf("cmd.Err() 에 오류가 안 심겼다: %v", cmd.Err())
	}
	if elapsed > 50*time.Millisecond {
		t.Fatalf("즉시 실패해야 하는데 %v 걸렸다", elapsed)
	}
}

// 회로가 닫혀 있으면 훅은 그냥 지나가야 한다 — 평소 동작을 방해하지 않는다.
func TestBreakerPassesThroughWhenClosed(t *testing.T) {
	b := &redisBreaker{}
	called := false
	hook := b.ProcessHook(func(_ context.Context, _ redis.Cmder) error {
		called = true
		return nil
	})
	cmd := redis.NewStringCmd(context.Background(), "get", "k")
	if err := hook(context.Background(), cmd); err != nil {
		t.Fatalf("정상인데 오류: %v", err)
	}
	if !called {
		t.Fatalf("정상인데 명령이 안 나갔다")
	}
}

/*
**다이얼 성공만으로는 회로가 닫히면 안 된다.**

2026-09-09 실측: Redis가 없을 때 `redis` 이름이 ISP 와일드카드 DNS로 넘어가 실제
웹서버를 가리키는데 **그 서버가 6379 TCP를 받아 준다.** 다이얼 성공을 회복으로 보면
회로가 5초마다 열렸다 닫혔다를 반복하고, 그때마다 한 요청이 대기 비용을 다시 문다.
*/
func TestBreakerDialSuccessDoesNotCloseCircuit(t *testing.T) {
	b := &redisBreaker{}
	for i := 0; i < redisBreakerThreshold; i++ {
		b.recordFailure(fakeNetErr{})
	}
	// 쿨다운이 지나 한 번 통과시키는 상황을 만든다.
	b.mu.Lock()
	b.openUntil = time.Now().Add(-time.Millisecond)
	b.mu.Unlock()

	// 다이얼은 성공한다(가로채기 서버가 받아 준다).
	hook := b.DialHook(func(_ context.Context, _, _ string) (net.Conn, error) {
		return nil, nil
	})
	if _, err := hook(context.Background(), "tcp", "redis:6379"); err != nil {
		t.Fatalf("다이얼이 실패했다: %v", err)
	}

	// 명령이 성공하기 전까지는 실패 이력이 남아 있어야 한다 — 다이얼 성공이
	// 카운터를 지웠다면 다음 실패 한 번으로는 회로가 다시 안 열린다.
	b.recordFailure(fakeNetErr{})
	if !b.isOpen() {
		t.Fatalf("다이얼 성공이 실패 이력을 지웠다 — 회로가 다시 열려야 한다")
	}
}

// 반대로, **명령이 성공하면** 회로는 닫힌다. 그것이 진짜 회복 신호다.
func TestBreakerClosesOnCommandSuccess(t *testing.T) {
	b := &redisBreaker{}
	b.recordFailure(fakeNetErr{})
	b.recordSuccess()

	b.recordFailure(fakeNetErr{})
	if b.isOpen() {
		t.Fatalf("명령 성공으로 초기화됐어야 하는데 회로가 열렸다")
	}
}

/*
**프로토콜 오류는 회복 신호가 아니다** — 「실패의 반대 = 성공」으로 접으면 안 된다.

이것이 2026-09-09 진동의 직접 원인이었다. ISP 가로채기 서버는 6379를 받아 준 뒤
Redis가 아닌 응답을 돌려주는데, 그 오류는 net.Error가 아니라서 `countsAsFailure`가
false다. 그것을 성공으로 접으면 회로가 5초마다 닫혔다 열렸다 한다.
*/
func TestBreakerTreatsProtocolErrorAsNeitherSuccessNorFailure(t *testing.T) {
	protoErr := errors.New("ERR unknown command 'GET'")

	if countsAsFailure(protoErr) {
		t.Fatalf("프로토콜 오류를 연결 실패로 셌다")
	}
	if countsAsHealthy(protoErr) {
		t.Fatalf("프로토콜 오류를 회복 신호로 셌다 — 회로가 진동한다")
	}

	// 열린 회로가 프로토콜 오류로 닫히면 안 된다.
	b := &redisBreaker{}
	for i := 0; i < redisBreakerThreshold; i++ {
		b.recordFailure(fakeNetErr{})
	}
	b.mu.Lock()
	b.openUntil = time.Now().Add(-time.Millisecond) // 쿨다운 만료
	b.mu.Unlock()

	hook := b.ProcessHook(func(_ context.Context, _ redis.Cmder) error { return protoErr })
	cmd := redis.NewStringCmd(context.Background(), "get", "k")
	_ = hook(context.Background(), cmd)

	// 실패 이력이 남아 있어야 한다 — 한 번 더 실패하면 바로 다시 열린다.
	b.recordFailure(fakeNetErr{})
	if !b.isOpen() {
		t.Fatalf("프로토콜 오류가 실패 이력을 지웠다")
	}
}

// 캐시 미스(redis.Nil)는 **회복 신호가 맞다** — 서버가 답한 것이다.
func TestBreakerCacheMissIsHealthySignal(t *testing.T) {
	if !countsAsHealthy(redis.Nil) {
		t.Fatalf("redis.Nil 은 서버가 답한 것이므로 회복 신호여야 한다")
	}
	if !countsAsHealthy(nil) {
		t.Fatalf("nil 은 회복 신호여야 한다")
	}
}
