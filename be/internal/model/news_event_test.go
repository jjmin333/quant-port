package model

import "testing"

func TestClassifyNewsEvent(t *testing.T) {
	cases := []struct {
		headline string
		want     NewsEventType
	}{
		{"한국투자證, 삼성전자 목표가 65만원 상향", NewsEventTargetUp},
		{"SK하이닉스 목표주가 하향 조정…HBM 둔화 우려", NewsEventTargetDown},
		{"실적 발표 후 목표가 상향 잇따라", NewsEventTargetUp}, // 목표가가 실적보다 먼저다
		{"카카오, 3분기 영업이익 시장 예상 상회", NewsEventEarnings},
		{"에코프로비엠, 2조원 규모 공급계약 체결", NewsEventContract},
		{"XX바이오 300억 유상증자 결정", NewsEventCapitalRaise},
		{"현대차, 1조 자사주 소각 발표", NewsEventShareholder},
		{"공정위, OO에 과징금 120억", NewsEventLegal},
		{"Morgan Stanley raises price target on NVIDIA to $200", NewsEventTargetUp},
		{"Apple beats estimates as iPhone sales jump", NewsEventEarnings},
		{"Microsoft to acquire gaming studio for $2 billion", NewsEventMnA},
		{"Tesla announces $10 billion share buyback", NewsEventShareholder},
		{"Analyst downgrades Intel to sell", NewsEventTargetDown},
		{"코스피, 외국인 매수에 2,700선 회복", NewsEventNone},
	}
	for _, c := range cases {
		if got := ClassifyNewsEvent(c.headline); got != c.want {
			t.Errorf("%q: got %q want %q", c.headline, got, c.want)
		}
	}
}

func TestClassifyDisclosureEvent(t *testing.T) {
	cases := map[string]NewsEventType{
		"단일판매ㆍ공급계약체결":            NewsEventContract,
		"주요사항보고서(유상증자결정)":        NewsEventCapitalRaise,
		"주요사항보고서(전환사채권발행결정)":     NewsEventCapitalRaise,
		"주요사항보고서(자기주식취득결정)":      NewsEventShareholder,
		"타법인주식및출자증권취득결정":         NewsEventMnA,
		"최대주주변경을수반하는주식담보제공계약체결":  NewsEventControlChange,
		"주권매매거래정지해제":             NewsEventNone, // 풀린 것은 정지가 아니다
		"[기재정정]주권매매거래정지":         NewsEventTradingHalt,
		"연결재무제표기준영업(잠정)실적(공정공시)": NewsEventEarnings,
		"임원ㆍ주요주주특정증권등소유상황보고서":    NewsEventNone, // 절차성
		"기업설명회(IR)개최(안내공시)":      NewsEventNone,
	}
	for title, want := range cases {
		if got := ClassifyDisclosureEvent(title); got != want {
			t.Errorf("%q: got %q want %q", title, got, want)
		}
	}
}
