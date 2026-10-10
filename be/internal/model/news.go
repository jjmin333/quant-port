package model

import "time"

type NewsArticle struct {
	ID            string    `json:"id"`
	Market        string    `json:"market"`
	Headline      string    `json:"headline"`
	HeadlineKo    string    `json:"headlineKo,omitempty"`
	Summary       string    `json:"summary,omitempty"`
	SummaryKo     string    `json:"summaryKo,omitempty"`
	SourceID      string    `json:"sourceId"`
	PublisherID   string    `json:"publisherId"`
	PublisherName string    `json:"publisherName"`
	Tags          []string  `json:"tags"`
	Symbols       []string  `json:"symbols,omitempty"`
	Keywords      []string  `json:"keywords,omitempty"`
	PublishedAt   time.Time `json:"publishedAt"`
	ViewCount     int       `json:"viewCount"`
	URL           string    `json:"url,omitempty"`
	OriginalURL   string    `json:"originalUrl,omitempty"`
	PortalURL     string    `json:"portalUrl,omitempty"`
	ImageURL      string    `json:"imageUrl,omitempty"`
	PreviewMode   string    `json:"previewMode,omitempty"`
	EmbedPolicy   string    `json:"embedPolicy,omitempty"`
	Highlighted   bool      `json:"highlighted,omitempty"`
	// Topic — stock(기본) · real_estate · rates · deposit · loan (2026-10-09 요청 4). 국내만 주식 밖이 있다.
	Topic string `json:"topic,omitempty"`
	// AssetTags — 제목에서 수집 때 뽑은 자산 칩(지역·금리). 누르면 자산 미리보기 모달.
	AssetTags   []NewsAssetTag `json:"assetTags,omitempty"`
	MatchedKeys []string       `json:"-"`
}

// NewsAssetTag is one asset chip on an article — kind/key are the asset quick-look's (region=lawdCd, rate=ECOS key).
type NewsAssetTag struct {
	Kind string `json:"kind"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type NewsListFilter struct {
	Market   string
	SourceID string
	Sort     string
	Query    string
	Symbol   string
	// Topic — 비우면 전부. NewsTopics 중 하나.
	Topic string
	Limit int
}

// NewsTopics — 국내 뉴스 카테고리(화면 칩 순서). 「전체」는 빈 값이다.
var NewsTopics = []string{"stock", "real_estate", "rates", "deposit", "loan"}

// NewsTopicStock is the default topic of every article that is not one of the asset topics.
const NewsTopicStock = "stock"
