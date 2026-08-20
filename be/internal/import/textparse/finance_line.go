package textparse

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	returnPctInParens = regexp.MustCompile(`\(([+\-~]?[\d,.]+)\s*%\)`)
	returnPctLoose    = regexp.MustCompile(`\(([+\-~]?[\d,.]+)\s*%`)
	krwAmountPattern  = regexp.MustCompile(`([+\-~]?[\d.,]+)\s*원`)
	krwMoneyToken     = regexp.MustCompile(`([+\-~]?\d{1,3}(?:[.,]\d{3})+|\d{1,3}(?:,\d{3})+)`)
	usdAmountPattern  = regexp.MustCompile(`([+\-~+]?\$)\s*([\d,]+\.?\d*)`)
)

// financeFields holds monetary OCR fragments for one holding block.
type financeFields struct {
	MarketValue *float64
	ProfitLoss  *float64
	ReturnRate  *float64
	Currency    string
}

func parseFinanceLine(line string) (financeFields, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return financeFields{}, false
	}

	var out financeFields
	hasData := false

	if rate, ok := extractReturnRate(trimmed); ok {
		out.ReturnRate = &rate
		hasData = true
	}

	if strings.Contains(trimmed, "원") {
		if amt, isPnL, ok := parseKRWAmount(trimmed); ok {
			hasData = true
			out.Currency = "KRW"
			if isPnL {
				out.ProfitLoss = &amt
			} else {
				out.MarketValue = &amt
			}
		}
	} else if amt, isPnL, ok := parseKRWBareAmount(trimmed); ok {
		hasData = true
		out.Currency = "KRW"
		if isPnL {
			out.ProfitLoss = &amt
		} else {
			out.MarketValue = &amt
		}
	}

	if strings.Contains(trimmed, "$") {
		if amt, isPnL, ok := parseUSDAmount(trimmed); ok {
			hasData = true
			if out.Currency == "" {
				out.Currency = "USD"
			}
			if isPnL {
				if out.ProfitLoss == nil {
					out.ProfitLoss = &amt
				}
			} else if out.MarketValue == nil {
				out.MarketValue = &amt
			}
		}
	} else if amt, isPnL, ok := parseUSDBareAmount(trimmed); ok {
		hasData = true
		out.Currency = "USD"
		if isPnL {
			if out.ProfitLoss == nil {
				out.ProfitLoss = &amt
			}
		} else if out.MarketValue == nil {
			out.MarketValue = &amt
		}
	}

	return out, hasData
}

func extractReturnRate(line string) (float64, bool) {
	m := returnPctInParens.FindStringSubmatch(line)
	if len(m) < 2 {
		m = returnPctLoose.FindStringSubmatch(line)
	}
	if len(m) < 2 {
		return 0, false
	}
	raw := strings.TrimSpace(m[1])
	sign := 1.0
	if strings.HasPrefix(raw, "-") || strings.HasPrefix(raw, "~") {
		sign = -1
		raw = strings.TrimLeft(raw, "-~")
	} else if strings.HasPrefix(raw, "+") {
		raw = strings.TrimPrefix(raw, "+")
	}
	v, err := parseQty(raw)
	if err != nil {
		return 0, false
	}
	return sign * v / 100, true
}

func parseKRWAmount(line string) (amount float64, isPnL bool, ok bool) {
	m := krwAmountPattern.FindStringSubmatch(line)
	if len(m) < 2 {
		return 0, false, false
	}
	raw := strings.TrimSpace(m[1])
	sign := 1.0
	if strings.HasPrefix(raw, "-") {
		sign = -1
		raw = strings.TrimPrefix(raw, "-")
		isPnL = true
	} else if strings.HasPrefix(raw, "~") {
		sign = -1
		isPnL = true
		raw = strings.TrimPrefix(raw, "~")
	} else if strings.HasPrefix(raw, "+") {
		isPnL = true
		raw = strings.TrimPrefix(raw, "+")
	}
	v, err := parseMoneyAmount(raw)
	if err != nil {
		return 0, false, false
	}
	amount = sign * v
	if returnPctInParens.MatchString(line) && !isPnL {
		// e.g. rare combined format; treat signed-looking parens as PnL context
	}
	if returnPctInParens.MatchString(line) {
		isPnL = true
	}
	if !isPnL && strings.Contains(line, "(") && strings.Contains(line, "%") {
		isPnL = true
	}
	return amount, isPnL, true
}

func parseKRWBareAmount(line string) (amount float64, isPnL bool, ok bool) {
	if strings.Contains(line, "$") || strings.Contains(line, "주") {
		return 0, false, false
	}
	if !returnPctInParens.MatchString(line) && !looksLikeBareKRWAmount(line) {
		return 0, false, false
	}
	amountPart := line
	if idx := strings.Index(line, "("); idx > 0 {
		amountPart = line[:idx]
	}
	if strings.Contains(amountPart, ",") && strings.Contains(amountPart, ".") {
		return parseKRWMixedPnLAmount(line)
	}
	m := krwMoneyToken.FindStringSubmatch(line)
	if len(m) < 2 {
		return 0, false, false
	}
	raw := strings.TrimSpace(m[1])
	sign := 1.0
	if strings.HasPrefix(raw, "-") {
		sign = -1
		raw = strings.TrimPrefix(raw, "-")
		isPnL = true
	} else if strings.HasPrefix(raw, "~") {
		sign = -1
		isPnL = true
		raw = strings.TrimPrefix(raw, "~")
	} else if strings.HasPrefix(raw, "+") {
		isPnL = true
		raw = strings.TrimPrefix(raw, "+")
	}
	v, err := parseMoneyAmount(raw)
	if err != nil || v < 1000 {
		return 0, false, false
	}
	if returnPctInParens.MatchString(line) {
		isPnL = true
	}
	return sign * v, isPnL, true
}

func parseKRWMixedPnLAmount(line string) (amount float64, isPnL bool, ok bool) {
	amountPart := line
	if idx := strings.Index(line, "("); idx > 0 {
		amountPart = line[:idx]
	}
	trimmed := strings.TrimSpace(amountPart)
	sign := 1.0
	if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "~") {
		sign = -1
		isPnL = true
	} else if strings.HasPrefix(trimmed, "+") {
		isPnL = true
	}
	var digits strings.Builder
	for _, r := range amountPart {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	if digits.Len() < 4 {
		return 0, false, false
	}
	v, err := strconv.ParseFloat(digits.String(), 64)
	if err != nil || v < 1000 {
		return 0, false, false
	}
	if returnPctInParens.MatchString(line) {
		isPnL = true
	}
	return sign * v, isPnL, true
}

func looksLikeBareKRWAmount(line string) bool {
	digits := 0
	for _, r := range line {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	return digits >= 5 && !containsHangul(line)
}

func parseMoneyAmount(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, " ", "")
	if raw == "" {
		return 0, strconv.ErrSyntax
	}

	sign := 1.0
	if strings.HasPrefix(raw, "-") {
		sign = -1
		raw = strings.TrimPrefix(raw, "-")
	} else if strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "~") {
		raw = strings.TrimLeft(raw, "+~")
	}

	if strings.Contains(raw, ",") && strings.Contains(raw, ".") {
		// OCR glitches: "1,225,.870", "-2.885,560" — drop grouping noise, keep digits.
		var b strings.Builder
		for _, r := range raw {
			if r >= '0' && r <= '9' {
				b.WriteRune(r)
			}
		}
		raw = b.String()
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return 0, err
		}
		return sign * v, nil
	}

	if strings.Contains(raw, ",") {
		raw = strings.ReplaceAll(raw, ",", "")
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return 0, err
		}
		return sign * v, nil
	}

	if strings.Contains(raw, ".") {
		parts := strings.Split(raw, ".")
		if len(parts) >= 2 && len(parts[len(parts)-1]) == 3 && allDotGroupsThreeDigits(parts) {
			raw = strings.Join(parts, "")
			v, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return 0, err
			}
			return sign * v, nil
		}
	}

	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	return sign * v, nil
}

func allDotGroupsThreeDigits(parts []string) bool {
	for i, p := range parts {
		if i == 0 {
			if len(p) < 1 || len(p) > 3 {
				return false
			}
			continue
		}
		if len(p) != 3 {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return len(parts) >= 2
}

var usdBareMoneyToken = regexp.MustCompile(`^([+\-~]?[\d,]+\.?\d*)`)

func parseUSDBareAmount(line string) (amount float64, isPnL bool, ok bool) {
	if strings.Contains(line, "$") || strings.Contains(line, "원") || strings.Contains(line, "주") {
		return 0, false, false
	}
	if !returnPctInParens.MatchString(line) && !returnPctLoose.MatchString(line) {
		return 0, false, false
	}
	m := usdBareMoneyToken.FindStringSubmatch(strings.TrimSpace(line))
	if len(m) < 2 {
		return 0, false, false
	}
	raw := strings.TrimSpace(m[1])
	sign := 1.0
	if strings.HasPrefix(raw, "-") {
		sign = -1
		raw = strings.TrimPrefix(raw, "-")
		isPnL = true
	} else if strings.HasPrefix(raw, "~") {
		sign = -1
		isPnL = true
		raw = strings.TrimPrefix(raw, "~")
	} else if strings.HasPrefix(raw, "+") {
		isPnL = true
		raw = strings.TrimPrefix(raw, "+")
	}
	v, err := parseMoneyAmount(raw)
	if err != nil || v <= 0 || v > 1_000_000 {
		return 0, false, false
	}
	if returnPctInParens.MatchString(line) {
		isPnL = true
	}
	return sign * v, isPnL, true
}

func parseUSDAmount(line string) (amount float64, isPnL bool, ok bool) {
	m := usdAmountPattern.FindStringSubmatch(line)
	if len(m) < 3 {
		return 0, false, false
	}
	prefix := m[1]
	raw := m[2]
	sign := 1.0
	if strings.Contains(prefix, "-") {
		sign = -1
		isPnL = true
	} else if strings.Contains(prefix, "+") || strings.Contains(prefix, "~") {
		isPnL = true
	}
	v, err := parseQty(raw)
	if err != nil {
		return 0, false, false
	}
	amount = sign * v
	if returnPctInParens.MatchString(line) {
		isPnL = true
	}
	return amount, isPnL, true
}

type pendingBlock struct {
	name        string
	qty         float64
	qtyLocked   bool
	marketValue *float64
	profitLoss  *float64
	returnRate  *float64
	currency    string
}

func (b *pendingBlock) reset() {
	b.name = ""
	b.qty = 0
	b.qtyLocked = false
	b.marketValue = nil
	b.profitLoss = nil
	b.returnRate = nil
	b.currency = ""
}

func (b *pendingBlock) applyFinance(f financeFields) {
	if f.MarketValue != nil {
		b.marketValue = f.MarketValue
	}
	if f.ProfitLoss != nil {
		b.profitLoss = f.ProfitLoss
	}
	if f.ReturnRate != nil {
		b.returnRate = f.ReturnRate
	}
	if f.Currency != "" {
		b.currency = f.Currency
	}
}

func (b *pendingBlock) setQty(qty float64, locked bool) {
	if qty <= 0 {
		return
	}
	if b.qtyLocked && b.qty > 0 {
		return
	}
	if b.qtyLocked && !locked {
		return
	}
	b.qty = qty
	if locked {
		b.qtyLocked = true
	}
}

func (b *pendingBlock) toParsedLine() ParsedLine {
	line := ParsedLine{
		Name:     b.name,
		Quantity: b.qty,
		Currency: b.currency,
	}
	if b.marketValue != nil {
		line.MarketValue = b.marketValue
	}
	if b.profitLoss != nil {
		line.ProfitLoss = b.profitLoss
	}
	if b.returnRate != nil {
		line.ReturnRate = b.returnRate
	}
	return line
}

func (b *pendingBlock) ready() bool {
	return b.name != "" && b.qty > 0
}
