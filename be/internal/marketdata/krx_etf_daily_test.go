package marketdata

import (
	"encoding/json"
	"testing"
)

// 실측 응답의 모양(문자열 숫자 · 빈 값 · "-")을 그대로 읽는다.
func TestMapETFDailyReadsKRXStrings(t *testing.T) {
	raw := `{"OutBlock_1":[
	  {"ISU_CD":"451060","TDD_CLSPRC":"35,870","NAV":"35877.56","INVSTASST_NETASST_TOTAMT":"452057318392",
	   "ACC_TRDVAL":"4796828380","IDX_IND_NM":"코스피 200","OBJ_STKPRC_IDX":"1113.30","FLUC_RT_IDX":"0.10"},
	  {"ISU_CD":"999999","TDD_CLSPRC":"-","NAV":"1"},
	  {"ISU_CD":"123456","TDD_CLSPRC":1000,"NAV":"-","FLUC_RT_IDX":""}
	]}`
	var w struct {
		Out []krxETFDailyRaw `json:"OutBlock_1"`
	}
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatal(err)
	}
	rows := mapETFDaily(w.Out)
	if len(rows) != 2 {
		t.Fatalf("rows = %d (종가 없는 행은 빠져야 한다)", len(rows))
	}
	r := rows[0]
	if r.Symbol != "451060" || r.Close != 35870 || *r.NAV != 35877.56 || *r.NetAssets != 452057318392 ||
		*r.TradeValue != 4796828380 || r.IndexName != "코스피 200" || *r.IndexChangeRate != 0.10 {
		t.Fatalf("parsed = %+v", r)
	}
	if rows[1].NAV != nil || rows[1].IndexChangeRate != nil {
		t.Fatalf("빈 값은 nil이어야 한다: %+v", rows[1])
	}
}
