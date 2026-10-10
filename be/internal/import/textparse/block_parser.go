package textparse

import (
	"strconv"
	"strings"
)

// parseHoldingsBlocks parses OCR text into holdings with optional finance fields.
func parseHoldingsBlocks(text string) []ParsedLine {
	lines := strings.Split(text, "\n")
	var out []ParsedLine
	seen := map[string]struct{}{}
	var block pendingBlock

	flushBlock := func() {
		if block.name == "" {
			return
		}
		recordParsedLineAny(&out, seen, block.toParsedLine())
		block.reset()
	}

	for _, raw := range lines {
		line := normalizeLine(raw)
		if line == "" {
			continue
		}

		if name, fin, ok := parseInlineNameEval(line); ok {
			flushBlock()
			block.name = name
			block.applyFinance(fin)
			continue
		}

		if qty, fin, ok := parseInlineQtyFinance(line); ok && block.name != "" {
			block.setQty(qty, true)
			block.applyFinance(fin)
			continue
		}

		if fin, ok := parseFinanceLine(line); ok {
			if block.name != "" {
				block.applyFinance(fin)
			} else {
				recordFinanceOrphan(&out, seen, fin)
			}
			continue
		}

		if isSkippableLine(line) {
			continue
		}

		if headerNoise.MatchString(line) {
			flushBlock()
			continue
		}

		if m := qtyPattern.FindStringSubmatch(line); len(m) >= 2 {
			qty, err := parseQty(m[1])
			if err != nil || qty <= 0 {
				continue
			}

			name := strings.TrimSpace(qtyPattern.ReplaceAllString(line, ""))
			name = cleanName(name)
			if name != "" && !isMoneyLikeName(name) && !looksLikeMoneyOrPnL(name) {
				flushBlock()
				recordParsedLineAny(&out, seen, ParsedLine{Name: name, Quantity: qty})
				continue
			}

			if block.name != "" {
				if block.qtyLocked && block.qty > 0 {
					flushBlock()
					recordOrphanQty(&out, seen, qty)
					continue
				}
				if block.qty > 0 {
					flushBlock()
					recordOrphanQty(&out, seen, qty)
					continue
				}
				block.setQty(qty, true)
				continue
			}
			recordOrphanQty(&out, seen, qty)
			continue
		}

		if block.name != "" {
			if q, ok := ocrQtyHint(line, block.name); ok {
				block.setQty(q, true)
				continue
			}
			if m := percentQty.FindStringSubmatch(strings.TrimSpace(line)); len(m) >= 2 {
				qty, err := parseQty(m[1])
				if err == nil && qty > 0 && qty <= 1_000_000 {
					block.setQty(qty, true)
					continue
				}
			}
			if m := bareQtyPattern.FindStringSubmatch(line); len(m) >= 2 {
				qty, err := parseQty(m[1])
				if err == nil && qty > 0 && qty <= 1_000_000 {
					block.setQty(qty, false)
					continue
				}
			}
			if m := singleDigitQty.FindStringSubmatch(line); len(m) >= 2 {
				qty, err := parseQty(m[1])
				if err == nil && qty > 0 {
					block.setQty(qty, false)
					continue
				}
			}
			if m := ocrTrailingQty.FindStringSubmatch(strings.TrimSpace(line)); len(m) >= 2 && !containsHangul(line) {
				qty, err := parseQty(m[1])
				if err == nil && qty > 0 && qty <= 1_000_000 {
					block.setQty(qty, false)
					continue
				}
			}
		}

		if name, qty, ok := splitNameQtySuffix(line); ok {
			flushBlock()
			recordParsedLineAny(&out, seen, ParsedLine{Name: name, Quantity: qty})
			continue
		}

		name := NormalizeDisplayName(extractNameFromLine(line))
		if name == "" || len([]rune(name)) < 2 {
			continue
		}
		if isMoneyLikeName(name) || looksLikeQtyOnly(line) {
			continue
		}
		if isGarbageName(name) {
			continue
		}
		if shouldSkipGarbageName(name, block.name) {
			continue
		}

		flushBlock()
		block.name = name
	}

	flushBlock()
	return out
}

func recordParsedLineAny(out *[]ParsedLine, seen map[string]struct{}, line ParsedLine) {
	line.Name = NormalizeDisplayName(line.Name)
	if line.Quantity > 0 {
		if !IsValidParsedLine(line.Name, line.Quantity) {
			return
		}
	} else if !isNameOnlyCandidate(line.Name) && !lineHasFinance(line) {
		return
	}
	key := line.Name + "|" + formatQtyKey(line.Quantity)
	if _, dup := seen[key]; dup {
		return
	}
	seen[key] = struct{}{}
	*out = append(*out, line)
}

func isNameOnlyCandidate(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) < 2 {
		return false
	}
	if isMoneyLikeName(name) || looksLikeMoneyOrPnL(name) || headerNoise.MatchString(name) {
		return false
	}
	if isGarbageName(name) || looksLikeOCRFragment(name) {
		return false
	}
	letters := 0
	for _, r := range name {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || (r >= '가' && r <= '힣') {
			letters++
		}
	}
	return letters >= 2
}

func recordOrphanQty(out *[]ParsedLine, seen map[string]struct{}, qty float64) {
	if qty <= 0 {
		return
	}
	key := "|orphan|" + formatQtyKey(qty)
	if _, dup := seen[key]; dup {
		return
	}
	seen[key] = struct{}{}
	*out = append(*out, ParsedLine{Quantity: qty})
}

func recordFinanceOrphan(out *[]ParsedLine, seen map[string]struct{}, fin financeFields) {
	if !financeFieldsReady(fin) {
		return
	}
	key := "|finance|" + financeFieldsKey(fin)
	if _, dup := seen[key]; dup {
		return
	}
	seen[key] = struct{}{}
	line := ParsedLine{Currency: fin.Currency}
	if fin.MarketValue != nil {
		line.MarketValue = fin.MarketValue
	}
	if fin.ProfitLoss != nil {
		line.ProfitLoss = fin.ProfitLoss
	}
	if fin.ReturnRate != nil {
		line.ReturnRate = fin.ReturnRate
	}
	*out = append(*out, line)
}

func financeFieldsReady(f financeFields) bool {
	return f.MarketValue != nil || f.ProfitLoss != nil || f.ReturnRate != nil
}

func financeFieldsKey(f financeFields) string {
	var b strings.Builder
	if f.MarketValue != nil {
		b.WriteString(formatQtyKey(*f.MarketValue))
	}
	b.WriteByte('|')
	if f.ProfitLoss != nil {
		b.WriteString(formatQtyKey(*f.ProfitLoss))
	}
	b.WriteByte('|')
	if f.ReturnRate != nil {
		b.WriteString(formatQtyKey(*f.ReturnRate))
	}
	return b.String()
}

func formatQtyKey(qty float64) string {
	return strconv.FormatFloat(qty, 'f', -1, 64)
}
