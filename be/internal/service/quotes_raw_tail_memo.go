package service

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/redis/go-redis/v9"
)

/*
`md:raw:1m` · `md:raw:1d` 원본 봉 배열의 **프로세스 안 짧은 기억**.

# 왜 필요한가

형성봉을 스냅샷에서 떼어낸 뒤(`quotes_cache.go`) Redis 출력의 대부분이 이 두 키로
옮겨 갔다. `md:raw:1m:000660`은 실측 **291,001바이트**이고, 이걸 읽는 자리는 전부
**작은 답 하나**를 얻으려고 통째로 받는다:

	statsFromRawCache      오늘의 시가·고가·저가·거래량 (어제 이전 봉은 건너뛴다)
	regularCloseFromRaw1m  정규장 마감 시각 직전의 종가 하나

이 둘은 시세 상세를 그릴 때마다 **종목마다** 불린다. MONITOR 3초 표본에서 `get
md:raw:1m` 14회 — 291KB × 초당 4.7회 ≈ **1.4MB/s**였다.

# 왜 기억해도 되는가

이 키를 채우는 것은 `storeRawTail`이고, 그건 캔들 갱신이 돌 때만 일어난다. 아래
`rawTailMemoTTL`은 그보다 훨씬 짧게 잡아, 늘어나는 낡음이 **폴러의 틱 간격 안**에
들어오게 한다. 게다가 같은 프로세스가 쓸 때는 기억을 **곧바로 갈아 끼우므로**
(`storeRawCandles`) 자기가 쓴 값을 낡게 보는 일은 없다.

배치 워커가 다른 프로세스에서 같은 키를 갈아 끼우는 경우가 남는데, 그때 API가
최대 `rawTailMemoTTL`만큼 예전 배열을 본다. 그 배열에서 뽑는 값(오늘 고저·거래량·
직전 종가)은 그 시간 안에 눈에 띄게 달라지지 않는다 — 초 단위로 움직이는 현재가는
애초에 이 경로가 아니라 터미널 스냅샷과 형성봉이 들고 있다.
*/

// rawTailMemoTTL — 가격 폴러의 틱(1초)·터미널 틱(2초)과 같은 눈금.
const rawTailMemoTTL = 2 * time.Second

type rawTailMemoEntry struct {
	candles []model.Candle
	at      time.Time
}

var (
	rawTailMemoMu sync.RWMutex
	rawTailMemo   = map[string]rawTailMemoEntry{}
)

// resetRawTailMemo clears the memo. Tests use it; nothing else should.
func resetRawTailMemo() {
	rawTailMemoMu.Lock()
	rawTailMemo = map[string]rawTailMemoEntry{}
	rawTailMemoMu.Unlock()
}

func rawTailMemoGet(key string, now time.Time) ([]model.Candle, bool) {
	rawTailMemoMu.RLock()
	entry, ok := rawTailMemo[key]
	rawTailMemoMu.RUnlock()
	if !ok || now.Sub(entry.at) > rawTailMemoTTL {
		return nil, false
	}
	return entry.candles, true
}

func rawTailMemoPut(key string, candles []model.Candle, now time.Time) {
	rawTailMemoMu.Lock()
	rawTailMemo[key] = rawTailMemoEntry{candles: candles, at: now}
	rawTailMemoMu.Unlock()
}

/*
loadRawCandles는 원본 봉 배열을 돌려준다 — 기억에 있으면 Redis를 건드리지 않는다.

**돌려주는 슬라이스를 고치면 안 된다.** 기억이 같은 배열을 여럿에게 나눠 주므로,
한쪽이 정렬하거나 잘라 내면 나머지가 그 결과를 본다. 지금 부르는 쪽들은 전부 읽기만
한다(합계·최댓값·마지막 값).
*/
func loadRawCandles(ctx context.Context, rdb *redis.Client, key string) ([]model.Candle, bool) {
	if rdb == nil {
		return nil, false
	}
	now := time.Now()
	if candles, ok := rawTailMemoGet(key, now); ok {
		return candles, len(candles) > 0
	}
	raw, err := rdb.Get(ctx, key).Bytes()
	if err != nil || len(raw) == 0 {
		// 없다는 사실도 기억한다 — 아직 안 채워진 종목을 목록이 훑을 때
		// 같은 헛걸음을 종목 수만큼 반복하지 않게.
		rawTailMemoPut(key, nil, now)
		return nil, false
	}
	var candles []model.Candle
	if json.Unmarshal(raw, &candles) != nil {
		rawTailMemoPut(key, nil, now)
		return nil, false
	}
	rawTailMemoPut(key, candles, now)
	return candles, len(candles) > 0
}

// storeRawCandles writes the tail and refreshes the memo in one step, so the
// writer never reads back its own value through a stale entry.
func (s *QuotesService) storeRawCandles(
	ctx context.Context,
	key string,
	candles []model.Candle,
	ttl time.Duration,
) {
	s.cacheSetJSON(ctx, key, candles, ttl)
	rawTailMemoPut(key, candles, time.Now())
}
