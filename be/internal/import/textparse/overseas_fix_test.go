package textparse

import "testing"

func TestParseHoldingsTextARMBlock(t *testing.T) {
	text := `.404홀딩스(600)
$13,392.21
50%`
	lines := ParseHoldingsText(text)
	t.Logf("%+v", lines)
	if len(lines) != 1 || lines[0].Quantity != 50 {
		t.Fatalf("unexpected: %+v", lines)
	}
}

func TestIsValidParsedLineARM(t *testing.T) {
	name := ".404홀딩스(600)"
	if !IsValidParsedLine(name, 50) {
		t.Fatalf("should be valid: fragment=%v money=%v", looksLikeOCRFragment(name), isMoneyLikeName(name))
	}
}

func TestIsSkippableLineARMName(t *testing.T) {
	if isSkippableLine(".404홀딩스(600)") {
		t.Fatal("ADR-like name should not be skippable")
	}
	if isSkippableLine("ARM #3-(ADR)") {
		t.Fatal("ADR ticker name should not be skippable")
	}
}

func TestParseHoldingsTextIrenQty55After50Pct(t *testing.T) {
	text := `아이렌
$2,168.84
55주
+6122.84(6.00%
.404홀딩스(600)
$13,392.21
50%`
	lines := ParseHoldingsText(text)
	for _, l := range lines {
		t.Logf("%q / %.0f", l.Name, l.Quantity)
	}
	for _, l := range lines {
		if l.Name == "아이렌" && l.Quantity != 55 {
			t.Fatalf("아이렌 want 55 got %+v", lines)
		}
	}
	foundARM := false
	for _, l := range lines {
		if l.Quantity == 50 && (containsHangul(l.Name) || containsARM(l.Name)) {
			foundARM = true
		}
	}
	if !foundARM {
		t.Fatalf("missing ARM row: %+v", lines)
	}
}

func containsARM(name string) bool {
	return len(name) >= 3 && (containsHangul(name) || isListingStyleName(name))
}
