package textparse

import (
	"sort"
	"strings"
)

// MergeMultiOCRPass merges rows from many OCR passes (full frame, columns, row bands).
// Quantity uses majority vote so one noisy pass cannot override several agreeing reads.
// Name-only rows (qty 0) are kept for FuzzyUnifyParsedLines to pair with qty rows.
func MergeMultiOCRPass(items []ParsedLine) []ParsedLine {
	type acc struct {
		display  string
		votes    map[float64]int
		finance  ParsedLine
		firstIdx int
	}
	groups := map[string]*acc{}
	order := make([]string, 0)

	for i, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		if !isNameOnlyCandidate(item.Name) && item.Quantity <= 0 {
			continue
		}
		if item.Quantity > 0 && !IsValidParsedLine(item.Name, item.Quantity) {
			continue
		}
		if item.Quantity <= 0 && !isNameOnlyCandidate(item.Name) {
			continue
		}
		if item.Quantity <= 0 && (isGarbageName(strings.TrimSpace(item.Name)) || looksLikeOCRFragment(strings.TrimSpace(item.Name))) {
			continue
		}

		key := mergeKey(item.Name)
		if key == "" {
			continue
		}
		g, ok := groups[key]
		if !ok {
			g = &acc{
				display:  NormalizeDisplayName(item.Name),
				votes:    map[float64]int{},
				finance:  item,
				firstIdx: i,
			}
			groups[key] = g
			order = append(order, key)
		}
		if item.Quantity > 0 {
			g.votes[item.Quantity]++
		}
		g.finance = pickRicherFinance(g.finance, item)
		if nameQualityScore(item.Name) > nameQualityScore(g.display) {
			g.display = NormalizeDisplayName(item.Name)
		}
	}

	sort.SliceStable(order, func(i, j int) bool {
		return groups[order[i]].firstIdx < groups[order[j]].firstIdx
	})

	out := make([]ParsedLine, 0, len(order))
	for _, key := range order {
		g := groups[key]
		line := g.finance
		line.Name = g.display
		line.Quantity = majorityQty(g.votes)
		if line.Quantity <= 0 {
			if !isNameOnlyCandidate(line.Name) {
				continue
			}
		}
		out = append(out, line)
	}
	return out
}

func majorityQty(votes map[float64]int) float64 {
	bestQty := 0.0
	bestCount := 0
	for qty, count := range votes {
		if qty <= 0 {
			continue
		}
		if count > bestCount || (count == bestCount && qty > bestQty) {
			bestQty = qty
			bestCount = count
		}
	}
	return bestQty
}
