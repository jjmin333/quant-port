// Command ocr-debug runs multi-pass OCR + parsing on a holdings screenshot.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/enact/quant-port/be/internal/import/imageutil"
	"github.com/enact/quant-port/be/internal/import/ocr"
	"github.com/enact/quant-port/be/internal/import/pipeline"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ocr-debug <image.png>")
		os.Exit(1)
	}
	src := os.Args[1]
	dir := filepath.Dir(src)
	work := filepath.Join(dir, "ocr_debug_work")
	_ = os.MkdirAll(work, 0o755)

	engine := ocr.DefaultEngine()
	if !engine.Available() {
		fmt.Fprintln(os.Stderr, "tesseract not available")
		os.Exit(1)
	}

	scaled := filepath.Join(work, "scaled_x2.png")
	_ = imageutil.UpscaleFile(src, scaled, 2)
	pre := filepath.Join(work, "preprocessed.png")
	if err := imageutil.PreprocessForOCR(scaled, pre); err != nil {
		pre = scaled
	}

	lines := pipeline.ParseHoldingsImage(engine, src, pre, work).Lines
	fmt.Println("=== parsed holdings ===")
	for _, l := range lines {
		fmt.Printf("  %s / %.0f주\n", l.Name, l.Quantity)
	}
	if len(lines) == 0 {
		os.Exit(1)
	}
}
