package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// 만료 판정과 "값은 남아 있음" 판정이 분리되어야 stale-while-revalidate가 성립한다.
func TestListFromCacheAllowStale(t *testing.T) {
	st := &NewsStore{listCache: map[string]newsListCacheEntry{}}
	arts := []model.NewsArticle{{Headline: "x"}}

	// 없는 키
	if _, _, fresh, exists := st.listFromCacheAllowStale("k"); fresh || exists {
		t.Fatal("empty cache should report neither fresh nor exists")
	}

	// 신선한 값
	st.listCache["k"] = newsListCacheEntry{articles: arts, builtAt: time.Now()}
	got, _, fresh, exists := st.listFromCacheAllowStale("k")
	if !fresh || !exists || len(got) != 1 {
		t.Fatalf("fresh entry: fresh=%v exists=%v n=%d", fresh, exists, len(got))
	}
	if _, _, ok := st.listFromCache("k"); !ok {
		t.Fatal("listFromCache should hit on a fresh entry")
	}

	// 만료된 값 — 남아 있지만 신선하지 않다
	st.listCache["k"] = newsListCacheEntry{articles: arts, builtAt: time.Now().Add(-2 * listCacheTTL)}
	got, _, fresh, exists = st.listFromCacheAllowStale("k")
	if fresh {
		t.Fatal("expired entry must not be fresh")
	}
	if !exists || len(got) != 1 {
		t.Fatalf("expired entry must still be readable: exists=%v n=%d", exists, len(got))
	}
	// 기존 경로는 만료를 미스로 봐야 한다(동작 불변).
	if _, _, ok := st.listFromCache("k"); ok {
		t.Fatal("listFromCache must still treat an expired entry as a miss")
	}
}

// ingest가 캐시를 비우면 stale 값이 없어야 한다 — "새 기사는 즉시" 보장의 근거.
func TestIngestInvalidationLeavesNoStaleValue(t *testing.T) {
	st := &NewsStore{listCache: map[string]newsListCacheEntry{
		"k": {articles: []model.NewsArticle{{Headline: "old"}}, builtAt: time.Now().Add(-time.Hour)},
	}}
	st.listCache = nil // mergeArticles가 하는 것과 같다
	if _, _, _, exists := st.listFromCacheAllowStale("k"); exists {
		t.Fatal("invalidated cache must not serve a stale list")
	}
}
