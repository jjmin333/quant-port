package costestimate_test

import (
	"math"
	"testing"

	"github.com/enact/quant-port/be/internal/import/costestimate"
	"github.com/enact/quant-port/be/internal/import/textparse"
)

func TestEstimateNAVERFromOCRText(t *testing.T) {
	text := `NAVER

4,047,794

19주

-892,2218! (18.06%)`
	lines := textparse.FilterLinesForLocale(
		textparse.MergeMultiOCRPass(textparse.ParseHoldingsText(text)),
		"domestic",
	)
	var naver textparse.ParsedLine
	for _, l := range lines {
		if l.Quantity == 19 {
			naver = l
		}
	}
	if naver.Quantity != 19 {
		t.Fatalf("lines=%+v", lines)
	}
	res := costestimate.Estimate(naver)
	if res.AvgPrice == nil {
		t.Fatalf("no avg: %+v", naver)
	}
	t.Logf("line=%+v avg=%v", naver, *res.AvgPrice)
	if math.Abs(*res.AvgPrice-260_000) > 1 {
		t.Fatalf("avg=%v want 260000", *res.AvgPrice)
	}
}
