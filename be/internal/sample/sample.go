// Package sample serves one fixed demo dataset per "내 투자" menu.
//
// Why the server owns this rather than the client:
//
//	Every one of these menus reads a login-gated endpoint, so a logged-out
//	visitor cannot be shown anything at all — there is no data path that reaches
//	them. Putting the fixtures behind a public route gives that visitor the real
//	screen, and keeps one definition for every client instead of a copy per
//	front end.
//
// The fixtures are Go values, not rows in a demo account. A demo user would sit
// in the same tables as everyone else, which means every aggregate, batch job
// and admin screen would need to remember to exclude it. These cannot leak into
// a query because they are never written.
//
// Dates are computed relative to now so the sample never looks abandoned: a
// stamp dated 2026-05-17 reads as stale the moment the calendar moves past it.
package sample

import (
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// Menu names the sample sets, one per "내 투자" entry.
type Menu string

const (
	MenuStamps    Menu = "stamps"
	MenuHoldings  Menu = "holdings"
	MenuWatchlist Menu = "watchlist"
	MenuJournal   Menu = "journal"
	MenuTargets   Menu = "targets"
)

// day is the unit every relative date below is built from.
const day = 24 * time.Hour

func f64(v float64) *float64 { return &v }
func i(v int) *int           { return &v }

// ago returns a UTC instant n days back, pinned to mid-session so the rendered
// KST/ET times look like something a person would actually have done.
func ago(now time.Time, days int) time.Time {
	return now.Add(-time.Duration(days) * day).Truncate(time.Hour).Add(4 * time.Hour)
}

func ahead(now time.Time, days int) time.Time {
	return now.Add(time.Duration(days) * day).Truncate(time.Hour).Add(6*time.Hour + 30*time.Minute)
}

func dateOf(t time.Time) string { return t.In(model.KST).Format("2006-01-02") }
