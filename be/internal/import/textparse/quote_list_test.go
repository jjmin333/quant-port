package textparse

import (
	"strings"
	"testing"
)

const sampleQuoteListPSM6 = `@  네vi우스 그룹                              Cex
-$8.72(3.36%)

nm 어도비                                   $258.32
~$0.43 (0.16%)

a  오라클                                       $152.77
-$051 (0.33%)

@ GE BLE                            $1,045.00
+$5.10(0.49%)

@B =                        $134.53
~$0.10(0.07%)

은 피그마                                     $23.71
-$0.04 (0.16%)`

func TestParseQuoteListText(t *testing.T) {
	lines := parseQuoteListText(sampleQuoteListPSM6)
	if len(lines) < 4 {
		t.Fatalf("want >=4 names got %d: %+v", len(lines), lines)
	}
	names := map[string]bool{}
	for _, l := range lines {
		if l.Quantity != 0 {
			t.Fatalf("quote list qty should be 0: %+v", l)
		}
		names[l.Name] = true
	}
	for _, sub := range []string{"네vi우스", "어도비", "오라클", "피그마"} {
		found := false
		for n := range names {
			if strings.Contains(n, sub) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing name containing %q in %+v", sub, names)
		}
	}
}

func TestInferQuoteListOnly(t *testing.T) {
	if !InferQuoteListOnly(sampleQuoteListPSM6) {
		t.Fatal("expected quote-only")
	}
	holdings := `해외주
맵러빈
15주
$1,966.68`
	if InferQuoteListOnly(holdings) {
		t.Fatal("holdings with 주 should not be quote-only")
	}
}
