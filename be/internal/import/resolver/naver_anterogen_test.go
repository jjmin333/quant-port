package resolver

import "testing"

func TestMatchBestCatalogNAVERNotAnterogen(t *testing.T) {
	catalog := []catalogEntry{
		{Symbol: "035420", Name: "NAVER"},
		{Symbol: "065660", Name: "안트로젠"},
	}
	cases := []string{
		"NAVER",
		"®  NAVER",
		"® NAVER",
		"NAVER 4,294,248",
		"4,294,248",
	}
	for _, raw := range cases {
		best, score, ok := matchBestCatalog(raw, catalog)
		if !ok {
			t.Logf("%q -> no match", raw)
			continue
		}
		t.Logf("%q -> %q (%s) score=%.3f", raw, best.Name, best.Symbol, score)
		if best.Symbol == "065660" {
			t.Fatalf("%q wrongly matched 안트로젠", raw)
		}
	}
}

func TestResolveKnownLatinTickers(t *testing.T) {
	catalog := []catalogEntry{
		{Symbol: "035420", Name: "NAVER"},
		{Symbol: "065660", Name: "안트로젠"},
	}
	for _, raw := range []string{"NAVER", "naver", "Naver"} {
		best, _, ok := matchBestCatalog(raw, catalog)
		if !ok || best.Symbol != "035420" {
			t.Fatalf("%q -> %+v ok=%v", raw, best, ok)
		}
	}
}
