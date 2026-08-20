package model

// InsiderTransaction is one insider ownership-change record — a DART
// 임원ㆍ주요주주 소유보고 row (KR) or an SEC Form 4 non-derivative transaction
// (US). Fields are optional because the two sources report different shapes:
// DART reports a point-in-time snapshot (total holdings + change since the
// prior report), Form 4 reports a discrete dated transaction (code, price,
// shares, resulting holdings).
type InsiderTransaction struct {
	Date             string   `json:"date"`
	ReporterName     string   `json:"reporterName"`
	Relation         string   `json:"relation,omitempty"`
	TransactionType  string   `json:"transactionType,omitempty"`
	SharesChange     *int64   `json:"sharesChange,omitempty"`
	ChangeRate       *float64 `json:"changeRate,omitempty"`
	SharesOwnedAfter *int64   `json:"sharesOwnedAfter,omitempty"`
	OwnershipRate    *float64 `json:"ownershipRate,omitempty"`
	PricePerShare    *float64 `json:"pricePerShare,omitempty"`
	Source           string   `json:"source"` // "dart" | "edgar"
}

// InsiderTransactionRow is one InsiderTransaction plus the identity the table
// stores it under. The view type above carries no key because the screen never
// needs one; the ingest path does, so that re-running a batch updates rows
// instead of duplicating them.
type InsiderTransactionRow struct {
	Symbol string
	// FilingKey identifies the originating filing — an EDGAR accession number,
	// or a content hash for DART (elestock returns no receipt number).
	FilingKey string
	// LineNo is the transaction's position inside that filing, 0 when the
	// source reports one row per filing.
	LineNo int
	Txn    InsiderTransaction
}

// InsiderTabView is the full response for GET /stocks/{symbol}/insider.
type InsiderTabView struct {
	Symbol       string               `json:"symbol"`
	AssetType    QuoteAssetType       `json:"assetType"`
	Transactions []InsiderTransaction `json:"transactions"`
	Note         string               `json:"note,omitempty"`
}
