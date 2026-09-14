package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

/*
Redis가 죽었을 때 **빠르게 포기해야** 폴백이 폴백 노릇을 한다.

이 저장소의 원칙은 「Redis 장애 시 API 500 금지 — MySQL로 떨어진다」(AGENT_STATE
개발 원칙 5)인데, 2026-09-09 실측에서 그 원칙이 지켜지지 않았다. Redis 컨테이너를
내리고 재 보니:

	GET /api/v1/stocks/005930/summary          502  10.000초
	GET /api/v1/quotes/005930/terminal-snapshot 404  10.003초

폴백 자체는 있었다 — 목록·지수·환율·랭킹은 전부 200이었다. 문제는 **떨어지는 데
걸린 시간**이다. 사용자에게 10초 멎은 화면은 500과 다르지 않고, 게다가 그 10초가
nginx의 인내를 넘겨 502가 됐다.

원인이 둘 겹쳤다.

 1. go-redis 기본값이 `DialTimeout 5s` · `MaxRetries 3`이다. 정상일 때 이 다이얼은
    **1밀리초 미만**이라(같은 도커 네트워크) 5초는 장애일 때만 쓰이는 값이다.
 2. **DNS가 실패를 늦춘다.** 컨테이너가 없으면 `redis`라는 이름이 NXDOMAIN이어야
    하는데, 도커 리졸버가 호스트 상위 DNS로 넘기고 **ISP가 와일드카드로 자기 IP를
    돌려준다**(실측 `dial tcp 218.38.137.27:6379: i/o timeout` — quant-port.com을
    가로채던 그 IP다). 그래서 「없는 호스트」가 아니라 「응답 없는 호스트」가 되어
    다이얼이 타임아웃 끝까지 간다.

그래서 타임아웃을 **정상 지연의 500배**로 조인다. 건강할 때는 닿을 수 없는 값이고,
죽었을 때는 즉시 MySQL로 떨어진다. Redis 블로킹 명령(BLPOP·XREAD·Subscribe)은
이 저장소에 없어서 짧은 ReadTimeout이 정상 동작을 깨지 않는다 — 스트림 허브는
전부 인메모리다.
*/
const (
	redisDialTimeout  = 500 * time.Millisecond
	redisReadTimeout  = 500 * time.Millisecond
	redisWriteTimeout = 500 * time.Millisecond
	// 한 번은 다시 시도한다 — 순간적인 연결 재활용 실패까지 장애로 볼 필요는 없다.
	// 기본값 3은 장애일 때 대기를 그만큼 곱한다.
	redisMaxRetries = 1
)

func NewRedis(url string) (*Redis, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	opts.DialTimeout = redisDialTimeout
	opts.ReadTimeout = redisReadTimeout
	opts.WriteTimeout = redisWriteTimeout
	opts.MaxRetries = redisMaxRetries

	client := redis.NewClient(opts)
	// 회로 차단기. 타임아웃만으로는 부족한 이유는 redis_breaker.go 머리글에 —
	// 한 요청이 Redis를 여러 번 부르므로 호출당 비용을 줄여도 합계가 전역 요청
	// 예산을 넘는다. 훅이라 26곳의 호출부를 하나도 고치지 않는다.
	client.AddHook(&redisBreaker{})

	r := &Redis{client: client}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return r, nil
}

func (r *Redis) Client() *redis.Client {
	if r == nil {
		return nil
	}
	return r.client
}

func (r *Redis) Ping(ctx context.Context) error {
	if r == nil || r.client == nil {
		return redis.ErrClosed
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return r.client.Ping(ctx).Err()
}

func (r *Redis) Status(ctx context.Context) string {
	if r == nil || r.client == nil {
		return "unavailable"
	}
	if err := r.Ping(ctx); err != nil {
		return "degraded: " + err.Error()
	}
	return "ok"
}

func (r *Redis) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}
