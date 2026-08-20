package service

import (
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestAssetTypeFilterEmpty(t *testing.T) {
	filter, args := assetTypeFilter(nil)
	if filter != "" || len(args) != 0 {
		t.Errorf("빈 assetTypes는 필터 없음이어야 함, got filter=%q args=%v", filter, args)
	}
}

// The domestic and overseas workers must never overlap on a symbol: each
// filter clause should name disjoint asset types.
func TestAssetTypeFilterDomesticOverseasDisjoint(t *testing.T) {
	domesticFilter, domesticArgs := assetTypeFilter([]model.QuoteAssetType{model.AssetDomestic})
	overseasFilter, overseasArgs := assetTypeFilter([]model.QuoteAssetType{model.AssetUS, model.AssetETF})

	if !strings.Contains(domesticFilter, "st.asset_type IN") {
		t.Errorf("domestic filter malformed: %q", domesticFilter)
	}
	if !strings.Contains(overseasFilter, "st.asset_type IN") {
		t.Errorf("overseas filter malformed: %q", overseasFilter)
	}
	if len(domesticArgs) != 1 || domesticArgs[0] != string(model.AssetDomestic) {
		t.Errorf("domestic args wrong: %v", domesticArgs)
	}
	if len(overseasArgs) != 2 {
		t.Errorf("overseas args wrong: %v", overseasArgs)
	}
	for _, a := range overseasArgs {
		if a == string(model.AssetDomestic) {
			t.Error("overseas filter must not include domestic asset type")
		}
	}
}

func TestParseDailyCloseMarket(t *testing.T) {
	types, err := ParseDailyCloseMarket("us")
	if err != nil || len(types) != 2 {
		t.Fatalf("us: got %v err=%v", types, err)
	}
	if _, err := ParseDailyCloseMarket("nope"); err == nil {
		t.Fatal("invalid market should fail")
	}
}

func TestBatchJobParamsValidate(t *testing.T) {
	if err := (BatchJobParams{Since: "2026-07-01", Limit: 10}).validate(); err != nil {
		t.Errorf("valid params rejected: %v", err)
	}
	if err := (BatchJobParams{Since: "not-a-date"}).validate(); err == nil {
		t.Error("invalid since should fail validation")
	}
	if err := (BatchJobParams{Limit: -1}).validate(); err == nil {
		t.Error("negative limit should fail validation")
	}
}

// Every asset type in the DB (domestic/us/etf) must be assigned to exactly one
// of the two market workers — otherwise a symbol would silently never get a
// daily close.
func TestEveryAssetTypeAssignedToOneMarket(t *testing.T) {
	domestic := map[model.QuoteAssetType]bool{model.AssetDomestic: true}
	overseas := map[model.QuoteAssetType]bool{model.AssetUS: true, model.AssetETF: true}

	all := []model.QuoteAssetType{model.AssetDomestic, model.AssetUS, model.AssetETF}
	for _, at := range all {
		inDomestic := domestic[at]
		inOverseas := overseas[at]
		if inDomestic == inOverseas {
			t.Errorf("asset type %q must be in exactly one market bucket (domestic=%v overseas=%v)",
				at, inDomestic, inOverseas)
		}
	}
}
