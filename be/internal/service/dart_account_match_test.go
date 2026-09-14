package service

import (
	"fmt"
	"testing"
)

func TestNormalizeDartAccountName(t *testing.T) {
	cases := []struct{ in, want string }{
		// 괄호는 통째로 사라진다 — DART가 그 안에 넣는 「(손실)」은 그 보고서에서만
		// 유효한 부호 정보이지 계정을 식별하는 정보가 아니다.
		{"영업이익(손실)", "영업이익"},
		{"수익(매출액)", "수익"},
		// 기간 표지가 사라져 같은 개념이 한 이름으로 모인다.
		{"당기순이익", "순이익"},
		{"분기순이익", "순이익"},
		{"반기순이익", "순이익"},
		// 실측 두 건 — 예전 매칭이 놓쳤던 이름들.
		{"기본주당순이익(손실)", "기본주당순이익"},
		{"기본주당분기순이익(손실)", "기본주당순이익"},
		{"희석주당분기순이익(손실)", "희석주당순이익"},
		// 공백·구분자 제거.
		{" 자산 총계 ", "자산총계"},
		{"기본및희석주당이익", "기본및희석주당이익"},
	}
	for _, c := range cases {
		if got := normalizeDartAccountName(c.in); got != c.want {
			t.Errorf("normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// 실측 응답 그대로. SK하이닉스 2025 연간(11011, CFS) 손익 항목 일부.
func TestMapDartRowsSKHynixAnnual(t *testing.T) {
	rows := []dartAccountRow{
		{StatementDiv: "BS", AccountName: "매출채권", Amount: "18199078000000"},
		{StatementDiv: "CIS", AccountName: "영업이익(손실)", Amount: "47206319000000"},
		{StatementDiv: "CIS", AccountName: "기타영업외수익", Amount: "333277000000"},
		{StatementDiv: "CIS", AccountName: "매출원가", Amount: "38455885000000"},
		{StatementDiv: "CIS", AccountName: "기본주당순이익(손실)", Amount: "62044"},
		{StatementDiv: "CIS", AccountName: "희석주당순이익(손실)", Amount: "60378"},
		{StatementDiv: "CIS", AccountName: "금융수익", Amount: "16373480000000"},
		{StatementDiv: "CIS", AccountName: "매출총이익(손실)", Amount: "58690790000000"},
		{StatementDiv: "CIS", AccountName: "매출액", Amount: "97146675000000"},
		{StatementDiv: "CIS", AccountName: "당기순이익(손실)", Amount: "45000000000000"},
		{StatementDiv: "BS", AccountName: "자산총계", Amount: "200000000000000"},
	}
	snap := mapDartRows(rows)

	if snap.Revenue == nil || *snap.Revenue != 97146675000000 {
		t.Fatalf("Revenue = %v, want 97146675000000", snap.Revenue)
	}
	if snap.OperatingIncome == nil || *snap.OperatingIncome != 47206319000000 {
		t.Fatalf("OperatingIncome = %v", snap.OperatingIncome)
	}
	// 예전에는 여기가 nil이었다 — 사전에 `기본주당순이익`이 없었다.
	if snap.EPSBasic == nil || *snap.EPSBasic != 62044 {
		t.Fatalf("EPSBasic = %v, want 62044", snap.EPSBasic)
	}
	if snap.EPSDiluted == nil || *snap.EPSDiluted != 60378 {
		t.Fatalf("EPSDiluted = %v, want 60378", snap.EPSDiluted)
	}
	if snap.NetIncome == nil || *snap.NetIncome != 45000000000000 {
		t.Fatalf("NetIncome = %v", snap.NetIncome)
	}
	if snap.TotalAssets == nil || *snap.TotalAssets != 200000000000000 {
		t.Fatalf("TotalAssets = %v", snap.TotalAssets)
	}
}

// SK하이닉스 2024 1Q(11013, CFS) — 매출을 `영업수익`으로 적은 분기.
// 이 한 분기 때문에 차트 중간에 구멍이 났다.
func TestMapDartRowsRevenueAsYeongeopSuik(t *testing.T) {
	rows := []dartAccountRow{
		{StatementDiv: "BS", AccountName: "매출채권", Amount: "7178507000000"},
		{StatementDiv: "CIS", AccountName: "영업이익(손실)", Amount: "2886029000000", AddAmount: "2886029000000"},
		{StatementDiv: "CIS", AccountName: "매출원가", Amount: "7634953000000"},
		{StatementDiv: "CIS", AccountName: "기본주당분기순이익(손실)", Amount: "2788", AddAmount: "2788"},
		{StatementDiv: "CIS", AccountName: "금융수익", Amount: "1117553000000"},
		{StatementDiv: "CIS", AccountName: "매출총이익(손실)", Amount: "4794645000000"},
		{StatementDiv: "CIS", AccountName: "영업수익", Amount: "12429598000000", AddAmount: "12429598000000"},
	}
	snap := mapDartRows(rows)
	if snap.Revenue == nil || *snap.Revenue != 12429598000000 {
		t.Fatalf("Revenue = %v, want 12429598000000 (영업수익)", snap.Revenue)
	}
	if snap.EPSBasic == nil || *snap.EPSBasic != 2788 {
		t.Fatalf("EPSBasic = %v, want 2788", snap.EPSBasic)
	}

	// 누적 열도 같은 규칙을 쓴다 — Q4를 「연간 − 3분기 누적」으로 만들 때 쓰인다.
	cum := mapDartCumulativeRows(rows)
	if cum.Revenue == nil || *cum.Revenue != 12429598000000 {
		t.Fatalf("cumulative Revenue = %v", cum.Revenue)
	}
}

// 매출과 헷갈리기 쉬운 이름들이 매출로 잡히면 안 된다. 빈칸보다 나쁜 것은
// 틀린 값이다.
func TestMapDartRowsRejectsLookalikes(t *testing.T) {
	rows := []dartAccountRow{
		{StatementDiv: "BS", AccountName: "매출채권", Amount: "999"},
		{StatementDiv: "CIS", AccountName: "매출원가", Amount: "888"},
		{StatementDiv: "CIS", AccountName: "매출총이익(손실)", Amount: "777"},
		{StatementDiv: "CIS", AccountName: "금융수익", Amount: "666"},
		{StatementDiv: "CIS", AccountName: "기타영업외수익", Amount: "555"},
		{StatementDiv: "CIS", AccountName: "기타수익", Amount: "444"},
	}
	snap := mapDartRows(rows)
	if snap.Revenue != nil {
		t.Fatalf("Revenue = %v, want nil (매출로 볼 행이 없다)", *snap.Revenue)
	}
	if snap.OperatingIncome != nil {
		t.Fatalf("OperatingIncome = %v, want nil", *snap.OperatingIncome)
	}
}

// 손익 항목을 재무상태표에서 집지 않는다.
func TestMapDartRowsRespectsStatementDivision(t *testing.T) {
	rows := []dartAccountRow{
		// 같은 이름이 BS에 있어도 매출로 쓰지 않는다.
		{StatementDiv: "BS", AccountName: "매출액", Amount: "111"},
		{StatementDiv: "CIS", AccountName: "매출액", Amount: "222"},
		// 잔액 항목이 손익계산서에 있어도 쓰지 않는다.
		{StatementDiv: "CIS", AccountName: "자본총계", Amount: "333"},
	}
	snap := mapDartRows(rows)
	if snap.Revenue == nil || *snap.Revenue != 222 {
		t.Fatalf("Revenue = %v, want 222 (CIS 행)", snap.Revenue)
	}
	if snap.TotalEquity != nil {
		t.Fatalf("TotalEquity = %v, want nil (BS 행이 없다)", *snap.TotalEquity)
	}
}

// sj_div가 없는 응답(옛 픽스처)에서는 거르지 않는다 — 예전 동작 유지.
func TestMapDartRowsWithoutStatementDivision(t *testing.T) {
	rows := []dartAccountRow{
		{AccountName: "매출액", Amount: "1000"},
		{AccountName: "당기순이익", Amount: "100"},
		{AccountName: "자본총계", Amount: "500"},
	}
	snap := mapDartRows(rows)
	if snap.Revenue == nil || *snap.Revenue != 1000 {
		t.Fatalf("Revenue = %v", snap.Revenue)
	}
	if snap.NetIncome == nil || *snap.NetIncome != 100 {
		t.Fatalf("NetIncome = %v", snap.NetIncome)
	}
	if snap.TotalEquity == nil || *snap.TotalEquity != 500 {
		t.Fatalf("TotalEquity = %v", snap.TotalEquity)
	}
}

// 계층: 총액이 있으면 귀속분을 쓰지 않고, 총액이 없을 때만 내려간다.
func TestMapDartRowsNetIncomeTiers(t *testing.T) {
	both := []dartAccountRow{
		{StatementDiv: "CIS", AccountName: "당기순이익(손실)", Amount: "1000"},
		{StatementDiv: "CIS", AccountName: "지배기업의 소유주에게 귀속되는 당기순이익", Amount: "900"},
	}
	if snap := mapDartRows(both); snap.NetIncome == nil || *snap.NetIncome != 1000 {
		t.Fatalf("NetIncome = %v, want 1000 (총액 우선)", snap.NetIncome)
	}

	onlyAttributable := []dartAccountRow{
		{StatementDiv: "CIS", AccountName: "지배기업의 소유주에게 귀속되는 당기순이익", Amount: "900"},
	}
	if snap := mapDartRows(onlyAttributable); snap.NetIncome == nil || *snap.NetIncome != 900 {
		t.Fatalf("NetIncome = %v, want 900 (2순위로 내려간다)", snap.NetIncome)
	}
}

// EPS도 계층이 있다: `기본`이 붙은 이름이 없을 때만 `주당순이익`을 본다.
func TestMapDartRowsEPSTiers(t *testing.T) {
	both := []dartAccountRow{
		{StatementDiv: "CIS", AccountName: "기본주당이익", Amount: "10"},
		{StatementDiv: "CIS", AccountName: "주당순이익", Amount: "20"},
	}
	if snap := mapDartRows(both); snap.EPSBasic == nil || *snap.EPSBasic != 10 {
		t.Fatalf("EPSBasic = %v, want 10 (기본 우선)", snap.EPSBasic)
	}

	plain := []dartAccountRow{{StatementDiv: "CIS", AccountName: "주당순이익", Amount: "20"}}
	if snap := mapDartRows(plain); snap.EPSBasic == nil || *snap.EPSBasic != 20 {
		t.Fatalf("EPSBasic = %v, want 20", snap.EPSBasic)
	}
}

// 빈 금액 문자열은 값이 아니다 — 0으로 읽으면 「이익 0」이라는 없는 사실이 된다.
func TestPickDartTieredSkipsEmptyAmounts(t *testing.T) {
	rows := []dartAccountRow{
		{StatementDiv: "CIS", AccountName: "매출액", Amount: "   "},
	}
	if v := pickDartTiered(rows, "revenue", dartAmountCurrent); v != nil {
		t.Fatalf("revenue = %v, want nil", *v)
	}
	// 누적 열이 비는 것은 연간보고서에서 정상이다.
	rows2 := []dartAccountRow{
		{StatementDiv: "CIS", AccountName: "매출액", Amount: "500", AddAmount: ""},
	}
	if v := pickDartTiered(rows2, "revenue", dartAmountAdd); v != nil {
		t.Fatalf("cumulative revenue = %v, want nil", *v)
	}
}

// 제일파마홀딩스(002620) 2025 연간 — EPS를 「지배기업 소유주지분 기본주당손익」으로
// 적었다. IFRS 주당이익은 원래 지배기업 소유주 귀속 기준이라, 접두사만 붙었을 뿐
// 개념이 다른 값이 아니다.
func TestMapDartRowsEPSWithOwnerPrefix(t *testing.T) {
	rows := []dartAccountRow{
		{StatementDiv: "CIS", AccountName: "지배기업 소유주지분 기본주당손익", Amount: "953"},
		{StatementDiv: "CIS", AccountName: "지배기업 소유주지분 희석주당손익", Amount: "950"},
	}
	snap := mapDartRows(rows)
	if snap.EPSBasic == nil || *snap.EPSBasic != 953 {
		t.Fatalf("EPSBasic = %v, want 953", snap.EPSBasic)
	}
	if snap.EPSDiluted == nil || *snap.EPSDiluted != 950 {
		t.Fatalf("EPSDiluted = %v, want 950", snap.EPSDiluted)
	}
}

// 접두사 없는 이름이 있으면 그쪽이 이긴다 — 접두사 되돌리기는 계층을 다 소진한
// 뒤에만 도는 마지막 한 번이다.
func TestMapDartRowsPlainEPSBeatsOwnerPrefixed(t *testing.T) {
	rows := []dartAccountRow{
		{StatementDiv: "CIS", AccountName: "지배기업 소유주지분 기본주당손익", Amount: "953"},
		{StatementDiv: "CIS", AccountName: "기본주당이익", Amount: "800"},
	}
	if snap := mapDartRows(rows); snap.EPSBasic == nil || *snap.EPSBasic != 800 {
		t.Fatalf("EPSBasic = %v, want 800", snap.EPSBasic)
	}
}

// **우선주 EPS를 보통주 자리에 넣지 않는다.** 대상(001680)의 2025 연결재무제표에는
// 우선주 EPS만 있다 — 그건 「데이터 없음」이 맞고, 채우면 다른 주식 종류의 값을
// 이 종목의 EPS라고 말하게 된다.
func TestMapDartRowsRejectsPreferredShareEPS(t *testing.T) {
	rows := []dartAccountRow{
		{StatementDiv: "CIS", AccountName: "우선주기본주당순이익(손실)", Amount: "-8447"},
		{StatementDiv: "CIS", AccountName: "제1우선주 기본주당순이익", Amount: "-8438"},
	}
	if snap := mapDartRows(rows); snap.EPSBasic != nil {
		t.Fatalf("EPSBasic = %v, want nil (우선주 EPS는 보통주 EPS가 아니다)", *snap.EPSBasic)
	}
}

func TestStripOwnerPrefix(t *testing.T) {
	cases := []struct{ in, want string }{
		{"지배기업소유주지분기본주당손익", "기본주당손익"},
		{"지배기업의소유주에게귀속되는기본주당이익", "기본주당이익"},
		{"기본주당이익", "기본주당이익"}, // 접두사가 없으면 그대로
		{"우선주기본주당순이익", "우선주기본주당순이익"},
	}
	for _, c := range cases {
		if got := stripOwnerPrefix(c.in); got != c.want {
			t.Errorf("stripOwnerPrefix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// 기간 표지 제거가 낱말 경계를 넘지 않는다. `지배기업소유주지분기본주당손익`의
// 「지**분기**본」이 잘려 나가면 접두사 되돌리기가 통째로 실패한다.
func TestNormalizeDoesNotCutAcrossWords(t *testing.T) {
	got := normalizeDartAccountName("지배기업 소유주지분 기본주당손익")
	if got != "지배기업소유주지분기본주당손익" {
		t.Fatalf("normalize = %q, want 지배기업소유주지분기본주당손익", got)
	}
	// 표지는 `순` 앞에서만 떨어진다.
	if got := normalizeDartAccountName("연결 당기순이익"); got != "순이익" {
		t.Fatalf("normalize(연결 당기순이익) = %q, want 순이익", got)
	}
}

// DART는 **거절도 HTTP 200으로** 준다. 그걸 못 알아보면 빈 결과가 조용히 만들어진다.
func TestDartErrorPayload(t *testing.T) {
	// corpCode.xml의 한도 소진 응답 — 실측 149바이트.
	xmlBody := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<result><status>020</status><message>사용한도를 초과하였습니다.</message></result>`)
	status, msg, ok := dartErrorPayload(xmlBody)
	if !ok || status != "020" {
		t.Fatalf("xml: status=%q ok=%v, want 020/true", status, ok)
	}
	if msg == "" {
		t.Fatal("xml: message가 비었다")
	}

	// 재무 엔드포인트는 같은 거절을 JSON으로 준다.
	jsonBody := []byte(`{"status":"020","message":"사용한도를 초과하였습니다."}`)
	if status, _, ok := dartErrorPayload(jsonBody); !ok || status != "020" {
		t.Fatalf("json: status=%q ok=%v, want 020/true", status, ok)
	}

	// 정상 응답과 ZIP은 오류가 아니다.
	if _, _, ok := dartErrorPayload([]byte(`{"status":"000","list":[]}`)); ok {
		t.Fatal("status 000을 오류로 봤다")
	}
	if _, _, ok := dartErrorPayload([]byte("PK\x03\x04binary")); ok {
		t.Fatal("ZIP을 오류로 봤다")
	}
	if _, _, ok := dartErrorPayload(nil); ok {
		t.Fatal("빈 응답을 오류로 봤다")
	}
}

// 한도 소진은 「그 분기 미제출」과 갈라야 한다 — 013은 건너뛰고 020은 멈춘다.
func TestIsDartQuotaExceeded(t *testing.T) {
	quota := fmt.Errorf("%w (corpCode)", ErrDartQuotaExceeded)
	if !IsDartQuotaExceeded(quota) {
		t.Fatal("감싼 한도 오류를 못 알아봤다")
	}
	if IsDartQuotaExceeded(fmt.Errorf("dart: no data for year 2023")) {
		t.Fatal("013(미제출)을 한도 소진으로 봤다")
	}
	if IsDartQuotaExceeded(nil) {
		t.Fatal("nil을 한도 소진으로 봤다")
	}
}
