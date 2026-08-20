package textparse

import (
	"regexp"
	"strings"
	"unicode"
)

var dateLikeInName = regexp.MustCompile(`\d{1,2}/\d{1,2}`)
var listingSuffix = regexp.MustCompile(`(?i)\s*\([A-Za-z0-9.\-]{2,10}\)\s*$`)

// InferListLocale guesses domestic vs overseas from OCR section headers.
func InferListLocale(ocrTexts ...string) string {
	combined := strings.ToLower(strings.Join(ocrTexts, "\n"))
	switch {
	case strings.Contains(combined, "국내"):
		return "domestic"
	case strings.Contains(combined, "해외"), strings.Contains(combined, "$"):
		return "overseas"
	default:
		return ""
	}
}

// FilterLinesForLocale drops rows whose name shape is unlikely for the detected list type.
func FilterLinesForLocale(lines []ParsedLine, locale string) []ParsedLine {
	if locale == "" {
		return lines
	}
	out := make([]ParsedLine, 0, len(lines))
	for _, line := range lines {
		if lineMatchesLocale(line.Name, locale) {
			out = append(out, line)
		}
	}
	return out
}

func lineMatchesLocale(name, locale string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	switch locale {
	case "domestic":
		return domesticNamePlausible(name)
	case "overseas":
		return overseasNamePlausible(name)
	default:
		return true
	}
}

func domesticNamePlausible(name string) bool {
	name = NormalizeDisplayName(name)
	if looksLikeOCRFragment(name) {
		return false
	}
	if isBrokerAppNoise(name) {
		return false
	}
	if isOCRLatinDebris(name) {
		return false
	}
	h := hangulRunes(name)
	latin := latinLettersOnly(name)
	words := len(strings.Fields(name))
	if h > 0 {
		return true
	}
	if listingSuffix.MatchString(name) || strings.Contains(strings.ToUpper(name), "ADR") {
		return latinLetterRunes(name) >= 2
	}
	// Short latin tickers on KR lists (NAVER, SK, etc.)
	if latin && words <= 2 && len([]rune(name)) <= 12 {
		upper := strings.ToUpper(name)
		vowels := 0
		for _, r := range upper {
			if strings.ContainsRune("AEIOUY", r) {
				vowels++
			}
		}
		// Drop consonant-only OCR debris (e.g. SSDI) mistaken for tickers.
		if len([]rune(upper)) <= 5 && vowels == 0 {
			return false
		}
		return true
	}
	return false
}

func overseasNamePlausible(name string) bool {
	name = NormalizeDisplayName(name)
	if looksLikeOCRFragment(name) {
		return false
	}
	if len(strings.Fields(name)) > 8 {
		return false
	}
	if listingSuffix.MatchString(name) || isTickerLike(strings.ToUpper(strings.NewReplacer(" ", "").Replace(name))) {
		return true
	}
	if hangulRunes(name) >= 3 {
		return true
	}
	if latinLetterRunes(name) >= 3 {
		return true
	}
	return false
}

func latinLetterRunes(s string) int {
	n := 0
	for _, r := range s {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			n++
		}
	}
	return n
}

// looksLikeOCRFragment detects finance-column debris and single-pass noise (not specific symbols).
func looksLikeOCRFragment(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return true
	}
	if strings.HasPrefix(name, "=") || strings.HasPrefix(name, "~") || strings.HasPrefix(name, "+") || strings.HasPrefix(name, "-") {
		return true
	}
	if strings.Contains(strings.ToUpper(name), "SPOL") {
		return true
	}
	if strings.Contains(name, "#") && strings.Contains(name, "(") {
		return true
	}
	if dateLikeInName.MatchString(name) {
		return true
	}
	if len(strings.Fields(name)) > 5 {
		return true
	}
	core := strings.TrimSpace(listingSuffix.ReplaceAllString(name, ""))
	if core == "" {
		return true
	}
	runes := []rune(core)
	if len(runes) < 2 {
		return true
	}
	// Spaced single-character Hangul debris (e.g. "호 도").
	if fields := strings.Fields(name); len(fields) >= 2 {
		allTiny := true
		for _, f := range fields {
			if len([]rune(f)) > 2 {
				allTiny = false
				break
			}
		}
		if allTiny {
			return true
		}
	}
	letters, digits, punct := 0, 0, 0
	for _, r := range runes {
		switch {
		case unicode.IsLetter(r):
			letters++
		case r >= '0' && r <= '9':
			digits++
		case unicode.IsSpace(r):
			// spaces between issuer + hangul (e.g. "TIGER 미국나스..") are not OCR debris
		default:
			punct++
		}
	}
	total := float64(len(runes))
	if total == 0 {
		return true
	}
	if float64(digits)/total > 0.35 {
		if hangulRunes(name) < 2 {
			return true
		}
	}
	if float64(punct)/total > 0.2 {
		if isListingStyleName(name) {
			return false
		}
		return true
	}
	if letters == 0 {
		return true
	}
	// Pure 1–2 char hangul debris (e.g. "안트" near NAVER PnL → false 안트로젠 resolve).
	if isPureShortHangulDebris(name) {
		return true
	}
	return false
}

func isPureShortHangulDebris(name string) bool {
	hangul, hasDigit, hasLatin := 0, false, false
	for _, r := range name {
		switch {
		case unicode.Is(unicode.Hangul, r):
			hangul++
		case r >= '0' && r <= '9':
			hasDigit = true
		case r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z':
			hasLatin = true
		}
	}
	return !hasLatin && !hasDigit && hangul >= 1 && hangul <= 2
}

func isListingStyleName(name string) bool {
	upper := strings.ToUpper(name)
	if listingSuffix.MatchString(name) {
		return latinLetterRunes(name) >= 2 || strings.Contains(hangulRunesOnly(name), "홀딩스")
	}
	if strings.Contains(upper, "ADR") && latinLetterRunes(name) >= 2 {
		return true
	}
	return latinLetterRunes(name) >= 3 && strings.ContainsAny(name, "#-()")
}

// LooksLikeOCRFragment is exported for import preview filtering.
func LooksLikeOCRFragment(name string) bool {
	return looksLikeOCRFragment(name)
}

// InferHoldingsLocale picks domestic/overseas for filtering; mixed Toss-style screens return "" (no drop).
func InferHoldingsLocale(ocrTexts []string, lines []ParsedLine) string {
	if hasMixedDomesticOverseasSection(ocrTexts...) {
		return ""
	}
	if locale := InferListLocale(ocrTexts...); locale != "" {
		return locale
	}
	return InferLocaleFromLines(lines)
}

func hasMixedDomesticOverseasSection(ocrTexts ...string) bool {
	combined := strings.Join(ocrTexts, "\n")
	hasDomestic := strings.Contains(combined, "국내") || strings.Count(combined, "원") >= 5
	hasOverseas := strings.Contains(combined, "해외") || strings.Count(combined, "$") >= 3
	return hasDomestic && hasOverseas
}

// InferLocaleFromLines guesses domestic when most parsed names contain Hangul.
func InferLocaleFromLines(lines []ParsedLine) string {
	if len(lines) == 0 {
		return ""
	}
	hangulRows := 0
	for _, l := range lines {
		if hangulRunes(l.Name) > 0 {
			hangulRows++
		}
	}
	if hangulRows*2 >= len(lines) {
		return "domestic"
	}
	return ""
}

func hangulRunes(s string) int {
	n := 0
	for _, r := range s {
		if unicode.Is(unicode.Hangul, r) {
			n++
		}
	}
	return n
}

func latinLettersOnly(s string) bool {
	hasLatin := false
	for _, r := range s {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			hasLatin = true
			continue
		}
		if r == ' ' || r == '.' || r == '-' || r == '&' || r == '(' || r == ')' {
			continue
		}
		return false
	}
	return hasLatin
}
