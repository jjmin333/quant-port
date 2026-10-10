package model

/*
「이 종목에 대한 내 기록」 — 흐름을 닫는 한 줄 (V2 요구 9).

# 왜 필요한가

실측(2026-09-19): 종목 화면이 사용자에 대해 아는 것은 `quote.isHeld` **한 비트뿐**
이었다. 내가 그 종목을 관심에 담았는지, 스탬프를 찍었는지, 목표가를 적었는지,
리포트를 썼는지 — 아무것도 모른다. **이미 보유 중인데도 「보유종목 등록」 버튼을
똑같이 내밀고 있었다.**

발견 → 근거 → 판단 → 반영 → 추적의 다섯 단계가 전부 만들어졌는데(Phase 1~6),
그 사이를 오갈 길이 없었다. 이 타입이 그 길이다.

# 없는 것도 남긴다

관심에 안 담았으면 `Watchlisted=false`로 나가고 화면은 그 칩을 **점선으로** 그린다.
지우지 않는 이유: 「아직 안 했다」도 흐름의 한 칸이고, 다음에 할 일을 가리킨다.

# 조회 하나로 끝낸다

다섯 가지를 따로 물으면 종목을 열 때마다 요청이 다섯이다. 전부 `(user_id, symbol)`
단일 조회라 스칼라 서브쿼리로 묶어 **한 번에** 받는다 (PRINCIPLES 「응답 속도 원칙」).
*/

// SymbolContext is what the signed-in user has already done with one symbol.
type SymbolContext struct {
	Symbol string `json:"symbol"`

	// 보유 — 수량이 0이면 보유가 아니다(전량 매도 후 행이 남아 있을 수 있다).
	Held     bool     `json:"held"`
	Quantity float64  `json:"quantity,omitempty"`
	AvgPrice *float64 `json:"avgPrice,omitempty"`
	Currency string   `json:"currency,omitempty"`

	Watchlisted bool `json:"watchlisted"`

	// StampCount / ReportCount — 개수만. 목록은 각자의 화면이 답한다.
	StampCount  int `json:"stampCount"`
	ReportCount int `json:"reportCount"`

	// TargetPrice — 내가 적은 목표가. 없으면 nil이고 화면은 점선 칩을 그린다.
	TargetPrice *float64 `json:"targetPrice,omitempty"`

	/*
		DiagnosisVerdict — 보유 이유 점검의 판정 (`DiagnosisVerdict` 값).

		**보유 중일 때만 채운다.** 안 가진 종목에 「보유 이유」를 말할 수 없고,
		그 계산은 보유 목록을 전제로 한다. 빈 문자열이면 화면은 그 칩을 안 그린다.
	*/
	DiagnosisVerdict string `json:"diagnosisVerdict,omitempty"`
	// DiagnosisSignals — 판정에 쓴 근거 수. 「약화 · 근거 1개」를 적기 위한 값.
	DiagnosisSignals int `json:"diagnosisSignals,omitempty"`
}

// HasAny는 그릴 것이 하나라도 있는지. 전부 비어 있으면 화면은 줄을 안 그린다 —
// 점선 칩 여섯 개만 늘어놓는 것은 안내가 아니라 잡음이다.
func (c SymbolContext) HasAny() bool {
	return c.Held || c.Watchlisted || c.StampCount > 0 || c.ReportCount > 0 || c.TargetPrice != nil
}
