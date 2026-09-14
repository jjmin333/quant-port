package repository

import (
	"context"
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
Redis가 죽어 있는 동안 **매 호출마다 죽어 있음을 다시 확인하지 않는다.**

# 왜 타임아웃만으로는 부족했나

2026-09-09 실측: Redis 컨테이너를 내리자 종목 요약이 **502를 10초 걸려** 돌려줬다.
다이얼 타임아웃을 5초에서 500ms로 줄여도(redis.go 참조) **그대로 10초였다.**

이유는 한 요청이 Redis를 여러 번 부르기 때문이다. 호출당 비용이 10분의 1이 되어도
호출 수만큼 곱해지면 전역 요청 예산(`REQUEST_TIMEOUT_SECONDS`, 기본 10초)을 다시
넘는다. 폴백 코드는 있는데 거기 닿기 전에 요청이 죽는 것이라, 고쳐야 할 것은
「한 번의 실패를 얼마나 빨리 아는가」가 아니라 **「이미 아는 실패를 몇 번 다시
확인하는가」**였다.

# 그래서 회로를 연다

연속 실패가 임계에 닿으면 회로를 열고, 그동안의 명령은 **네트워크를 건드리지 않고
즉시 오류**를 돌려준다. 호출부는 평소의 Redis 오류와 똑같이 보므로 이미 있는
MySQL 폴백이 그대로 탄다 — 26곳의 호출부를 하나도 고치지 않는다. 훅이 클라이언트
한 곳에 붙어 전부를 덮는다.

# 무엇을 실패로 세지 않는가

**`redis.Nil`(키 없음)은 실패가 아니다.** 캐시 미스는 정상 동작이고, 이것을 세면
평소에 회로가 열려 캐시가 통째로 꺼진다. 명령 자체의 오류(잘못된 타입 등)도 세지
않는다 — 서버는 멀쩡히 응답한 것이다. 세는 것은 **연결이 안 되거나 응답이 없는
경우**뿐이다.
*/

// ErrRedisCircuitOpen — 회로가 열려 있어 부르지 않았다. 호출부는 다른 Redis
// 오류와 구분할 필요가 없다(둘 다 「캐시를 못 썼다」이다).
var ErrRedisCircuitOpen = errors.New("redis: circuit open (recent failures)")

const (
	// 연속 실패 이만큼이면 연다. 1이면 순간적인 연결 재활용 실패에도 열려
	// 캐시가 꺼지고, 크면 그만큼 느린 요청이 더 나간다.
	redisBreakerThreshold = 2
	/*
		여는 시간. 이 값이 곧 **장애 복구를 늦게 아는 최대 시간**이다.

		짧게 잡은 이유: Redis가 돌아왔는데도 캐시가 꺼져 있으면 그동안 부하가 전부
		MySQL로 간다. 이 프로젝트의 DB는 1 ECPU라(운영) 그 상태를 길게 끌 수 없다.
		5초면 죽어 있는 동안의 호출은 거의 다 즉시 실패하고, 살아난 뒤에는 늦어도
		5초 안에 캐시가 돌아온다.
	*/
	redisBreakerCooldown = 5 * time.Second
)

type redisBreaker struct {
	mu        sync.Mutex
	failures  int
	openUntil time.Time
	// loggedOpen — 열릴 때 한 번만 로그를 남긴다. 매 호출마다 적으면 장애 중에
	// 로그가 초당 수백 줄이 되어 정작 원인이 묻힌다.
	loggedOpen bool
}

// isOpen reports whether calls should be short-circuited right now.
func (b *redisBreaker) isOpen() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.openUntil.IsZero() || time.Now().After(b.openUntil) {
		return false
	}
	return true
}

func (b *redisBreaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failures == 0 && b.openUntil.IsZero() {
		return
	}
	if b.loggedOpen {
		log.Printf("redis: 회로 닫힘 — 캐시를 다시 쓴다")
	}
	b.failures = 0
	b.openUntil = time.Time{}
	b.loggedOpen = false
}

func (b *redisBreaker) recordFailure(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.failures < redisBreakerThreshold {
		return
	}
	b.openUntil = time.Now().Add(redisBreakerCooldown)
	if !b.loggedOpen {
		log.Printf("redis: 연속 실패 %d회 — %s 동안 회로를 열고 MySQL 폴백으로 간다 (%v)",
			b.failures, redisBreakerCooldown, err)
		b.loggedOpen = true
	}
}

/*
countsAsFailure — 이 오류가 「Redis가 죽었다」를 뜻하는가.

`redis.Nil`은 캐시 미스라 정상이고, 컨텍스트 취소는 **부르는 쪽 사정**이라
Redis 탓이 아니다(사용자가 화면을 닫으면 매번 취소가 난다 — 그걸 세면 붐빌 때
회로가 열린다). 남는 것은 연결 실패·타임아웃이다.
*/
func countsAsFailure(err error) bool {
	if err == nil || errors.Is(err, redis.Nil) {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		// 부르는 쪽 데드라인일 수도 있지만, Redis가 죽었을 때 가장 흔한 모습이기도
		// 하다. 세지 않으면 정작 필요한 순간에 회로가 안 열린다.
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

/*
countsAsHealthy — 이 결과가 **「Redis가 살아 있다」를 증명**하는가.

`countsAsFailure`의 반대가 아니다. 셋으로 갈라야 한다:

	nil · redis.Nil       → 살아 있다      (회로를 닫는다)
	연결 실패 · 타임아웃    → 죽었다        (회로를 연다)
	그 밖(프로토콜·명령 오류) → 알 수 없다   (아무것도 하지 않는다)

셋째 칸이 없으면 2026-09-09 실측에서 본 **진동**이 난다 — Redis가 없을 때 ISP
가로채기 서버가 6379를 받아 주고 Redis가 아닌 응답을 돌려주는데, 그것이 net.Error가
아니라서 「실패가 아니다 = 성공」으로 접히면 회로가 5초마다 닫혔다 열렸다 한다.
그때마다 한 요청이 대기 비용을 다시 문다.

`WRONGTYPE` 같은 진짜 명령 오류도 이 칸이다 — 서버는 멀쩡히 답한 것이지만, 그것을
회복 신호로 쓸 이유도 없다.
*/
func countsAsHealthy(err error) bool {
	return err == nil || errors.Is(err, redis.Nil)
}

func (b *redisBreaker) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if b.isOpen() {
			return nil, ErrRedisCircuitOpen
		}
		conn, err := next(ctx, network, addr)
		if err != nil {
			b.recordFailure(err)
			return nil, err
		}
		/*
			다이얼 **성공은 회로를 닫지 않는다.**

			2026-09-09 실측에서 이것 때문에 회로가 5초마다 「닫힘 → 즉시 재실패」를
			반복했다. Redis가 없을 때 `redis`라는 이름이 ISP 와일드카드 DNS로
			넘어가 실제 웹서버(218.38.137.27)를 가리키는데, **그 서버가 6379 TCP를
			받아 준다.** 그래서 다이얼은 성공하고 Redis 프로토콜에서 실패한다.

			회로를 닫아도 되는 근거는 「연결이 됐다」가 아니라 **「Redis가 명령에
			답했다」**뿐이다. 그래서 닫는 일은 ProcessHook에만 맡긴다.
		*/
		return conn, nil
	}
}

func (b *redisBreaker) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if b.isOpen() {
			// 명령에도 오류를 심어 둔다 — 호출부가 반환값이 아니라 `cmd.Err()`를
			// 보는 자리가 있다.
			cmd.SetErr(ErrRedisCircuitOpen)
			return ErrRedisCircuitOpen
		}
		err := next(ctx, cmd)
		switch {
		case countsAsFailure(err):
			b.recordFailure(err)
		case countsAsHealthy(err):
			b.recordSuccess()
		}
		return err
	}
}

func (b *redisBreaker) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		if b.isOpen() {
			for _, cmd := range cmds {
				cmd.SetErr(ErrRedisCircuitOpen)
			}
			return ErrRedisCircuitOpen
		}
		err := next(ctx, cmds)
		switch {
		case countsAsFailure(err):
			b.recordFailure(err)
		case countsAsHealthy(err):
			b.recordSuccess()
		}
		return err
	}
}
