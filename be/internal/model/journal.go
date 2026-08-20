package model

// JournalNote is one memo.
//
// PriceAtWrite is the linked stock's price at the moment the note was saved.
// It is written once and never recalculated: a note is a record, not a
// prediction, so the server attaches no verdict to it — only the number that
// makes the note readable again months later.
type JournalNote struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Symbol     string `json:"symbol,omitempty"`
	SymbolName string `json:"symbolName,omitempty"`
	/** Nil when the note has no linked stock, or predates the snapshot column. */
	PriceAtWrite  *float64 `json:"priceAtWrite,omitempty"`
	PriceCurrency string   `json:"priceCurrency,omitempty"`
	/** Trading day the snapshot price belongs to (YYYY-MM-DD). */
	PriceDate string   `json:"priceDate,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Pinned    bool     `json:"pinned"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

// JournalTagPresets are offered in the compose form. Free-form tags are also
// accepted — the presets exist so that the common four are spelled the same way
// by everyone, which is what makes tag filtering worth anything.
var JournalTagPresets = []string{"매수근거", "리스크", "관찰", "복기"}
