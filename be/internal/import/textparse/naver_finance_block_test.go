package textparse

import "testing"

func TestParseNAVERFinanceBlock(t *testing.T) {
	text := `NAVER
4.294.248원
19주
-645.752원013.07%
®  NAVER                               4,294,248
195                          645,752 (13.07%)`
	for _, l := range ParseHoldingsText(text) {
		if l.Name != "" || l.Quantity > 0 {
			t.Logf("name=%q qty=%.0f valid=%v frag=%v", l.Name, l.Quantity,
				IsValidParsedLine(l.Name, l.Quantity), LooksLikeOCRFragment(l.Name))
		}
	}
	merged := MergeParsedLines(ParseHoldingsText(text))
	for _, l := range merged {
		t.Logf("merged %q qty=%.0f", l.Name, l.Quantity)
	}
}
