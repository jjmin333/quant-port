package textparse

import "testing"

func TestMergeKeyDistinctETFIssuers(t *testing.T) {
	kodex := mergeKey("KODEX 미국나스.")
	tiger := mergeKey("TIGER 미국나스.")
	if kodex == tiger || kodex == "" || tiger == "" {
		t.Fatalf("kodex=%q tiger=%q want distinct", kodex, tiger)
	}
}

func TestRepairDomesticETFOCRName(t *testing.T) {
	cases := map[string]string{
		"006×미국나스.":         "KODEX 미국나스",
		"KODEX 미국나스.":       "KODEX 미국나스.",
		"TIGER 미국나스.":       "TIGER 미국나스.",
		"으으. TIGER 미국나스..": "TIGER 미국나스.",
	}
	for in, want := range cases {
		if got := NormalizeDisplayName(in); got != want {
			t.Fatalf("%q -> %q want %q", in, got, want)
		}
	}
}

func TestFuzzyNameMatchDistinctETF(t *testing.T) {
	if fuzzyNameMatch("KODEX 미국나스.", "TIGER 미국나스.") {
		t.Fatal("KODEX and TIGER must not fuzzy-merge")
	}
}
