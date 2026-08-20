// Command ocr-cost-debug runs OCR pipeline and prints avg price estimates.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/enact/quant-port/be/internal/import/costestimate"
	"github.com/enact/quant-port/be/internal/import/ocr"
	"github.com/enact/quant-port/be/internal/import/pipeline"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ocr-cost-debug <image.png>")
		os.Exit(1)
	}
	src := os.Args[1]
	work := filepath.Join(filepath.Dir(src), "ocr_cost_debug_work")
	_ = os.MkdirAll(work, 0o755)

	engine := ocr.DefaultEngine()
	if !engine.Available() {
		fmt.Fprintln(os.Stderr, "tesseract not available")
		os.Exit(1)
	}

	lines := pipeline.ParseHoldingsImage(engine, src, src, work).Lines
	fmt.Println("=== holdings + avg price ===")
	for _, l := range lines {
		cost := costestimate.Estimate(l)
		fmt.Printf("%s / %.0f주", l.Name, l.Quantity)
		if l.MarketValue != nil {
			fmt.Printf(" eval=%.0f", *l.MarketValue)
		}
		if l.ProfitLoss != nil {
			fmt.Printf(" pnl=%.0f", *l.ProfitLoss)
		}
		if l.ReturnRate != nil {
			fmt.Printf(" ret=%.2f%%", *l.ReturnRate*100)
		}
		if cost.AvgPrice != nil {
			fmt.Printf(" => avg=%.0f (%s %s)", *cost.AvgPrice, cost.Method, cost.Currency)
		} else {
			fmt.Print(" => avg=(none)")
		}
		fmt.Println()
	}
	if len(lines) == 0 {
		os.Exit(1)
	}
}
