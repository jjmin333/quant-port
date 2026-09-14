package textparse

import (

	"regexp"

	"strconv"

	"strings"

	"unicode"

)

var (

	qtyPattern     = regexp.MustCompile(`(\d[\d,]*)\s*주`)

	bareQtyPattern = regexp.MustCompile(`^(\d[\d,]*)\s*%?$`)

	percentQty     = regexp.MustCompile(`^(\d[\d,]*)%$`)

	nameQtySuffix  = regexp.MustCompile(`^(.+?)\s+(\d{1,7})$`)

	singleDigitQty = regexp.MustCompile(`^(\d{1,3})$`)

	ocrTrailingQty = regexp.MustCompile(`^(\d[\d,]*)\D+$`)

	headerNoise    = regexp.MustCompile(`(?i)^(총|합계|평가|손익|수익|원|USD|₩|보유|종목|잔고|계좌|증권|주문|체결|시간|현재|전일|등락|거래|금액|비중|수량|해외|국내|내주식|국내주식|내투자|내 투자|직접|설정|omit|재가|현재가|평가금|더보기|탐색|관심|혜택|메뉴|채우기|보내기|베뉴|환전|소수점)`)

	brokerUINoise  = regexp.MustCompile(`(?i)(키움자산|다른자산|다른자|미래에셋|한국투자|NH투자|신한투자|KB증권|계좌의\s*총\s*자산|개\s*려나|가나다순|가격순|수량순)`)

	garbageName    = regexp.MustCompile(`(?i)^[@«»_a-z]{1,3}$|^[\p{Hangul}]{1}$|^[.\s]+$|^[a-z]\s+\S`)

	moneyLikeName  = regexp.MustCompile(`^[\d,.]+%?$|^[\d,.]+원?$`)

)

type ParsedLine struct {
	Name     string

	Quantity float64

	MarketValue *float64

	ProfitLoss *float64

	ReturnRate *float64

	Currency string
}

func ParseHoldingsText(text string) []ParsedLine {
	return parseHoldingsBlocks(text)
}

// FilterParsedLines removes OCR noise rows before preview/resolver.

func FilterParsedLines(items []ParsedLine) []ParsedLine {
	out := make([]ParsedLine, 0, len(items))

	for _, item := range items {
		if IsValidParsedLine(item.Name, item.Quantity) {
			out = append(out, item)
		}
	}

	return out
}

// FilterQuoteListLines keeps name-only rows from watchlist/quote OCR.
func FilterQuoteListLines(items []ParsedLine) []ParsedLine {
	out := make([]ParsedLine, 0, len(items))
	for _, item := range items {
		name := NormalizeDisplayName(item.Name)
		if item.Quantity == 0 && isNameOnlyCandidate(name) {
			out = append(out, ParsedLine{Name: name, Quantity: 0})
		}
	}
	return out
}

// IsValidParsedLine returns false for money-like OCR fragments and zero qty rows.

func IsValidParsedLine(name string, qty float64) bool {
	raw := strings.TrimSpace(name)
	if qty <= 0 || raw == "" {
		return false
	}

	if len([]rune(raw)) < 2 {
		return false
	}

	if isGarbageName(raw) || looksLikeOCRFragment(raw) {
		return false
	}

	name = NormalizeDisplayName(raw)

	if isMoneyLikeName(name) || looksLikeMoneyOrPnL(name) || nameEmbedsAmount(name) {
		return false
	}

	if isGarbageName(name) || headerNoise.MatchString(name) {
		return false
	}

	if isBrokerAppNoise(name) {
		return false
	}

	if looksLikeOCRFragment(name) {
		return false
	}

	letters := 0

	for _, r := range name {
		if unicode.IsLetter(r) {
			letters++
		}
	}

	return letters >= 1
}

func recordLine(out *[]ParsedLine, seen map[string]struct{}, name string, qty float64) {
	if !IsValidParsedLine(name, qty) {
		return
	}

	key := name + "|" + strconv.FormatFloat(qty, 'f', -1, 64)

	if _, dup := seen[key]; dup {
		return
	}

	seen[key] = struct{}{}

	*out = append(*out, ParsedLine{Name: name, Quantity: qty})
}

func isSkippableLine(line string) bool {
	trimmed := strings.TrimSpace(line)

	if trimmed == "" {
		return true
	}

	if strings.Contains(trimmed, "주") {
		return false
	}

	if percentQty.MatchString(trimmed) {
		return false
	}

	if singleDigitQty.MatchString(trimmed) {
		return false
	}

	if strings.HasSuffix(trimmed, "원") {
		return true
	}

	if strings.HasPrefix(trimmed, "$") || strings.HasPrefix(trimmed, "-$") ||

		strings.HasPrefix(trimmed, "+$") || strings.HasPrefix(trimmed, "~$") {
		return true
	}

	if strings.Contains(trimmed, "%") && strings.ContainsAny(trimmed, "$+-~") {
		return true
	}

	return looksLikeMoneyOrPnL(trimmed) || isMoneyLikeName(trimmed)
}

func isMoneyLikeName(s string) bool {
	s = strings.TrimSpace(s)

	if s == "" {
		return true
	}

	if moneyLikeName.MatchString(s) {
		return true
	}

	if strings.HasSuffix(s, "원") && digitRatio(s) >= 0.4 {
		return true
	}

	return digitRatio(s) >= 0.75 && len([]rune(s)) >= 4
}

func nameEmbedsAmount(name string) bool {
	name = strings.TrimSpace(name)
	if strings.Count(name, ",") >= 1 && digitRatio(name) >= 0.2 {
		return true
	}
	fields := strings.Fields(name)
	if len(fields) >= 2 {
		tail := fields[len(fields)-1]
		if digitRatio(tail) >= 0.5 && len([]rune(tail)) >= 4 {
			return true
		}
	}
	return false
}

func digitRatio(s string) float64 {
	if s == "" {
		return 0
	}

	digits := 0

	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits++
		}
	}

	return float64(digits) / float64(len([]rune(s)))
}

func looksLikeMoneyOrPnL(s string) bool {
	s = strings.TrimSpace(s)

	if s == "" || strings.Contains(s, "주") {
		return false
	}

	if bareQtyPattern.MatchString(s) || percentQty.MatchString(s) {
		return false
	}

	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") || strings.HasPrefix(s, "~") {
		if strings.ContainsAny(s, "$%(") {
			return true
		}
	}

	if hangulRunes(s) >= 2 {
		return false
	}

	if !strings.ContainsAny(s, "$,.)") && !strings.HasSuffix(s, "원") {
		return false
	}

	return digitRatio(s) >= 0.45
}

func ocrQtyHint(line, pendingName string) (float64, bool) {
	trimmed := strings.TrimSpace(line)
	if m := singleDigitQty.FindStringSubmatch(trimmed); len(m) >= 2 {
		qty, err := parseQty(m[1])
		if err == nil && qty > 0 && qty <= 999 {
			return qty, true
		}
	}
	if m := orphanQtySuffix.FindStringSubmatch(trimmed); len(m) >= 2 {
		qty, err := parseQty(m[1])
		if err == nil && qty > 0 && qty <= 999 {
			return qty, true
		}
	}
	if hangulRunes(pendingName) >= 2 {
		if qty, ok := decodeOCRLetterQty(trimmed); ok {
			return qty, true
		}
	}
	return 0, false
}

// decodeOCRLetterQty interprets 1–3 letter OCR debris as digits using common OCR confusions (not symbol-specific).
func decodeOCRLetterQty(line string) (float64, bool) {
	if line == "" || len(line) > 3 {
		return 0, false
	}
	allLetters := true
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			continue
		}
		allLetters = false
		break
	}
	if !allLetters {
		return 0, false
	}
	var b strings.Builder
	for i := 0; i < len(line); i++ {
		d, ok := ocrConfusionDigit(line[i])
		if !ok {
			return 0, false
		}
		b.WriteByte(d)
	}
	raw := b.String()
	if len(raw) == 1 {
		qty, err := parseQty(raw)
		if err != nil || qty <= 0 || qty > 999 {
			return 0, false
		}
		return qty, true
	}
	// Multi-letter lines: accept only when every char maps to the same digit (one blob misread).
	first := raw[0]
	for i := 1; i < len(raw); i++ {
		if raw[i] != first {
			return 0, false
		}
	}
	qty, err := parseQty(string(first))
	if err != nil || qty <= 0 || qty > 999 {
		return 0, false
	}
	return qty, true
}

func ocrConfusionDigit(c byte) (byte, bool) {
	switch c {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return c, true
	case 'o', 'O', 'q', 'Q', 'D':
		return '0', true
	case 'l', 'I', 'i', '|', '!':
		return '1', true
	case 'z', 'Z':
		return '2', true
	case 'e', 'E':
		return '3', true
	case 'a', 'A', 'h', 'H':
		return '4', true
	case 's', 'S':
		return '5', true
	case 'G', 'b':
		return '6', true
	case 'T', 't':
		return '7', true
	case 'B':
		return '8', true
	case 'g':
		return '9', true
	default:
		return 0, false
	}
}

var orphanQtySuffix = regexp.MustCompile(`^(\d{1,3})\s*주?$`)

func shouldSkipGarbageName(name, pending string) bool {
	if pending == "" {
		return false
	}

	if len([]rune(pending)) < 3 {
		return false
	}

	return isGarbageName(name)
}

func isBrokerAppNoise(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if brokerUINoise.MatchString(name) {
		return true
	}
	upper := strings.ToUpper(name)
	if (strings.HasPrefix(upper, "KO ") || strings.HasPrefix(upper, "KI ")) && strings.Contains(name, "키움") {
		return true
	}
	return false
}

var domesticLatinTickers = map[string]struct{}{
	"NAVER": {}, "SK": {}, "LG": {}, "KT": {}, "GS": {}, "HD": {}, "CJ": {}, "POSCO": {},
}

// isOCRLatinDebris drops 3–6 letter all-caps OCR garbage on domestic lists (e.g. HIGHS, ASHE).
func isOCRLatinDebris(name string) bool {
	if hangulRunes(name) > 0 {
		return false
	}
	if etfIssuerPrefix(name) != "" {
		return false
	}
	token := strings.ToUpper(strings.Fields(strings.TrimSpace(name))[0])
	if _, ok := domesticLatinTickers[token]; ok {
		return false
	}
	if len(token) < 3 || len(token) > 6 {
		return false
	}
	for _, r := range token {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func isGarbageName(name string) bool {
	return garbageName.MatchString(strings.TrimSpace(name))
}

func splitNameQtySuffix(line string) (string, float64, bool) {
	m := nameQtySuffix.FindStringSubmatch(strings.TrimSpace(line))

	if len(m) < 3 {
		return "", 0, false
	}

	name := cleanName(m[1])

	qty, err := parseQty(m[2])

	if err != nil || qty <= 0 || qty > 1_000_000 {
		return "", 0, false
	}

	if name == "" || len([]rune(name)) < 2 {
		return "", 0, false
	}

	if isMoneyLikeName(name) || looksLikeMoneyOrPnL(name) || headerNoise.MatchString(name) {
		return "", 0, false
	}

	return name, qty, true
}

func extractNameFromLine(line string) string {
	if idx := strings.Index(line, "$"); idx > 0 {
		line = strings.TrimSpace(line[:idx])
	}

	if idx := strings.Index(line, "원"); idx > 0 && digitRatio(line[:idx]) > 0.5 {
		line = strings.TrimSpace(line[:idx])
	}

	line = strings.TrimLeft(line, "@«»_·•|- ")

	name := cleanName(line)

	if isMoneyLikeName(name) {
		return ""
	}

	return name
}

func looksLikeQtyOnly(line string) bool {
	stripped := strings.TrimSpace(qtyPattern.ReplaceAllString(line, ""))

	return stripped == ""
}

func normalizeLine(s string) string {
	s = strings.TrimSpace(s)

	s = strings.ReplaceAll(s, "\t", " ")

	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}

	return s
}

func cleanName(s string) string {
	replacer := strings.NewReplacer(

		"|", " ",

		"·", " ",

		"•", " ",

		"[", " ",

		"]", " ",

	)

	s = replacer.Replace(s)

	s = strings.TrimSpace(s)

	parts := strings.Fields(s)

	if len(parts) == 0 {
		return ""
	}

	if len(parts) > 8 {
		parts = parts[:8]
	}

	return strings.Join(parts, " ")
}

func parseQty(raw string) (float64, error) {
	raw = strings.ReplaceAll(raw, ",", "")

	raw = strings.ReplaceAll(strings.TrimSpace(raw), " ", "")

	return strconv.ParseFloat(raw, 64)
}

func MergeParsedLines(items []ParsedLine) []ParsedLine {
	quoteLines := FilterQuoteListLines(items)

	items = FilterParsedLines(items)

	merged := map[string]float64{}

	display := map[string]string{}

	finance := map[string]ParsedLine{}

	order := []string{}

	for _, item := range items {
		key := mergeKey(item.Name)

		if key == "" {
			continue
		}

		if _, ok := merged[key]; !ok {
			order = append(order, key)

			display[key] = strings.TrimSpace(item.Name)

			merged[key] = item.Quantity

			finance[key] = item

			continue
		}

		existing := merged[key]
		if item.Quantity > existing*5 || existing > item.Quantity*5 {
			if item.Quantity < existing || (item.MarketValue != nil && finance[key].MarketValue == nil) {
				merged[key] = item.Quantity
				display[key] = strings.TrimSpace(item.Name)
				finance[key] = item
			}
			continue
		}

		merged[key] += item.Quantity

		finance[key] = mergeCombinedFinance(finance[key], item, merged[key])
	}

	out := make([]ParsedLine, 0, len(order))

	seen := map[string]struct{}{}

	for _, key := range order {
		line := finance[key]

		line.Name = display[key]

		line.Quantity = merged[key]

		out = append(out, line)

		seen[key] = struct{}{}
	}

	for _, q := range quoteLines {
		key := mergeKey(q.Name)
		if key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		out = append(out, q)
	}

	return out
}

// MergeDualOCRPass combines two OCR parses of the same image without summing duplicate rows.

func MergeDualOCRPass(items []ParsedLine) []ParsedLine {
	items = FilterParsedLines(items)

	merged := map[string]float64{}

	display := map[string]string{}

	finance := map[string]ParsedLine{}

	order := []string{}

	for _, item := range items {
		key := mergeKey(item.Name)

		if key == "" {
			continue
		}

		if _, ok := merged[key]; !ok {
			order = append(order, key)

			display[key] = strings.TrimSpace(item.Name)

			merged[key] = item.Quantity

			finance[key] = item

			continue
		}

		merged[key] = mergeDualQty(merged[key], item.Quantity)

		finance[key] = pickRicherFinance(finance[key], item)
	}

	out := make([]ParsedLine, 0, len(order))

	for _, key := range order {
		line := finance[key]

		line.Name = display[key]

		line.Quantity = merged[key]

		out = append(out, line)
	}

	return out
}

func mergeDualQty(existing, next float64) float64 {
	if existing <= 0 {
		return next
	}

	if next <= 0 {
		return existing
	}

	if existing == next {
		return existing
	}

	if existing < next {
		return existing
	}

	return next
}

func mergeKey(name string) string {
	name = NormalizeDisplayName(name)
	key := strings.ToLower(name)
	var hangul, latin strings.Builder
	for _, r := range key {
		if unicode.Is(unicode.Hangul, r) {
			hangul.WriteRune(r)
		} else if r >= 'a' && r <= 'z' {
			latin.WriteRune(r)
		}
	}
	// Keep ETF issuer prefix (KODEX/TIGER) — same index name must not collapse rows.
	if latin.Len() >= 2 && hangul.Len() >= 2 {
		return latin.String() + hangul.String()
	}
	if hangul.Len() >= 2 {
		return hangul.String()
	}
	if latin.Len() > 0 {
		return latin.String() + hangul.String()
	}
	return hangul.String()
}
