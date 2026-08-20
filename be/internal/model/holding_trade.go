package model

import "time"

// HoldingTrade is one manual buy/sell entry. Together they are the source of
// truth for a symbol's quantity and average cost — user_holdings only caches
// the derived numbers.
type HoldingTrade struct {
	ID        int64     `json:"id"`
	Symbol    string    `json:"symbol"`
	Name      string    `json:"name,omitempty"`
	Side      string    `json:"side"` // "buy" | "sell"
	Quantity  float64   `json:"quantity"`
	Price     float64   `json:"price"`
	Currency  string    `json:"currency"`
	FxRate    *float64  `json:"fxRate,omitempty"`
	TradeDate string    `json:"tradeDate"` // YYYY-MM-DD
	Fee       float64   `json:"fee"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// HoldingPosition is what a symbol's trade list adds up to. Sells reduce the
// quantity but never the average cost — average cost is a weighted mean of the
// buys that are still held, which is how brokerages report it.
type HoldingPosition struct {
	Symbol string `json:"symbol"`
	// Remaining quantity after buys - sells.
	Quantity float64 `json:"quantity"`
	// Weighted average buy price in Currency, or 0 when only sells exist.
	AvgPrice float64 `json:"avgPrice"`
	Currency string  `json:"currency"`
	// Realized gain from sells, in Currency. Informational only.
	RealizedPnL float64 `json:"realizedPnl"`
	TradeCount  int     `json:"tradeCount"`
	FirstDate   string  `json:"firstDate,omitempty"`
}

// ValuationPoint is one day of the portfolio valuation series.
type ValuationPoint struct {
	Date string `json:"date"` // YYYY-MM-DD
	// Market value of everything held that day, converted to KRW.
	ValueKRW float64 `json:"valueKrw"`
	// Cumulative cost of what was held that day, converted to KRW.
	CostKRW float64 `json:"costKrw"`
}

// ValuationSeries answers "is my portfolio going up lately" from real trade
// history, not from today's quantity projected backwards.
type ValuationSeries struct {
	Points []ValuationPoint `json:"points"`
	// Symbols with trades that had no daily close rows to price them with. The
	// UI must say so — a curve missing a symbol is not the portfolio's curve.
	MissingSymbols []string `json:"missingSymbols,omitempty"`
	// Symbols the user holds with no trade history at all. Their value cannot
	// appear on any past day, so the series is not the whole portfolio.
	UntrackedSymbols []string `json:"untrackedSymbols,omitempty"`
	ExchangeRate     float64  `json:"exchangeRate"`
}
