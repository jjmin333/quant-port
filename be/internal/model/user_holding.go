package model

import "time"

type UserHolding struct {
	ID           int64     `json:"id"`
	Symbol       string    `json:"symbol"`
	Name         string    `json:"name,omitempty"`
	Quantity     float64   `json:"quantity"`
	AvgPrice     *float64  `json:"avgPrice,omitempty"`
	CostCurrency string    `json:"costCurrency,omitempty"`
	BrokerHint   string    `json:"brokerHint,omitempty"`
	Source       string    `json:"source"`
	UpdatedAt    time.Time `json:"updatedAt"`
	// Sector · SectorLabel — 공통 11섹터(stock_sectors). 모르면 비어 있다 — 보유
	// 구성 도넛은 그 종목을 「미분류」 조각으로 그린다. 라벨은 서버가 준다(화면이
	// 한국어 섹터 이름을 따로 들고 있지 않게).
	Sector      string `json:"sector,omitempty"`
	SectorLabel string `json:"sectorLabel,omitempty"`
	// ThesisAxes — 살 때 본 축(최대 셋). 보유 이유 점검이 종합 대신 이 축들을 추적한다.
	ThesisAxes []string `json:"thesisAxes,omitempty"`
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
	ID             string   `json:"id"`
	RawName        string   `json:"rawName"`
	Symbol         string   `json:"symbol,omitempty"`
	Name           string   `json:"name,omitempty"`
	Quantity       float64  `json:"quantity"`
	AvgPrice       *float64 `json:"avgPrice,omitempty"`
	CostCurrency   string   `json:"costCurrency,omitempty"`
	Confidence     float64  `json:"confidence"`
	Selected       bool     `json:"selected"`
	NeedsReview    bool     `json:"needsReview"`
	BrokerHint     string   `json:"brokerHint,omitempty"`
	ResolveError   string   `json:"resolveError,omitempty"`
	SourceImageIdx int      `json:"sourceImageIdx"`
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
	JobID        string              `json:"jobId"`
	UserID       int64               `json:"userId"`
	Status       ImportJobStatus     `json:"status"`
	Progress     ImportJobProgress   `json:"progress"`
	Preview      []ImportPreviewRow  `json:"preview,omitempty"`
	ImageResults []ImportImageResult `json:"imageResults,omitempty"`
	Warning      string              `json:"warning,omitempty"`
	Error        string              `json:"error,omitempty"`
	CreatedAt    time.Time           `json:"createdAt"`
	UpdatedAt    time.Time           `json:"updatedAt"`
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
