package marketdata

import (
	"strings"
	"time"
)

var newsPubDateLayouts = []string{
	"Mon, 02 Jan 2006 15:04:05 -07:00",
	"Mon, 02 Jan 2006 15:04:05 -0700",
	time.RFC1123Z,
	time.RFC1123,
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05",
	time.RFC3339,
}

// ParseNewsPubDate parses provider pubDate strings. Returns zero time when unknown.
func ParseNewsPubDate(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range newsPubDateLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Time{}
}
