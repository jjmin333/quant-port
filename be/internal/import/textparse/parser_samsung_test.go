package textparse

import "testing"

func TestParseHoldingsTextActualSamsungScreenshot(t *testing.T) {	text := `국내주식

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

	lines := FilterLinesForLocale(
		MergeMultiOCRPass(ParseHoldingsText(text)),
		"domestic",
	)
	wantQty := map[string]float64{
		"삼성전자":  34,
		"한화솔루션": 55,
		"한화시스템": 23,
		"NAVER": 19,
	}
	for key, qty := range wantQty {
		if !containsQty(lines, key, qty) {
			t.Fatalf("missing %s %v주 in %+v", key, qty, lines)
		}
	}
	byQty := map[float64]ParsedLine{}
	for _, l := range lines {
		if l.Quantity > 0 {
			byQty[l.Quantity] = l
		}
	}
	checkEval := func(qty float64, minEval float64) {
		l, ok := byQty[qty]
		if !ok {
			t.Fatalf("missing qty %v", qty)
		}
		if l.MarketValue == nil || *l.MarketValue < minEval {
			t.Fatalf("qty %v eval too low: %+v", qty, l)
		}
	}
	checkEval(4, 1_000_000)
	checkEval(34, 5_000_000)
	checkEval(55, 1_000_000)
	checkEval(23, 1_000_000)
	checkEval(19, 3_000_000)
}


func containsQty(lines []ParsedLine, nameSubstr string, qty float64) bool {
	for _, l := range lines {
		if l.Quantity == qty && (l.Name == nameSubstr || mergeKey(l.Name) == mergeKey(nameSubstr)) {
			return true
		}
	}
	return false
}
