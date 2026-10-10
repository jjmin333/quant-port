package service

import (
	"context"
	"math"
	"sort"
	
)

/*
금융상품 분석 화면 맨 위 「AI 시장 분석」 입력 (2026-10-09 사용자 요청 6 — 비교가 아니라 정보 + AI 분석).

홈 부동산·금리 브리핑과 같은 길(briefs 단계 · asset_ai_notes kind "brief")로 두 키를 더한다:

	fin_savings  정기예금·적금 — 권역(은행·저축은행)별 12개월 기본·최고금리 평균, 최고금리 상위 3, 기준금리
	fin_loans    주담대·전세·신용 — 종류별 평균금리 평균·최저, 낮은 상위 3, 기준금리

숫자는 반올림(0.01%p)해 해시가 같으면 부르지 않는다 — 공시는 한 달에 한 번 바뀌어 대부분의 날은 0콜, 기사 제목이
바뀌는 날만 다시 쓴다(하루 최대 2콜). 기사 제목은 topicNews(예·적금/대출 · 금리, 3일 · 5건)로만 붙는다.
*/

type finGroupAgg struct{ base, max []float64 }

func avgOf(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// finGroupKey — 은행(020000)과 저축은행(030300)만 따로, 나머지(여신·보험 등)는 "etc".
func finGroupKey(g string) string {
	switch g {
	case "020000":
		return "bank"
	case "030300":
		return "sb"
	}
	return "etc"
}

// finMarketInputs builds the two brief inputs from the active product table. 공시가 없으면 그 키는 빠진다.
func (s *AssetAIService) finMarketInputs(ctx context.Context, baseRate float64) map[string]map[string]any {
	out := map[string]map[string]any{}
	if s.fin == nil {
		return out
	}
	today := s.now().In(kstLoc()).Format("20060102")

	savings := map[string]any{}
	for _, kind := range []string{"deposit", "saving"} {
		items, err := s.fin.List(ctx, kind, today)
		if err != nil || len(items) == 0 {
			continue
		}
		groups := map[string]*finGroupAgg{}
		type top struct {
			name string
			max  float64
		}
		var tops []top
		for _, p := range items {
			r := ProductRates(kind, p.Options)
			if !r.OK || r.Term != 12 {
				continue
			}
			g := groups[finGroupKey(p.Group)]
			if g == nil {
				g = &finGroupAgg{}
				groups[finGroupKey(p.Group)] = g
			}
			g.base, g.max = append(g.base, r.Base), append(g.max, r.Max)
			tops = append(tops, top{p.CompanyName + " " + p.ProductName, r.Max})
		}
		sort.Slice(tops, func(i, j int) bool { return tops[i].max > tops[j].max })
		k := map[string]any{"n": len(tops)}
		for name, g := range groups {
			if name == "etc" {
				continue
			}
			k[name] = []float64{r2(avgOf(g.base)), r2(avgOf(g.max))}
		}
		var t3 [][]any
		for _, t := range tops[:min(3, len(tops))] {
			t3 = append(t3, []any{clipRunes(t.name, 30), r2(t.max)})
		}
		k["top"] = t3
		savings[kind] = k
	}
	if len(savings) > 0 {
		if baseRate > 0 {
			savings["baseRate"] = r2(baseRate)
		}
		out["fin_savings"] = savings
	}

	loans := map[string]any{}
	for _, kind := range []string{"mortgage", "rent", "credit"} {
		items, err := s.fin.List(ctx, kind, today)
		if err != nil || len(items) == 0 {
			continue
		}
		var avgs, mins []float64
		type low struct {
			name string
			avg  float64
		}
		var lows []low
		for _, p := range items {
			r := ProductRates(kind, p.Options)
			if !r.OK || r.Base <= 0 {
				continue
			}
			avgs = append(avgs, r.Base)
			if r.Min > 0 {
				mins = append(mins, r.Min)
			}
			lows = append(lows, low{p.CompanyName + " " + p.ProductName, r.Base})
		}
		if len(avgs) == 0 {
			continue
		}
		sort.Slice(lows, func(i, j int) bool { return lows[i].avg < lows[j].avg })
		minAll := math.Inf(1)
		for _, m := range mins {
			minAll = math.Min(minAll, m)
		}
		k := map[string]any{"n": len(avgs), "avg": r2(avgOf(avgs))}
		if !math.IsInf(minAll, 1) {
			k["min"] = r2(minAll)
		}
		var l3 [][]any
		for _, l := range lows[:min(3, len(lows))] {
			l3 = append(l3, []any{clipRunes(l.name, 30), r2(l.avg)})
		}
		k["low"] = l3
		loans[kind] = k
	}
	if len(loans) > 0 {
		if baseRate > 0 {
			loans["baseRate"] = r2(baseRate)
		}
		out["fin_loans"] = loans
	}
	return out
}
