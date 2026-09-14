package service

import (
	"context"
	"sort"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

/*
인트라데이 저장 계층.

1분봉은 차트만 쓰는데도 가장 빠르게 자란다(실측 하루 약 61만 행 · 1.5GB/월).
그대로 두면 두 가지가 동시에 무너진다 — 스토리지가 차고, `bar_interval='1d'`만
쓰는 배치가 1분봉까지 훑느라 함께 느려진다.

그래서 오래된 인트라데이는 **지우지 않고 접는다**. 해상도만 낮추므로 차트는
계속 그려지고, Toss 재호출도 늘지 않는다.

	1m ──3일──▶ 5m ──7일──▶ 10m ──14일──▶ 30m ──60일──▶ 삭제

60일에서 1일봉으로 접지 않고 **삭제**하는 이유: `1d`는 이미 독립적으로, 더
정확하게(원본 세션 거래량), 더 길게(2009년~) 존재한다. 인트라데이 커버리지는
부분이라 접은 1일봉은 거래량이 모자라고, 같은 PK를 덮어써 `stock_daily_closes`
투영과 `stock_scores`까지 오염시킨다 (실측: 가장 조밀한 005930조차 79,033주 부족).

계층 해상도가 서로 나눠떨어지게 골라져 있어(5|10, 10|30) 모든 요청 간격이
자기 만료일까지 정확히 만들어진다. 접기가 결합법칙을 만족한다는 것은
candles_tiering_test.go가 고정한다.

# 60분봉 계층이 없는 이유 (2026-08-24)

**60분봉은 기능에서 제거됐다.** 인터벌 중 혼자 압도적으로 비쌌기 때문이다 —
`candleFetchCount`가 `120 × 분`이라 60분봉 첫 화면 한 번이 1분봉 7,200개,
즉 Toss `/candles` **36호출**이었다(1분봉·일봉은 각 1호출, 30분봉은 18호출).
그 버킷은 사용자가 차트를 처음 열 때 쓰는 것과 같아서, 60분봉을 여는 행위가
다른 사람의 차트를 밀어냈다.

계층에서 빠지면서 **30분봉이 마지막 계층**이 됐고, 그 `ageDays`를 30 → 60으로
올려 **인트라데이 총 보관 기간 60일을 그대로 유지**한다. 30~60일 구간이
60분봉 대신 30분봉으로 남으므로 행 수는 약 2배가 되지만(추정 +40MB),
그 구간의 **해상도는 오히려 좋아진다**.

`intradayTierMaxAgeDays`가 여전히 60인 것은 그래서다 — 마지막 계층의
`ageDays`와 같아야 하며 그 불변식은 candles_tiering_test.go가 고정한다.
*/
type intradayTier struct {
	interval model.CandleInterval
	minutes  int
	// ageDays — 이 계층에 머무는 최대 나이. 롤업 배치만 쓰고, 조회는 쓰지 않는다
	// (아래 collectTieredBars 주석 참조).
	ageDays int
}

var intradayTiers = []intradayTier{
	{model.Interval1m, 1, 3},
	{model.Interval5m, 5, 7},
	{model.Interval10m, 10, 14},
	// 마지막 계층. ageDays가 곧 인트라데이 만료일이다 — 롤업의 prune이
	// intradayTierMaxAgeDays가 아니라 **이 값**을 읽는다(candle_rollup.go의
	// 「마지막 계층 만료」 블록). 60분봉을 걷어내면서 30 → 60으로 올렸다.
	{model.Interval30m, 30, 60},
}

// intradayTierMaxAgeDays — 인트라데이가 보관되는 총 기간. 이보다 오래된 마지막
// 계층(30분봉)은 롤업 배치가 지운다. 마지막 계층의 ageDays와 반드시 같아야 하며
// candles_tiering_test.go가 그것을 고정한다.
const intradayTierMaxAgeDays = 60

// tiersUsableFor returns the tiers whose bars can build `interval`.
// 해상도가 요청 간격을 나눠떨어뜨려야 한다 — 30분봉은 1·5·10·30분봉에서 만들 수
// 있지만 60분봉에서는 못 만든다.
func tiersUsableFor(interval model.CandleInterval) []intradayTier {
	minutes := intervalMinutes(interval)
	if minutes <= 0 {
		return nil
	}
	out := make([]intradayTier, 0, len(intradayTiers))
	for _, t := range intradayTiers {
		if t.minutes <= minutes && minutes%t.minutes == 0 {
			out = append(out, t)
		}
	}
	return out
}

/*
collectTieredBars — 여러 계층에서 읽어 `interval` 버킷으로 접어 돌려준다.

**계층 경계를 시각으로 계산해 건너뛰지 않는다.** 롤업 배치가 며칠 밀리거나
멈추면 실제 데이터가 경계와 어긋나는데, 그때 경계를 믿으면 멀쩡히 있는 봉을
건너뛴다. 계층마다 같은 커서로 독립해 읽고 비어 있으면 그냥 넘어간다 — 파티션
덕에 빈 계층 조회는 PK 프로브 하나라 사실상 공짜다.

**출력 버킷 하나는 한 계층에서만 온다.** 더 고운 계층을 먼저 처리하고 이미 찬
버킷은 건너뛴다. 이게 없으면 거래량이 두 번 더해진다 — 계층은 원래 시간을
겹치지 않게 나누지만, `getCandlesPageUncached`(Toss 백필)가 이미 접힌 구간에
1분봉을 되넣기 때문에 겹침이 실제로 생긴다.
*/
func (s *QuotesService) collectTieredBars(
	ctx context.Context,
	symbol string,
	interval model.CandleInterval,
	before string,
	targetBars int,
) ([]model.Candle, bool) {
	if s.candles == nil || !s.candles.HasTable(ctx) {
		return nil, false
	}
	minutes := intervalMinutes(interval)
	tiers := tiersUsableFor(interval)
	if minutes <= 1 || len(tiers) == 0 || targetBars <= 0 {
		return nil, false
	}

	byBucket := make(map[string]model.Candle, targetBars)
	anyRow := false

	for _, tier := range tiers {
		if len(byBucket) >= targetBars {
			break
		}
		// 이 계층에서 한 버킷을 채우는 데 필요한 봉 수 × 남은 버킷 수.
		// +perBucket은 커서에 걸친 불완전 버킷 몫이다.
		perBucket := minutes / tier.minutes
		need := (targetBars-len(byBucket))*perBucket + perBucket

		bars, _, err := s.candles.ListBefore(ctx, symbol, tier.interval, before, need)
		if err != nil || len(bars) == 0 {
			continue
		}
		anyRow = true

		for _, c := range aggregateCandles(bars, interval) {
			// **더 완전한 쪽이 이긴다.** 「고운 계층 우선」이 아니다.
			//
			// Toss 백필(getCandlesPageUncached)이 이미 접힌 구간에 1분봉을
			// 되넣으면 그 버킷을 1분봉 한두 개가 덮게 되는데, 옆에는 같은
			// 버킷을 5개 전부로 만든 5분봉이 있다. 고운 쪽을 무조건 택하면
			// 거래량이 무너진다 — 실측 005930 08-19 16:10 버킷이
			// 255,170 → 18,402주가 됐다.
			if prev, taken := byBucket[c.Time]; taken && !supersedes(c, prev) {
				continue
			}
			byBucket[c.Time] = c
		}
	}

	if !anyRow || len(byBucket) == 0 {
		return nil, false
	}

	out := make([]model.Candle, 0, len(byBucket))
	for _, c := range byBucket {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return parseQuoteCandleTime(out[i].Time).Before(parseQuoteCandleTime(out[j].Time))
	})
	return trimCandlesTail(out, targetBars), true
}

/*
dropUnretainedIntraday — 인트라데이 보관 한계를 넘는 1분봉은 저장하지 않는다.

집계 차트를 깊이 넘기면 `getCandlesPageUncached`가 페이지당 최대 7,260개의 1분봉을
Toss에서 받아 저장한다. 그 구간이 보관 한계(60일) 밖이면 **다음 롤업이 곧바로
지운다** — 쓰고 지우기만 하는 순수 낭비다.

실측(2026-08-23, 롤업 26분 뒤): 30분 동안 되넣어진 1분봉 59,426개 중 47,514개
(80%)가 90~99일 전 구간이었다. 60일 안쪽 것은 그대로 저장한다 — 그것은 낭비가
아니라 해당 계층을 채우는 정상 적재이고, 롤업이 알맞은 해상도로 접는다.

일봉(1d)에는 적용하지 않는다. 1d는 영구 보관이고 2009년치까지 의미가 있다.
*/
func dropUnretainedIntraday(base model.CandleInterval, bars []model.Candle) []model.Candle {
	if base != model.Interval1m || len(bars) == 0 {
		return bars
	}
	cutoff := time.Now().In(model.KST).AddDate(0, 0, -intradayTierMaxAgeDays)
	out := make([]model.Candle, 0, len(bars))
	for _, c := range bars {
		t := parseQuoteCandleTime(c.Time)
		if t.IsZero() || !t.Before(cutoff) {
			out = append(out, c)
		}
	}
	return out
}

// supersedes reports whether `next` should replace `prev` for the same bucket.
//
// 판단 기준은 **덮개의 두께**(그 봉을 만든 1분봉 수)이지 계층의 고움이 아니다.
// 같은 시각을 두 계층이 덮는 것은 예외가 아니다 — Toss 백필이 이미 접힌 구간에
// 1분봉을 되넣기 때문이다. 그때 고운 쪽을 무조건 택하면 되넣어진 1분봉 한두 개가
// 완전한 5분봉을 덮어써 거래량이 무너진다.
func supersedes(next, prev model.Candle) bool {
	return candleCoverage(next) > candleCoverage(prev)
}

// tierSnapshotLimit — 스냅샷을 만들 때 계층에서 읽어 둘 최대 봉 수.
//
// 가장 깊은 인트라데이 간격(60분 × 120봉 = 7,200분)을 덮을 만큼이면 나머지
// 간격은 전부 덮인다. 계층이 굵을수록 같은 시간을 적은 봉으로 덮으므로 읽는
// 행 수가 함께 줄어든다 — 계층화 뒤 스냅샷이 읽는 총 행 수는 오히려 감소한다
// (기존: 1분봉 7,200행으로 5일치 / 계층: 약 3,500행으로 60일치).
func tierSnapshotLimit(tier intradayTier) int {
	n := maxIntradayRawBars / tier.minutes
	if n < intradayDisplayBars {
		n = intradayDisplayBars
	}
	return n
}

/*
intradaySnapshotSource — 스냅샷 재구축용 **혼합 해상도** 시계열.

`rebuildAggregatedSnapshots`는 폴러 틱마다(30초) 심볼당 한 번 돈다. 그래서 요청
간격마다 계층을 따로 훑으면(간격 4 × 계층 5 = 20쿼리) 폴러가 DB를 갈아 버린다.
대신 여기서 **계층당 한 번씩만** 읽어 하나의 시계열로 만들고, 호출부가 그것을
메모리에서 각 간격으로 접는다.

`fresh`는 Toss에서 막 받아 온 1분봉이다(형성 중인 봉 포함). 가장 신선하므로
가장 먼저 자리를 잡고, 계층 봉은 그 위에 겹치지 않는 것만 얹는다.

**시간 범위가 겹치면 굵은 봉을 버린다.** 계층은 원래 시간을 겹치지 않게 나누지만
`getCandlesPageUncached`(Toss 백필)가 이미 접힌 구간에 1분봉을 되넣어 겹침이
실제로 생기고, 겹친 채로 접으면 그 버킷의 거래량이 두 번 더해진다.

반환값을 **저장 경로에 넘기면 안 된다** — `storeRawTail`은 받은 배열을 통째로
하나의 bar_interval로 기록하므로, 혼합 해상도를 넘기면 5분봉이 1분봉 행으로
저장된다. 저장에는 순수 1분봉(listIntradayBarsFromDB)을 쓴다.
*/
func (s *QuotesService) intradaySnapshotSource(
	ctx context.Context,
	symbol string,
	fresh []model.Candle,
) []model.Candle {
	if s.candles == nil || !s.candles.HasTable(ctx) {
		return fresh
	}

	// held[i] == false 이면 굵은 봉에 밀려난 자리다(마지막에 걸러낸다).
	var pool []model.Candle
	held := make([]bool, 0, maxIntradayRawBars)
	// owner: 분(minute) → 그 분을 덮고 있는 pool 인덱스
	owner := make(map[int64]int, maxIntradayRawBars)

	// add는 [start, start+minutes) 를 덮는 봉을 넣는다. 그 구간을 이미 덮고 있던
	// 봉들의 **총 덮개**가 이 봉보다 두껍거나 같으면 넣지 않고, 얇으면 그것들을
	// 밀어내고 자리를 차지한다.
	add := func(c model.Candle, start int64, minutes int) {
		cov := candleCoverage(c)
		prevIdx := map[int]struct{}{}
		for m := int64(0); m < int64(minutes); m++ {
			if i, ok := owner[start+m]; ok && held[i] {
				prevIdx[i] = struct{}{}
			}
		}
		prevCov := 0
		for i := range prevIdx {
			prevCov += candleCoverage(pool[i])
		}
		if len(prevIdx) > 0 && prevCov >= cov {
			return
		}
		for i := range prevIdx {
			held[i] = false
		}
		pool = append(pool, c)
		held = append(held, true)
		idx := len(pool) - 1
		for m := int64(0); m < int64(minutes); m++ {
			owner[start+m] = idx
		}
	}

	// Toss에서 갓 받은 1분봉이 가장 신선하므로 먼저 자리를 잡는다.
	for _, c := range fresh {
		if t := parseQuoteCandleTime(c.Time); !t.IsZero() {
			add(c, t.Unix()/60, 1)
		}
	}
	for _, tier := range intradayTiers {
		bars, _, err := s.candles.ListBefore(ctx, symbol, tier.interval, "", tierSnapshotLimit(tier))
		if err != nil || len(bars) == 0 {
			continue
		}
		for _, c := range bars {
			if t := parseQuoteCandleTime(c.Time); !t.IsZero() {
				add(c, t.Unix()/60, tier.minutes)
			}
		}
	}

	out := make([]model.Candle, 0, len(pool))
	for i, c := range pool {
		if held[i] {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return sortCandlesAsc(out)
}
