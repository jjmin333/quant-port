package handler

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
)

/*
예시 화면 캐시.

예시는 **비로그인 방문자의 첫 화면**이라 우리 서비스에서 가장 자주 열리는
화면인데, 동시에 가장 비싼 화면이었다 — 실측(2026-08-23) `/sample/watchlist`
476쿼리 + `/sample/holdings` 147쿼리 = **방문자 1명당 약 460~623 DB 쿼리**.
nginx는 `proxy_cache off`이고 핸들러에는 `Cache-Control` 헤더뿐이라 서버측
캐시가 없었다. DB 전체 부하가 241쿼리/초인 상태에서 동시 방문 10명이면 25배가
된다.

픽스처가 6종목·5종목 고정이라 **모든 방문자가 같은 응답을 본다**. 캐시 키가
하나뿐이므로 캐싱 효율이 최대다 — 방문자가 몇이든 TTL당 한 번만 계산한다.

# 빈 값을 캐싱하지 않는다 (이 파일의 존재 이유)

예시 행은 시세를 못 얻으면 **통째로 빠진다** — `internal/sample/menus.go`의

	if quote == nil || quote.CurrentPrice <= 0 { continue }

Toss가 잠깐 흔들리거나 장 시작 직후 시세가 아직 안 붙은 순간에 계산하면 목록이
짧거나 비어서 나온다. 그것을 캐싱하면 **그 뒤 TTL이 끝날 때까지 모든 방문자가
깨진 예시 화면을 본다.** 첫 화면이라 그대로 이탈이다.

그래서 캐싱 조건은 「픽스처 행이 **전부** 살아 있을 것」이다. 짧으면 그대로
응답하되 캐시에는 넣지 않아, 다음 요청이 다시 시도한다. 최악의 경우 캐시가 안
붙어 매번 계산하는 것뿐 — 지금과 같고, 틀린 화면을 굳히지는 않는다.

Redis가 죽어 있어도 같다. 읽기 실패는 미스로, 쓰기 실패는 로그만 남기고
넘어간다 — 캐시는 가속 장치이지 정합성의 근거가 아니다.
*/
const sampleCacheTTL = 60 * time.Second

// sampleBuildWait bounds how long a waiter blocks for the in-flight builder.
// 넘기면 스스로 계산한다 — 기다리다 타임아웃으로 빈 화면을 주는 것보다 낫다.
const sampleBuildWait = 5 * time.Second

// sampleFlight collapses concurrent rebuilds of the same key.
//
// 캐시가 비어 있는 순간에 방문자가 몰리면(배포 직후, TTL 만료 직후) 각자
// 460쿼리를 돌린다 — 100명이면 46,000쿼리다. 소유자 하나만 계산하고 나머지는
// 그 결과를 기다린다.
type sampleFlight struct {
	mu sync.Mutex
	in map[string]chan struct{}
}

func newSampleFlight() *sampleFlight {
	return &sampleFlight{in: make(map[string]chan struct{})}
}

// begin returns (done, true) when the caller owns the build and must close done,
// or (existing, false) when a build is already in flight.
func (f *sampleFlight) begin(key string) (chan struct{}, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ch, ok := f.in[key]; ok {
		return ch, false
	}
	ch := make(chan struct{})
	f.in[key] = ch
	return ch, true
}

func (f *sampleFlight) done(key string, ch chan struct{}) {
	f.mu.Lock()
	delete(f.in, key)
	f.mu.Unlock()
	close(ch)
}

/*
sampleCached — 캐시에서 읽고, 없으면 만들고, **완전할 때만** 저장한다.

want는 픽스처 행 수다. 결과가 그보다 짧으면 시세가 덜 붙은 상태이므로 저장하지
않는다.
*/
func sampleCached[T any](
	ctx context.Context,
	rdb *repository.Redis,
	flight *sampleFlight,
	key string,
	want int,
	build func() []T,
) []T {
	if rdb == nil || rdb.Client() == nil {
		return build()
	}
	client := rdb.Client()

	if out, ok := sampleCacheGet[T](ctx, rdb, key); ok {
		return out
	}

	done, owner := flight.begin(key)
	if !owner {
		// 다른 요청이 만들고 있다. 끝나면 그 결과가 캐시에 있다.
		timer := time.NewTimer(sampleBuildWait)
		defer timer.Stop()
		select {
		case <-done:
			if out, ok := sampleCacheGet[T](ctx, rdb, key); ok {
				return out
			}
		case <-timer.C:
		case <-ctx.Done():
			return nil
		}
		// 소유자가 불완전한 결과를 얻어 저장하지 않았거나 늦었다 — 직접 만든다.
		return build()
	}
	defer flight.done(key, done)

	out := build()

	// **불완전하면 캐싱하지 않는다.** 위 헤더 참조.
	if !shouldCacheSample(want, len(out)) {
		return out
	}
	blob, err := json.Marshal(out)
	if err != nil {
		return out
	}
	if err := client.Set(ctx, key, blob, sampleCacheTTL).Err(); err != nil {
		log.Printf("sample cache: set %s: %v", key, err)
	}
	return out
}

/*
shouldCacheSample — 이 결과를 캐시에 넣어도 되는가.

`want`는 픽스처 행 수, `got`은 실제로 만들어진 행 수다. 시세를 못 얻은 종목은
행이 통째로 빠지므로 got < want는 「시장 데이터가 덜 붙은 상태」를 뜻한다.
그것을 저장하면 TTL이 끝날 때까지 모든 방문자가 깨진 예시 화면을 본다.

got > want는 있을 수 없지만(픽스처보다 많이 만들어질 수 없다) 방어적으로 통과시킨다.
*/
func shouldCacheSample(want, got int) bool {
	return want > 0 && got >= want
}

func sampleCacheGet[T any](ctx context.Context, rdb *repository.Redis, key string) ([]T, bool) {
	blob, err := rdb.Client().Get(ctx, key).Bytes()
	if err != nil || len(blob) == 0 {
		return nil, false
	}
	var out []T
	if err := json.Unmarshal(blob, &out); err != nil {
		// 저장 형식이 바뀌었거나 깨졌다 — 미스로 처리하고 새로 만든다.
		return nil, false
	}
	if len(out) == 0 {
		// 빈 배열이 들어가 있으면 신뢰하지 않는다. 애초에 저장하지 않지만,
		// 옛 배포가 남긴 값이 있을 수 있다.
		return nil, false
	}
	return out, true
}
