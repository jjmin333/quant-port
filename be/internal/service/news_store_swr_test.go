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

/*
주말 폴링 완화 — 평일은 그대로여야 하고, **어떤 격자의 티커도 굶지 않아야** 한다.

뉴스 신선도는 체감 품질이라 평일을 늦추지 않는다. 주말만 솎아 내는 이유와
근거 수치는 weekendTickGate 헤더에 있다.

두 번째 반복문이 이 시험의 핵심이다. 예전 구현(`now.Minute()%14 != 0`)은 **매 분
물어볼 때만** 14분처럼 보였고, 실제 티커처럼 2분·8분 격자 위에서 물으면 격자에 따라
평균 60분이 되거나 아예 한 번도 통과하지 못했다 — 2026-09-13에 해외 뉴스가 멈춘
원인이 그것이다. 그래서 여기서는 **실제 주기와 오프셋 그대로** 돌려 본다.
*/
func TestWeekendTickGate(t *testing.T) {
	// 2026-08-24는 월요일.
	monday := time.Date(2026, 8, 24, 13, 3, 0, 0, kst)
	var weekday weekendTickGate
	for m := 0; m < 60; m++ {
		if !weekday.allow(monday.Add(time.Duration(m) * time.Minute)) {
			t.Fatalf("평일 %d분 틱이 스킵됐다 — 평일은 손대지 않는다", m)
		}
	}

	// 2026-08-22는 토요일. 실제 티커 격자 그대로 — news_store.go의 Run 참조.
	saturday := time.Date(2026, 8, 22, 13, 0, 0, 0, kst)
	step := newsPollInterval / 5
	grids := []struct {
		name     string
		interval time.Duration
		offset   time.Duration
	}{
		{"naver", newsPollInterval, 0},
		{"rss", newsPollInterval, step},
		{"rssUS", newsPollInterval, 2 * step},
		{"finnhubGeneral", newsPollInterval, 3 * step},
		{"googleNews", newsPollInterval, 4 * step},
		{"finnhubCompany", newsUSCompanyPollInterval, step / 2},
	}
	for _, g := range grids {
		// 프로세스가 언제 떴는지는 정해져 있지 않다. 예전 구현이 죽은 이유가
		// 바로 그 「언제 떴는가」였으므로, 1분 단위로 시작 시각을 옮겨 가며 센다.
		for shift := 0; shift < 14; shift++ {
			start := saturday.Add(time.Duration(shift) * time.Minute).Add(g.offset)
			var gate weekendTickGate
			passed := 0
			var last time.Time
			for tick := g.interval; tick <= 6*time.Hour; tick += g.interval {
				now := start.Add(tick)
				if !gate.allow(now) {
					continue
				}
				if !last.IsZero() && now.Sub(last) < weekendNewsMinGap {
					t.Fatalf("%s(shift %d): %s 만에 두 번 통과 — 완화가 안 됐다",
						g.name, shift, now.Sub(last))
				}
				last = now
				passed++
			}
			// 6시간 / 14분 = 최대 25회. 격자가 성기면 그보다 적지만, 주기가
			// 14분 + 한 격자 이내로는 반드시 돌아야 한다.
			minPasses := int(6 * time.Hour / (weekendNewsMinGap + g.interval))
			if passed < minPasses {
				t.Fatalf("%s(shift %d): 6시간에 %d번만 통과 — 최소 %d번은 돌아야 한다 (굶는 격자)",
					g.name, shift, passed, minPasses)
			}
		}
	}
}
