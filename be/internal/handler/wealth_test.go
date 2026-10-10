package handler

import (
	"encoding/json"
	"testing"
)

func TestValidateAsset(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	cases := []struct {
		name string
		body assetBody
		ok   bool
	}{
		{"apartment by comps", assetBody{AssetType: "real_estate", Name: "우리 집",
			Detail: json.RawMessage(`{"propertyType":"apt","lawdCd":"11680","area":84.9,"complexKey":"11680-4394"}`)}, true},
		{"apartment without region", assetBody{AssetType: "real_estate", Name: "집",
			Detail: json.RawMessage(`{"propertyType":"apt","area":84}`)}, false},
		{"bad lawd code", assetBody{AssetType: "real_estate", Name: "집",
			Detail: json.RawMessage(`{"propertyType":"apt","lawdCd":"11680';--","area":84}`)}, false},
		{"user estimated needs value", assetBody{AssetType: "real_estate", Name: "상가", ValuationMethod: "user_estimated"}, false},
		{"deposit", assetBody{AssetType: "deposit", Name: "예금",
			Detail: json.RawMessage(`{"principal":10000000,"rate":3.5,"startDate":"2026-01-02","maturityDate":"2027-01-02"}`)}, true},
		{"deposit maturity before start", assetBody{AssetType: "deposit", Name: "예금",
			Detail: json.RawMessage(`{"principal":1,"rate":3,"startDate":"2026-01-02","maturityDate":"2025-01-02"}`)}, false},
		{"rate out of range", assetBody{AssetType: "saving", Name: "적금",
			Detail: json.RawMessage(`{"monthly":100000,"months":12,"rate":45,"startDate":"2026-01-02"}`)}, false},
		{"crypto", assetBody{AssetType: "crypto", Name: "비트코인", Quantity: f(0.5), Detail: json.RawMessage(`{"market":"krw-btc"}`)}, true},
		{"crypto bad market", assetBody{AssetType: "crypto", Name: "x", Quantity: f(1), Detail: json.RawMessage(`{"market":"BTC-ETH"}`)}, false},
		{"gold needs grams", assetBody{AssetType: "gold", Name: "금"}, false},
		{"negative value", assetBody{AssetType: "art", Name: "그림", UserValue: f(-1)}, false},
		{"ownership over 100%", assetBody{AssetType: "art", Name: "그림", UserValue: f(1), OwnershipRatio: f(1.5)}, false},
		{"unknown type", assetBody{AssetType: "stock", Name: "x"}, false},
	}
	for _, c := range cases {
		a, msg := validateAsset(c.body)
		if (msg == "") != c.ok {
			t.Fatalf("%s: ok=%v msg=%q", c.name, c.ok, msg)
		}
		if c.ok && c.body.AssetType == "crypto" && string(a.Detail) != `{"market":"KRW-BTC"}` {
			t.Fatalf("crypto market not normalised: %s", a.Detail)
		}
	}
}
