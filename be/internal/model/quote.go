package model

type QuoteCurrency string

const (
	CurrencyKRW QuoteCurrency = "KRW"
	CurrencyUSD QuoteCurrency = "USD"
)

type QuoteAssetType string

const (
	AssetDomestic QuoteAssetType = "domestic"
	AssetUS       QuoteAssetType = "us"
	AssetETF      QuoteAssetType = "etf"
)

type CandleInterval string

const (
	Interval1m  CandleInterval = "1m"
	Interval5m  CandleInterval = "5m"
	Interval10m CandleInterval = "10m"
	Interval30m CandleInterval = "30m"
	Interval60m CandleInterval = "60m"
	Interval1d  CandleInterval = "1d"
	Interval1w  CandleInterval = "1w"
	Interval1M  CandleInterval = "1M"
)

type MarketIndex struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Value      float64       `json:"value"`
	Change     float64       `json:"change"`
	ChangeRate float64       `json:"changeRate"`
	Currency   QuoteCurrency `json:"currency"`
	Badge      string        `json:"badge,omitempty"`
	Sparkline  []float64     `json:"sparkline,omitempty"`
	UpdatedAt  string        `json:"updatedAt,omitempty"`
	DataKind   IndexDataKind `json:"dataKind,omitempty"`
}

// IndexDataKind describes whether a value is live or end-of-day.
type IndexDataKind string

const (
	IndexDataKindLive IndexDataKind = "live"
	IndexDataKindEOD  IndexDataKind = "eod"
)

type IndexGroupMeta struct {
	UpdatedAt string `json:"updatedAt"`
	Source    string `json:"source"`
}

type QuoteSummary struct {
	Symbol         string         `json:"symbol"`
	Name           string         `json:"name"`
	Market         string         `json:"market"`
	Currency       QuoteCurrency  `json:"currency"`
	AssetType      QuoteAssetType `json:"assetType"`
	CurrentPrice   float64        `json:"currentPrice"`
	Change         float64        `json:"change"`
	ChangeRate     float64        `json:"changeRate"`
	Volume         int64          `json:"volume"`
	TradingValue   float64        `json:"tradingValue"`
	IsWatchlisted  bool           `json:"isWatchlisted"`
	IsHeld         bool           `json:"isHeld"`
}

type QuoteDetail struct {
	QuoteSummary
	Open        float64  `json:"open"`
	High        float64  `json:"high"`
	Low         float64  `json:"low"`
	PrevClose   float64  `json:"prevClose"`
	Week52High  float64  `json:"week52High"`
	Week52Low   float64  `json:"week52Low"`
	MarketCap   float64  `json:"marketCap"`
	PER         *float64 `json:"per"`
	PBR         *float64 `json:"pbr"`
	UpdatedAt   string   `json:"updatedAt"`
}

type OrderBookLevel struct {
	Price      float64 `json:"price"`
	Quantity   int64   `json:"quantity"`
	OrderCount int     `json:"orderCount"`
}

type OrderBook struct {
	Symbol    string           `json:"symbol"`
	Asks      []OrderBookLevel `json:"asks"`
	Bids      []OrderBookLevel `json:"bids"`
	Timestamp string           `json:"timestamp"`
}

type TradeTick struct {
	ID       string  `json:"id"`
	Symbol   string  `json:"symbol"`
	Time     string  `json:"time"`
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
	Side     string  `json:"side"`
}

type Candle struct {
	Time   string  `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
}
