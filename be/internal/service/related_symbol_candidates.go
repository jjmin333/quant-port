package service

import "strings"

// knownRelatedETFOverrides lists leveraged/inverse ETF tickers that do not follow
// the generic prefix heuristic (e.g. SNXX for SNDK).
var knownRelatedETFOverrides = map[string][]string{
	"QQQ":  {"TQQQ", "SQQQ", "QLD", "PSQ"},
	"SNDK": {"SNXX", "SNDQ", "SNDU", "SNDG"},
}

// relatedSymbolCandidates returns ETF symbol guesses for a primary underlying ticker.
func relatedSymbolCandidates(primary string) []string {
	sym := strings.ToUpper(strings.TrimSpace(primary))
	if sym == "" {
		return nil
	}

	seen := map[string]struct{}{sym: {}}
	out := make([]string, 0, 12)

	add := func(candidate string) {
		candidate = strings.ToUpper(strings.TrimSpace(candidate))
		if candidate == "" || candidate == sym {
			return
		}
		if len(candidate) > 6 {
			return
		}
		if _, dup := seen[candidate]; dup {
			return
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}

	for _, c := range knownRelatedETFOverrides[sym] {
		add(c)
	}

	if len(sym) == 3 {
		add("T" + sym)
		add("S" + sym)
	}

	if len(sym) >= 3 {
		prefix := sym[:3]
		for _, suffix := range []string{"Q", "U", "G", "X", "D", "L"} {
			add(prefix + suffix)
		}
	}

	if len(sym) >= 2 {
		add(sym[:2] + "XX")
	}

	return out
}
