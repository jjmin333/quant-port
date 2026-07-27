package model

import "time"

type NewsReaderContent struct {
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Byline      string    `json:"byline,omitempty"`
	Excerpt     string    `json:"excerpt,omitempty"`
	SiteName    string    `json:"siteName,omitempty"`
	TextContent string    `json:"textContent"`
	ContentHTML string    `json:"contentHtml,omitempty"`
	PublishedAt time.Time `json:"publishedAt,omitempty"`
	FetchedAt   time.Time `json:"fetchedAt"`
}

type NewsUsDemoArticle struct {
	NewsArticle
	ReaderMode bool `json:"readerMode"`
}
