package repository

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestSearchRelevanceScore_qqqRelatedTickers(t *testing.T) {
	qqq := model.Stock{Symbol: "QQQ", EnglishName: "Invesco QQQ Trust", AssetType: model.AssetETF}
	tqqq := model.Stock{Symbol: "TQQQ", EnglishName: "ProShares UltraPro QQQ", AssetType: model.AssetETF}
	qld := model.Stock{Symbol: "QLD", EnglishName: "ProShares Ultra QQQ", AssetType: model.AssetETF}
	qqqm := model.Stock{Symbol: "QQQM", EnglishName: "Invesco NASDAQ 100 ETF", AssetType: model.AssetETF}

	if got := searchRelevanceScore("qqq", qqq, true); got < 10000 {
		t.Fatalf("QQQ exact: got %d", got)
	}
	if got := searchRelevanceScore("qqq", qqqm, true); got < 5000 {
		t.Fatalf("QQQM prefix: got %d", got)
	}
	if got := searchRelevanceScore("qqq", tqqq, true); got < 1200 {
		t.Fatalf("TQQQ contains: got %d", got)
	}
	if got := searchRelevanceScore("qqq", qld, true); got < 3500 {
		t.Fatalf("QLD english word: got %d", got)
	}
}

func TestRankSearchResults_qqqOrdersExactBeforeContains(t *testing.T) {
	items := []model.Stock{
		{Symbol: "TQQQ", EnglishName: "ProShares UltraPro QQQ", AssetType: model.AssetETF},
		{Symbol: "QQQM", EnglishName: "Invesco NASDAQ 100 ETF", AssetType: model.AssetETF},
		{Symbol: "QQQ", EnglishName: "Invesco QQQ Trust", AssetType: model.AssetETF},
		{Symbol: "QLD", EnglishName: "ProShares Ultra QQQ", AssetType: model.AssetETF},
	}
	ranked := rankSearchResults(items, "qqq", true)
	if len(ranked) < 4 {
		t.Fatalf("expected 4 results, got %d", len(ranked))
	}
	if ranked[0].Symbol != "QQQ" {
		t.Fatalf("expected QQQ first, got %s", ranked[0].Symbol)
	}
}

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

func TestSearchRelevanceScore_shortLatinMatchesDomesticEnglishPrefix(t *testing.T) {
	skHynix := model.Stock{
		Symbol:      "000660",
		Name:        "SK하이닉스",
		EnglishName: "SK hynix Inc.",
		AssetType:   model.AssetDomestic,
	}
	if got := searchRelevanceScore("sk", skHynix, true); got < 3000 {
		t.Fatalf("expected SK english prefix match for domestic, got %d", got)
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

func TestRankSearchResults_skHynixCommonStockBeforeDerivatives(t *testing.T) {
	items := []model.Stock{
		{Symbol: "0192L0", Name: "RISE SK하이닉스단일종목레버리지", AssetType: model.AssetETF, SecurityType: "ETF"},
		{Symbol: "000660", Name: "SK하이닉스", EnglishName: "SK hynix Inc.", AssetType: model.AssetDomestic, SecurityType: "STOCK"},
		{Symbol: "SKHY", Name: "SK하이닉스(ADR)", AssetType: model.AssetUS, SecurityType: "STOCK"},
		{Symbol: "520101", Name: "미래에셋 레버리지 SK하이닉스 단일종목ETN", AssetType: model.AssetETF, SecurityType: "ETN"},
	}
	ranked := rankSearchResults(items, "sk하이닉스", false)
	if len(ranked) < 4 {
		t.Fatalf("expected 4 results, got %d", len(ranked))
	}
	if ranked[0].Symbol != "000660" {
		t.Fatalf("expected 000660 first, got %s", ranked[0].Symbol)
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
