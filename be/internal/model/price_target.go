package model

// PriceTarget is one user-set price level ladder for a symbol.
//
// Stop and second target are optional because most people start with a single
// number; the fields exist so that the rest — "where do I cut it", "where do I
// take half off" — stops being prose inside Note.
type PriceTarget struct {
	ID          int64   `json:"id"`
	Symbol      string  `json:"symbol"`
	TargetPrice float64 `json:"targetPrice"`
	/** Second, further target. Nil when the user set only one level. */
	TargetPrice2 *float64 `json:"targetPrice2,omitempty"`
	/** Where the thesis is wrong. Nil when the user set no stop. */
	StopPrice *float64 `json:"stopPrice,omitempty"`
	/** Price when the target was first registered — measured, never supplied. */
	BasePrice *float64 `json:"basePrice,omitempty"`
	/** Trading day BasePrice belongs to (YYYY-MM-DD). */
	BaseDate string `json:"baseDate,omitempty"`
	Currency string `json:"currency"`
	Note     string `json:"note"`

	// ---- derived on read, never stored (see 048_price_target_levels.sql) ----

	/** First day the daily high/low touched the target, "" when never. */
	ReachedDate string `json:"reachedDate,omitempty"`
	/** Highest daily high since BaseDate. */
	PeakPrice *float64 `json:"peakPrice,omitempty"`
	/** Lowest daily low since BaseDate. */
	TroughPrice *float64 `json:"troughPrice,omitempty"`

	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type BulkRegisterTradeItem struct {
	Symbol    string  `json:"symbol"`
	Quantity  float64 `json:"quantity"`
	Price     float64 `json:"price"`
	Currency  string  `json:"currency"`
	TradeDate string  `json:"tradeDate"`
}
