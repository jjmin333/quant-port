package service

import (
	"math/rand"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

/*
계층 저장(1m→5m→10m→30m)의 전제는 하나다: **접기가 결합법칙을 만족한다.**

조회 경로가 저장된 5분봉을 30분봉으로 다시 접기 때문에, 그 결과가 1분봉에서
직접 접은 것과 다르면 화면의 봉이 보관 기간에 따라 달라진다 — 같은 날짜의 같은
차트가 어제와 오늘 다르게 보인다는 뜻이다.

`truncateCandleTime`이 KST 자정 기준으로 (hour*60+minute)/N*N 을 쓰므로 N이
서로 나눠떨어지는 한 성립해야 하지만, 그건 코드를 읽은 결론이지 실측이 아니다.
아래 테스트가 그 결론을 고정한다.
*/

// tieringSeries builds 1m bars with irregular OHLCV and optional gaps so
// partial buckets are exercised too.
func tieringSeries(t *testing.T, start string, count int, keep func(i int) bool) []model.Candle {
	t.Helper()
	t0 := parseQuoteCandleTime(start)
	if t0.IsZero() {
		t.Fatalf("bad start %q", start)
	}
	rng := rand.New(rand.NewSource(42))
	out := make([]model.Candle, 0, count)
	for i := 0; i < count; i++ {
		if keep != nil && !keep(i) {
			continue
		}
		ts := t0.Add(time.Duration(i) * time.Minute)
		base := 100 + rng.Float64()*50
		out = append(out, model.Candle{
			Time:   formatCandleTime(ts, model.Interval1m),
			Open:   base,
			High:   base + rng.Float64()*5,
			Low:    base - rng.Float64()*5,
			Close:  base + rng.Float64()*2 - 1,
			Volume: int64(rng.Intn(10_000) + 1),
		})
	}
	return out
}

func assertSameBars(t *testing.T, label string, got, want []model.Candle) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len=%d want %d", label, len(got), len(want))
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Time != w.Time || g.Open != w.Open || g.High != w.High ||
			g.Low != w.Low || g.Close != w.Close || g.Volume != w.Volume {
			t.Fatalf("%s: bar %d\n got  %+v\n want %+v", label, i, g, w)
		}
	}
}

// 1m→5m→10m→30m 캐스케이드가 1m에서 직접 접은 것과 같아야 한다.
func TestAggregateCandles_CascadeMatchesDirect(t *testing.T) {
	raw := tieringSeries(t, "2026-03-25T09:00:00+09:00", 8*60, nil)

	steps := []model.CandleInterval{
		model.Interval5m, model.Interval10m, model.Interval30m,
	}

	cascaded := raw
	for _, iv := range steps {
		cascaded = aggregateCandles(cascaded, iv)
		direct := aggregateCandles(raw, iv)
		assertSameBars(t, "cascade->"+string(iv), cascaded, direct)
	}
}

// 구멍이 있는(부분 버킷) 계열에서도 결합법칙이 유지되어야 한다.
// 롤업 배치는 완전한 버킷만 접지 않는다 — 부분 버킷도 접고 src_bars에 개수를
// 남기므로, 부분 버킷의 재접기가 직접 접기와 같아야 한다.
func TestAggregateCandles_CascadeMatchesDirect_WithGaps(t *testing.T) {
	// 7분마다 한 칸씩 빠뜨려 5·10·30·60 어느 버킷에도 딱 맞지 않는 구멍을 만든다.
	raw := tieringSeries(t, "2026-03-25T09:00:00+09:00", 8*60, func(i int) bool {
		return i%7 != 3
	})

	for _, iv := range []model.CandleInterval{
		model.Interval5m, model.Interval10m, model.Interval30m,
	} {
		direct := aggregateCandles(raw, iv)
		via5 := aggregateCandles(aggregateCandles(raw, model.Interval5m), iv)
		assertSameBars(t, "gaps 5m->"+string(iv), via5, direct)
	}
}

// 자정을 넘는 구간 — 버킷이 KST 자정 기준이므로 날짜 경계에서 어긋나면 안 된다.
// 미국장 1분봉은 KST로 22:30~05:00이라 이 경로를 매일 지난다.
func TestAggregateCandles_CascadeAcrossMidnightKST(t *testing.T) {
	raw := tieringSeries(t, "2026-03-25T22:00:00+09:00", 8*60, nil)

	for _, iv := range []model.CandleInterval{
		model.Interval10m, model.Interval30m,
	} {
		direct := aggregateCandles(raw, iv)
		via5 := aggregateCandles(aggregateCandles(raw, model.Interval5m), iv)
		assertSameBars(t, "midnight 5m->"+string(iv), via5, direct)

		via10 := aggregateCandles(aggregateCandles(raw, model.Interval10m), iv)
		if iv != model.Interval10m {
			assertSameBars(t, "midnight 10m->"+string(iv), via10, direct)
		}
	}
}

// 거래량은 접어도 총합이 보존되어야 한다 — 롤업이 거래량을 잃으면 되돌릴 수 없다.
func TestAggregateCandles_VolumePreserved(t *testing.T) {
	raw := tieringSeries(t, "2026-03-25T09:00:00+09:00", 6*60, func(i int) bool {
		return i%11 != 5
	})
	var want int64
	for _, c := range raw {
		want += c.Volume
	}

	agg := raw
	for _, iv := range []model.CandleInterval{
		model.Interval5m, model.Interval10m, model.Interval30m,
	} {
		agg = aggregateCandles(agg, iv)
		var got int64
		for _, c := range agg {
			got += c.Volume
		}
		if got != want {
			t.Fatalf("%s: volume=%d want %d", iv, got, want)
		}
	}
}

/*
같은 버킷을 두 계층이 덮을 때 **더 완전한 쪽**이 이겨야 한다.

실측 회귀(2026-08-22, 005930 08-19 16:10 버킷): 롤업이 만든 5분봉은 1분봉 5개를
모두 담아 거래량 255,170으로 정확했는데, 그 뒤 Toss 백필이 같은 구간에 1분봉을
되넣었다. 조회가 「고운 계층 우선」으로 골라 되넣어진 1분봉 하나(18,402주)로
완전한 5분봉을 덮어썼다.

Toss 백필은 사용자가 과거를 스크롤할 때마다 일어나므로 예외 상황이 아니다.
*/
func TestSupersedes_PrefersMoreCompleteBar(t *testing.T) {
	rolled5m := model.Candle{Time: "2026-08-19T16:10:00+09:00", Volume: 255170, SrcBars: 5}
	partial1m := model.Candle{Time: "2026-08-19T16:10:00+09:00", Volume: 18402, SrcBars: 1}

	if supersedes(partial1m, rolled5m) {
		t.Fatal("부분 1분봉이 완전한 5분봉을 덮어썼다 — 거래량이 무너진다")
	}
	if !supersedes(rolled5m, partial1m) {
		t.Fatal("완전한 봉이 부분 봉을 대체하지 못했다")
	}
	// 같은 덮개면 먼저 자리잡은 쪽(더 고운 계층)을 유지한다 — 무의미한 교체 방지.
	if supersedes(rolled5m, model.Candle{SrcBars: 5}) {
		t.Fatal("덮개가 같은데 교체했다")
	}
}

// 접힌 봉은 자기 덮개를 이어서 세야 한다. SrcBars가 0으로 남으면
// supersedes가 모든 봉을 「1분봉 하나」로 보고 판단을 못 한다.
func TestAggregateCandles_CarriesCoverage(t *testing.T) {
	raw := tieringSeries(t, "2026-03-25T09:00:00+09:00", 60, nil)
	for _, iv := range []model.CandleInterval{model.Interval5m, model.Interval10m, model.Interval30m} {
		agg := aggregateCandles(raw, iv)
		want := intervalMinutes(iv)
		for _, c := range agg {
			if c.SrcBars != want {
				t.Fatalf("%s bar %s: SrcBars=%d want %d", iv, c.Time, c.SrcBars, want)
			}
		}
	}

	// 이미 접힌 봉을 다시 접으면 덮개가 합산되어야 한다(캐스케이드).
	five := aggregateCandles(raw, model.Interval5m)
	thirty := aggregateCandles(five, model.Interval30m)
	for _, c := range thirty {
		if c.SrcBars != 30 {
			t.Fatalf("cascade 5m->30m bar %s: SrcBars=%d want 30", c.Time, c.SrcBars)
		}
	}
}

/*
보관 한계 밖의 1분봉은 저장하지 않는다.

집계 차트를 깊이 넘기면 Toss 폴백이 페이지당 최대 7,260개의 1분봉을 받아 온다.
그 구간이 60일 밖이면 다음 롤업이 곧바로 지우므로 쓰기 자체가 낭비다 —
실측(2026-08-23)으로 되넣어진 1분봉의 80%가 90~99일 전 구간이었다.

60일 안쪽은 그대로 저장해야 한다. 그것은 낭비가 아니라 해당 계층을 채우는
정상 적재이고, 걸러 버리면 그 구간이 어느 계층에도 안 들어간다.
*/
func TestDropUnretainedIntraday(t *testing.T) {
	now := time.Now().In(model.KST)
	mk := func(d int) model.Candle {
		return model.Candle{Time: formatCandleTime(now.AddDate(0, 0, -d), model.Interval1m)}
	}
	bars := []model.Candle{mk(90), mk(61), mk(59), mk(10), mk(0)}

	got := dropUnretainedIntraday(model.Interval1m, bars)
	if len(got) != 3 {
		t.Fatalf("1m: kept %d bars, want 3 (59·10·0일)", len(got))
	}

	// 일봉은 영구 보관이라 걸러내면 안 된다 — 2009년치까지 의미가 있다.
	if n := len(dropUnretainedIntraday(model.Interval1d, bars)); n != len(bars) {
		t.Fatalf("1d: kept %d bars, want %d — 일봉을 잘라내면 안 된다", n, len(bars))
	}
}

// 「before X」 페이지는 X 이후 봉을 담지 않는다.
// 집계 간격에서 커서가 버킷 한가운데를 가리키면 같은 버킷이 두 페이지에 나뉘어
// 들어간다 — 실측으로 005930 60분봉 2023-03-27T19:00 봉이 두 번 나왔다.
func TestDropCandlesAtOrAfter(t *testing.T) {
	page := CandlesPage{Candles: []model.Candle{
		{Time: "2026-03-25T09:00:00+09:00"},
		{Time: "2026-03-25T10:00:00+09:00"},
		{Time: "2026-03-25T11:00:00+09:00"}, // 커서와 동일 — 버려야 한다
		{Time: "2026-03-25T12:00:00+09:00"}, // 커서보다 미래 — 버려야 한다
	}}

	got := dropCandlesAtOrAfter(page, "2026-03-25T11:00:00+09:00")
	if len(got.Candles) != 2 {
		t.Fatalf("len=%d want 2: %+v", len(got.Candles), got.Candles)
	}
	if got.Candles[1].Time != "2026-03-25T10:00:00+09:00" {
		t.Fatalf("last=%q", got.Candles[1].Time)
	}

	// 입력을 제자리로 자르지 않아야 한다 — 캐시된 스냅샷과 배열을 공유할 수 있다.
	if len(page.Candles) != 4 {
		t.Fatalf("input mutated: len=%d want 4", len(page.Candles))
	}
	// 커서가 없으면 원본 그대로.
	if n := len(dropCandlesAtOrAfter(page, "").Candles); n != 4 {
		t.Fatalf("empty cursor changed the page: len=%d want 4", n)
	}
}

// 계층 해상도는 요청 간격을 나눠떨어뜨려야만 쓸 수 있다.
// 30분봉은 1·5·10·30분봉에서 만들 수 있지만 60분봉에서는 못 만든다 — 이 규칙이
// 깨지면 조회가 굵은 봉을 잘게 쪼갠 것처럼 그려 낸다.
func TestTiersUsableFor(t *testing.T) {
	cases := map[model.CandleInterval][]model.CandleInterval{
		model.Interval5m:  {model.Interval1m, model.Interval5m},
		model.Interval10m: {model.Interval1m, model.Interval5m, model.Interval10m},
		model.Interval30m: {model.Interval1m, model.Interval5m, model.Interval10m, model.Interval30m},
		model.Interval1m:  {model.Interval1m},
		model.Interval1d:  nil,
	}
	for interval, want := range cases {
		got := tiersUsableFor(interval)
		if len(got) != len(want) {
			t.Fatalf("%s: %d tiers, want %d", interval, len(got), len(want))
		}
		for i := range want {
			if got[i].interval != want[i] {
				t.Fatalf("%s tier %d = %s want %s", interval, i, got[i].interval, want[i])
			}
		}
	}
}

// 계층 나이가 단조 증가해야 한다 — 어긋나면 롤업이 데이터를 앞뒤로 옮긴다.
// 그리고 각 계층은 다음 계층의 약수여야 캐스케이드가 정확하다.
func TestIntradayTiers_WellFormed(t *testing.T) {
	for i, tier := range intradayTiers {
		if tier.minutes != intervalMinutes(tier.interval) {
			t.Fatalf("tier %s: minutes=%d but intervalMinutes=%d",
				tier.interval, tier.minutes, intervalMinutes(tier.interval))
		}
		if i == 0 {
			continue
		}
		prev := intradayTiers[i-1]
		if tier.ageDays <= prev.ageDays {
			t.Fatalf("tier %s ageDays=%d must exceed %s ageDays=%d",
				tier.interval, tier.ageDays, prev.interval, prev.ageDays)
		}
		if tier.minutes%prev.minutes != 0 {
			t.Fatalf("tier %s (%dm) is not a multiple of %s (%dm) — cascade would be wrong",
				tier.interval, tier.minutes, prev.interval, prev.minutes)
		}
	}
	last := intradayTiers[len(intradayTiers)-1]
	if last.ageDays != intradayTierMaxAgeDays {
		t.Fatalf("last tier ageDays=%d but intradayTierMaxAgeDays=%d", last.ageDays, intradayTierMaxAgeDays)
	}
}

// 버킷 경계 정렬 — SQL 롤업이 쓰는 벽시계 산술과 같은 결과여야 한다.
// SQL:  DATE(bar_time) + INTERVAL (FLOOR((HOUR*60+MINUTE)/N)*N) MINUTE
// Go :  truncateCandleTime(t, N)
// 둘이 갈라지면 롤업이 만든 봉과 조회가 기대하는 봉의 시각이 어긋난다.
func TestTruncateCandleTime_MatchesSQLWallClockArithmetic(t *testing.T) {
	for _, n := range []int{5, 10, 30, 60} {
		for minute := 0; minute < 24*60; minute++ {
			ts := time.Date(2026, 3, 25, minute/60, minute%60, 0, 0, model.KST)
			got := truncateCandleTime(ts, n)

			bucket := (ts.Hour()*60 + ts.Minute()) / n * n
			want := time.Date(2026, 3, 25, bucket/60, bucket%60, 0, 0, model.KST)

			if !got.Equal(want) {
				t.Fatalf("n=%d ts=%s got=%s want=%s", n, ts, got, want)
			}
		}
	}
}

/*
델타 페치가 1분봉 계층 밖으로 나가지 않아야 한다.

gapMinutes는 벽시계라 장이 닫혀 있으면 실제 새 봉 수보다 훨씬 커진다. 그대로
요청하면 이미 접힌 구간의 1분봉을 다시 받아 되넣고, 다음 롤업이 다시 지운다.
실측(2026-08-23 일요일): 005930 gap 1,751분 → 1,766봉 요청 → 2.4 거래일 전까지.
*/
func TestIntradayDeltaMaxBars_StaysInsideOneMinuteTier(t *testing.T) {
	// 저장된 하루치 커버리지(실측 720봉)를 넘지 않아야 1분봉 계층(3일) 안에 머문다.
	if intradayDeltaMaxBars > intradayTiers[0].ageDays*720 {
		t.Fatalf("delta 상한 %d가 1분봉 계층(%d일)을 넘는다",
			intradayDeltaMaxBars, intradayTiers[0].ageDays)
	}
	// 정상 틱(1~2분 지연)에서는 상한이 걸리지 않아야 한다 — 걸리면 델타의 의미가 없다.
	if intradayDeltaMaxBars <= intradayDeltaFetchBufferBars {
		t.Fatalf("delta 상한 %d가 버퍼 %d 이하 — 정상 틱까지 잘린다",
			intradayDeltaMaxBars, intradayDeltaFetchBufferBars)
	}
}
