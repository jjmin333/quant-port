package fnguide

import (
	"math"
	"testing"
)

// 아래 픽스처는 2026-08 실제 응답에서 그대로 떠 온 행들이다. 손으로 만든 예제를
// 쓰지 않은 이유는, 이 파서가 다루는 네 가지 함정이 전부 "관측해서야 알게 된"
// 것들이기 때문이다 — `[정정]` 접두, `(연결)`/`(개별)` 접미, `FREQ=D`,
// `_G`가 숫자가 아니라 문자열로 오는 것.
const fixture = `{"dataset":[
{"TYPE":"1","TYPE_NM":"잠정실적발표","DT":"2026/07/07","CMP_CD":"005930","CMP_NM":"삼성전자(연결)","YYMM":"2026/06","FREQ":"2Q","FREQ_NM":null,
 "SALES_PRC_E":1745314.290000000,"OPERATION_PROFIT_E":848366.630000000,"NET_PROFIT_E":734933.330000000,
 "SALES_PRC_P":1710000.000000000,"OPERATION_PROFIT_P":894000.000000000,"NET_PROFIT_P":null,
 "SALES_PRC_G":27.732577044929091,"OPERATION_PROFIT_G":"56.2","NET_PROFIT_G":null},
{"TYPE":"1","TYPE_NM":"잠정실적발표","DT":"2026/07/30","CMP_CD":"005930","CMP_NM":"[정정]삼성전자(연결)","YYMM":"2026/06","FREQ":"2Q","FREQ_NM":null,
 "SALES_PRC_E":1745314.290000000,"OPERATION_PROFIT_E":848366.630000000,"NET_PROFIT_E":734933.330000000,
 "SALES_PRC_P":1714995.000000000,"OPERATION_PROFIT_P":894924.000000000,"NET_PROFIT_P":716245.000000000,
 "SALES_PRC_G":28.105690625244542,"OPERATION_PROFIT_G":"56.4","NET_PROFIT_G":"51.7"},
{"TYPE":"1","TYPE_NM":"잠정실적발표","DT":"2026/01/29","CMP_CD":"005930","CMP_NM":"[정정]삼성전자(연결)","YYMM":"2025/12","FREQ":"D","FREQ_NM":null,
 "SALES_PRC_E":3311714.230000000,"OPERATION_PROFIT_E":420354.540000000,"NET_PROFIT_E":416138.760000000,
 "SALES_PRC_P":3336059.000000000,"OPERATION_PROFIT_P":436011.000000000,"NET_PROFIT_P":452068.000000000,
 "SALES_PRC_G":10.88008068364125,"OPERATION_PROFIT_G":"33.2","NET_PROFIT_G":"31.2"},
{"TYPE":"1","TYPE_NM":"잠정실적발표","DT":"2026/08/03","CMP_CD":"004020","CMP_NM":"현대제철(개별)","YYMM":"2026/06","FREQ":"2Q","FREQ_NM":null,
 "SALES_PRC_E":null,"OPERATION_PROFIT_E":null,"NET_PROFIT_E":null,
 "SALES_PRC_P":60000.000000000,"OPERATION_PROFIT_P":1500.000000000,"NET_PROFIT_P":null,
 "SALES_PRC_G":null,"OPERATION_PROFIT_G":null,"NET_PROFIT_G":null},
{"TYPE":"1","TYPE_NM":"잠정실적발표","DT":"2026/08/03","CMP_CD":"","CMP_NM":"코드없음(연결)","YYMM":"2026/06","FREQ":"2Q",
 "SALES_PRC_E":null,"OPERATION_PROFIT_E":null,"NET_PROFIT_E":null,
 "SALES_PRC_P":null,"OPERATION_PROFIT_P":null,"NET_PROFIT_P":null,
 "SALES_PRC_G":null,"OPERATION_PROFIT_G":null,"NET_PROFIT_G":null},
{"TYPE":"1","TYPE_NM":"잠정실적발표","DT":"2026/08/03","CMP_CD":"999999","CMP_NM":"기간이상(연결)","YYMM":"2026/06","FREQ":"H1",
 "SALES_PRC_E":null,"OPERATION_PROFIT_E":null,"NET_PROFIT_E":null,
 "SALES_PRC_P":null,"OPERATION_PROFIT_P":null,"NET_PROFIT_P":null,
 "SALES_PRC_G":null,"OPERATION_PROFIT_G":null,"NET_PROFIT_G":null},
{"TYPE":"1","TYPE_NM":"잠정실적발표","DT":"2026/08/05","CMP_CD":"012450","CMP_NM":"적자전환기업(연결)","YYMM":"2026/06","FREQ":"2Q",
 "SALES_PRC_E":1000.000000000,"OPERATION_PROFIT_E":50.000000000,"NET_PROFIT_E":null,
 "SALES_PRC_P":900.000000000,"OPERATION_PROFIT_P":-30.000000000,"NET_PROFIT_P":-40.000000000,
 "SALES_PRC_G":-10.0,"OPERATION_PROFIT_G":"적전","NET_PROFIT_G":"적지"}
]}`

func TestParseDataset(t *testing.T) {
	rows, err := ParseDataset([]byte(fixture))
	if err != nil {
		t.Fatalf("ParseDataset: %v", err)
	}
	// 일곱 행 중 둘은 버려진다 — 종목코드 없는 행과 FREQ가 해석 불가한 행.
	// 이름·날짜만 있는 행을 통과시키면 캘린더에 클릭도 안 되는 줄이 남는다.
	if len(rows) != 5 {
		t.Fatalf("rows = %d, want 5 (code-less and unknown-FREQ rows dropped)", len(rows))
	}

	orig := rows[0]
	if orig.Symbol != "005930" || orig.Name != "삼성전자" {
		t.Errorf("symbol/name = %q/%q, want 005930/삼성전자", orig.Symbol, orig.Name)
	}
	if orig.Basis != "연결" {
		t.Errorf("basis = %q, want 연결", orig.Basis)
	}
	if orig.Revised {
		t.Error("first row must not be flagged as a revision")
	}
	if got := orig.AnnouncedAt.Format("2006-01-02"); got != "2026-07-07" {
		t.Errorf("announcedAt = %s, want 2026-07-07", got)
	}
	if orig.FiscalYear != 2026 || orig.FiscalPeriod != "2Q" {
		t.Errorf("fiscal = %d %s, want 2026 2Q", orig.FiscalYear, orig.FiscalPeriod)
	}
	if got := orig.PeriodEnd.Format("2006-01-02"); got != "2026-06-30" {
		t.Errorf("periodEnd = %s, want 2026-06-30 (month end)", got)
	}
	// 억원 → 원. 1,710,000억 = 171조.
	if !approx(orig.RevenueAct, 171_000_000_000_000) {
		t.Errorf("revenueAct = %v, want 1.71e14", deref(orig.RevenueAct))
	}
	// null은 0이 아니라 "없음"이다. 0으로 눕히면 순이익 0원을 발표한 것이 된다.
	if orig.NetIncomeAct != nil {
		t.Errorf("netIncomeAct = %v, want nil for JSON null", *orig.NetIncomeAct)
	}
	// `_G`는 문자열로 온다 ("56.2"). json.Number로 받지 않으면 여기서 죽는다.
	if !approx(orig.OpIncomeQoQ, 56.2) {
		t.Errorf("opIncomeQoQ = %v, want 56.2", deref(orig.OpIncomeQoQ))
	}
	// QoQ는 비율이므로 억원 배수를 곱하면 안 된다.
	if !approx(orig.RevenueQoQ, 27.732577044929091) {
		t.Errorf("revenueQoQ = %v, want ~27.73 (ratio, unscaled)", deref(orig.RevenueQoQ))
	}

	revision := rows[1]
	if !revision.Revised {
		t.Error("[정정] row must be flagged as a revision")
	}
	if revision.Name != "삼성전자" {
		t.Errorf("revision name = %q — [정정] prefix must be stripped", revision.Name)
	}
	// 정정 회차도 원 발표와 **같은 회계기간 키**를 가져야 저장 시 병합된다.
	if revision.FiscalYear != orig.FiscalYear || revision.FiscalPeriod != orig.FiscalPeriod ||
		revision.Basis != orig.Basis {
		t.Error("revision must share (year, period, basis) with the row it revises")
	}

	// FREQ=D → FY. 그리고 연도는 발표일(2026)이 아니라 YYMM(2025)에서 온다 —
	// 12월 결산은 이듬해에 발표되므로 발표일에서 뽑으면 한 해가 밀린다.
	annual := rows[2]
	if annual.FiscalPeriod != "FY" {
		t.Errorf("FREQ=D → period %q, want FY", annual.FiscalPeriod)
	}
	if annual.FiscalYear != 2025 {
		t.Errorf("annual fiscalYear = %d, want 2025 (from YYMM, not the 2026 filing date)", annual.FiscalYear)
	}
	// DART 확정치 333,605,938,000,000원과 맞물리는 값 — 단위 회귀를 여기서 잡는다.
	if !approx(annual.RevenueAct, 333_605_900_000_000) {
		t.Errorf("annual revenueAct = %v, want ~3.336e14", deref(annual.RevenueAct))
	}

	// 컨센서스가 없는 발표도 있다(중소형주). 그때는 est만 nil이고 행은 살아남아야
	// 한다 — 캘린더의 목적은 "누가 언제 발표하나"이고 서프라이즈는 부가다.
	noConsensus := rows[3]
	if noConsensus.Basis != "개별" {
		t.Errorf("basis = %q, want 개별", noConsensus.Basis)
	}
	if noConsensus.RevenueEst != nil || noConsensus.OpIncomeEst != nil {
		t.Error("missing consensus must stay nil, not zero")
	}
	if !approx(noConsensus.RevenueAct, 6_000_000_000_000) {
		t.Errorf("revenueAct = %v, want 6e12", deref(noConsensus.RevenueAct))
	}

	// **증감률 자리에 한국어가 온다.** "적전"(적자전환)·"적지"(적자지속)·"흑전".
	// 처음엔 json.Number를 써서 이 한 행이 그 달 400행을 통째로 날렸다 —
	// 202507·202508 두 달이 실제로 그렇게 유실됐다. 이 행이 살아 있고,
	// 숫자가 아닌 증감률만 비어 있어야 한다.
	label := rows[4]
	if label.Symbol != "012450" {
		t.Fatalf("row[4] symbol = %q, want 012450 — a non-numeric _G must not drop the row", label.Symbol)
	}
	if label.OpIncomeQoQ != nil {
		t.Errorf(`opIncomeQoQ = %v, want nil for "적전" (0으로 눕히면 적자전환이 증감률 0%%으로 뜬다)`,
			*label.OpIncomeQoQ)
	}
	if label.NetIncomeQoQ != nil {
		t.Errorf(`netIncomeQoQ = %v, want nil for "적지"`, *label.NetIncomeQoQ)
	}
	// 같은 행의 숫자 필드는 멀쩡해야 한다 — 라벨 하나가 행 전체를 오염시키지 않는다.
	if !approx(label.RevenueQoQ, -10) {
		t.Errorf("revenueQoQ = %v, want -10 (numeric sibling must survive)", deref(label.RevenueQoQ))
	}
	if !approx(label.OpIncomeAct, -3_000_000_000) {
		t.Errorf("opIncomeAct = %v, want -3e9 (음수 발표치도 그대로)", deref(label.OpIncomeAct))
	}
}

func TestParseNameEdgeCases(t *testing.T) {
	cases := []struct {
		in      string
		name    string
		basis   string
		revised bool
	}{
		{"삼성전자(연결)", "삼성전자", "연결", false},
		{"[정정]삼성전자(연결)", "삼성전자", "연결", true},
		{"현대제철(개별)", "현대제철", "개별", false},
		// 회사 이름 자체에 괄호가 있는 경우 — 연결/개별이 아니면 떼지 않는다.
		{"한국전력(주)", "한국전력(주)", "", false},
		// 접미가 없는 행도 있을 수 있다. 이름만 남는다.
		{"이름만", "이름만", "", false},
		// 닫히지 않은 대괄호에서 무한 루프에 빠지지 않아야 한다.
		{"[정정 삼성전자(연결)", "[정정 삼성전자", "연결", false},
	}
	for _, tc := range cases {
		name, basis, revised := parseName(tc.in)
		if name != tc.name || basis != tc.basis || revised != tc.revised {
			t.Errorf("parseName(%q) = (%q,%q,%v), want (%q,%q,%v)",
				tc.in, name, basis, revised, tc.name, tc.basis, tc.revised)
		}
	}
}

func TestParseDatasetEmptyMonth(t *testing.T) {
	// 미래 달은 항상 이 모양으로 온다. 에러가 아니라 "아직 없음"이다 —
	// 여기서 실패로 처리하면 배치가 매일 실패로 기록된다.
	rows, err := ParseDataset([]byte(`{"dataset":[]}`))
	if err != nil {
		t.Fatalf("empty dataset must not error: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %d, want 0", len(rows))
	}
}

func approx(got *float64, want float64) bool {
	if got == nil {
		return false
	}
	return math.Abs(*got-want) <= math.Abs(want)*1e-6+1e-9
}

func deref(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}
