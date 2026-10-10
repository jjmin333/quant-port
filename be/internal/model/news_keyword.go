package model

import "time"

type NewsPollKeyword struct {
	ID        int64     `json:"id"`
	Market    string    `json:"market"`
	Keyword   string    `json:"keyword"`
	// Topic — "" = 종목 뉴스 큐에 섞이는 일반 키워드 · real_estate · rates · deposit · loan = 주제 차선 검색어(2026-10-09).
	Topic     string    `json:"topic"`
	Enabled   bool      `json:"enabled"`
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
