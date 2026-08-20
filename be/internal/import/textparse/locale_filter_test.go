package textparse

import "testing"

func TestLooksLikeOCRFragment(t *testing.T) {
	fragments := []string{
		"= Tea S827 08 2!",
		"ALDEL FINL II INC UNIT 26/09/2",
		"안트",
	}
	for _, s := range fragments {
		if !looksLikeOCRFragment(s) {
			t.Fatalf("expected fragment: %q", s)
		}
	}
	if IsValidParsedLine("e NAVER", 19) {
		t.Fatal("expected e NAVER to be invalid")
	}
	valid := []string{"삼성전자", "NAVER", "한화시스템", "삼성5미", "16전자", "ARM 2SA(ADR)"}
	for _, s := range valid {
		if looksLikeOCRFragment(s) {
			t.Fatalf("unexpected fragment: %q", s)
		}
	}
}

func TestFilterLinesForLocaleDomestic(t *testing.T) {
	lines := FilterLinesForLocale([]ParsedLine{
		{Name: "삼성전자", Quantity: 34},
		{Name: "NAVER", Quantity: 19},
		{Name: "TEADS HOLDING CO", Quantity: 7},
		{Name: "ALDEL FINL II INC UNIT 26/09/2", Quantity: 192},
	}, "domestic")
	if len(lines) != 2 {
		t.Fatalf("unexpected: %+v", lines)
	}
}

func TestFilterLinesForLocaleDomesticKeepsADR(t *testing.T) {
	lines := FilterLinesForLocale([]ParsedLine{
		{Name: "브로드컴", Quantity: 34},
		{Name: "TSMC(ADR)", Quantity: 12},
	}, "domestic")
	if len(lines) != 2 {
		t.Fatalf("want ADR + hangul on domestic list, got %+v", lines)
	}
}

func TestFilterLinesForLocaleMixedKeepsOverseasTickers(t *testing.T) {
	in := []ParsedLine{
		{Name: "브로드컴", Quantity: 34},
		{Name: "TSMC(ADR)", Quantity: 12},
		{Name: "XOVR", Quantity: 180},
	}
	if len(FilterLinesForLocale(in, "")) != 3 {
		t.Fatal("mixed/empty locale should keep all rows")
	}
}

func TestInferHoldingsLocale_mixedTossScreen(t *testing.T) {
	ocr := []string{"해외주식\n$5,148\n국내주식\nSK하이닉스"}
	lines := []ParsedLine{{Name: "브로드컴", Quantity: 34}, {Name: "TSMC(ADR)", Quantity: 12}}
	if loc := InferHoldingsLocale(ocr, lines); loc != "" {
		t.Fatalf("mixed screen should not filter by locale, got %q", loc)
	}
}
