package textparse

import "testing"

func TestParseHoldingsTextDomesticScreenshot(t *testing.T) {
	// Preprocessed OCR representative of domestic brokerage app.
	text := `국내주식
잠성9미
4주
삼성전자
35주
한화솔루션
55주
한화시스템
23주
현대차
4주
NAVER
19주`
	lines := FilterParsedLines(ParseHoldingsText(text))
	want := map[string]float64{
		"잠성9미":  4,
		"삼성전자":  35,
		"한화솔루션": 55,
		"한화시스템": 23,
		"현대차":   4,
		"NAVER": 19,
	}
	for name, qty := range want {
		found := false
		for _, l := range lines {
			if l.Name == name && l.Quantity == qty {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing %s %v in %+v", name, qty, lines)
		}
	}
}

func TestFilterParsedLinesRejectsMoneyNoise(t *testing.T) {
	lines := FilterParsedLines([]ParsedLine{
		{Name: "1,544,67281", Quantity: 4},
		{Name: "재가", Quantity: 8272},
		{Name: "삼성전자", Quantity: 35},
	})
	if len(lines) != 1 || lines[0].Name != "삼성전자" {
		t.Fatalf("unexpected: %+v", lines)
	}
}

func TestParseHoldingsTextOverseasScreenshot(t *testing.T) {
	text := `아우스터

$2,031.14

60주

$2.86 (0.14%)

아이렌

$1,930.38

55주

~$115.62(5.65%)

어플라이드 머티리얼즈

$1,966.68

@

+$294.68 (17.62%)

aR

대

$2,371.15

27주

코어위브

$2,082.24

30%

~$104.76 (4.79%)

ARM 2SA(ADR)

$12,776.32

++$1,891.18 (17.37%)

50`
	lines := ParseHoldingsText(text)
	want := map[string]float64{
		"아우스터":         60,
		"아이렌":          55,
		"코어위브":         30,
		"ARM 2SA(ADR)": 50,
	}
	for name, qty := range want {
		found := false
		for _, l := range lines {
			if l.Name == name && l.Quantity == qty {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing %s %v in %+v", name, qty, lines)
		}
	}
}

func TestParseHoldingsTextNameQtySuffix(t *testing.T) {
	lines := ParseHoldingsText("ARM 홀딩스(ADR) 600")
	if len(lines) != 1 || lines[0].Name != "ARM 홀딩스(ADR)" || lines[0].Quantity != 600 {
		t.Fatalf("unexpected: %+v", lines)
	}
}

func TestMergeParsedLinesPreservesDisplayName(t *testing.T) {
	merged := MergeParsedLines([]ParsedLine{{Name: "ARM 2SA(ADR)", Quantity: 50}})
	if len(merged) != 1 || merged[0].Name != "ARM 2SA(ADR)" {
		t.Fatalf("unexpected: %+v", merged)
	}
}

func TestParseHoldingsText(t *testing.T) {
	text := `
삼성전자
10주
SK하이닉스 5주
Apple Inc 3주
총 평가금액 1,000,000원
`
	lines := ParseHoldingsText(text)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
	foundSamsung := false
	for _, l := range lines {
		if l.Name == "삼성전자" && l.Quantity == 10 {
			foundSamsung = true
		}
	}
	if !foundSamsung {
		t.Fatalf("missing samsung line: %+v", lines)
	}
}

func TestMergeParsedLines(t *testing.T) {
	merged := MergeParsedLines([]ParsedLine{
		{Name: "삼성전자", Quantity: 3},
		{Name: "삼성전자", Quantity: 7},
	})
	if len(merged) != 1 || merged[0].Quantity != 10 {
		t.Fatalf("unexpected merge: %+v", merged)
	}
}
