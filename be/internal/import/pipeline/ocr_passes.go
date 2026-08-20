package pipeline

import (
	"strings"

	"github.com/enact/quant-port/be/internal/import/imageutil"
	"github.com/enact/quant-port/be/internal/import/ocr"
	"github.com/enact/quant-port/be/internal/import/textparse"
)

const leftColumnWidth = 0.55

const leftColumnUpscale = 3

const financeColumnLeft = 0.35

// ImageParseResult holds merged holdings/quote lines and raw OCR passes for validation.
type ImageParseResult struct {
	Lines    []textparse.ParsedLine
	OCRTexts []string
}

// ParseHoldingsImage runs OCR on full frame + left column (no per-row bands — they
// bleed PnL text into phantom tickers). Results are merged, then locale/fragment filters apply.
func ParseHoldingsImage(engine ocr.Engine, croppedPath, preprocessedPath, workDir string) ImageParseResult {
	if !engine.Available() {
		return ImageParseResult{}
	}

	var ocrTexts []string
	var combined []textparse.ParsedLine

	appendPass := func(path string, psms ...string) {
		if strings.TrimSpace(path) == "" {
			return
		}
		if len(psms) == 0 {
			psms = []string{"11"}
		}
		for _, psm := range psms {
			text, err := engine.RecognizeFilePSM(path, psm)
			if err != nil || strings.TrimSpace(text) == "" {
				continue
			}
			ocrTexts = append(ocrTexts, text)
			combined = append(combined, textparse.ParseHoldingsText(text)...)
		}
	}

	appendPass(croppedPath, "11", "6")
	if preprocessedPath != croppedPath {
		appendPass(preprocessedPath, "11", "6")
	}

	leftRaw := workDir + "/left_col.png"
	leftScaled := workDir + "/left_col_x2.png"
	if imageutil.CropRelative(croppedPath, leftRaw, 0, 0, leftColumnWidth, 1) == nil {
		_ = imageutil.UpscaleFile(leftRaw, leftScaled, leftColumnUpscale)
		appendPass(leftScaled, "11", "6", "4")
	}

	financeRaw := workDir + "/finance_col.png"
	financeScaled := workDir + "/finance_col_x2.png"
	if imageutil.CropRelative(croppedPath, financeRaw, financeColumnLeft, 0.12, 1-financeColumnLeft, 0.72) == nil {
		_ = imageutil.UpscaleFile(financeRaw, financeScaled, 2)
		appendPass(financeScaled, "11", "6", "4")
	}

	if len(combined) == 0 {
		return ImageParseResult{OCRTexts: ocrTexts}
	}

	merged := textparse.MergeMultiOCRPass(combined)
	locale := textparse.InferHoldingsLocale(ocrTexts, merged)
	if textparse.InferQuoteListOnly(ocrTexts...) {
		if quotes := textparse.ParseQuoteListFromOCRTexts(ocrTexts...); len(quotes) > 0 {
			merged = textparse.FilterQuoteListLines(quotes)
			if locale == "" {
				locale = "overseas"
			}
		}
	} else {
		merged = textparse.AttachSatelliteFinance(merged, combined)
		if locale == "overseas" {
			merged = textparse.CollapseSimilarRows(textparse.FilterParsedLines(textparse.FuzzyUnifyParsedLines(merged)))
		} else {
			merged = textparse.CollapseSimilarRows(textparse.FilterParsedLines(merged))
		}
		return ImageParseResult{
			Lines:    textparse.FilterLinesForLocale(merged, locale),
			OCRTexts: ocrTexts,
		}
	}
	return ImageParseResult{
		Lines:    textparse.FilterLinesForLocale(merged, locale),
		OCRTexts: ocrTexts,
	}
}
