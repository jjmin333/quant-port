package model

import "time"

type NewsPollKeyword struct {
	ID        int64     `json:"id"`
	Market    string    `json:"market"`
	Keyword   string    `json:"keyword"`
	Enabled   bool      `json:"enabled"`
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
