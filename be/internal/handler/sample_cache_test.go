package handler

import (
	"sync"
	"testing"

	"github.com/enact/quant-port/be/internal/sample"
)

/*
예시 캐시의 유일한 위험은 **불완전한 결과를 굳히는 것**이다.

예시 행은 시세를 못 얻으면 통째로 빠진다(internal/sample/menus.go의
`if quote == nil || quote.CurrentPrice <= 0 { continue }`). Toss가 잠깐 흔들린
순간에 계산한 짧은 목록을 캐싱하면, TTL이 끝날 때까지 모든 방문자가 깨진 첫
화면을 본다.

「짧으면 저장하지 않는다」가 그 방어이고, 이 테스트가 그것을 고정한다.
*/
func TestShouldCacheSample(t *testing.T) {
	cases := []struct {
		name       string
		want, got  int
		wantCached bool
	}{
		{"완전 — 캐싱한다", 6, 6, true},
		{"한 종목 빠짐 — 캐싱하지 않는다", 6, 5, false},
		{"전부 빠짐 — 캐싱하지 않는다", 6, 0, false},
		{"픽스처가 비었으면 판정 불가 — 캐싱하지 않는다", 0, 0, false},
		{"기대보다 많음(있을 수 없지만) — 통과", 6, 7, true},
	}
	for _, c := range cases {
		if got := shouldCacheSample(c.want, c.got); got != c.wantCached {
			t.Fatalf("%s: shouldCacheSample(%d,%d)=%v want %v", c.name, c.want, c.got, got, c.wantCached)
		}
	}
}

// 픽스처가 비어 있으면 캐시 판정 기준이 사라져 아무 결과나 「완전」이 된다.
// 그러면 시세가 전부 빠진 빈 목록을 캐싱하게 된다.
func TestSampleSeedCounts_NotZero(t *testing.T) {
	if sample.WatchlistSeedCount() == 0 {
		t.Fatal("watchlist 픽스처가 비었다 — 캐시 완전성 판정이 무력해진다")
	}
	if sample.HoldingSeedCount() == 0 {
		t.Fatal("holdings 픽스처가 비었다")
	}
}

// 캐시가 비어 있는 순간 방문자가 몰려도 계산은 한 번만 돌아야 한다.
// (배포 직후·TTL 만료 직후에 각자 460쿼리를 돌리면 100명이면 46,000쿼리다.)
func TestSampleFlight_CollapsesConcurrentBuilds(t *testing.T) {
	f := newSampleFlight()
	const key = "sample:test"

	done, owner := f.begin(key)
	if !owner {
		t.Fatal("첫 호출이 소유자가 아니다")
	}

	var wg sync.WaitGroup
	var owners int32
	var mu sync.Mutex
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, isOwner := f.begin(key); isOwner {
				mu.Lock()
				owners++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if owners != 0 {
		t.Fatalf("동시 요청 %d개가 소유자가 됐다 — 계산이 중복된다", owners)
	}

	f.done(key, done)

	// 끝난 뒤에는 다음 요청이 새 소유자가 되어야 한다(TTL 만료 후 재계산).
	if _, isOwner := f.begin(key); !isOwner {
		t.Fatal("빌드가 끝났는데 다음 요청이 소유자가 되지 못했다")
	}
}
