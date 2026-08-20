package marketdata

import (
	"regexp"
	"strings"
)

var exchangeTickerRe = regexp.MustCompile(`(?i)(?:NYSE|NASDAQ|AMEX|OTC(?:QX|PK)?)\s*:\s*([A-Z]{1,5})`)
var possessiveHeadlineRe = regexp.MustCompile(`(?i)^([A-Z][A-Za-z0-9&][A-Za-z0-9&.\- ]{0,28}?)'s\b`)
var cashtagRe = regexp.MustCompile(`\$([A-Z]{1,5})(?:[^A-Za-z]|$)`)
var headlineTickerMoveRe = regexp.MustCompile(`(?i)^([A-Z]{1,5})\s+\d+(?:\.\d+)?%`)

// ExtractPossessiveHeadlineSubject returns the leading company name in titles like "Tesla's ...".
func ExtractPossessiveHeadlineSubject(headline string) string {
	headline = strings.TrimSpace(headline)
	if headline == "" {
		return ""
	}
	if m := possessiveHeadlineRe.FindStringSubmatch(headline); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// ExtractCashtags finds tickers written as "$AAPL" in headlines and summaries.
func ExtractCashtags(text string) []string {
	text = strings.ToUpper(text)
	seen := map[string]struct{}{}
	var out []string
	for _, m := range cashtagRe.FindAllStringSubmatch(text, -1) {
		if len(m) < 2 {
			continue
		}
		sym := strings.ToUpper(strings.TrimSpace(m[1]))
		if sym == "" || !isLikelyTicker(sym) {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}
	return out
}

// ExtractHeadlineTickers finds a leading Latin ticker before a percentage move, e.g. "IREN 17.55%".
func ExtractHeadlineTickers(headline string) []string {
	headline = strings.TrimSpace(headline)
	if headline == "" {
		return nil
	}
	m := headlineTickerMoveRe.FindStringSubmatch(headline)
	if len(m) < 2 {
		return nil
	}
	sym := strings.ToUpper(strings.TrimSpace(m[1]))
	if sym == "" || !isLikelyTicker(sym) {
		return nil
	}
	return []string{sym}
}

// ExtractExchangeTickers finds tickers like "NYSE: KO" or "NASDAQ: MSFT" in wire text.
func ExtractExchangeTickers(text string) []string {
	text = strings.ToUpper(text)
	seen := map[string]struct{}{}
	var out []string
	for _, m := range exchangeTickerRe.FindAllStringSubmatch(text, -1) {
		if len(m) < 2 {
			continue
		}
		sym := strings.ToUpper(strings.TrimSpace(m[1]))
		if sym == "" || !isLikelyTicker(sym) {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}
	return out
}

// ParseRSSTickerList splits comma-separated feed tickers (e.g. Nasdaq rss tickers field).
func ParseRSSTickerList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		sym := strings.ToUpper(strings.TrimSpace(part))
		if sym == "" || !isLikelyTicker(sym) {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}
	return out
}

func isLikelyTicker(sym string) bool {
	if len(sym) < 1 || len(sym) > 5 {
		return false
	}
	for _, r := range sym {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func MergeTickerSymbols(existing []string, extra ...string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(existing)+len(extra))
	for _, sym := range append(existing, extra...) {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" || !isLikelyTicker(sym) {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}
	return out
}
