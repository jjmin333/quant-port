package resolver

import "testing"

func TestCatalogMatchScoreAntPrefix(t *testing.T) {
	for _, raw := range []string{"안트", "트로", "트로젠", "안트로젠"} {
		score := catalogMatchScore(raw, "안트로젠")
		t.Logf("%q vs 안트로젠 score=%.3f highConf=%v auto=%v",
			raw, score, score >= fuzzyHighConfThreshold, score >= fuzzyAutoMatchThreshold)
	}
	best, score, ok := matchBestCatalog("안트", []catalogEntry{
		{Symbol: "035420", Name: "NAVER"},
		{Symbol: "065660", Name: "안트로젠"},
	})
	if ok || best.Symbol == "065660" {
		t.Fatalf("안트 should not match 안트로젠: ok=%v best=%+v score=%.3f", ok, best, score)
	}
}
