package model

import "time"

/*
글 안의 데이터 태그 (2026-09-28 사용자 요청 2).

	@삼성전자        본문 속 종목 칩
	@삼성전자:점수   그 지표 값 — 쓴 순간 값 + 달라졌으면 지금 값
	#SK하이닉스      그 줄 아래 종목 카드
	@[TIGER 미국S&P500]  이름에 띄어쓰기가 있으면 대괄호

지표 이름은 스크리너 필드(`ScreenFields`)의 Label·Aliases다 — 두 화면이 같은 말을 쓴다.
*/

// MentionMaxSymbols — 글 하나에 태그할 수 있는 종목 수.
const MentionMaxSymbols = 10

// Mention target types.
const (
	MentionTargetCommunityPost = "community_post"
	MentionTargetResearchCall  = "research_call"
)

// MentionToken — 본문에서 찾은 토큰 하나를 화면이 그대로 다시 찾도록 알려 준다.
// Raw는 정규식이 잡은 글자 그대로, Len은 그중 태그가 차지하는 글자 수(룬)다 —
// 「@삼성전자가」의 「가」처럼 뒤에 붙은 조사는 태그가 아니다.
type MentionToken struct {
	Raw    string `json:"raw"`
	Len    int    `json:"len"`
	Symbol string `json:"symbol"`
	Field  string `json:"field,omitempty"`
	Card   bool   `json:"card,omitempty"`
	// Kind/Key — 자산 태그(2026-10-03)일 때만. Symbol은 빈칸, Field는 apt.jeonse 같은 자산 지표 id.
	Kind string `json:"kind,omitempty"`
	Key  string `json:"key,omitempty"`
}

// ContentMention — 태그된 종목 하나. 카드가 기존 CommunitySymbol 모양을 그대로 쓴다.
type ContentMention struct {
	CommunitySymbol
	// Snap/Now — 스크리너 필드 id → 값(화면 단위, 시가총액은 조).
	Snap map[string]float64 `json:"snap"`
	Now  map[string]float64 `json:"now,omitempty"`
}

// MentionSet is what a detail view carries.
type MentionSet struct {
	Tokens  []MentionToken   `json:"tokens"`
	Symbols []ContentMention `json:"symbols"`
	// Fields — 토큰이 부른 지표의 이름·단위·자릿수(화면이 카탈로그를 따로 받지 않게). 자산 지표(apt.* …)도 여기.
	Fields map[string]ScreenField `json:"fields,omitempty"`
	// Assets — 자산 태그(단지·지역·상품·금리). 없으면 빠진다.
	Assets []AssetMention `json:"assets,omitempty"`
}

// FillFields sets Fields from the tokens.
func (m *MentionSet) FillFields() {
	for _, t := range m.Tokens {
		if t.Field == "" {
			continue
		}
		f, ok := LookupScreenField(t.Field)
		if !ok {
			f, ok = LookupAssetField(t.Field)
		}
		if ok {
			if m.Fields == nil {
				m.Fields = map[string]ScreenField{}
			}
			m.Fields[f.ID] = f
		}
	}
	// 자산 카드·모달은 토큰이 부르지 않은 지표도 그린다 — 값이 있는 자산 지표는 전부 이름·단위를 싣는다.
	for _, a := range m.Assets {
		for _, vals := range []map[string]float64{a.Snap, a.Now} {
			for id := range vals {
				if f, ok := LookupAssetField(id); ok {
					if m.Fields == nil {
						m.Fields = map[string]ScreenField{}
					}
					m.Fields[id] = f
				}
			}
		}
	}
}

// MentionSnap is the stored JSON.
type MentionSnap struct {
	Fields     map[string]float64 `json:"fields"`
	Price      *float64           `json:"price,omitempty"`
	ChangeRate *float64           `json:"changeRate,omitempty"`
	Currency   string             `json:"currency,omitempty"`
	Score      *int               `json:"score,omitempty"`
	At         time.Time          `json:"-"`
}
