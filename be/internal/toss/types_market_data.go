package toss

type OrderbookEntry struct {
	Price  string `json:"price"`
	Volume string `json:"volume"`
}

type OrderbookResponse struct {
	Timestamp *string         `json:"timestamp"`
	Currency  Currency        `json:"currency"`
	Asks      []OrderbookEntry `json:"asks"`
	Bids      []OrderbookEntry `json:"bids"`
}

type PriceResponse struct {
	Symbol    string   `json:"symbol"`
	Timestamp *string  `json:"timestamp"`
	LastPrice string   `json:"lastPrice"`
	Currency  Currency `json:"currency"`
}

type TradeResponse struct {
	Price     string   `json:"price"`
	Volume    string   `json:"volume"`
	Timestamp string   `json:"timestamp"`
	Currency  Currency `json:"currency"`
}

type PriceLimitResponse struct {
	Timestamp       string   `json:"timestamp"`
	UpperLimitPrice *string  `json:"upperLimitPrice"`
	LowerLimitPrice *string  `json:"lowerLimitPrice"`
	Currency        Currency `json:"currency"`
}

type CandleInterval string

const (
	CandleInterval1m CandleInterval = "1m"
	CandleInterval1d CandleInterval = "1d"
)

type Candle struct {
	Timestamp  string   `json:"timestamp"`
	OpenPrice  string   `json:"openPrice"`
	HighPrice  string   `json:"highPrice"`
	LowPrice   string   `json:"lowPrice"`
	ClosePrice string   `json:"closePrice"`
	Volume     string   `json:"volume"`
	Currency   Currency `json:"currency"`
}

type CandlePageResponse struct {
	Candles    []Candle `json:"candles"`
	NextBefore *string  `json:"nextBefore"`
}
