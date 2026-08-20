package textparse

import "testing"

func TestFuzzyNameMatchIrenMamiren(t *testing.T) {
	a := hangulRunesOnly("아이렌")
	b := hangulRunesOnly("마미렌")
	t.Logf("sim=%v suffix1=%v len=%d/%d", similarityRatio(a, b), hangulSuffixMatch(a, b, 1), len(a), len(b))
	if !fuzzyNameMatch("아이렌", "마미렌") {
		t.Fatal("should merge IREN OCR variants")
	}
}

func TestFuzzyNameMatchAppliedNotMaplebin(t *testing.T) {
	a := hangulRunesOnly("어플라이드 머티리얼즈")
	b := hangulRunesOnly("맵러빈")
	t.Logf("sim=%v suffix=%v prefix=%v tail=%v", similarityRatio(a, b), hangulSuffixMatch(a, b, 1), hangulPrefixMatch("어플라이드 머티리얼즈", "맵러빈", 2), hangulSharedTail(a, b, 4))
	if fuzzyNameMatch("어플라이드 머티리얼즈", "맵러빈") {
		t.Fatal("should not merge applied with maplebin OCR")
	}
	if !fuzzyNameMatch("어플라이드 머티리얼즈", "머플라이드 머티리얼즈") {
		t.Fatal("should merge applied OCR variants")
	}
}

// A parenthetical of bare digits is OCR damage, not a listing suffix: real ones
// are alphabetic. This used to leak through because listingSuffix accepts
// [A-Za-z0-9.-] and so matched "(600)" before the digit branch could rewrite it,
// which shipped "ARM 홀딩스(600)" into the holdings import.
func TestNormalizeListingDisplayNameRepairsDigitSuffix(t *testing.T) {
	cases := map[string]string{
		".404홀딩스(600)":  "ARM 홀딩스(ADR)",
		"ARM 홀딩스(404)":  "ARM 홀딩스(ADR)",
		"ARM #3-(ADR)":  "ARM 홀딩스(ADR)",
		"ARM 홀딩스(ADR)":  "ARM 홀딩스(ADR)",
		"ARM 홀딩스":       "ARM 홀딩스(ADR)",
		// 글자가 섞인 접미사는 진짜 구분자일 수 있으므로 건드리지 않는다.
		"ARM 홀딩스(NYSE)": "ARM 홀딩스(NYSE)",
		// 홀딩스도 ARM도 아닌 이름은 이 경로를 타지 않는다.
		"오라클": "오라클",
	}
	for in, want := range cases {
		if got := normalizeListingDisplayName(in); got != want {
			t.Errorf("normalizeListingDisplayName(%q) = %q, want %q", in, got, want)
		}
	}
}

// The two suffix patterns overlap, which is what made the repair unreachable.
// Pinning it here so a future edit to either regex has to confront the overlap
// rather than silently re-break the digit case.
func TestListingSuffixOverlapsDigitListingSuffix(t *testing.T) {
	const digits = "X홀딩스(600)"
	if listingSuffix.FindString(digits) == "" {
		t.Fatal("listingSuffix no longer matches a digit-only suffix — the ordering guard in normalizeListingDisplayName can be simplified")
	}
	if !digitListingSuffix.MatchString(digits) {
		t.Fatal("digitListingSuffix no longer matches (600)")
	}
	if !isDigitsOnlyParen("(600)") || isDigitsOnlyParen("(ADR)") || isDigitsOnlyParen("()") {
		t.Fatal("isDigitsOnlyParen misclassified")
	}
}
