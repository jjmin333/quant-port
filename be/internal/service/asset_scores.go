package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
)

/*
AssetScoreService — 부동산·금융상품·공매 규칙 점수 (2026-10-01 사용자 요청 「주식처럼 점수」). LLM 아님, DB만.

점수는 **추천이 아니라 시장 지표를 한 숫자로 줄인 것**이다(0~100, 같은 묶음 안 백분위의 가중 평균).
축과 원값(facts)을 늘 함께 저장해 화면이 「왜 이 점수인가」를 보일 수 있게 한다.

	re_complex  아파트 단지(aptSeq)  묶음 = 시도(법정동 앞 2자리)
	            모멘텀 35 (최근 3개월 ㎡당 중앙가 ÷ 그 전 9개월) · 거래 25 (6개월 건수) ·
	            전세 받침 20 (전세가율, 80% 넘으면 깎음 — 역전세 위험) · 고점 위치 20 (최근 3개월 ÷ 12개월 월별 최고)
	re_region   시군구  4주 지수 변화 40 · 12주 20 · 거래량 추세 20 · 전세지수 4주 20
	fin_product 예·적금  기본금리 50 · 최고금리 15 · 우대 의존(최고−기본 작을수록) 15 · 가입 제한 없음 10 · 권역 10
	            대출     평균금리(낮을수록) 60 · 금리 폭(좁을수록) 20 · 권역 20
	onbid       공매 회차  할인(최저가/감정가 낮을수록, 같은 용도·시도 안) 60 · 유찰 1~3회 20 · 주의 표지 없음 20
*/
type AssetScoreService struct {
	data  *repository.PublicDataRepo
	fin   *repository.FinProductRepo
	intel *repository.AssetIntelRepo
	// news — 시군구 근거의 「최근 30일 기사 N건 · 최신 제목」(2026-10-09 결정 A). nil이면 그 칸 없이.
	news *repository.NewsRepo
	now  func() time.Time
}

func NewAssetScoreService(data *repository.PublicDataRepo, fin *repository.FinProductRepo, intel *repository.AssetIntelRepo) *AssetScoreService {
	return &AssetScoreService{data: data, fin: fin, intel: intel, now: time.Now}
}

// WithNews wires the news table for region evidence and returns s (부트스트랩 구조체 리터럴 안에서 쓴다).
func (s *AssetScoreService) WithNews(n *repository.NewsRepo) *AssetScoreService {
	s.news = n
	return s
}

// Run is the asset_scores job.
func (s *AssetScoreService) Run(ctx context.Context) (map[string]any, string, error) {
	result := map[string]any{}
	today := kstToday(s.now()).Format("2006-01-02")
	var parts []string
	for _, step := range []struct {
		name string
		fn   func(context.Context, string) (int, error)
	}{
		{"re_directory", s.complexDirectory},
		{"re_complex", s.complexes},
		{"re_region", s.regions},
		{"fin_product", s.products},
		{"onbid", s.onbid},
	} {
		n, err := step.fn(ctx, today)
		if err != nil {
			return result, "", fmt.Errorf("%s: %w", step.name, err)
		}
		result[step.name] = n
		parts = append(parts, fmt.Sprintf("%s %d", step.name, n))
	}
	return result, strings.Join(parts, " · "), nil
}

// pctRank returns each value's percentile (0~100) within vals. 같은 값은 같은 순위.
func pctRank(vals []float64) []float64 {
	n := len(vals)
	out := make([]float64, n)
	if n == 0 {
		return out
	}
	if n == 1 {
		out[0] = 50
		return out
	}
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return vals[idx[a]] < vals[idx[b]] })
	for i := 0; i < n; {
		j := i
		for j+1 < n && vals[idx[j+1]] == vals[idx[i]] {
			j++
		}
		p := float64(i+j) / 2 / float64(n-1) * 100
		for k := i; k <= j; k++ {
			out[idx[k]] = p
		}
		i = j + 1
	}
	return out
}

func gradeOf(score float64) string {
	switch {
	case score >= 80:
		return "A"
	case score >= 65:
		return "B"
	case score >= 45:
		return "C"
	case score >= 30:
		return "D"
	}
	return "E"
}

func r1(v float64) float64 { return math.Round(v*10) / 10 }

func rawJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// ── 아파트 단지 ────────────────────────────────────────────────────────────────

type complexAgg struct {
	key, name, lawd, umd string
	build                *int
	recent, older, all   []float64 // ㎡당 매매가(원)
	monthly              map[string][]float64
	jeonse               []float64 // ㎡당 전세 보증금(6개월)
	sale6                []float64 // ㎡당 매매가(6개월)
	trades6              int
	lastDeal             string
	lastPrice            int64
}

func (s *AssetScoreService) complexes(ctx context.Context, today string) (int, error) {
	regions, err := s.data.Regions(ctx)
	if err != nil {
		return 0, err
	}
	now := kstToday(s.now())
	since := now.AddDate(-1, 0, 0)
	cut3, cut6 := now.AddDate(0, -3, 0).Format("2006-01-02"), now.AddDate(0, -6, 0).Format("2006-01-02")
	type row struct {
		agg     *complexAgg
		mom     float64
		jr      float64
		peak    float64
		premium float64
		unit    float64
	}
	bySido := map[string][]row{}
	for _, g := range regions {
		trades, err := s.data.RegionTrades(ctx, g.LawdCd, "apt", since)
		if err != nil {
			return 0, err
		}
		aggs := map[string]*complexAgg{}
		var regionUnits []float64
		for _, t := range trades {
			if t.ComplexKey == "" || t.Area <= 0 || t.Cancelled {
				continue
			}
			a := aggs[t.ComplexKey]
			if a == nil {
				a = &complexAgg{key: t.ComplexKey, name: t.ComplexName, lawd: g.LawdCd, umd: t.UmdNm, build: t.BuildYear, monthly: map[string][]float64{}}
				aggs[t.ComplexKey] = a
			}
			if t.DealType == "sale" {
				if t.Direct || t.Price == nil {
					continue
				}
				u := float64(*t.Price) / t.Area
				regionUnits = append(regionUnits, u)
				a.all = append(a.all, u)
				a.monthly[t.DealDate[:7]] = append(a.monthly[t.DealDate[:7]], u)
				if t.DealDate >= cut3 {
					a.recent = append(a.recent, u)
				} else {
					a.older = append(a.older, u)
				}
				if t.DealDate >= cut6 {
					a.sale6 = append(a.sale6, u)
					a.trades6++
				}
				if t.DealDate > a.lastDeal {
					a.lastDeal, a.lastPrice = t.DealDate, *t.Price
				}
			} else if t.Deposit != nil && (t.Monthly == nil || *t.Monthly == 0) && t.DealDate >= cut6 {
				a.jeonse = append(a.jeonse, float64(*t.Deposit)/t.Area)
			}
		}
		regionMed := median(regionUnits)
		for _, a := range aggs {
			if len(a.recent) < 2 || len(a.older) < 2 || len(a.all) < 5 {
				continue
			}
			r := row{agg: a}
			r.unit = median(a.recent)
			r.mom = (r.unit/median(a.older) - 1) * 100
			if len(a.jeonse) >= 2 && len(a.sale6) > 0 {
				r.jr = median(a.jeonse) / median(a.sale6) * 100
			}
			peak := 0.0
			for _, m := range a.monthly {
				if v := median(m); v > peak {
					peak = v
				}
			}
			if peak > 0 {
				r.peak = r.unit / peak * 100
			}
			if regionMed > 0 {
				r.premium = (r.unit/regionMed - 1) * 100
			}
			sido := g.LawdCd[:2]
			bySido[sido] = append(bySido[sido], r)
		}
	}
	var list []repository.AssetScore
	for sido, rows := range bySido {
		mom, liq, jr, peak := make([]float64, len(rows)), make([]float64, len(rows)), make([]float64, len(rows)), make([]float64, len(rows))
		for i, r := range rows {
			mom[i], liq[i], peak[i] = r.mom, float64(r.agg.trades6), r.peak
			// 전세가율은 높을수록 받침이 되지만 80% 넘으면 역전세 위험 — 80에서 접는다.
			j := r.jr
			if j > 80 {
				j = 160 - j
			}
			jr[i] = j
		}
		pm, pl, pj, pp := pctRank(mom), pctRank(liq), pctRank(jr), pctRank(peak)
		for i, r := range rows {
			jPart := pj[i]
			if r.jr == 0 {
				jPart = 50 // 전세 거래가 없으면 중립
			}
			score := pm[i]*0.35 + pl[i]*0.25 + jPart*0.20 + pp[i]*0.20
			list = append(list, repository.AssetScore{
				Key: r.agg.key, Name: r.agg.name, Group: r.agg.lawd, AsOf: today,
				Score: int(math.Round(score)), Grade: gradeOf(score),
				Parts: rawJSON(map[string]float64{"momentum": r1(pm[i]), "liquidity": r1(pl[i]), "jeonse": r1(jPart), "peak": r1(pp[i])}),
				Facts: rawJSON(map[string]any{
					"momentumPct": r1(r.mom), "trades6m": r.agg.trades6, "jeonseRatio": r1(r.jr),
					"peakPct": r1(r.peak), "premiumPct": r1(r.premium), "unitPerM2": math.Round(r.unit),
					"pyeongPrice": math.Round(r.unit * 3.3058), "lastDeal": r.agg.lastDeal, "lastPrice": r.agg.lastPrice,
					"umdNm": r.agg.umd, "buildYear": r.agg.build, "sido": sido,
				}),
			})
		}
	}
	return len(list), s.intel.ReplaceScores(ctx, "re_complex", list)
}

// ── 시군구 ──────────────────────────────────────────────────────────────────

// idxFacts — 주간 매매지수(1·4·12주)·전세지수(4주) 변화 %. 지역 이름으로 R-ONE 분류를 찾는 건 호출자가.
type idxFacts struct {
	idx, w1, w4, w12, j4 float64
	hasIndex             bool
}

func (s *AssetScoreService) indexFacts(ctx context.Context, saleCls int, saleOK bool, jeonseCls int, jeonseOK bool, since time.Time) idxFacts {
	var f idxFacts
	if saleOK {
		if pts, err := s.data.IndexSeries(ctx, roneWeeklySale, saleCls, 0, since); err == nil && len(pts) >= 13 {
			last := pts[len(pts)-1].Value
			f.idx = last
			f.w1 = (last/pts[len(pts)-2].Value - 1) * 100
			f.w4 = (last/pts[len(pts)-5].Value - 1) * 100
			f.w12 = (last/pts[len(pts)-13].Value - 1) * 100
			f.hasIndex = true
		}
	}
	if jeonseOK {
		if pts, err := s.data.IndexSeries(ctx, roneWeeklyJeonse, jeonseCls, 0, since); err == nil && len(pts) >= 5 {
			f.j4 = (pts[len(pts)-1].Value/pts[len(pts)-5].Value - 1) * 100
		}
	}
	return f
}

// volTrend — 이번 달·지난달은 신고 기한 때문에 덜 찼다: 그 전 3개월 대 다시 그 전 3개월.
func volTrend(months []repository.RegionMonth) (recent, prior int, pct float64) {
	if len(months) < 8 {
		return 0, 0, 0
	}
	m := months[:len(months)-1]
	for _, x := range m[len(m)-4 : len(m)-1] {
		recent += x.Count
	}
	for _, x := range m[len(m)-7 : len(m)-4] {
		prior += x.Count
	}
	if prior > 0 {
		pct = (float64(recent)/float64(prior) - 1) * 100
	}
	return recent, prior, pct
}

func (s *AssetScoreService) regions(ctx context.Context, today string) (int, error) {
	regions, err := s.data.Regions(ctx)
	if err != nil {
		return 0, err
	}
	now := kstToday(s.now())
	since := now.AddDate(0, 0, -100)
	type row struct {
		g                   repository.ReRegion
		f                   idxFacts
		volTrd              float64
		recentVol, priorVol int
	}
	var rows []row
	monthsBy := map[string][]repository.RegionMonth{}
	for _, g := range regions {
		saleCls, saleOK := RoneClsFor(ctx, s.data, roneWeeklySale, g.Sido, g.Name)
		jCls, jOK := RoneClsFor(ctx, s.data, roneWeeklyJeonse, g.Sido, g.Name)
		r := row{g: g, f: s.indexFacts(ctx, saleCls, saleOK, jCls, jOK, since)}
		if months, err := s.data.RegionMonthly(ctx, g.LawdCd, "apt"); err == nil {
			monthsBy[g.LawdCd] = months
			r.recentVol, r.priorVol, r.volTrd = volTrend(months)
		}
		if r.f.hasIndex {
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		return 0, s.intel.ReplaceScores(ctx, "re_region", nil)
	}
	w4, w12, j4, vt := make([]float64, len(rows)), make([]float64, len(rows)), make([]float64, len(rows)), make([]float64, len(rows))
	for i, r := range rows {
		w4[i], w12[i], j4[i], vt[i] = r.f.w4, r.f.w12, r.f.j4, r.volTrd
	}
	parts, scores := regionScores(w4, w12, j4, vt)
	ranks := rankDesc(scores)
	// 근거 칸만 — 점수식에는 넣지 않는다(기사 수는 지역 크기·언론 관심에 치우쳐 가격 신호로 검증 전).
	newsBy := map[string]repository.RegionNews{}
	if s.news != nil {
		if m, err := s.news.RegionNewsStats(ctx, now.AddDate(0, 0, -30)); err == nil {
			newsBy = m
		}
	}
	list := make([]repository.AssetScore, 0, len(rows))
	subs := make([]areaSub, 0, len(rows))
	for i, r := range rows {
		facts := map[string]any{
			"index": r1(r.f.idx), "w1Pct": r2(r.f.w1), "w4Pct": r2(r.f.w4), "w12Pct": r2(r.f.w12),
			"jeonse4Pct": r2(r.f.j4), "volTrendPct": r1(r.volTrd),
			"recentVol": r.recentVol, "priorVol": r.priorVol, "sido": r.g.Sido, "name": r.g.Name,
			"rank": ranks[i], "of": len(rows),
		}
		if nw, ok := newsBy[r.g.LawdCd]; ok && nw.Count > 0 {
			facts["news30"], facts["newsTitle"] = nw.Count, nw.Latest
		}
		list = append(list, repository.AssetScore{
			Key: r.g.LawdCd, Name: r.g.Sido + " " + r.g.Name, Group: r.g.LawdCd[:2], AsOf: today,
			Score: int(math.Round(scores[i])), Grade: gradeOf(scores[i]), Parts: rawJSON(parts[i]),
			Facts: rawJSON(facts),
		})
		subs = append(subs, areaSub{code: r.g.LawdCd[:2], name: r.g.Name, w4: r.f.w4, score: scores[i]})
	}
	if err := s.intel.ReplaceScores(ctx, "re_region", list); err != nil {
		return 0, err
	}
	n, err := s.areas(ctx, today, regions, monthsBy, subs, since)
	return len(list) + n, err
}

// regionScores — 시군구·시도 공통: 4주 지수 40 · 12주 20 · 거래량 추세 20 · 전세지수 4주 20 (같은 묶음 안 백분위).
func regionScores(w4, w12, j4, vt []float64) ([]map[string]float64, []float64) {
	p4, p12, pj, pv := pctRank(w4), pctRank(w12), pctRank(j4), pctRank(vt)
	parts := make([]map[string]float64, len(w4))
	scores := make([]float64, len(w4))
	for i := range w4 {
		scores[i] = p4[i]*0.4 + p12[i]*0.2 + pv[i]*0.2 + pj[i]*0.2
		parts[i] = map[string]float64{"w4": r1(p4[i]), "w12": r1(p12[i]), "volume": r1(pv[i]), "jeonse": r1(pj[i])}
	}
	return parts, scores
}

func r2(v float64) float64 { return math.Round(v*100) / 100 }

// rankDesc — 큰 값이 1위(같은 값은 같은 순위).
func rankDesc(vals []float64) []int {
	out := make([]int, len(vals))
	for i, v := range vals {
		out[i] = 1
		for _, w := range vals {
			if w > v {
				out[i]++
			}
		}
	}
	return out
}

// areaSub — 시도·전국 사실표에 넣는 하위 지역 한 줄.
type areaSub struct {
	code, name string
	w4, score  float64
}

// areaAgg — 시도 또는 전국의 월별 거래 합과 하위 지역.
type areaAgg struct {
	code, name string
	months     map[string]*repository.RegionMonth
	subs       []areaSub
}

func newAreaAgg(code, name string) *areaAgg {
	return &areaAgg{code: code, name: name, months: map[string]*repository.RegionMonth{}}
}

func (a *areaAgg) add(ms []repository.RegionMonth) {
	for _, m := range ms {
		x := a.months[m.YM]
		if x == nil {
			x = &repository.RegionMonth{YM: m.YM}
			a.months[m.YM] = x
		}
		x.AvgPerM2 += m.AvgPerM2 * float64(m.Count) // 가중 합으로 모았다가 facts에서 나눈다
		x.Count += m.Count
		x.RentCount += m.RentCount
	}
}

func pickSubs(xs []areaSub) [][]any {
	out := [][]any{}
	for _, x := range xs {
		out = append(out, []any{x.name, r2(x.w4)})
	}
	return out
}

// facts — 지수 변화 + 거래량 추세 + 오른/내린 하위 지역 수 + 많이 오른·내린 3곳 + 최근 13개월 거래.
func (a *areaAgg) facts(f idxFacts) (map[string]any, float64) {
	yms := make([]string, 0, len(a.months))
	for ym := range a.months {
		yms = append(yms, ym)
	}
	sort.Strings(yms)
	ms := make([]repository.RegionMonth, 0, len(yms))
	for _, ym := range yms {
		m := *a.months[ym]
		if m.Count > 0 {
			m.AvgPerM2 = math.Round(m.AvgPerM2 / float64(m.Count))
		}
		ms = append(ms, m)
	}
	recent, prior, vt := volTrend(ms)
	up, down := 0, 0
	var sum float64
	for _, x := range a.subs {
		sum += x.score
		switch {
		case x.w4 >= 0.05:
			up++
		case x.w4 <= -0.05:
			down++
		}
	}
	sorted := append([]areaSub(nil), a.subs...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].w4 > sorted[j].w4 })
	top := pickSubs(sorted[:min(3, len(sorted))])
	bottom := [][]any{}
	for i := len(sorted) - 1; i >= max(3, len(sorted)-3); i-- {
		bottom = append(bottom, []any{sorted[i].name, r2(sorted[i].w4)})
	}
	avg := 0.0
	if len(a.subs) > 0 {
		avg = sum / float64(len(a.subs))
	}
	if len(ms) > 13 {
		ms = ms[len(ms)-13:]
	}
	return map[string]any{
		"index": r1(f.idx), "w1Pct": r2(f.w1), "w4Pct": r2(f.w4), "w12Pct": r2(f.w12), "jeonse4Pct": r2(f.j4),
		"volTrendPct": r1(vt), "recentVol": recent, "priorVol": prior, "name": a.name,
		"regions": len(a.subs), "up": up, "down": down, "flat": len(a.subs) - up - down, "avgScore": r1(avg),
		"top": top, "bottom": bottom, "months": ms,
	}, vt
}

// areas — 시도(re_sido, 시도끼리 백분위)와 전국(re_nation, 비교할 짝이 없어 점수 없이 사실만) (2026-10-02 사용자:
// 「구를 안 골라도 시도 전체 기준으로」). 지수는 R-ONE의 시도·전국 분류, 거래량은 시군구 월별 합 —
// 조회 때 큰 합계 쿼리를 돌리지 않게 facts에 13개월을 싣는다.
func (s *AssetScoreService) areas(ctx context.Context, today string, regions []repository.ReRegion, monthsBy map[string][]repository.RegionMonth,
	subs []areaSub, since time.Time) (int, error) {
	byCode := map[string]*areaAgg{}
	nation := newAreaAgg("00", "전국")
	for _, g := range regions {
		code := g.LawdCd[:2]
		if byCode[code] == nil {
			byCode[code] = newAreaAgg(code, g.Sido)
		}
		byCode[code].add(monthsBy[g.LawdCd])
		nation.add(monthsBy[g.LawdCd])
	}
	for _, sub := range subs {
		if a := byCode[sub.code]; a != nil {
			a.subs = append(a.subs, sub)
		}
		nation.subs = append(nation.subs, sub)
	}
	indexOf := func(name string) idxFacts {
		saleCls, saleOK, _ := s.data.IndexClsByName(ctx, roneWeeklySale, name, name)
		jCls, jOK, _ := s.data.IndexClsByName(ctx, roneWeeklyJeonse, name, name)
		return s.indexFacts(ctx, saleCls, saleOK, jCls, jOK, since)
	}

	type sidoRow struct {
		a     *areaAgg
		f     idxFacts
		facts map[string]any
		vt    float64
	}
	var rows []sidoRow
	codes := make([]string, 0, len(byCode))
	for c := range byCode {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	for _, c := range codes {
		a := byCode[c]
		f := indexOf(SidoShort(a.name))
		fm, vt := a.facts(f)
		if f.hasIndex {
			rows = append(rows, sidoRow{a: a, f: f, facts: fm, vt: vt})
		}
	}
	list := make([]repository.AssetScore, 0, len(rows))
	if len(rows) > 0 {
		w4, w12, j4, vt := make([]float64, len(rows)), make([]float64, len(rows)), make([]float64, len(rows)), make([]float64, len(rows))
		for i, r := range rows {
			w4[i], w12[i], j4[i], vt[i] = r.f.w4, r.f.w12, r.f.j4, r.vt
		}
		parts, scores := regionScores(w4, w12, j4, vt)
		ranks := rankDesc(scores)
		for i, r := range rows {
			r.facts["rank"], r.facts["of"] = ranks[i], len(rows)
			list = append(list, repository.AssetScore{Key: r.a.code, Name: r.a.name, Group: "sido", AsOf: today,
				Score: int(math.Round(scores[i])), Grade: gradeOf(scores[i]), Parts: rawJSON(parts[i]), Facts: rawJSON(r.facts)})
		}
	}
	if err := s.intel.ReplaceScores(ctx, "re_sido", list); err != nil {
		return 0, err
	}
	nf, _ := nation.facts(indexOf("전국"))
	sidos := make([]areaSub, 0, len(rows))
	for _, r := range rows {
		sidos = append(sidos, areaSub{name: SidoShort(r.a.name), w4: r.f.w4})
	}
	sort.SliceStable(sidos, func(i, j int) bool { return sidos[i].w4 > sidos[j].w4 })
	nf["sidos"] = pickSubs(sidos)
	if err := s.intel.ReplaceScores(ctx, "re_nation", []repository.AssetScore{{Key: "00", Name: "전국", Group: "nation", AsOf: today,
		Parts: rawJSON(map[string]float64{}), Facts: rawJSON(nf)}}); err != nil {
		return 0, err
	}
	return len(list) + 1, nil
}

// ── 금융상품 ────────────────────────────────────────────────────────────────

// FinRates — 상품 한 줄에서 점수·이력에 쓰는 금리.
type FinRates struct {
	Term      int     // 예·적금: 고른 기간(개월)
	Base, Max float64 // 예·적금 기본·최고 / 대출 평균·최고
	Min       float64 // 대출 최저
	OK        bool
}

func optNum(o map[string]any, k string) (float64, bool) {
	switch v := o[k].(type) {
	case float64:
		return v, true
	case string:
		var f float64
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%g", &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

// ProductRates picks the representative rate: 예·적금은 12개월(없으면 가장 가까운 기간)의 가장 좋은 옵션,
// 대출은 옵션 전체의 평균 금리 중 가장 낮은 것과 그 범위.
func ProductRates(kind string, options json.RawMessage) FinRates {
	var opts []map[string]any
	_ = json.Unmarshal(options, &opts)
	out := FinRates{}
	switch kind {
	case "deposit", "saving":
		bestDist := math.MaxFloat64
		for _, o := range opts {
			t, ok := optNum(o, "save_trm")
			if !ok {
				continue
			}
			base, _ := optNum(o, "intr_rate")
			mx, _ := optNum(o, "intr_rate2")
			dist := math.Abs(t - 12)
			if dist < bestDist || (dist == bestDist && base > out.Base) {
				out = FinRates{Term: int(t), Base: base, Max: math.Max(mx, base), OK: true}
				bestDist = dist
			}
		}
	case "mortgage", "rent":
		for _, o := range opts {
			avg, ok := optNum(o, "lend_rate_avg")
			mn, _ := optNum(o, "lend_rate_min")
			mx, _ := optNum(o, "lend_rate_max")
			if !ok || avg <= 0 {
				avg = (mn + mx) / 2
			}
			if avg <= 0 {
				continue
			}
			if !out.OK || avg < out.Base {
				out = FinRates{Base: avg, Min: mn, Max: mx, OK: true}
			}
		}
	case "credit":
		for _, o := range opts {
			if t, _ := o["crdt_lend_rate_type"].(string); t != "A" {
				continue
			}
			avg, ok := optNum(o, "crdt_grad_avg")
			if !ok || avg <= 0 {
				continue
			}
			lo, _ := optNum(o, "crdt_grad_1")
			hi, _ := optNum(o, "crdt_grad_13")
			out = FinRates{Base: avg, Min: lo, Max: hi, OK: true}
		}
	}
	return out
}

func groupWeight(grp string) float64 {
	switch grp {
	case "020000":
		return 100 // 은행
	case "030300":
		return 60 // 저축은행
	}
	return 70
}

func (s *AssetScoreService) products(ctx context.Context, today string) (int, error) {
	var list []repository.AssetScore
	todayKST := strings.ReplaceAll(today, "-", "")
	for _, kind := range []string{"deposit", "saving", "mortgage", "rent", "credit"} {
		items, err := s.fin.List(ctx, kind, todayKST)
		if err != nil {
			return 0, err
		}
		type row struct {
			p repository.FinProduct
			r FinRates
		}
		var rows []row
		for _, p := range items {
			if r := ProductRates(kind, p.Options); r.OK {
				rows = append(rows, row{p, r})
			}
		}
		if len(rows) == 0 {
			continue
		}
		base, mx, dep, spread := make([]float64, len(rows)), make([]float64, len(rows)), make([]float64, len(rows)), make([]float64, len(rows))
		for i, r := range rows {
			base[i], mx[i] = r.r.Base, r.r.Max
			dep[i] = -(r.r.Max - r.r.Base) // 우대 의존이 작을수록 좋다
			spread[i] = -(r.r.Max - r.r.Min)
			if kind != "deposit" && kind != "saving" {
				base[i] = -r.r.Base // 대출은 낮을수록 좋다
			}
		}
		pb, pm, pd, ps := pctRank(base), pctRank(mx), pctRank(dep), pctRank(spread)
		for i, r := range rows {
			var b map[string]any
			_ = json.Unmarshal(r.p.Base, &b)
			deny, _ := optNum(b, "join_deny")
			open := 0.0
			if deny == 1 {
				open = 100
			}
			var score float64
			var parts map[string]float64
			if kind == "deposit" || kind == "saving" {
				score = pb[i]*0.5 + pm[i]*0.15 + pd[i]*0.15 + open*0.1 + groupWeight(r.p.Group)*0.1
				parts = map[string]float64{"base": r1(pb[i]), "max": r1(pm[i]), "conditions": r1(pd[i]), "open": open, "group": groupWeight(r.p.Group)}
			} else {
				score = pb[i]*0.6 + ps[i]*0.2 + groupWeight(r.p.Group)*0.2
				parts = map[string]float64{"rate": r1(pb[i]), "range": r1(ps[i]), "group": groupWeight(r.p.Group)}
			}
			list = append(list, repository.AssetScore{
				Key: fmt.Sprint(r.p.ID), Name: r.p.ProductName, Group: kind, AsOf: today,
				Score: int(math.Round(score)), Grade: gradeOf(score), Parts: rawJSON(parts),
				Facts: rawJSON(map[string]any{"term": r.r.Term, "base": r.r.Base, "max": r.r.Max, "min": r.r.Min,
					"company": r.p.CompanyName, "finGroup": r.p.Group, "joinDeny": deny}),
			})
			// 공시월 금리 이력(「지난달보다」 표시).
			bv, mv := r.r.Base, r.r.Max
			_ = s.intel.UpsertRateHistory(ctx, r.p.ID, r.p.DclsMonth, &bv, &mv)
		}
	}
	return len(list), s.intel.ReplaceScores(ctx, "fin_product", list)
}

// ── 공매 ────────────────────────────────────────────────────────────────────

// OnbidFlags — 공매 물건의 주의 표지(이름·주소에서 읽는다).
func OnbidFlags(it repository.OnbidItem) []string {
	f := []string{} // JSON에서 null이 아니라 []
	text := it.Name + " " + it.Address
	if strings.Contains(text, "지분") {
		f = append(f, "지분")
	}
	if strings.Contains(text, "일괄") {
		f = append(f, "일괄매각")
	}
	if strings.Contains(text, "토지만") || strings.Contains(text, "건물만") {
		f = append(f, "토지·건물 별도")
	}
	if strings.Contains(text, "*") {
		f = append(f, "주소 비공개")
	}
	if it.FailCount >= 6 {
		f = append(f, "유찰 많음")
	}
	if it.MinBidRatio != nil && *it.MinBidRatio > 100 {
		f = append(f, "감정가 초과")
	}
	return f
}

func (s *AssetScoreService) onbid(ctx context.Context, today string) (int, error) {
	items, err := s.data.OpenOnbidItems(ctx)
	if err != nil {
		return 0, err
	}
	groups := map[string][]int{}
	for i, it := range items {
		if it.MinBidRatio == nil {
			continue
		}
		g := it.Sido + "|" + it.UsageM
		groups[g] = append(groups[g], i)
	}
	var list []repository.AssetScore
	for g, idx := range groups {
		disc := make([]float64, len(idx))
		for k, i := range idx {
			disc[k] = -*items[i].MinBidRatio
		}
		pd := pctRank(disc)
		for k, i := range idx {
			it := items[i]
			fails := 40.0
			if it.FailCount >= 1 && it.FailCount <= 3 {
				fails = 100
			} else if it.FailCount > 6 {
				fails = 10
			}
			flags := OnbidFlags(it)
			clean := 100.0 - math.Min(100, float64(len(flags))*40)
			score := pd[k]*0.6 + fails*0.2 + clean*0.2
			list = append(list, repository.AssetScore{
				Key: fmt.Sprintf("%s|%d", it.CltrMngNo, it.PbctCdtnNo), Name: it.Name, Group: strings.Split(g, "|")[0], AsOf: today,
				Score: int(math.Round(score)), Grade: gradeOf(score),
				Parts: rawJSON(map[string]float64{"discount": r1(pd[k]), "fails": fails, "clean": clean}),
				Facts: rawJSON(map[string]any{"ratio": r1(*it.MinBidRatio), "fails": it.FailCount, "flags": flags, "usage": it.UsageM}),
			})
		}
	}
	return len(list), s.intel.ReplaceScores(ctx, "onbid", list)
}

// complexDirectory refreshes the apartment search list (re_complexes) — 점수가 없는 단지도 검색에는 나와야 한다(2026-10-09).
func (s *AssetScoreService) complexDirectory(ctx context.Context, _ string) (int, error) {
	n, err := s.data.RebuildComplexDirectory(ctx)
	return int(n), err
}
