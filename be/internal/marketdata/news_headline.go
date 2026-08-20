package marketdata

import (
	"regexp"
	"strings"
	"unicode"
)

var trailingPublisherSuffixRe = regexp.MustCompile(`(?i)\s+[-–—|]\s+[^-–—|]{2,80}$`)

// NormalizeNewsHeadlineKey collapses whitespace and strips wire-specific noise for dedupe.
func NormalizeNewsHeadlineKey(headline string) string {
	headline = strings.TrimSpace(headline)
	if headline == "" {
		return ""
	}
	lower := strings.ToLower(headline)
	for _, prefix := range []string{"financialjuice:", "financial juice:"} {
		if strings.HasPrefix(lower, prefix) {
			headline = strings.TrimSpace(headline[len(prefix):])
			lower = strings.ToLower(headline)
			break
		}
	}
	headline = trailingPublisherSuffixRe.ReplaceAllString(headline, "")
	headline = strings.TrimSpace(strings.TrimSuffix(headline, "..."))
	headline = strings.TrimSpace(strings.TrimSuffix(headline, "…"))

	var b strings.Builder
	b.Grow(len(headline))
	prevSpace := false
	for _, r := range strings.ToLower(headline) {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// NewsHeadlineMatchKeys returns normalized keys used to detect duplicate headlines.
func NewsHeadlineMatchKeys(headline, headlineKo string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(raw string) {
		key := NormalizeNewsHeadlineKey(raw)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	add(headline)
	add(headlineKo)
	return out
}
