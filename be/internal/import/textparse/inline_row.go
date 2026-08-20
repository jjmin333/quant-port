package textparse

import (
	"regexp"
	"strings"
)

var inlineNameEvalLine = regexp.MustCompile(`^(.+?)\s+([+\-~]?[\d.,]+)\s*원`)

// parseInlineNameEval extracts "NAME 13,973,000원" Kiwoom-style rows.
func parseInlineNameEval(line string) (name string, fin financeFields, ok bool) {
	m := inlineNameEvalLine.FindStringSubmatch(strings.TrimSpace(line))
	if len(m) < 3 {
		return "", financeFields{}, false
	}
	name = NormalizeDisplayName(extractNameFromLine(strings.TrimSpace(m[1])))
	if name == "" || isMoneyLikeName(name) || looksLikeOCRFragment(name) {
		return "", financeFields{}, false
	}
	amtLine := strings.TrimSpace(m[2]) + "원"
	f, ok := parseFinanceLine(amtLine)
	if !ok || f.MarketValue == nil {
		return "", financeFields{}, false
	}
	return name, f, true
}

// parseInlineQtyFinance extracts "785주 -2,885,560 (-17.12%)" rows.
func parseInlineQtyFinance(line string) (qty float64, fin financeFields, ok bool) {
	m := qtyPattern.FindStringSubmatch(line)
	if len(m) < 2 {
		return 0, financeFields{}, false
	}
	q, err := parseQty(m[1])
	if err != nil || q <= 0 {
		return 0, financeFields{}, false
	}
	rest := strings.TrimSpace(qtyPattern.ReplaceAllString(line, ""))
	if rest == "" {
		return 0, financeFields{}, false
	}
	f, ok := parseFinanceLine(rest)
	if !ok {
		return 0, financeFields{}, false
	}
	return q, f, true
}
