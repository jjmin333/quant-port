package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/redis/go-redis/v9"
)

/*
형성봉을 스냅샷에서 떼어낸 것에 대한 회귀 시험 (2026-09-03).

지키려는 것은 두 가지다:

 1. **화면이 보는 값은 그대로다.** `loadCandleSnapshot`은 나뉘기 전과 같은 구조체를
    돌려줘야 한다 — 완료봉과 형성봉이 함께 있는 스냅샷.
 2. **2초마다 오가는 바이트가 실제로 작아진다.** 이 변경의 목적이 그것이므로,
    형성봉 갱신이 쓰는 바이트를 스냅샷 크기와 직접 비교한다.
*/

func newFormingTestService(t *testing.T) (*QuotesService, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return &QuotesService{redis: client}, mr
}

func sampleSnapshot(bars int) model.CandleSnapshot {
	completed := make([]model.Candle, 0, bars)
	for i := 0; i < bars; i++ {
		completed = append(completed, model.Candle{
			Time:   time.Date(2026, 9, 3, 9, i, 0, 0, time.UTC).Format(time.RFC3339),
			Open:   1000 + float64(i),
			High:   1010 + float64(i),
			Low:    990 + float64(i),
			Close:  1005 + float64(i),
			Volume: int64(100 + i),
		})
	}
	return model.CandleSnapshot{
		Symbol:    "000660",
		Interval:  model.Interval1m,
		Completed: completed,
		Forming:   &model.Candle{Time: "2026-09-03T14:00:00Z", Open: 1600, High: 1610, Low: 1590, Close: 1600, Volume: 42},
		Rev:       7,
		Source:    "toss",
	}
}

func TestCandleSnapshotRoundTripKeepsForming(t *testing.T) {
	s, _ := newFormingTestService(t)
	ctx := context.Background()
	want := sampleSnapshot(5)

	s.cacheCandleSnapshot(ctx, want)

	got, ok := s.loadCandleSnapshot(ctx, "000660", model.Interval1m)
	if !ok {
		t.Fatal("snapshot not found after write")
	}
	if len(got.Completed) != len(want.Completed) {
		t.Fatalf("completed bars = %d, want %d", len(got.Completed), len(want.Completed))
	}
	if got.Forming == nil {
		t.Fatal("forming bar lost across the split")
	}
	if got.Forming.Close != want.Forming.Close || got.Forming.Time != want.Forming.Time {
		t.Fatalf("forming = %+v, want %+v", *got.Forming, *want.Forming)
	}
	if got.Rev != want.Rev || got.Source != want.Source {
		t.Fatalf("rev/source = %d/%s, want %d/%s", got.Rev, got.Source, want.Rev, want.Source)
	}
}

// 스냅샷 칸에는 형성봉이 남아 있으면 안 된다 — 남아 있으면 봉이 닫힐 때마다
// 두 곳을 맞춰야 하고, 무엇보다 이 변경으로 아낀 바이트가 도로 들어간다.
func TestCachedSnapshotBodyCarriesNoFormingBar(t *testing.T) {
	s, mr := newFormingTestService(t)
	s.cacheCandleSnapshot(context.Background(), sampleSnapshot(5))

	body, err := mr.Get("md:snap:000660:1m")
	if err != nil {
		t.Fatalf("snapshot key missing: %v", err)
	}
	if strings.Contains(body, "\"forming\"") {
		t.Fatalf("snapshot body still embeds the forming bar: %s", body)
	}
	if _, err := mr.Get("md:form:000660:1m"); err != nil {
		t.Fatalf("forming key missing: %v", err)
	}
}

/*
이 시험이 이 변경의 **이유**다.

형성봉 갱신 한 번이 만지는 바이트가 스냅샷 전체보다 훨씬 작아야 한다. 120봉짜리
1분봉은 실제 화면이 그리는 분량이고, 그 왕복이 2초마다 간격 다섯 개씩 돌고 있었다.
*/
func TestFormingRefreshTouchesFarLessThanTheSnapshot(t *testing.T) {
	s, mr := newFormingTestService(t)
	ctx := context.Background()
	s.cacheCandleSnapshot(ctx, sampleSnapshot(120))

	body, _ := mr.Get("md:snap:000660:1m")
	forming, _ := mr.Get("md:form:000660:1m")

	if len(forming)*10 > len(body) {
		t.Fatalf("forming payload %dB is not small next to the snapshot %dB", len(forming), len(body))
	}
}

// 형성봉만 갱신하면 스냅샷 칸은 **한 바이트도** 바뀌지 않는다.
func TestStoreFormingCandleLeavesSnapshotUntouched(t *testing.T) {
	s, mr := newFormingTestService(t)
	ctx := context.Background()
	s.cacheCandleSnapshot(ctx, sampleSnapshot(20))
	before, _ := mr.Get("md:snap:000660:1m")

	s.storeFormingCandle(ctx, "000660", model.Interval1m, &model.Candle{
		Time: "2026-09-03T14:00:00Z", Open: 1600, High: 1650, Low: 1590, Close: 1648, Volume: 99,
	})

	after, _ := mr.Get("md:snap:000660:1m")
	if before != after {
		t.Fatal("snapshot body changed on a forming-only update")
	}
	got, ok := s.loadCandleSnapshot(ctx, "000660", model.Interval1m)
	if !ok || got.Forming == nil {
		t.Fatal("merged snapshot lost the forming bar")
	}
	if got.Forming.Close != 1648 || got.Forming.High != 1650 {
		t.Fatalf("forming = %+v, want close 1648 / high 1650", *got.Forming)
	}
}

/*
TTL은 **스냅샷이 정한 값을 유지한다**.

`storeFormingCandle`이 자기 TTL을 새로 정하면 일봉의 형성봉만 90초 만에 사라진다 —
일봉 스냅샷은 1시간을 산다.
*/
func TestStoreFormingCandleKeepsTTL(t *testing.T) {
	s, mr := newFormingTestService(t)
	ctx := context.Background()
	snap := sampleSnapshot(3)
	snap.Interval = model.Interval1d
	snap.Source = "db" // 일봉 = 1시간 TTL
	s.cacheCandleSnapshot(ctx, snap)

	want := time.Duration(model.MarketDataCandleHistCacheSec) * time.Second
	if got := mr.TTL("md:form:000660:1d"); got != want {
		t.Fatalf("forming TTL after snapshot write = %v, want %v", got, want)
	}

	mr.FastForward(30 * time.Second)
	s.storeFormingCandle(ctx, "000660", model.Interval1d, &model.Candle{Time: "x", Close: 1})

	if got := mr.TTL("md:form:000660:1d"); got != want-30*time.Second {
		t.Fatalf("forming TTL after refresh = %v, want %v", got, want-30*time.Second)
	}
}

/*
없는 칸에는 쓰지 않는다(`XX`).

`KEEPTTL`은 없는 키에 쓰면 **만료 없는 키**를 만든다. 스냅샷이 만료된 뒤 형성봉만
영원히 남으면 그건 새는 자리다.
*/
func TestStoreFormingCandleDoesNotResurrectAnExpiredKey(t *testing.T) {
	s, mr := newFormingTestService(t)
	s.storeFormingCandle(context.Background(), "000660", model.Interval1m, &model.Candle{Time: "x", Close: 1})

	if _, err := mr.Get("md:form:000660:1m"); err == nil {
		t.Fatal("forming key was created without a snapshot behind it")
	}
}

// 예전 형식(형성봉이 스냅샷 안에 박힌 캐시)도 그대로 읽힌다 — 배포 직후 90초 동안
// 살아 있는 키들이다.
func TestLoadCandleSnapshotReadsLegacyEmbeddedForming(t *testing.T) {
	s, mr := newFormingTestService(t)
	legacy := `{"symbol":"000660","interval":"1m","completed":[{"time":"t","open":1,"high":1,"low":1,"close":1,"volume":1}],` +
		`"forming":{"time":"2026-09-03T14:00:00Z","open":1600,"high":1610,"low":1590,"close":1607,"volume":5},"rev":3,"source":"toss"}`
	_ = mr.Set("md:snap:000660:1m", legacy)

	got, ok := s.loadCandleSnapshot(context.Background(), "000660", model.Interval1m)
	if !ok {
		t.Fatal("legacy snapshot not readable")
	}
	if got.Forming == nil || got.Forming.Close != 1607 {
		t.Fatalf("legacy forming bar lost: %+v", got.Forming)
	}
}

/*
장이 닫혀 형성봉이 사라지면 **다음 스냅샷 쓰기가 그것을 지운다.**

형성봉 칸을 지우지 않고 두면, 예전 형성봉이 다음 조회에서 다시 붙어 마감된 차트에
움직이는 봉 하나가 남는다.
*/
func TestSnapshotWriteWithoutFormingClearsTheFormingKey(t *testing.T) {
	s, _ := newFormingTestService(t)
	ctx := context.Background()
	s.cacheCandleSnapshot(ctx, sampleSnapshot(3))

	closed := sampleSnapshot(4)
	closed.Forming = nil
	s.cacheCandleSnapshot(ctx, closed)

	got, ok := s.loadCandleSnapshot(ctx, "000660", model.Interval1m)
	if !ok {
		t.Fatal("snapshot missing")
	}
	if got.Forming != nil {
		t.Fatalf("stale forming bar survived the close: %+v", *got.Forming)
	}
}

// redis가 없으면 조용히 실패한다 — 캐시는 거들 뿐이라 없다고 터지면 안 된다.
func TestFormingHelpersTolerateNilRedis(t *testing.T) {
	s := &QuotesService{}
	ctx := context.Background()
	s.cacheCandleSnapshot(ctx, sampleSnapshot(2))
	s.storeFormingCandle(ctx, "000660", model.Interval1m, &model.Candle{Time: "x"})
	if _, ok := s.loadFormingCandle(ctx, "000660", model.Interval1m); ok {
		t.Fatal("loadFormingCandle reported a hit with no redis")
	}
	if _, ok := s.loadCandleSnapshot(ctx, "000660", model.Interval1m); ok {
		t.Fatal("loadCandleSnapshot reported a hit with no redis")
	}
}

/*
`cacheKeyExists` — **있는지만** 묻는 길.

`md:raw:1m:*`는 실측 291KB인데, 「이미 채워져 있나」를 확인하자고 그걸 통째로 받아
길이만 재고 버리던 자리가 둘 있었다(`warmRawCacheFromDB`).
*/
func TestCacheKeyExists(t *testing.T) {
	s, mr := newFormingTestService(t)
	ctx := context.Background()

	if s.cacheKeyExists(ctx, "md:raw:1m:000660") {
		t.Fatal("reported a missing key as present")
	}
	_ = mr.Set("md:raw:1m:000660", `[{"time":"t","close":1}]`)
	if !s.cacheKeyExists(ctx, "md:raw:1m:000660") {
		t.Fatal("reported a present key as missing")
	}

	// redis가 없으면 「없다」고 답한다 — 그래야 부르는 쪽이 DB에서 채우는 길로 간다.
	if (&QuotesService{}).cacheKeyExists(ctx, "md:raw:1m:000660") {
		t.Fatal("reported present with no redis")
	}
}
