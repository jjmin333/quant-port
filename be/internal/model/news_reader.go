package model

import "time"

type NewsReaderContent struct {
	URL             string    `json:"url"`
	Title           string    `json:"title"`
	OriginalTitle   string    `json:"originalTitle,omitempty"`
	Byline          string    `json:"byline,omitempty"`
	Excerpt         string    `json:"excerpt,omitempty"`
	OriginalExcerpt string    `json:"originalExcerpt,omitempty"`
	SiteName        string    `json:"siteName,omitempty"`
	ImageURL        string    `json:"imageUrl,omitempty"`
	TextContent     string    `json:"textContent"`
	ContentHTML     string    `json:"contentHtml,omitempty"`
	PublishedAt     time.Time `json:"publishedAt,omitempty"`
	FetchedAt       time.Time `json:"fetchedAt"`
	Translated      bool      `json:"translated,omitempty"`
}

type NewsUsDemoArticle struct {
	NewsArticle
	ReaderMode bool `json:"readerMode"`
}
