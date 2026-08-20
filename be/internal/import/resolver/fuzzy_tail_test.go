package resolver

import "testing"

func TestLatinSubsequenceMatch(t *testing.T) {
	if !latinSubsequenceMatch("sdi", "si") {
		t.Fatal("expected si subsequence of sdi")
	}
}

func TestCatalogMatchScoreSamsung5mi(t *testing.T) {
	raw := "삼성5미"
	best, score, ok := matchBestCatalog(raw, []catalogEntry{
		{Symbol: "006400", Name: "삼성SDI"},
		{Symbol: "006405", Name: "삼성SDI우"},
		{Symbol: "028260", Name: "삼성물산"},
	})
	if !ok || best.Symbol != "006400" {
		t.Fatalf("match ok=%v best=%+v score=%.3f", ok, best, score)
	}
	if score < 0.48 {
		t.Fatalf("score too low: %.3f", score)
	}
}
