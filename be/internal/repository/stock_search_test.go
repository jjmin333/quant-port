package repository

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestSearchRelevanceScore_armSkipsPharmaSubstring(t *testing.T) {
	pharma := model.Stock{
		Symbol:      "123456",
		Name:        "Some Pharma",
		EnglishName: "ABC Pharmaceutical Co Ltd",
		AssetType:   model.AssetDomestic,
	}
	if got := searchRelevanceScore("arm", pharma, true); got > 0 {
		t.Fatalf("expected 0 for pharma english substring, got %d", got)
	}

	arm := model.Stock{
		Symbol:      "ARM",
		Name:        "Arm Holdings",
		EnglishName: "Arm Holdings plc American Depositary Shares",
		AssetType:   model.AssetUS,
	}
	if got := searchRelevanceScore("arm", arm, true); got < 5000 {
		t.Fatalf("expected high score for ARM ticker, got %d", got)
	}
}

func TestSearchRelevanceScore_latinExcludesDomesticNameMatch(t *testing.T) {
	domestic := model.Stock{
		Symbol:      "005930",
		Name:        "Samsung Electronics",
		EnglishName: "Samsung Electronics Co Ltd",
		AssetType:   model.AssetDomestic,
	}
	if got := searchRelevanceScore("samsung", domestic, true); got > 0 {
		t.Fatalf("latin query should not rank domestic via english name, got %d", got)
	}
}

func TestSearchRelevanceScore_koreanNameMatch(t *testing.T) {
	oracle := model.Stock{
		Symbol:      "ORCL",
		Name:        "오라클",
		EnglishName: "Oracle Corporation Common Stock",
		AssetType:   model.AssetUS,
	}
	if got := searchRelevanceScore("오라클", oracle, false); got < 4000 {
		t.Fatalf("expected korean name match, got %d", got)
	}
}

func TestRankSearchResults_popularityTiebreak(t *testing.T) {
	large := int64(10_000_000_000)
	small := int64(100_000_000)
	items := []model.Stock{
		{Symbol: "AAA", EnglishName: "Alpha Corporation", AssetType: model.AssetUS, SharesOutstanding: &small},
		{Symbol: "AAB", EnglishName: "Alpha Beta Corporation", AssetType: model.AssetUS, SharesOutstanding: &large},
	}
	ranked := rankSearchResults(items, "alpha", true)
	if len(ranked) != 2 {
		t.Fatalf("expected 2 results, got %d", len(ranked))
	}
	if ranked[0].Symbol != "AAB" {
		t.Fatalf("expected larger-cap AAB first, got %s", ranked[0].Symbol)
	}
}

func TestIsLatinOnlyQuery(t *testing.T) {
	if !IsLatinOnlyQuery("oracle") {
		t.Fatal("oracle should be latin-only")
	}
	if IsLatinOnlyQuery("오라클") {
		t.Fatal("hangul query should not be latin-only")
	}
}
