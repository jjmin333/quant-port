package textparse

import "testing"

func TestIsBrokerAppNoise(t *testing.T) {
	noisy := []string{
		"KO 키움자산 다른자",
		"Ki 키움자산 다른자산",
		"키움자산",
	}
	for _, n := range noisy {
		if !isBrokerAppNoise(n) {
			t.Fatalf("%q should be broker noise", n)
		}
		if IsValidParsedLine(n, 72) {
			t.Fatalf("%q should not be valid holding", n)
		}
	}
	if isBrokerAppNoise("삼성전자") || isBrokerAppNoise("KODEX 미국나스") {
		t.Fatal("real holdings should not be broker noise")
	}
	if isOCRLatinDebris("HIGHS") || isOCRLatinDebris("ASHE") {
		// expected
	} else {
		t.Fatal("HIGHS/ASHE should be latin debris")
	}
	if isOCRLatinDebris("NAVER") {
		t.Fatal("NAVER should not be debris")
	}
}
