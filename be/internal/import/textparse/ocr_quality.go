package textparse

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// stripLeadingOCRNoise removes 1–3 char latin OCR debris before the real name token.
func stripLeadingOCRNoise(name string) string {
	parts := strings.Fields(strings.TrimSpace(name))
	for len(parts) > 1 {
		tok := parts[0]
		r := []rune(tok)
		if len(r) <= 3 && !containsHangul(tok) && !isTickerLike(tok) {
			parts = parts[1:]
			continue
		}
		break
	}
	return strings.Join(parts, " ")
}

func containsHangul(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Hangul, r) {
			return true
		}
	}
	return false
}

func isTickerLike(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			continue
		}
		return false
	}
	return len(s) >= 2 && len(s) <= 6
}

func nameQualityScore(name string) int {
	name = strings.TrimSpace(name)
	if name == "" || looksLikeOCRFragment(name) {
		return -1000
	}
	score := hangulRunes(name) * 3
	score += len([]rune(name)) / 4
	if stripLeadingOCRNoise(name) != name {
		score -= 20
	}
	score -= strings.Count(name, " ")
	if digitRatio(name) > 0.2 {
		score -= 10
	}
	if isTickerLike(strings.ToUpper(strings.NewReplacer(" ", "").Replace(name))) {
		score += 5
	}
	return score
}

func hangulPrefixMatch(a, b string, n int) bool {
	ra := hangulRunesOnly(a)
	rb := hangulRunesOnly(b)
	if len(ra) < n || len(rb) < n {
		return false
	}
	return ra[:n] == rb[:n]
}

func hangulRunesOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.Is(unicode.Hangul, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// CollapseSimilarRows merges rows that share quantity and clearly refer to the same line.
func CollapseSimilarRows(lines []ParsedLine) []ParsedLine {
	if len(lines) <= 1 {
		return lines
	}
	out := make([]ParsedLine, 0, len(lines))
	for _, line := range lines {
		merged := false
		for i, existing := range out {
			if existing.Quantity != line.Quantity {
				continue
			}
			if !shadowDuplicateName(existing.Name, line.Name) {
				continue
			}
			if nameQualityScore(line.Name) > nameQualityScore(existing.Name) {
				out[i] = line
			}
			merged = true
			break
		}
		if !merged {
			out = append(out, line)
		}
	}
	return out
}

func shadowDuplicateName(a, b string) bool {
	pa, pb := etfIssuerPrefix(a), etfIssuerPrefix(b)
	if pa != "" && pb != "" && pa != pb {
		return false
	}
	if hangulPrefixMatch(a, b, 2) {
		return true
	}
	ka := strings.ToUpper(NormalizeDisplayName(a))
	kb := strings.ToUpper(NormalizeDisplayName(b))
	if len(ka) >= 3 && len(kb) >= 3 {
		if similarityRatio(ka, kb) >= 0.65 {
			return true
		}
	}
	return false
}

func similarityRatio(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	ra, rb := []rune(a), []rune(b)
	dist := levenshteinDistance(ra, rb)
	maxLen := len(ra)
	if lb := len(rb); lb > maxLen {
		maxLen = lb
	}
	if maxLen == 0 {
		return 0
	}
	return 1 - float64(dist)/float64(maxLen)
}

func levenshteinDistance(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = minInt3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func minInt3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

var domesticETFIssuers = []string{"KODEX", "TIGER", "KBSTAR", "ARIRANG", "HANARO", "SOL", "ACE", "PLUS", "RISE", "TIME", "KOSEF"}

func etfIssuerPrefix(name string) string {
	upper := strings.ToUpper(strings.TrimSpace(name))
	for _, prefix := range domesticETFIssuers {
		if upper == prefix || strings.HasPrefix(upper, prefix+" ") {
			return prefix
		}
	}
	return ""
}

var kodexOCRNoise = regexp.MustCompile(`(?i)^(?:006|k0|ko?)[\d×x.\s]*`)

func repairDomesticETFOCRName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return name
	}
	h := hangulRunesOnly(name)
	if !strings.Contains(h, "미국나스") {
		return name
	}
	upper := strings.ToUpper(name)
	if strings.Contains(upper, "TIGER") {
		if !strings.HasPrefix(upper, "TIGER ") && !strings.HasPrefix(upper, "TIGER") {
			if i := strings.Index(upper, "TIGER"); i >= 0 {
				name = strings.TrimSpace(name[i:])
			}
		}
		return name
	}
	if strings.HasPrefix(upper, "KODEX") {
		return name
	}
	if strings.Contains(name, "×") || kodexOCRNoise.MatchString(name) {
		if idx := strings.Index(h, "미국"); idx >= 0 {
			return "KODEX " + h[idx:]
		}
		return "KODEX " + h
	}
	return name
}

// NormalizeDisplayName trims OCR noise prefixes before display/resolver.
func NormalizeDisplayName(name string) string {
	name = norm.NFKC.String(strings.TrimSpace(name))
	name = strings.TrimLeft(name, ".·•")
	name = stripLeadingOCRNoise(name)
	name = strings.Trim(name, `"«»'`)
	for strings.HasSuffix(name, "..") {
		name = strings.TrimSuffix(name, ".")
	}
	name = strings.TrimSpace(name)
	name = repairDomesticHoldingsOCRName(name)
	return repairDomesticETFOCRName(name)
}

func repairDomesticHoldingsOCRName(name string) string {
	h := hangulRunesOnly(name)
	if strings.Contains(h, "하이닉") || strings.Contains(h, "하미닉") {
		return "SK하이닉스"
	}
	upper := strings.ToUpper(strings.NewReplacer(" ", "").Replace(name))
	if upper == "SSDI" || strings.HasPrefix(upper, "SSDI") {
		return "삼성SDI"
	}
	// Toss OCR: digit/latin debris prefix before "전자" (→ LG전자).
	if hangulRunes(name) == 2 && strings.HasSuffix(h, "전자") {
		return "LG전자"
	}
	return name
}
