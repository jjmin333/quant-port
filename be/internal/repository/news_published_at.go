package repository

import "time"

var newsPublishedAtMin = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// NormalizeNewsPublishedAt ensures published_at is valid for MySQL strict mode.
// Zero or pre-2000 timestamps fall back to the current UTC time.
func NormalizeNewsPublishedAt(t time.Time) time.Time {
	if t.IsZero() || t.Before(newsPublishedAtMin) {
		return time.Now().UTC()
	}
	return t.UTC()
}
