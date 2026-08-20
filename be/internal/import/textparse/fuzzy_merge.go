package textparse

import (
	"regexp"
	"strings"
)

var digitListingSuffix = regexp.MustCompile(`\(\d{2,4}\)`)

// FuzzyUnifyParsedLines merges rows from different OCR passes that refer to the same holding.
func FuzzyUnifyParsedLines(lines []ParsedLine) []ParsedLine {
	if len(lines) <= 1 {
		return lines
	}
	used := make([]bool, len(lines))
	out := make([]ParsedLine, 0, len(lines))
	for i := range lines {
		if used[i] {
			continue
		}
		cluster := []ParsedLine{lines[i]}
		used[i] = true
		for j := i + 1; j < len(lines); j++ {
			if used[j] {
				continue
			}
			if shouldFuzzyMerge(cluster, lines[j]) {
				cluster = append(cluster, lines[j])
				used[j] = true
			}
		}
		out = append(out, mergeFuzzyCluster(cluster))
	}
	return FilterParsedLines(out)
}

func shouldFuzzyMerge(cluster []ParsedLine, candidate ParsedLine) bool {
	for _, existing := range cluster {
		if existing.Quantity > 0 && candidate.Quantity > 0 && existing.Quantity != candidate.Quantity {
			continue
		}
		if fuzzyNameMatch(existing.Name, candidate.Name) {
			return true
		}
		if existing.Quantity > 0 && candidate.Quantity > 0 && existing.Quantity == candidate.Quantity {
			if garbageHangulPair(existing.Name, candidate.Name) {
				return true
			}
		}
	}
	return false
}

func fuzzyNameMatch(a, b string) bool {
	a = NormalizeDisplayName(a)
	b = NormalizeDisplayName(b)
	if a == b {
		return true
	}
	pa, pb := etfIssuerPrefix(a), etfIssuerPrefix(b)
	if pa != "" && pb != "" && pa != pb {
		return false
	}
	if adrListingRelated(a, b) {
		return true
	}
	ha, hb := hangulRunesOnly(a), hangulRunesOnly(b)
	hra, hrb := []rune(ha), []rune(hb)
	if len(hra) >= 2 && len(hrb) >= 2 {
		if similarityRatio(ha, hb) >= 0.55 {
			return true
		}
		if hangulPrefixMatch(a, b, 2) {
			return true
		}
		if len(hra) >= 4 && len(hrb) >= 4 && hangulSuffixMatch(ha, hb, 2) {
			return true
		}
		if len(hra) == len(hrb) && len(hra) >= 3 && len(hra) <= 5 && hangulSuffixMatch(ha, hb, 1) {
			if similarityRatio(ha, hb) >= 0.33 {
				return true
			}
		}
		if hangulSharedTail(ha, hb, 4) {
			return true
		}
	}
	return false
}

func hangulSuffixMatch(a, b string, n int) bool {
	ra, rb := []rune(a), []rune(b)
	if len(ra) < n || len(rb) < n {
		return false
	}
	return string(ra[len(ra)-n:]) == string(rb[len(rb)-n:])
}

func hangulSharedTail(a, b string, n int) bool {
	ra, rb := []rune(a), []rune(b)
	if len(ra) < n || len(rb) < n {
		return false
	}
	for i := n; i <= len(ra) && i <= len(rb); i++ {
		if string(ra[len(ra)-i:]) == string(rb[len(rb)-i:]) {
			return true
		}
	}
	return false
}

func adrListingRelated(a, b string) bool {
	a = NormalizeDisplayName(a)
	b = NormalizeDisplayName(b)
	combined := strings.ToUpper(a + " " + b)
	ha := hangulRunesOnly(a)
	hb := hangulRunesOnly(b)
	hasHoldings := strings.Contains(ha, "홀딩스") || strings.Contains(hb, "홀딩스")
	hasARM := strings.Contains(combined, "ARM")
	hasADR := strings.Contains(combined, "ADR") || digitListingSuffix.MatchString(a) || digitListingSuffix.MatchString(b)
	if !hasHoldings || (!hasARM && !hasADR) {
		return false
	}
	if hasARM {
		return true
	}
	return strings.Contains(ha, "홀딩스") && strings.Contains(hb, "홀딩스")
}

func mergeFuzzyCluster(cluster []ParsedLine) ParsedLine {
	line := cluster[0]
	for _, item := range cluster[1:] {
		line = pickRicherFinance(line, item)
	}
	display := normalizeListingDisplayName(pickBestClusterName(cluster))
	line.Name = display
	line.Quantity = pickClusterQty(cluster, display)
	return line
}

func pickClusterQty(cluster []ParsedLine, display string) float64 {
	display = NormalizeDisplayName(display)
	var matched float64
	for _, item := range cluster {
		if item.Quantity <= 0 {
			continue
		}
		if item.Name == display || fuzzyNameMatch(display, item.Name) {
			if matched == 0 {
				matched = item.Quantity
			} else if matched != item.Quantity {
				matched = 0
				break
			}
		}
	}
	if matched > 0 {
		return matched
	}
	votes := map[float64]int{}
	for _, item := range cluster {
		if item.Quantity > 0 {
			votes[item.Quantity]++
		}
	}
	return majorityQty(votes)
}

func pickBestClusterName(cluster []ParsedLine) string {
	best := cluster[0].Name
	bestScore := nameQualityScore(best)
	for _, item := range cluster[1:] {
		name := NormalizeDisplayName(item.Name)
		score := nameQualityScore(name)
		if score > bestScore {
			bestScore = score
			best = name
		}
	}
	return best
}

func normalizeListingDisplayName(name string) string {
	name = NormalizeDisplayName(name)
	upper := strings.ToUpper(name)
	h := hangulRunesOnly(name)
	if !strings.Contains(h, "홀딩스") && !strings.Contains(upper, "ARM") {
		return name
	}
	// 꼬리 괄호는 한 번만 뽑고 그 내용으로 판정한다.
	//
	// 예전에는 listingSuffix를 먼저 보고 비었을 때만 숫자 괄호를 (ADR)로 고쳤는데,
	// listingSuffix의 문자군 `[A-Za-z0-9.\-]{2,10}`이 숫자 괄호를 **포함**하므로
	// 그 분기는 정작 필요할 때 도달하지 않았다. `.404홀딩스(600)`이 (600)을 그대로
	// 달고 "ARM 홀딩스(600)"으로 나온 이유다.
	suffix := strings.TrimSpace(listingSuffix.FindString(name))
	// 괄호 안이 숫자뿐이면 상장 구분자가 아니다 — 실제 접미사는 (ADR)·(NYSE)처럼
	// 글자로 되어 있고, 숫자만 남은 것은 OCR이 (ADR)을 (600)·(404)로 읽은 결과다.
	if isDigitsOnlyParen(suffix) {
		suffix = "(ADR)"
	}
	if suffix == "" && strings.Contains(upper, "ADR") {
		suffix = "(ADR)"
	}
	if strings.Contains(h, "홀딩스") || strings.Contains(upper, "ARM") {
		if suffix != "" {
			return "ARM 홀딩스" + suffix
		}
		return "ARM 홀딩스(ADR)"
	}
	return name
}

// isDigitsOnlyParen reports whether s is "(…)" with nothing but digits inside.
func isDigitsOnlyParen(s string) bool {
	if len(s) < 3 || s[0] != '(' || s[len(s)-1] != ')' {
		return false
	}
	for _, r := range s[1 : len(s)-1] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func garbageHangulPair(a, b string) bool {
	a = NormalizeDisplayName(a)
	b = NormalizeDisplayName(b)
	ha, hb := hangulRunes(a), hangulRunes(b)
	if ha >= 3 && hb >= 3 {
		return false
	}
	if isListingStyleName(a) || isListingStyleName(b) {
		return false
	}
	if ha >= 3 && (looksLikeOCRFragment(b) || (hb == 0 && latinLetterRunes(b) <= 4)) {
		return true
	}
	if hb >= 3 && (looksLikeOCRFragment(a) || (ha == 0 && latinLetterRunes(a) <= 4)) {
		return true
	}
	return false
}
