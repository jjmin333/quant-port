package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

// 캐시 키는 거래일과 정규화된 조건 전부로 갈려야 한다 — 하나라도 빠지면 다른 조건의
// 결과가 「score ≥ 70」 칸 옆에 조용히 뜬다.
func TestScreenResultKey(t *testing.T) {
	min70 := 70.0
	base := model.ScreenQuery{Market: "domestic", Rules: []model.ScreenRule{{Field: "score", Min: &min70}}, Limit: 50}
	k := screenResultKey("2026-09-24", base)
	if k != screenResultKey("2026-09-24", base) {
		t.Fatal("same query must map to the same key")
	}
	other := base
	other.Market = "overseas"
	sorted := base
	sorted.SortBy, sorted.SortAsc = "score", true
	min80 := 80.0
	tighter := base
	tighter.Rules = []model.ScreenRule{{Field: "score", Min: &min80}}
	for name, key := range map[string]string{
		"date":   screenResultKey("2026-09-25", base),
		"market": screenResultKey("2026-09-24", other),
		"sort":   screenResultKey("2026-09-24", sorted),
		"bound":  screenResultKey("2026-09-24", tighter),
	} {
		if key == k {
			t.Errorf("%s must change the key", name)
		}
	}
}

func TestScreenResultCacheCap(t *testing.T) {
	s := &ScreenerService{}
	for i := 0; i < screenResultMax+3; i++ {
		s.rememberResult(string(rune('a'+i%26))+string(rune(i)), model.ScreenResult{})
	}
	if len(s.results) > screenResultMax {
		t.Fatalf("cache must stay bounded: %d", len(s.results))
	}
}
