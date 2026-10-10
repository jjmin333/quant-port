package model

import "time"

// Notification — 앱 안 알림 한 줄(헤더 종 아이콘, 2026-10-10). Kind: stock | re_trade | fin_rate.
type Notification struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Href      string    `json:"href"`
	CreatedAt time.Time `json:"createdAt"`
	Read      bool      `json:"read"`
}
