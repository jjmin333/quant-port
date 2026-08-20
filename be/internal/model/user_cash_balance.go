package model

import "time"

type UserCashBalance struct {
	Currency  string    `json:"currency"`
	Amount    float64   `json:"amount"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

type UserCashBalances struct {
	KRW float64 `json:"krw"`
	USD float64 `json:"usd"`
}
