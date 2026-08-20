package costestimate_test

import (
	"testing"

	"github.com/enact/quant-port/be/internal/import/costestimate"
	"github.com/enact/quant-port/be/internal/import/textparse"
)

func TestEstimateSamsungScreenshotOCRText(t *testing.T) {
	text := `국내주식

삼성5미

1.852.010원

ae

89,9908! (4.63%)

삼성전자

8.176.383원

34주

~4,216,61781 (34.02%)

한화솔루션

1.994.953원

55주

+122,2038 (6.52%)

한화시스템

1.689.162원

23주

-222.138원(11.62%:

현대차

1.610.530원

a

~473,47081 (22.71%)

16전자"

1.274.056원

주

-129,444원(9.225

NAVER

4,047,794

19주

-892,2218! (18.06%)`

	lines := textparse.FilterLinesForLocale(
		textparse.MergeMultiOCRPass(textparse.ParseHoldingsText(text)),
		"domestic",
	)
	byQty := map[float64]textparse.ParsedLine{}
	for _, l := range lines {
		if l.Quantity > 0 {
			byQty[l.Quantity] = l
		}
	}
	checkQty := func(qty float64, wantAvg float64) {
		l, ok := byQty[qty]
		if !ok {
			t.Fatalf("missing qty %v", qty)
		}
		res := costestimate.Estimate(l)
		if res.AvgPrice == nil {
			t.Fatalf("qty %v missing avg: line=%+v", qty, l)
		}
	if mathAbs(*res.AvgPrice-wantAvg) > 1 {
		t.Fatalf("qty %v avg=%v want %v (line=%+v)", qty, *res.AvgPrice, wantAvg, l)
	}
	}
	// SDI orphan qty row in text fixture; verify via direct OCR block fields.
	for _, l := range lines {
		if l.Quantity == 4 && l.MarketValue != nil && *l.MarketValue > 1_800_000 {
			res := costestimate.Estimate(l)
			if res.AvgPrice == nil || mathAbs(*res.AvgPrice-485_500) > 1 {
				t.Fatalf("SDI avg=%v want 485500 line=%+v", res.AvgPrice, l)
			}
			break
		}
	}
	checkQty(34, 364_500)
	checkQty(19, 260_000)
}

func mathAbs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
