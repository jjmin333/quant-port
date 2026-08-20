package model

type WatchlistItem struct {
	ID         int64   `json:"id"`
	Symbol     string  `json:"symbol"`
	CreatedAt  string  `json:"createdAt"`
	AddedPrice float64 `json:"addedPrice,omitempty"`
}

type WatchlistMetrics struct {
	AddedAt              string   `json:"addedAt"`
	ChangeRatePrevClose  float64  `json:"changeRatePrevClose"`
	PrevClose            float64  `json:"prevClose,omitempty"`
	ChangeRatePrevWeek   *float64 `json:"changeRatePrevWeek,omitempty"`
	PrevWeekClose        float64  `json:"prevWeekClose,omitempty"`
	ChangeRatePrevMonth  *float64 `json:"changeRatePrevMonth,omitempty"`
	PrevMonthClose       float64  `json:"prevMonthClose,omitempty"`
	Week52High           float64  `json:"week52High,omitempty"`
	Week52Low            float64  `json:"week52Low,omitempty"`
	ChangeRateWeek52High *float64 `json:"changeRateWeek52High,omitempty"`
	ChangeRateWeek52Low  *float64 `json:"changeRateWeek52Low,omitempty"`
}
