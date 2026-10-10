package textparse

import (
	"regexp"
	"strings"
)

var quoteLineLeadNoise = regexp.MustCompile(`^[@«»_·•|nm a\d]+\s*|^@\d*\s*|^(?:은|는|이|가)\s+`)

// InferQuoteListOnly reports watchlist/quote screens ($ prices, no share qty).
func InferQuoteListOnly(ocrTexts ...string) bool {
	combined := strings.Join(ocrTexts, "\n")
	if strings.Count(combined, "$") < 3 {
		return false
	}
	if qtyPattern.MatchString(combined) {
		return false
	}
	if strings.Contains(combined, "원") && krwMoneyToken.MatchString(combined) {
		return false
	}
	names := ParseQuoteListFromOCRTexts(ocrTexts...)
	return len(names) >= 2
}

// ParseQuoteListFromOCRTexts extracts symbol names from quote/watchlist OCR (qty always 0).
func ParseQuoteListFromOCRTexts(ocrTexts ...string) []ParsedLine {
	seen := map[string]struct{}{}
	var out []ParsedLine
	for _, text := range ocrTexts {
		for _, line := range parseQuoteListText(text) {
			key := mergeKey(line.Name)
			if key == "" {
				continue
			}
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, line)
		}
	}
	return out
}

func parseQuoteListText(text string) []ParsedLine {
	var out []ParsedLine
	var pending string

	flushPending := func() {
		if name := normalizeQuoteListName(pending); name != "" && isNameOnlyCandidate(name) {
			out = append(out, ParsedLine{Name: name, Quantity: 0})
		}
		pending = ""
	}

	for _, raw := range strings.Split(text, "\n") {
		line := normalizeLine(raw)
		if line == "" {
			continue
		}
		if isQuoteListNoiseLine(line) {
			flushPending()
			continue
		}

		if strings.Contains(line, "$") {
			if name := normalizeQuoteListName(extractNameBeforeDollar(line)); name != "" && isNameOnlyCandidate(name) {
				flushPending()
				out = append(out, ParsedLine{Name: name, Quantity: 0})
				continue
			}
			flushPending()
			continue
		}

		if looksLikeQuoteNameLine(line) {
			flushPending()
			pending = line
		}
	}
	flushPending()
	return out
}

func isQuoteListNoiseLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	if headerNoise.MatchString(trimmed) {
		return true
	}
	if strings.HasPrefix(trimmed, "$") && !containsHangul(trimmed) {
		return true
	}
	if !containsHangul(trimmed) && (returnPctInParens.MatchString(trimmed) || strings.HasPrefix(trimmed, "-$") || strings.HasPrefix(trimmed, "+$") || strings.HasPrefix(trimmed, "~$")) {
		return true
	}
	if isMoneyLikeName(trimmed) || looksLikeMoneyOrPnL(trimmed) {
		return true
	}
	return false
}

func looksLikeQuoteNameLine(line string) bool {
	if !containsHangul(line) {
		return false
	}
	if strings.Contains(line, "$") {
		return false
	}
	if returnPctInParens.MatchString(line) {
		return false
	}
	h := hangulRunesOnly(line)
	return len([]rune(h)) >= 2
}

func extractNameBeforeDollar(line string) string {
	idx := strings.Index(line, "$")
	if idx <= 0 {
		return ""
	}
	return strings.TrimSpace(line[:idx])
}

func trimTrailingLatinNoise(name string) string {
	parts := strings.Fields(name)
	for len(parts) > 1 {
		last := parts[len(parts)-1]
		if len(last) <= 4 && latinLetterRunes(last) == len([]rune(last)) {
			upper := strings.ToUpper(last)
			if _, ok := domesticLatinTickers[upper]; !ok && etfIssuerPrefix(name) == "" {
				parts = parts[:len(parts)-1]
				continue
			}
		}
		break
	}
	return strings.Join(parts, " ")
}

func normalizeQuoteListName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = quoteLineLeadNoise.ReplaceAllString(raw, "")
	raw = strings.TrimLeft(raw, "@«»_·•|- ")
	name := cleanName(raw)
	name = trimTrailingLatinNoise(name)
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return NormalizeDisplayName(name)
}
