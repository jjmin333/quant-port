package resolver

import "testing"

func TestNameMatchScoreMaplebinTypo(t *testing.T) {
	score := catalogMatchScore("맵러빈", "앱러빈")
	t.Logf("score=%v suffix=%v", score, ocrHangulSuffixScore(normalizeMatchKey("맵러빈"), normalizeMatchKey("앱러빈")))
	if score < fuzzyAutoMatchThreshold {
		t.Fatalf("맵러빈 vs 앱러빈 score=%v want>=%v", score, fuzzyAutoMatchThreshold)
	}
	catalog := []catalogEntry{
		{Symbol: "APP", Name: "앱러빈", EnglishName: "AppLovin"},
		{Symbol: "AMAT", Name: "어플라이드 머티리얼즈"},
		{Symbol: "IREN", Name: "아이렌"},
	}
	best, sc, ok := matchBestCatalog("맵러빈", catalog)
	if !ok || best.Symbol != "APP" {
		t.Fatalf("match failed ok=%v best=%+v score=%v", ok, best, sc)
	}
}

func TestNameMatchScoreMaplebinEnglishCatalog(t *testing.T) {
	score := catalogMatchScore("맵러빈", "Applovin Corporation Class A Common Stock")
	t.Logf("english score=%v", score)
	if score < fuzzyHighConfThreshold {
		t.Fatalf("맵러빈 vs English catalog score=%v want>=%v", score, fuzzyHighConfThreshold)
	}
	if s := catalogMatchScore("맵러빈", "Applied Materials Inc"); s >= fuzzyAutoMatchThreshold {
		t.Fatalf("맵러빈 vs Applied Materials score=%v want low", s)
	}
	catalog := []catalogEntry{
		{Symbol: "APP", Name: "Applovin Corporation Class A Common Stock", EnglishName: "Applovin Corporation Class A Common Stock"},
		{Symbol: "AMAT", Name: "어플라이드 머티리얼즈", EnglishName: "Applied Materials Inc"},
		{Symbol: "AAPL", Name: "애플", EnglishName: "Apple Inc"},
		{Symbol: "IREN", Name: "아이렌"},
	}
	best, sc, ok := matchBestCatalog("맵러빈", catalog)
	if !ok || best.Symbol != "APP" {
		t.Fatalf("english catalog match failed ok=%v best=%+v score=%v", ok, best, sc)
	}
}
