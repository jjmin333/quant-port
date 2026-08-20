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
