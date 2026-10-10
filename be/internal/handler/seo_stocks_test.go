package handler

import (
	"testing"
	"time"
)

func seoSet(symbols ...string) map[string]bool {
	out := make(map[string]bool, len(symbols))
	for _, s := range symbols {
		out[s] = true
	}
	return out
}

// 사이트맵(옛 목록)에 실린 종목이 목록 교체 직후 페이지에서 noindex가 되면 안 된다.
func TestSEOPageIndexable_keepsRetiredListDuringGrace(t *testing.T) {
	h := &SEOStocksHandler{}
	t0 := time.Date(2026, 9, 26, 7, 0, 0, 0, time.UTC)
	h.install(t0, "2026-09-24", []string{"AAPL", "URBN"}, seoSet("AAPL", "URBN"))
	t1 := t0.Add(time.Hour)
	cur := seoSet("AAPL", "TGTX")
	h.install(t1, "2026-09-24", []string{"AAPL", "TGTX"}, cur)

	if !h.pageIndexable(t1.Add(time.Minute), "URBN", cur) {
		t.Fatal("URBN dropped out a minute ago — still indexable during grace")
	}
	if !h.pageIndexable(t1, "TGTX", cur) {
		t.Fatal("new entrant must be indexable at once")
	}
	if h.pageIndexable(t1.Add(seoIndexGrace+time.Minute), "URBN", cur) {
		t.Fatal("after grace, a dropped stock is noindex")
	}
	if h.pageIndexable(t1, "ZZZZ", cur) {
		t.Fatal("never-listed stock must stay noindex")
	}
}

// 같은 목록을 다시 뽑은 것은 교체가 아니다 — 유예 명단이 쌓이지 않는다.
func TestSEOInstall_sameListDoesNotRetire(t *testing.T) {
	h := &SEOStocksHandler{}
	t0 := time.Date(2026, 9, 26, 7, 0, 0, 0, time.UTC)
	h.install(t0, "d", []string{"A"}, seoSet("A"))
	h.install(t0.Add(time.Hour), "d", []string{"A"}, seoSet("A"))
	if len(h.retired) != 0 {
		t.Fatalf("retired = %d, want 0", len(h.retired))
	}
	h.install(t0.Add(2*time.Hour), "d", []string{"B"}, seoSet("B"))
	h.install(t0.Add(2*time.Hour+seoIndexGrace+time.Minute), "d", []string{"C"}, seoSet("C"))
	if len(h.retired) != 1 || !h.retired[0].set["B"] {
		t.Fatalf("expired retired lists must be pruned, got %+v", h.retired)
	}
}
