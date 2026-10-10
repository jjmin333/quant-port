package service

import (
	"fmt"
	"testing"
)

// sectorFixture builds n banks (PER 5..) and n chip makers (PER 30..) in one
// cap group — the case that makes 시총 그룹 valuation read like a sector test.
func sectorFixture(n int, withSector bool) []*scoreInput {
	var out []*scoreInput
	for i := 0; i < n; i++ {
		bankPER := 5.0 + float64(i)*0.1
		chipPER := 30.0 + float64(i)
		bank := &scoreInput{symbol: fmt.Sprintf("B%02d", i), assetType: "domestic", market: "KOSPI", peer: "domestic:KOSPI:cap3", per: &bankPER}
		chip := &scoreInput{symbol: fmt.Sprintf("C%02d", i), assetType: "domestic", market: "KOSPI", peer: "domestic:KOSPI:cap3", per: &chipPER}
		if withSector {
			bank.sector = "financials"
			chip.sector = "information_technology"
		}
		out = append(out, bank, chip)
	}
	return out
}

// 섹터 그룹이 켜지면 가장 싼 반도체주가 반도체 안에서 1등이 된다 — 시총 그룹에서는
// 은행 전부보다 비싸서 바닥이었다.
func TestSectorPeersRankValuationWithinSector(t *testing.T) {
	inputs := sectorFixture(25, true)
	cov := assignSectorPeers(inputs)
	if !cov["domestic"].enabled {
		t.Fatalf("coverage 100%% should enable sector peers, got %+v", cov["domestic"])
	}
	norm := normalizeUniverse(inputs)
	cheapestChip := norm["C00"]["per"]
	if cheapestChip < 90 {
		t.Errorf("cheapest chip PER percentile = %.1f, want top of its sector", cheapestChip)
	}
	if lvl := inputs[1].metricLevels["per"]; lvl != "domestic:information_technology" {
		t.Errorf("per ranked in %q, want sector group", lvl)
	}
}

// 문턱 아래에서는 누구도 섹터 그룹에 들어가지 않는다 — 반쯤 켜면 같은 지표가 두
// 잣대로 섞인다.
func TestSectorPeersStayOffBelowCoverage(t *testing.T) {
	inputs := sectorFixture(25, true)
	// 절반의 섹터를 지워 커버리지를 50%로.
	for i, in := range inputs {
		if i%2 == 0 {
			in.sector = ""
		}
	}
	cov := assignSectorPeers(inputs)
	if cov["domestic"].enabled {
		t.Fatalf("50%% coverage must stay off, got %+v", cov["domestic"])
	}
	for _, in := range inputs {
		if in.sectorPeer != "" {
			t.Fatalf("%s got sector peer %q below the gate", in.symbol, in.sectorPeer)
		}
	}
	normalizeUniverse(inputs)
	if lvl := inputs[1].metricLevels["per"]; lvl != "domestic:KOSPI:cap3" {
		t.Errorf("per ranked in %q, want the cap group", lvl)
	}
}

// 추세는 섹터와 무관하다 — 재무 지표만 섹터 사슬을 탄다.
func TestPeerLevelsSectorOnlyForFundamentals(t *testing.T) {
	in := &scoreInput{assetType: "us", market: "NASDAQ", peer: "us:NASDAQ:cap2", sectorPeer: "us:health_care"}
	if got := peerLevels(in, false)[0]; got != "us:NASDAQ:cap2" {
		t.Errorf("technical chain starts at %q", got)
	}
	if got := peerLevels(in, true)[0]; got != "us:health_care" {
		t.Errorf("fundamental chain starts at %q", got)
	}
}
