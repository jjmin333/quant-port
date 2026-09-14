package repository

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
Redis 타임아웃은 **기본값보다 훨씬 짧아야** 한다.

이 테스트가 지키는 것은 성능이 아니라 **폴백의 의미**다. 2026-09-09 실측에서
Redis를 내리자 종목 요약이 502를, 터미널 스냅샷이 404를 **각각 10초 걸려서**
돌려줬다. 폴백 코드는 이미 있었는데 거기까지 가는 데 10초가 걸린 것이다 —
사용자에게 10초 멎은 화면은 500과 다르지 않다.

기본값(dial 5s · MaxRetries 3)으로 돌아가면 그 10초가 그대로 돌아온다. 그래서
숫자 자체를 고정한다. 근거는 redis.go 머리글에.
*/
func TestRedisTimeoutsAreAggressive(t *testing.T) {
	// ParseURL 은 타임아웃을 0으로 둔다 — go-redis 가 NewClient 안에서 기본값으로
	// 채우기 때문이다. 그래서 「기본값이 얼마인지」는 클라이언트를 만들어 봐야 안다.
	// (NewRedis 자체는 실제 서버에 Ping 하므로 단위 테스트에서 부를 수 없다.
	//  NewClient 는 연결하지 않으므로 여기서 안전하다.)
	base, err := redis.ParseURL("redis://127.0.0.1:6379/0")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	defaults := redis.NewClient(base).Options()
	t.Cleanup(func() { _ = redis.NewClient(base).Close() })

	// 기본값이 우리가 고른 값보다 크다는 것부터 확인한다 — 기본값이 언젠가
	// 짧아지면 이 조정은 의미가 없어지고, 그때 이 테스트가 알려준다.
	if defaults.DialTimeout <= redisDialTimeout {
		t.Errorf("go-redis 기본 DialTimeout(%v)이 우리 값(%v)보다 크지 않다 — 조정이 무의미해졌다",
			defaults.DialTimeout, redisDialTimeout)
	}
	if defaults.MaxRetries <= redisMaxRetries {
		t.Errorf("go-redis 기본 MaxRetries(%d)가 우리 값(%d)보다 크지 않다",
			defaults.MaxRetries, redisMaxRetries)
	}

	opts, err := redis.ParseURL("redis://127.0.0.1:6379/0")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	opts.DialTimeout = redisDialTimeout
	opts.ReadTimeout = redisReadTimeout
	opts.WriteTimeout = redisWriteTimeout
	opts.MaxRetries = redisMaxRetries

	// 정상 다이얼은 같은 도커 네트워크에서 1ms 미만이다. 1초를 넘으면 「빠르게
	// 포기한다」가 아니고, nginx 인내를 다시 넘길 여지가 생긴다.
	if opts.DialTimeout > time.Second {
		t.Errorf("DialTimeout %v — 1초를 넘으면 장애 시 화면이 멎는다", opts.DialTimeout)
	}
	if opts.ReadTimeout > time.Second {
		t.Errorf("ReadTimeout %v — 1초를 넘으면 장애 시 화면이 멎는다", opts.ReadTimeout)
	}
	if opts.WriteTimeout > time.Second {
		t.Errorf("WriteTimeout %v", opts.WriteTimeout)
	}
	if opts.MaxRetries > 1 {
		t.Errorf("MaxRetries %d — 재시도는 장애일 때 대기를 그만큼 곱한다", opts.MaxRetries)
	}

	// 최악의 경우(다이얼 타임아웃 × 시도 횟수)가 nginx가 참아 주는 시간 안에
	// 확실히 들어와야 한다. 실측에서 502가 난 지점이 10초였다.
	worst := opts.DialTimeout * time.Duration(opts.MaxRetries+1)
	if worst >= 5*time.Second {
		t.Errorf("최악 대기 %v — 폴백이 늦어 502가 다시 난다", worst)
	}
}
