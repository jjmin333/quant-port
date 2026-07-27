package toss

import "encoding/json"

// Envelope is the standard Toss BFF success response wrapper.
type Envelope[T any] struct {
	Result T `json:"result"`
}

// ErrorBody is the standard Toss BFF error response.
type ErrorBody struct {
	Error APIError `json:"error"`
}

type APIError struct {
	RequestID string          `json:"requestId"`
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type Currency string

const (
	CurrencyKRW Currency = "KRW"
	CurrencyUSD Currency = "USD"
)

type MarketCountry string

const (
	MarketKR MarketCountry = "KR"
	MarketUS MarketCountry = "US"
)
