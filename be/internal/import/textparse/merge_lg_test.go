package textparse

import "testing"

func TestRepairDomesticHoldingsOCRNameLG(t *testing.T) {
	cases := map[string]string{
		"16전자":  "LG전자",
		"L6전자":  "LG전자",
		"1L6전자": "LG전자",
		"LG전자":  "LG전자",
		"삼성전자":  "삼성전자",
	}
	for in, want := range cases {
		if got := NormalizeDisplayName(in); got != want {
			t.Fatalf("%q → %q want %q", in, got, want)
		}
	}
}

func TestHasMixedDomesticOverseasFromWonAndDollar(t *testing.T) {
	won := "1원\n2원\n3원\n4원\n5원\n"
	if !hasMixedDomesticOverseasSection(won, "Broadcom $1\n$2\n$3\n") {
		t.Fatal("expected mixed from 원 + $")
	}
}

func TestNameEmbedsAmount(t *testing.T) {
	if !nameEmbedsAmount("삼성전자 8,956,7398") {
		t.Fatal("expected embedded amount")
	}
	if nameEmbedsAmount("삼성전자") {
		t.Fatal("plain name should pass")
	}
}

func TestMergeParsedLinesThreeScreens(t *testing.T) {
	lines := []ParsedLine{
		{Name: "삼성5미", Quantity: 4},
		{Name: "삼성전자", Quantity: 34},
		{Name: "한화솔루션", Quantity: 55},
		{Name: "한화시스템", Quantity: 23},
		{Name: "현대차", Quantity: 4},
		{Name: "16전자", Quantity: 7},
		{Name: "NAVER", Quantity: 19},
		{Name: "머플라이드 머티리얼즈", Quantity: 4},
		{Name: "ARM 홀딩스(ADR)", Quantity: 50},
		{Name: "맵러빈", Quantity: 15},
		{Name: "오라클", Quantity: 40},
		{Name: "ARM 홀딩스(ADR)", Quantity: 502},
		{Name: "브로드컴", Quantity: 34},
		{Name: "셀레스티카", Quantity: 26},
		{Name: "TSMC(ADR)", Quantity: 12},
		{Name: "XOVR", Quantity: 180},
		{Name: "SK하이닉스", Quantity: 23},
		{Name: "SK하이닉스", Quantity: 34},
	}
	out := MergeParsedLines(lines)
	armQty := 0.0
	lgFound := false
	skQty := 0.0
	for _, l := range out {
		if l.Name == "ARM 홀딩스(ADR)" {
			armQty = l.Quantity
		}
		if NormalizeDisplayName(l.Name) == "LG전자" {
			lgFound = true
		}
		if l.Name == "SK하이닉스" {
			skQty = l.Quantity
		}
	}
	if armQty != 50 {
		t.Fatalf("ARM qty=%v want 50", armQty)
	}
	if !lgFound {
		t.Fatal("LG전자 missing")
	}
	if skQty != 23 && skQty != 57 {
		t.Fatalf("SK qty=%v want 23 (or legacy sum 57)", skQty)
	}
}
