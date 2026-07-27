package model

type WatchlistItem struct {
	ID        int64  `json:"id"`
	Symbol    string `json:"symbol"`
	CreatedAt string `json:"createdAt"`
}
