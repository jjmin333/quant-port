package model

import "time"

// OverseasDisplayDate maps a Finnhub US session calendar date to the KST calendar
// day Korean users expect on the earnings calendar.
//
// DB `event_date` stays on the US trading day so post-earnings reactions line up
// with US session closes. Only read paths shift the `date` field in API responses.
//
//	bmo — before the US open (~KST same calendar evening) → same day
//	amc — after the US close (~KST next morning) → +1 day
//	dmh — during the US session (~KST early next morning) → +1 day
func OverseasDisplayDate(usDate, session string) string {
	if usDate == "" {
		return usDate
	}
	switch session {
	case EarningsSessionAfterClose, EarningsSessionDuringHours:
		d, err := time.ParseInLocation("2006-01-02", usDate, time.UTC)
		if err != nil {
			return usDate
		}
		return d.AddDate(0, 0, 1).Format("2006-01-02")
	default:
		return usDate
	}
}

// ApplyOverseasDisplayDate rewrites Date on one US row (and its alternates) for
// calendar APIs. Call only after collapse and reaction attachment.
func ApplyOverseasDisplayDate(ev *EarningsEvent) {
	if ev == nil || ev.Market != string(AssetUS) {
		return
	}
	ev.Date = OverseasDisplayDate(ev.Date, ev.Session)
	for i := range ev.Alternates {
		ApplyOverseasDisplayDate(&ev.Alternates[i])
	}
}

// ApplyOverseasDisplayDates shifts US calendar dates for KST-facing responses.
func ApplyOverseasDisplayDates(events []EarningsEvent) {
	for i := range events {
		ApplyOverseasDisplayDate(&events[i])
	}
}

// ExpandUSQueryFrom widens the DB window by one day so amc/dmh rows whose US
// trading date is the day before still appear on the requested KST day.
func ExpandUSQueryFrom(from time.Time, market string) time.Time {
	if market == string(AssetUS) || market == "" {
		return from.AddDate(0, 0, -1)
	}
	return from
}

// FilterEventsByCalendarDate keeps rows whose Date (already display-shifted for
// US) falls in [from, to] inclusive.
func FilterEventsByCalendarDate(events []EarningsEvent, from, to time.Time) []EarningsEvent {
	if len(events) == 0 {
		return events
	}
	fromKey := from.Format("2006-01-02")
	toKey := to.Format("2006-01-02")
	out := make([]EarningsEvent, 0, len(events))
	for _, ev := range events {
		if ev.Date >= fromKey && ev.Date <= toKey {
			out = append(out, ev)
		}
	}
	return out
}
