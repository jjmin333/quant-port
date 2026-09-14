package model

// Sync kinds — 종목 단위 적재 배치가 symbol_sync_state에 자기 진행 상황을
// 기록할 때 쓰는 구분자. 문자열을 직접 쓰지 않는 이유는 배치·조회·관리자
// 커버리지 세 곳이 같은 값을 말해야 하기 때문이다.
const (
	SyncKindInsiderDART    = "insider_dart"
	SyncKindInsiderEdgar   = "insider_edgar"
	SyncKindFinancialsDART = "financials_dart"
	SyncKindFinancialsSEC  = "financials_edgar"
	SyncKindWarnings       = "warnings"
	// 종목 지표(stock_fundamentals) 갱신. 분기 재무(financials_*)와 **다른 kind**다 —
	// 같은 DART를 쓰지만 채우는 표가 다르고(지표 한 행 vs 분기 여러 행) 호출 수도
	// 달라, 로테이션 커서를 공유하면 한쪽이 다른 쪽의 순서를 밀어 버린다.
	SyncKindFundamentals = "fundamentals"
	// 일별 종가 구멍 메우기가 Toss에 물어본 결과. `not_found`는 **그 종목을 상대가
	// 취급하지 않는다**는 뜻이고, 30일 쿨다운으로 다시 묻지 않는다.
	SyncKindDailyCloseToss = "daily_close_toss"
)

// Sync outcomes recorded per symbol. `empty` is deliberately distinct from
// `ok`: without it a batch cannot tell "no data exists upstream" from "not
// fetched yet", and would keep re-fetching silent symbols forever — and the
// screen could not tell the user which of the two it is looking at.
const (
	SyncOK       = "ok"
	SyncEmpty    = "empty"
	SyncError    = "error"
	SyncNotFound = "not_found"
)
