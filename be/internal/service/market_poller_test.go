package service

import "testing"

func TestCapActiveTargets_boundsToLimit(t *testing.T) {
	active := map[string]struct{}{
		"AAA": {}, "BBB": {}, "CCC": {}, "DDD": {}, "EEE": {},
	}
	got := capActiveTargets(active, 3)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	seen := map[string]struct{}{}
	for _, sym := range got {
		if _, dup := seen[sym]; dup {
			t.Fatalf("duplicate symbol %s in result", sym)
		}
		seen[sym] = struct{}{}
		if _, inActive := active[sym]; !inActive {
			t.Fatalf("result contains %s, not in the active set", sym)
		}
	}
}

func TestCapActiveTargets_returnsAllWhenUnderLimit(t *testing.T) {
	active := map[string]struct{}{"AAA": {}, "BBB": {}}
	got := capActiveTargets(active, 40)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
}

func TestCapActiveTargets_emptyActive(t *testing.T) {
	got := capActiveTargets(map[string]struct{}{}, 40)
	if len(got) != 0 {
		t.Fatalf("len=%d want 0", len(got))
	}
}
