package model

// StockWarningRow is one 지정 상태(투자주의·투자경고·거래정지 등) as stored.
//
// 소스(Toss) 응답의 모양을 그대로 담는다. 화면이 이 값을 표시만 하므로,
// 날짜를 시간 타입으로 정규화하지 않는다 — 형식이 우리 통제 밖이고, 파싱에
// 실패하면 그 종목의 경고가 통째로 사라진다.
type StockWarningRow struct {
	WarningType string  `json:"warningType"`
	Exchange    *string `json:"exchange"`
	StartDate   *string `json:"startDate"`
	EndDate     *string `json:"endDate"`
}
