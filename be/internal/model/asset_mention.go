package model

import (
	"strings"
	"time"
)

/*
글 안의 자산 태그 (2026-10-03 사용자 결정 ③ — docs/mockups/asset-ai-tags-v1.html).

	@아파트[송파구 헬리오시티]          단지 칩(최근 실거래)
	@아파트[송파구 헬리오시티]:전세가율  그 지표 값 — 쓴 순간 값 + 달라졌으면 지금 값(종목 태그와 같은 규칙)
	@지역[송파구]:4주 · @상품[우리은행 WON플러스예금]:최고금리 · @금리[국고채 3년]
	#아파트[…] · #상품[…]              그 줄 아래 카드

종류 이름(아파트·지역·상품·금리)을 대괄호 앞에 붙여 종목 이름과 겹치지 않게 한다. 공매는 넣지 않는다(사용자 결정).
*/

const (
	AssetKindApt     = "apt"
	AssetKindRegion  = "region"
	AssetKindProduct = "product"
	AssetKindRate    = "rate"
	// AssetKindSido — 시·도와 전국(키: 법정동 앞 두 자리, 전국 "00"). 기사 칩·미리보기 전용 — 글 태그 낱말은 없다(2026-10-10).
	AssetKindSido = "sido"
)

// AssetKindWords — 태그 글자 → 종류. 순서는 정규식과 같게(서비스·FE가 같은 낱말을 쓴다).
var AssetKindWords = []struct{ Word, Kind string }{
	{"아파트", AssetKindApt}, {"지역", AssetKindRegion}, {"상품", AssetKindProduct}, {"금리", AssetKindRate},
}

// MentionMaxAssets — 글 하나에 태그할 수 있는 자산 수(종목 10개와 따로).
const MentionMaxAssets = 10

// AssetMention — 태그된 자산 하나.
type AssetMention struct {
	Kind string `json:"kind"`
	Key  string `json:"key"`
	Name string `json:"name"`
	// Sub — 이름 옆 한 줄(지역 · 금융회사).
	Sub  string `json:"sub,omitempty"`
	Href string `json:"href"`
	// Snap/Now — 지표 id(apt.jeonse …) → 값. Now는 미리보기에서 비운다.
	Snap   map[string]float64 `json:"snap"`
	Now    map[string]float64 `json:"now,omitempty"`
	SnapAt *time.Time         `json:"snapAt,omitempty"`
	Score  *int               `json:"score,omitempty"`
	Grade  string             `json:"grade,omitempty"`
	// Note — 저장된 AI 한 줄(단지·지역 해설, 상품 우대조건 정리). 조회에서 LLM을 부르지 않는다.
	Note string `json:"note,omitempty"`
	// Extra — 모달이 쓰는 글자(최근 거래 「09-21 · 12층 · 84㎡」, 금리 기준일 등).
	Extra map[string]string `json:"extra,omitempty"`
}

type assetFieldDef struct {
	field   ScreenField
	aliases []string
}

func af(kind, id, label, unit string, decimals int, higher bool, aliases ...string) assetFieldDef {
	return assetFieldDef{ScreenField{ID: kind + "." + id, Label: label, Unit: unit, Decimals: decimals, HigherIsBetter: higher}, append([]string{label}, aliases...)}
}

var assetFieldDefs = []assetFieldDef{
	af(AssetKindApt, "price", "최근거래", "원", 0, true, "실거래", "거래가", "가격"),
	af(AssetKindApt, "pyeong", "평당가", "원", 0, true),
	af(AssetKindApt, "jeonse", "전세가율", "%", 0, true, "전세"),
	af(AssetKindApt, "peak", "고점대비", "%", 0, true, "고점"),
	af(AssetKindApt, "mom", "3개월변화", "%", 1, true, "3개월", "변화", "모멘텀"),
	af(AssetKindApt, "trades", "6개월거래", "건", 0, true, "거래"),
	af(AssetKindApt, "score", "점수", "점", 0, true),
	af(AssetKindRegion, "w1", "1주", "%", 2, true),
	af(AssetKindRegion, "w4", "4주", "%", 2, true),
	af(AssetKindRegion, "w12", "12주", "%", 2, true),
	af(AssetKindRegion, "jeonse", "전세4주", "%", 2, true, "전세"),
	af(AssetKindRegion, "vol", "거래량", "%", 0, true, "거래"),
	af(AssetKindRegion, "score", "점수", "점", 0, true),
	af(AssetKindSido, "w1", "1주", "%", 2, true),
	af(AssetKindSido, "w4", "4주", "%", 2, true),
	af(AssetKindSido, "w12", "12주", "%", 2, true),
	af(AssetKindSido, "jeonse", "전세4주", "%", 2, true, "전세"),
	af(AssetKindSido, "vol", "거래량", "%", 0, true, "거래"),
	af(AssetKindSido, "score", "점수", "점", 0, true),
	af(AssetKindProduct, "base", "기본금리", "%", 2, true, "기본"),
	af(AssetKindProduct, "max", "최고금리", "%", 2, true, "최고"),
	af(AssetKindProduct, "score", "점수", "점", 0, true),
	af(AssetKindRate, "value", "금리", "%", 2, false, "값"),
}

// LookupAssetField returns one asset field by id (apt.jeonse …).
func LookupAssetField(id string) (ScreenField, bool) {
	for _, d := range assetFieldDefs {
		if d.field.ID == id {
			return d.field, true
		}
	}
	return ScreenField{}, false
}

// AssetFieldsOf lists a kind's fields (편집기 자동완성·도움말).
func AssetFieldsOf(kind string) []ScreenField {
	var out []ScreenField
	for _, d := range assetFieldDefs {
		if strings.HasPrefix(d.field.ID, kind+".") {
			out = append(out, d.field)
		}
	}
	return out
}

/*
MatchAssetFieldAlias — 그 종류의 지표 이름 중 run의 **가장 긴 앞부분**과 같은 것. 「전세가율이」→ apt.jeonse, 4.
「거래가」는 「거래」(6개월거래)보다 길어서 최근거래로 간다.
*/
func MatchAssetFieldAlias(kind, run string) (string, int) {
	runes := []rune(run)
	for n := len(runes); n > 0; n-- {
		key := mentionAliasKey(string(runes[:n]))
		for _, d := range assetFieldDefs {
			if !strings.HasPrefix(d.field.ID, kind+".") {
				continue
			}
			for _, a := range d.aliases {
				if mentionAliasKey(a) == key {
					return d.field.ID, n
				}
			}
		}
	}
	return "", 0
}
