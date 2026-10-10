package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

// 자산 태그(2026-10-03) — 종류 낱말 + 대괄호, 지표, 카드. 종목 해석은 자산 토큰을 건드리지 않는다.
func TestAssetMentionParseResolve(t *testing.T) {
	body := "요즘 @아파트[송파구 헬리오시티]:전세가율이 52%, @지역[송파구]:4주 흐름. #상품[우리은행 WON플러스예금] 그리고 @금리[국고채 3년]은? @삼성전자도"
	parsed := parseMentions(body)
	ix := &assetNameIndex{byText: map[string]map[string]string{}, entries: map[string]assetEntry{}}
	ix.add(assetEntry{kind: model.AssetKindApt, key: "11710-1", name: "헬리오시티"}, "송파구 헬리오시티", "헬리오시티")
	ix.add(assetEntry{kind: model.AssetKindRegion, key: "11710", name: "서울 송파구"}, "송파구")
	ix.add(assetEntry{kind: model.AssetKindProduct, key: "42", name: "WON플러스예금"}, "우리은행 WON플러스예금")
	ix.add(assetEntry{kind: model.AssetKindRate, key: "ktb3", name: "국고채 3년"}, "국고채 3년")

	tokens, names := resolveAssetMentions(parsed, ix.lookup)
	if len(tokens) != 4 {
		t.Fatalf("tokens: %+v", tokens)
	}
	apt := tokens[0]
	if apt.Kind != "apt" || apt.Key != "11710-1" || apt.Field != "apt.jeonse" || apt.Card {
		t.Fatalf("apt: %+v", apt)
	}
	// 「@아파트[송파구 헬리오시티]:전세가율」까지가 태그, 뒤의 「이」는 글자.
	if want := len([]rune("@아파트[송파구 헬리오시티]:전세가율")); apt.Len != want {
		t.Fatalf("apt len %d want %d", apt.Len, want)
	}
	if tokens[1].Field != "region.w4" || !tokens[2].Card || tokens[2].Kind != "product" || tokens[3].Key != "ktb3" {
		t.Fatalf("others: %+v", tokens[1:])
	}
	if got := names["apt|11710-1"]; len(got) != 1 || got[0] != "송파구 헬리오시티" {
		t.Fatalf("names: %+v", names)
	}
	// 종목 해석은 자산 토큰을 보지 않는다.
	stockTokens, _ := resolveMentions(parsed, func(n string) (string, bool) { return "005930", n == "삼성전자" })
	if len(stockTokens) != 1 || stockTokens[0].Symbol != "005930" {
		t.Fatalf("stock tokens: %+v", stockTokens)
	}
	if got := assetMentionOrder(tokens); len(got) != 4 || got[0] != "apt|11710-1" {
		t.Fatalf("order: %v", got)
	}
}

func TestAssetFieldAliasAndPlainText(t *testing.T) {
	if id, n := model.MatchAssetFieldAlias("apt", "거래가는"); id != "apt.price" || n != 3 {
		t.Fatalf("거래가: %s %d", id, n)
	}
	if id, _ := model.MatchAssetFieldAlias("apt", "거래"); id != "apt.trades" {
		t.Fatalf("거래: %s", id)
	}
	if id, _ := model.MatchAssetFieldAlias("region", "전세가율"); id != "region.jeonse" {
		t.Fatalf("region 전세: %s", id)
	}
	if id, _ := model.MatchAssetFieldAlias("rate", "4주"); id != "" {
		t.Fatalf("rate 4주 should not match: %s", id)
	}
	if got := plainMentionText("@아파트[송파구 헬리오시티]:전세가율 좋다"); got != "송파구 헬리오시티 전세가율 좋다" {
		t.Fatalf("plain: %q", got)
	}
}

func TestAssetHref(t *testing.T) {
	for key, want := range map[string]string{
		"11710-4321":      "/real-estate?complex=11710-4321&lawd=11710",
		"41135|정자동|1|파크뷰": "/real-estate?complex=41135%7C%EC%A0%95%EC%9E%90%EB%8F%99%7C1%7C%ED%8C%8C%ED%81%AC%EB%B7%B0&lawd=41135",
	} {
		if got := assetHref("apt", key); got != want {
			t.Errorf("%s → %s want %s", key, got, want)
		}
	}
	if assetHref("product", "42") != "/finance/products/42" || assetHref("rate", "ktb3") != "/finance/rates" {
		t.Fatal("product/rate href")
	}
}
