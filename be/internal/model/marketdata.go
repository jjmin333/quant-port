package model

// Market data cache / stream revision and snapshot types.

const (
	MarketDataQuoteCacheSec      = 1
	MarketDataOrderBookCacheSec  = 1
	MarketDataTradesCacheSec     = 1
	MarketDataIndicesCacheSec    = 5
	MarketDataListCacheSec       = 3 // catalog quotes only; user flags stripped before cache (see stripQuoteListForCache)
	MarketDataCandleHistCacheSec = 3600        // completed daily+ bars
	MarketDataCandleTailCacheSec = 90          // intraday tail
	MarketDataLastGoodCacheSec   = 30 * 24 * 3600 // stale detail/trades when market closed
)

type CandleSnapshot struct {
	Symbol     string         `json:"symbol"`
	Interval   CandleInterval `json:"interval"`
	Completed  []Candle       `json:"completed"`
	Forming    *Candle        `json:"forming,omitempty"`
	NextBefore *string        `json:"nextBefore,omitempty"`
	Rev        int64          `json:"rev"`
	Source     string         `json:"source"`
}

type QuoteTerminalSnapshot struct {
	Detail    QuoteDetail  `json:"detail"`
	OrderBook OrderBook    `json:"orderBook"`
	Trades    []TradeTick  `json:"trades"`
	Source    string       `json:"source"`
}

// StreamEvent is pushed over SSE to chart clients.
type StreamEvent struct {
	Type     string         `json:"type"` // snapshot | bar_update | bar_close
	Symbol   string         `json:"symbol"`
	Interval CandleInterval `json:"interval,omitempty"`
	Bar      *Candle        `json:"bar,omitempty"`
	Forming  *Candle        `json:"forming,omitempty"`
	Rev      int64          `json:"rev"`
}

// QuoteStreamEvent is pushed over SSE for live header/detail price updates.
type QuoteStreamEvent struct {
	Type          string  `json:"type"` // quote_update
	Symbol        string  `json:"symbol"`
	CurrentPrice  float64 `json:"currentPrice"`
	Change        float64 `json:"change"`
	ChangeRate    float64 `json:"changeRate"`
	PrevClose     float64 `json:"prevClose"`
	RegularClose  float64 `json:"regularClose,omitempty"`
	MarketSession string  `json:"marketSession,omitempty"`
	UpdatedAt     string  `json:"updatedAt"`
}
