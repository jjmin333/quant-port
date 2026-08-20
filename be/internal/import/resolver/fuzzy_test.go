package resolver

import "testing"

func TestNameMatchScoreOCRTypos(t *testing.T) {
	cases := []struct {
		raw      string
		catalog  string
		minScore float64
	}{
		{"삼성5미", "삼성SDI", 0.48},
		{"16전자", "LG전자", 0.48},
		{"현디자", "현대차", 0.48},
		{"삼성전자", "삼성전자", 1.0},
		{"NAVER", "NAVER", 1.0},
	}
	for _, tc := range cases {
		score := catalogMatchScore(tc.raw, tc.catalog)
		if score < tc.minScore {
			t.Fatalf("%q vs %q score=%v want>=%v", tc.raw, tc.catalog, score, tc.minScore)
		}
	}
}

func TestMatchBestCatalogAmbiguousRejected(t *testing.T) {
	catalog := []catalogEntry{
		{Symbol: "009830", Name: "한화솔루션"},
		{Symbol: "272210", Name: "한화시스템"},
	}
	_, _, ok := matchBestCatalog("한화솔루션", catalog)
	if !ok {
		t.Fatal("expected exact hangul match")
	}
}

func TestMatchBestCatalog(t *testing.T) {
	catalog := []catalogEntry{
		{Symbol: "006400", Name: "삼성SDI"},
		{Symbol: "006405", Name: "삼성SDI우"},
		{Symbol: "005380", Name: "현대차"},
		{Symbol: "005930", Name: "삼성전자"},
		{Symbol: "066570", Name: "LG전자"},
	}
	best, _, ok := matchBestCatalog("16전자", catalog)
	if !ok || best.Symbol != "066570" {
		t.Fatalf("unexpected lg match: ok=%v best=%+v", ok, best)
	}
	best, _, ok = matchBestCatalog("삼성5미", catalog)
	if !ok || best.Symbol != "006400" {
		t.Fatalf("unexpected sdi match: ok=%v best=%+v", ok, best)
	}
	best, _, ok = matchBestCatalog("현디자", catalog)
	if !ok || best.Symbol != "005380" {
		t.Fatalf("unexpected hyundai match: ok=%v best=%+v", ok, best)
	}
}
