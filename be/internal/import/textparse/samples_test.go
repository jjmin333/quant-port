package textparse

import "testing"

func TestParseHoldingsTextDomesticMinimalList(t *testing.T) {
	text := `국내주식
카카오
120주
네이버
50주
SK하이닉스
10주`
	lines := FilterLinesForLocale(MergeMultiOCRPass(ParseHoldingsText(text)), "domestic")
	want := map[string]float64{
		"카카오":    120,
		"네이버":    50,
		"SK하이닉스": 10,
	}
	assertHoldingsMap(t, lines, want)
}

func TestParseHoldingsTextDomesticInlineQty(t *testing.T) {
	text := `삼성전자 34주
LG전자 7주
현대차 4주`
	lines := MergeMultiOCRPass(ParseHoldingsText(text))
	want := map[string]float64{
		"삼성전자": 34,
		"LG전자": 7,
		"현대차":  4,
	}
	assertHoldingsMap(t, lines, want)
}

func assertHoldingsMap(t *testing.T, lines []ParsedLine, want map[string]float64) {
	t.Helper()
	for name, qty := range want {
		found := false
		for _, l := range lines {
			if l.Name == name && l.Quantity == qty {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing %s %v주 in %+v", name, qty, lines)
		}
	}
	if len(lines) != len(want) {
		t.Fatalf("expected %d lines, got %d: %+v", len(want), len(lines), lines)
	}
}
