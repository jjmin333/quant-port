package model

// BriefingEvent is a dated disclosure or earnings headline frozen onto stamps
// and research calls. The name is historical (it used to live on the home
// portfolio briefing); the JSON shape is part of stored snapshots.
type BriefingEvent struct {
	Date   string `json:"date"`
	Symbol string `json:"symbol"`
	Event  string `json:"event"`
}
