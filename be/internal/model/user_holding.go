package model

import "time"

type UserHolding struct {
	ID         int64     `json:"id"`
	Symbol     string    `json:"symbol"`
	Name       string    `json:"name,omitempty"`
	Quantity   float64   `json:"quantity"`
	AvgPrice   *float64  `json:"avgPrice,omitempty"`
	CostCurrency string  `json:"costCurrency,omitempty"`
	BrokerHint string    `json:"brokerHint,omitempty"`
	Source     string    `json:"source"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type ImportJobStatus string

const (
	ImportJobPending    ImportJobStatus = "pending"
	ImportJobProcessing ImportJobStatus = "processing"
	ImportJobPreview    ImportJobStatus = "preview"
	ImportJobFailed     ImportJobStatus = "failed"
	ImportJobCommitted  ImportJobStatus = "committed"
)

type ImportCropRect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type ImportImageMeta struct {
	Crop       *ImportCropRect `json:"crop,omitempty"`
	BrokerHint string          `json:"brokerHint,omitempty"`
}

type ImportPreviewRow struct {
	ID              string   `json:"id"`
	RawName         string   `json:"rawName"`
	Symbol          string   `json:"symbol,omitempty"`
	Name            string   `json:"name,omitempty"`
	Quantity        float64  `json:"quantity"`
	AvgPrice        *float64 `json:"avgPrice,omitempty"`
	CostCurrency    string   `json:"costCurrency,omitempty"`
	Confidence      float64  `json:"confidence"`
	Selected        bool     `json:"selected"`
	NeedsReview     bool     `json:"needsReview"`
	BrokerHint      string   `json:"brokerHint,omitempty"`
	ResolveError    string   `json:"resolveError,omitempty"`
	SourceImageIdx  int      `json:"sourceImageIdx"`
}

type ImportJobProgress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// ImportImageResult — 이미지 1장당 OCR 처리 결과 (index: 0-based).
type ImportImageResult struct {
	Index   int    `json:"index"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type ImportJobSnapshot struct {
	JobID         string              `json:"jobId"`
	UserID        int64               `json:"userId"`
	Status        ImportJobStatus     `json:"status"`
	Progress      ImportJobProgress   `json:"progress"`
	Preview       []ImportPreviewRow  `json:"preview,omitempty"`
	ImageResults  []ImportImageResult `json:"imageResults,omitempty"`
	Warning       string              `json:"warning,omitempty"`
	Error         string              `json:"error,omitempty"`
	CreatedAt     time.Time           `json:"createdAt"`
	UpdatedAt     time.Time           `json:"updatedAt"`
}

type ImportCommitRow struct {
	ID           string   `json:"id"`
	Symbol       string   `json:"symbol"`
	Quantity     float64  `json:"quantity"`
	AvgPrice     *float64 `json:"avgPrice,omitempty"`
	CostCurrency string   `json:"costCurrency,omitempty"`
	Selected     bool     `json:"selected"`
}

type ImportCommitRequest struct {
	Rows            []ImportCommitRow `json:"rows"`
	AddToWatchlist  bool              `json:"addToWatchlist"`
	ReplaceExisting bool              `json:"replaceExisting"`
}
