package repository

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestRankRelatedProducts(t *testing.T) {
	items := []model.Stock{
		{Symbol: "QLD", Name: "ProShares Ultra QQQ", AssetType: model.AssetETF},
		{Symbol: "TQQQ", Name: "ProShares UltraPro QQQ", AssetType: model.AssetETF},
	}
	ranked := RankRelatedProducts([]string{"qqq", "sandisk"}, items)
	if len(ranked) != 2 {
		t.Fatalf("expected 2, got %d", len(ranked))
	}
}

func TestCompanyNameAnchors(t *testing.T) {
	anchors := CompanyNameAnchors("Sandisk Corporation Common Stock")
	if len(anchors) == 0 || anchors[0] != "sandisk" {
		t.Fatalf("unexpected anchors: %v", anchors)
	}
}

func TestRelatedProductScoreSymbolAnchor(t *testing.T) {
	items := []model.Stock{
		{Symbol: "SNXX", Name: "SNXX", EnglishName: "TRADR 2X LONG SNDK DAILY ETF", AssetType: model.AssetETF},
	}
	ranked := RankRelatedProducts([]string{"sndk"}, items)
	if len(ranked) != 1 {
		t.Fatalf("expected 1 ranked item, got %d", len(ranked))
	}
}
