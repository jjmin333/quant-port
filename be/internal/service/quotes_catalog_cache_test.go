package service

import (
	"strconv"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func TestAssetTypeCache_hitAndMiss(t *testing.T) {
	c := newAssetTypeCache()

	if _, ok := c.get("005930"); ok {
		t.Fatal("빈 캐시가 값을 돌려줬다")
	}

	c.put("005930", model.AssetDomestic)
	got, ok := c.get("005930")
	if !ok || got != model.AssetDomestic {
		t.Fatalf("캐시 적중 실패: got=%q ok=%v", got, ok)
	}
}

func TestAssetTypeCache_expires(t *testing.T) {
	c := newAssetTypeCache()
	c.put("AAPL", model.AssetUS)

	// TTL을 지난 항목은 없는 셈 쳐야 한다 — 참조 배치가 자산 구분을 고쳐도
	// 프로세스가 살아 있는 내내 옛 값을 들고 있으면 세션 판정이 영원히 틀린다.
	c.mu.Lock()
	c.entries["AAPL"] = assetTypeCacheEntry{
		value: model.AssetUS,
		at:    time.Now().Add(-assetTypeCacheTTL - time.Second),
	}
	c.mu.Unlock()

	if _, ok := c.get("AAPL"); ok {
		t.Fatal("TTL이 지난 항목이 적중했다")
	}
}

func TestAssetTypeCache_forget(t *testing.T) {
	c := newAssetTypeCache()
	c.put("TSLA", model.AssetUS)
	// 소문자·공백으로 불러도 같은 항목을 지워야 한다.
	c.Forget(" tsla ")
	if _, ok := c.get("TSLA"); ok {
		t.Fatal("Forget이 항목을 안 지웠다")
	}
}

func TestAssetTypeCache_boundedBySize(t *testing.T) {
	c := newAssetTypeCache()
	for i := 0; i < assetTypeCacheMax+10; i++ {
		c.put(strconv.Itoa(i), model.AssetDomestic)
	}
	c.mu.RLock()
	n := len(c.entries)
	c.mu.RUnlock()
	if n > assetTypeCacheMax {
		t.Fatalf("캐시가 상한을 넘었다: %d > %d — 종목 수만큼 자라면 누수다", n, assetTypeCacheMax)
	}
}

func TestAssetTypeCache_nilSafe(t *testing.T) {
	// QuotesService가 이 필드 없이 만들어지는 경로(테스트 픽스처 등)에서도
	// 죽지 않아야 한다 — 캐시는 최적화지 필수 의존성이 아니다.
	var c *assetTypeCache
	if _, ok := c.get("005930"); ok {
		t.Fatal("nil 캐시가 값을 돌려줬다")
	}
	c.put("005930", model.AssetDomestic) // panic 안 나면 통과
	c.Forget("005930")
}

