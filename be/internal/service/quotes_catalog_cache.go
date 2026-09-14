package service

import (
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

/*
종목 카탈로그 인메모리 캐시 — 목록 화면의 DB 조회를 걷어낸다.

# 왜 필요했나

`quoteAssetType`은 `stocks` 테이블을 **종목마다 한 번씩** 읽는다. 그런데 이 함수는
목록 경로의 거의 모든 갈래에 있다 — `marketSessionFor`(가격 해석의 첫 줄),
`prevCloseFromDailyBars`(등락률 기준가), `detectMarketSession`. 그래서 100줄짜리
목록 한 번이 같은 테이블을 수백 번 읽었다.

실측(2026-09-01, `Com_select` 전역 카운터 차분):

	/rankings KR 100종목(마감)   186~313 SELECT
	/rankings US 100종목(pre)    626 SELECT
	/quotes/batch 100종목        1,601 SELECT

**장이 열려 있을 때가 더 비싸다** — 세션이 살아 있으면 체결·호가·형성봉 갈래가
전부 열리고, 그 갈래마다 자산 구분을 다시 묻기 때문이다.

# 왜 인메모리인가

값이 **아주 작고**(문자열 하나) **거의 안 바뀐다** — `stocks`는 참조 배치가 하루
단위로 채운다. Redis에 두면 종목마다 왕복이 한 번 생겨서, 없애려던 비용을 더 싼
비용으로 바꾸는 데 그친다. API는 인스턴스 하나로 도는 구성이라(2 OCPU ARM 1대)
프로세스 안에 두는 것이 가장 싸다.

TTL 10분은 "배치가 자산 구분을 고쳐도 한 화면 안에서는 안 바뀐다"와 "상장 정보가
바뀐 종목이 하루 종일 틀리지는 않는다" 사이다. 캐시가 틀렸을 때의 최악은 세션
판정이 10분 늦게 맞춰지는 것이고, 가격·순위는 영향을 받지 않는다.
*/
const (
	assetTypeCacheTTL = 10 * time.Minute

	/*
		assetTypeCacheMax — 캐시가 들 수 있는 종목 수의 상한.

		전체 종목은 국내 약 4,400 + 해외 약 13,500이다. 상한을 넘으면 통째로 비운다 —
		LRU를 두는 편이 이론적으로는 낫지만, 이 캐시가 실제로 담는 것은 **지금 화면에
		뜬 종목**이라 작업 집합이 상한보다 훨씬 작다. 상한은 누수 방지용 안전장치다.
	*/
	assetTypeCacheMax = 20000
)

type assetTypeCacheEntry struct {
	value model.QuoteAssetType
	at    time.Time
}

type assetTypeCache struct {
	mu      sync.RWMutex
	entries map[string]assetTypeCacheEntry
}

func newAssetTypeCache() *assetTypeCache {
	return &assetTypeCache{entries: make(map[string]assetTypeCacheEntry)}
}

func (c *assetTypeCache) get(symbol string) (model.QuoteAssetType, bool) {
	if c == nil {
		return "", false
	}
	c.mu.RLock()
	entry, ok := c.entries[symbol]
	c.mu.RUnlock()
	if !ok || time.Since(entry.at) > assetTypeCacheTTL {
		return "", false
	}
	return entry.value, true
}

func (c *assetTypeCache) put(symbol string, value model.QuoteAssetType) {
	if c == nil || symbol == "" || value == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= assetTypeCacheMax {
		c.entries = make(map[string]assetTypeCacheEntry, assetTypeCacheMax/4)
	}
	c.entries[symbol] = assetTypeCacheEntry{value: value, at: time.Now()}
}

// Forget drops one symbol — 참조 배치가 그 종목의 카탈로그 행을 고쳤을 때 쓴다.
func (c *assetTypeCache) Forget(symbol string) {
	if c == nil {
		return
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return
	}
	c.mu.Lock()
	delete(c.entries, sym)
	c.mu.Unlock()
}
