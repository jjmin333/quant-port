package service

import (
	"encoding/json"
	"testing"

	"github.com/enact/quant-port/be/internal/marketdata"
)

// 옵션이 상품에 붙고, 원문 금리 숫자가 그대로 남는지 — 테스트 값이며 실제 상품이 아니다.
func TestBuildFinRowsAttachesOptions(t *testing.T) {
	page := marketdata.FinlifePage{
		Base: []map[string]any{
			{"fin_co_no": "T1", "fin_prdt_cd": "P1", "kor_co_nm": "테스트은행", "fin_prdt_nm": "테스트예금", "dcls_month": "202609", "dcls_end_day": nil},
			{"fin_co_no": "T1", "fin_prdt_cd": "P2", "kor_co_nm": "테스트은행", "fin_prdt_nm": "옵션없음"},
			{"fin_co_no": "", "fin_prdt_cd": "P3"},
		},
		Options: []map[string]any{
			{"fin_co_no": "T1", "fin_prdt_cd": "P1", "dcls_month": "202609", "save_trm": "12", "intr_rate": 3.1, "intr_rate2": 3.5},
			{"fin_co_no": "T1", "fin_prdt_cd": "P1", "save_trm": "6", "intr_rate": 2.9, "intr_rate2": nil},
		},
	}
	rows, err := buildFinRows("deposit", "020000", page)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (빈 회사코드 제외)", len(rows))
	}
	var opts []map[string]any
	if err := json.Unmarshal(rows[0].Options, &opts); err != nil {
		t.Fatal(err)
	}
	if len(opts) != 2 || opts[0]["intr_rate2"] != 3.5 || opts[0]["fin_co_no"] != nil {
		t.Fatalf("options = %v", opts)
	}
	if string(rows[1].Options) != "[]" {
		t.Fatalf("옵션 없는 상품은 [] — got %s", rows[1].Options)
	}
	if rows[0].DclsEndDay != "" || rows[0].ProductName != "테스트예금" {
		t.Fatalf("row = %+v", rows[0])
	}
}

func TestSafeHomepage(t *testing.T) {
	cases := map[string]string{
		"https://bank.example":      "https://bank.example",
		"www.bank.example":          "https://www.bank.example",
		"javascript:alert(1)":       "",
		"":                          "",
		"www.x.example/\"><script>": "",
	}
	for in, want := range cases {
		if got := safeHomepage(in); got != want {
			t.Errorf("safeHomepage(%q) = %q, want %q", in, got, want)
		}
	}
}
