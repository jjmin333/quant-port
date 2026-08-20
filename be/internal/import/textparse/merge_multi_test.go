package textparse

import "testing"

func TestMergeMultiOCRPassMajorityQty(t *testing.T) {
	merged := MergeMultiOCRPass([]ParsedLine{
		{Name: "한화시스템", Quantity: 23},
		{Name: "한화시스템", Quantity: 23},
		{Name: "한화시스템", Quantity: 19},
	})
	if len(merged) != 1 || merged[0].Quantity != 23 {
		t.Fatalf("unexpected: %+v", merged)
	}
}

func TestMergeMultiOCRPassDropsGarbageNames(t *testing.T) {
	merged := MergeMultiOCRPass([]ParsedLine{
		{Name: ". 삼성전자", Quantity: 34},
		{Name: "e NAVER", Quantity: 192},
		{Name: "삼성전자", Quantity: 34},
	})
	if len(merged) != 1 || merged[0].Name != "삼성전자" {
		t.Fatalf("unexpected: %+v", merged)
	}
}
