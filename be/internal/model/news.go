package model

import "time"

type NewsArticle struct {
	ID            string    `json:"id"`
	Market        string    `json:"market"`
	Headline      string    `json:"headline"`
	Summary       string    `json:"summary,omitempty"`
	SourceID      string    `json:"sourceId"`
	PublisherID   string    `json:"publisherId"`
	PublisherName string    `json:"publisherName"`
	Tags          []string  `json:"tags"`
	Symbols       []string  `json:"symbols,omitempty"`
	PublishedAt   time.Time `json:"publishedAt"`
	ViewCount     int       `json:"viewCount"`
	URL           string    `json:"url,omitempty"`
	EmbedPolicy   string    `json:"embedPolicy,omitempty"`
	Highlighted   bool      `json:"highlighted,omitempty"`
	MatchedKeys   []string  `json:"-"`
}

type NewsListFilter struct {
	Market   string
	SourceID string
	Sort     string
	Query    string
	Symbol   string
	Limit    int
}
