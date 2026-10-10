package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/repository"
)

// 시도·전국 집계: 월별 합(평당가는 건수 가중), 거래량 추세, 오른·내린 수, 많이 오른·내린 곳, 순위.
func TestAreaAggFacts(t *testing.T) {
	a := newAreaAgg("11", "서울특별시")
	var ms []repository.RegionMonth
	for i, ym := range []string{"202601", "202602", "202603", "202604", "202605", "202606", "202607", "202608"} {
		ms = append(ms, repository.RegionMonth{YM: ym, Count: 10 + i, AvgPerM2: 100, RentCount: 1})
	}
	a.add(ms)
	a.add([]repository.RegionMonth{{YM: "202608", Count: 2, AvgPerM2: 400}})
	a.subs = []areaSub{{name: "강남구", w4: 0.5, score: 80}, {name: "노원구", w4: -0.2, score: 20}, {name: "중구", w4: 0, score: 50},
		{name: "송파구", w4: 0.3, score: 70}}
	f, vt := a.facts(idxFacts{w4: 0.12, hasIndex: true})

	got := f["months"].([]repository.RegionMonth)
	if last := got[len(got)-1]; last.Count != 19 || last.AvgPerM2 != 132 { // (17*100 + 2*400) / 19 = 131.6
		t.Fatalf("last month = %+v", last)
	}
	// 마지막 달 제외 → 직전 3개월(4·5·6월 = 13+14+15) 대 그 전 3개월(1·2·3월 = 10+11+12).
	if f["recentVol"] != 42 || f["priorVol"] != 33 || vt < 27.2 || vt > 27.3 {
		t.Fatalf("vol = %v %v %v", f["recentVol"], f["priorVol"], vt)
	}
	if f["up"] != 2 || f["down"] != 1 || f["flat"] != 1 {
		t.Fatalf("breadth = %v %v %v", f["up"], f["down"], f["flat"])
	}
	top, bottom := f["top"].([][]any), f["bottom"].([][]any)
	if top[0][0] != "강남구" || len(bottom) != 1 || bottom[0][0] != "노원구" {
		t.Fatalf("top %v bottom %v", top, bottom)
	}
	if r := rankDesc([]float64{50, 80, 50, 10}); r[0] != 2 || r[1] != 1 || r[2] != 2 || r[3] != 4 {
		t.Fatalf("rank = %v", r)
	}
}
