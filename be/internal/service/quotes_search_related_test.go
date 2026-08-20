package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestRelatedSearchAnchors(t *testing.T) {
	primary := model.Stock{
		Symbol:      "SNDK",
		Name:        "샌디스크",
		EnglishName: "Sandisk Corporation Common Stock",
	}
	anchors := relatedSearchAnchors("샌디스크", primary)
	if len(anchors) < 3 {
		t.Fatalf("expected hangul + symbol + english anchors, got %v", anchors)
	}
	hasSymbol := false
	for _, a := range anchors {
		if a == "sndk" {
			hasSymbol = true
		}
	}
	if !hasSymbol {
		t.Fatalf("expected sndk anchor in %v", anchors)
	}
}

func TestPickSearchPrimary_prefersCommonStockOverADR(t *testing.T) {
	items := []model.Stock{
		{Symbol: "SKHY", Name: "SK하이닉스(ADR)", AssetType: model.AssetUS},
		{Symbol: "000660", Name: "SK하이닉스", AssetType: model.AssetDomestic},
	}
	got := pickSearchPrimary(items)
	if got.Symbol != "000660" {
		t.Fatalf("expected domestic common stock, got %s", got.Symbol)
	}
}

func TestMergeSearchResultsPrimaryFirst(t *testing.T) {
	primary := []model.Stock{{Symbol: "QQQ", Name: "Invesco QQQ Trust"}}
	related := []model.Stock{
		{Symbol: "TQQQ", Name: "ProShares UltraPro QQQ"},
		{Symbol: "QLD", Name: "ProShares Ultra QQQ"},
	}
	merged := mergeSearchResults(primary, related, 5)
	if len(merged) != 3 || merged[0].Symbol != "QQQ" {
		t.Fatalf("unexpected merge: %+v", merged)
	}
}
