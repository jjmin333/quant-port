package resolver

import (
	"fmt"
	"testing"
)

func TestMatchBestCatalogSamsungDigitTail(t *testing.T) {
	catalog := []catalogEntry{
		{Symbol: "006400", Name: "삼성SDI"},
		{Symbol: "006405", Name: "삼성SDI우"},
		{Symbol: "005930", Name: "삼성전자"},
		{Symbol: "028260", Name: "삼성물산"},
		{Symbol: "207940", Name: "삼성바이오로직스"},
		{Symbol: "018260", Name: "삼성에스디에스"},
	}
	for i := 1; i <= 40; i++ {
		catalog = append(catalog, catalogEntry{
			Symbol: fmt.Sprintf("9%04d", i),
			Name:   fmt.Sprintf("삼성테스트%d", i),
		})
	}
	best, score, ok := matchBestCatalog("삼성5미", catalog)
	if !ok || best.Symbol != "006400" {
		t.Fatalf("삼성5미: ok=%v best=%+v score=%.3f", ok, best, score)
	}
}

func TestMatchBestCatalogElectronicsPrefixCollision(t *testing.T) {
	catalog := []catalogEntry{
		{Symbol: "066570", Name: "LG전자"},
		{Symbol: "005930", Name: "삼성전자"},
	}
	for i := 0; i < 120; i++ {
		catalog = append(catalog, catalogEntry{
			Symbol: fmt.Sprintf("7%04d", i),
			Name:   fmt.Sprintf("%c%c전자", 'A'+i/26, 'A'+i%26),
		})
	}
	for _, raw := range []string{"16전자", "L6전자"} {
		best, _, ok := matchBestCatalog(raw, catalog)
		if !ok || best.Symbol != "066570" {
			t.Fatalf("%q: ok=%v best=%+v", raw, ok, best)
		}
	}
}
