package model

import "time"

// Stock is the persisted security master row (Toss StockInfo + catalog metadata).
type Stock struct {
	Symbol            string         `json:"symbol"`
	Name              string         `json:"name"`
	EnglishName       string         `json:"englishName"`
	ISINCode          string         `json:"isinCode"`
	Market            string         `json:"market"`
	SecurityType      string         `json:"securityType"`
	Currency          QuoteCurrency  `json:"currency"`
	AssetType         QuoteAssetType `json:"assetType"`
	LogoURL           string         `json:"logoUrl,omitempty"`
	Status            string         `json:"status"`
	SharesOutstanding *int64         `json:"sharesOutstanding,omitempty"`
	ListDate          *time.Time     `json:"listDate,omitempty"`
	DelistDate        *time.Time     `json:"delistDate,omitempty"`
	UpdatedAt         time.Time      `json:"updatedAt"`
}

type StockSearchParams struct {
	Query     string
	Market    string // "", "domestic", "us"
	AssetType QuoteAssetType
	Limit     int
	Offset    int
}
