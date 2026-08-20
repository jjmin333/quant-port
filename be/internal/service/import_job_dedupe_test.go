package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestDedupeResolvedPhantomRows(t *testing.T) {
	avg := 260_000.0
	rows := dedupeResolvedPhantomRows([]model.ImportPreviewRow{
		{ID: "row-1", Symbol: "035420", Name: "NAVER", Quantity: 19, Confidence: 0.9},
		{ID: "row-2", Symbol: "065660", Name: "안트로젠", Quantity: 19, AvgPrice: &avg, Confidence: 0.9},
		{ID: "row-3", Symbol: "005930", Name: "삼성전자", Quantity: 34, Confidence: 0.92},
	})
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d: %+v", len(rows), rows)
	}
	for _, row := range rows {
		if row.Symbol == "065660" {
			t.Fatalf("phantom anterogen not dropped: %+v", rows)
		}
	}
}

func TestDedupePreviewBySymbol(t *testing.T) {
	rows := dedupePreviewBySymbol([]model.ImportPreviewRow{
		{ID: "row-1", Symbol: "ARM", Name: "ARM 홀딩스(ADR)", RawName: "ARM 홀딩스(ADR)", Quantity: 50, Confidence: 0.92},
		{ID: "row-2", Symbol: "CCJ", Name: "카메코", RawName: "카메코", Quantity: 27, Confidence: 0.92},
		{ID: "row-3", Symbol: "ARM", Name: "ARM 홀딩스(ADR)", RawName: "ARM 홀딩스(ADR)", Quantity: 50, Confidence: 0.7},
	})
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d: %+v", len(rows), rows)
	}
	if rows[0].Symbol != "ARM" || rows[0].Quantity != 50 {
		t.Fatalf("unexpected ARM row: %+v", rows[0])
	}
	if rows[1].Symbol != "CCJ" {
		t.Fatalf("unexpected second row: %+v", rows[1])
	}
}

func TestDedupePreviewBySymbolSumsDistinctQuantities(t *testing.T) {
	avg35 := 364_500.0
	avg109 := 359_262.0
	rows := dedupePreviewBySymbol([]model.ImportPreviewRow{
		{ID: "row-1", Symbol: "005930", Name: "삼성전자", Quantity: 35, AvgPrice: &avg35, CostCurrency: "KRW"},
		{ID: "row-2", Symbol: "005930", Name: "삼성전자", Quantity: 109, AvgPrice: &avg109, CostCurrency: "KRW"},
	})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %+v", rows)
	}
	if rows[0].Quantity != 144 {
		t.Fatalf("qty=%v want 144", rows[0].Quantity)
	}
	if rows[0].AvgPrice == nil {
		t.Fatal("expected merged avg price")
	}
	want := (avg35*35 + avg109*109) / 144
	if mathAbs(*rows[0].AvgPrice-want) > 1 {
		t.Fatalf("avg=%v want~%v", *rows[0].AvgPrice, want)
	}
}

func mathAbs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
