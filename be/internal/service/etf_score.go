package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
ETFScoreService — 국내 ETF 점검 (2026-09-24 사용자 결정: 국내 ETF · 종목 화면 + 랭킹 ETF 탭).

# 왜 주식 점수와 다른 잣대인가

ETF는 회사가 아니라 **지수를 담는 그릇**이다. 수익성·성장·가치는 그릇의 성질이 아니다. 같은
지수를 따르는 두 상품을 가르는 것은 그릇이 얼마나 잘 만들어졌는가다:

	괴리율   가격이 NAV에서 얼마나 떨어져 거래되나 (20일 평균 |가격/NAV − 1|) — 낮을수록 좋다
	추적오차 NAV가 기초지수를 얼마나 벗어나나 (60일, 연율) — 낮을수록 좋다
	유동성   20일 거래대금 중앙값 — 높을수록 좋다(사고팔 때 호가가 촘촘하다)
	규모     순자산 — 클수록 좋다(상장폐지·운용 중단 위험이 작다)

넷을 같은 **비교 집단** 안의 백분위로 바꿔 평균한다. 총보수는 소스가 없어 빠졌다 — 화면이
그렇게 적는다.

# 비교 집단: 일반 / 채권·금리 / 레버리지·인버스

레버리지·인버스는 매일 배율을 맞추느라 거래가 많고 괴리가 구조적으로 크다. 일반 ETF와 한 줄로
세우면 전부 바닥에 깔린다. 기초지수 대비 베타로 가른다(|배율|이 1.5 넘거나 음수).
반대로 채권·금리·MMF는 지수가 거의 안 움직여 괴리·추적오차가 **원래** 작다 — 기초지수 연 변동성 5% 미만을
따로 묶는다(첫 산출 때 「일반」 상위 5개가 전부 MMF·CD였다).

# 추적오차의 두 함정

  - **분배금**: 분배락 날 NAV가 분배금만큼 떨어진다 — 지수는 그대로다. 하루 잔차가 3%를 넘는
    날은 분배·자료 오류로 보고 뺀다.
  - **시차**: 해외 지수 ETF의 NAV는 전날 해외 종가를 반영한다. 같은 날과 하루 늦춘 날 둘 다
    맞춰 보고 **잔차가 작은 쪽**을 쓴다. 틀린 정렬로 재면 멀쩡한 상품이 추적을 못 하는 것처럼 보인다.
*/
type ETFScoreService struct {
	krx  *marketdata.KRXClient
	repo *repository.ETFMetricsRepo
}

func NewETFScoreService(krx *marketdata.KRXClient, repo *repository.ETFMetricsRepo) *ETFScoreService {
	return &ETFScoreService{krx: krx, repo: repo}
}

const (
	etfBackfillDays     = 90 // 달력일. 거래일 약 60일 — 추적오차 창을 채운다.
	etfCallPause        = 7 * time.Second
	etfWindowSessions   = 61
	etfPremiumSessions  = 20
	etfMinSessions      = 20
	etfResidualCap      = 0.03
	etfMinPeerGroupSize = 20

	ETFPeerPlain     = "plain"
	ETFPeerLeveraged = "leveraged"
	// ETFPeerLowVol — 채권·금리·머니마켓. 기초지수가 거의 안 움직여 괴리·추적오차가 원래 작다 —
	// 주식형과 한 줄로 세우면 이 상품들이 상위를 다 가져간다(2026-09-24 첫 산출 실측: 상위 5개가 전부 MMF·CD).
	ETFPeerLowVol = "lowvol"
	// etfLowVolIndexPct — 기초지수 연 변동성이 이 값(%) 미만이면 채권·금리 집단.
	etfLowVolIndexPct = 5.0
)

// Run fetches missing days up to yesterday, then scores the newest stored day.
// limit>0 caps how many calendar days back the first fetch reaches.
func (s *ETFScoreService) Run(ctx context.Context, limit int) (map[string]any, string, error) {
	if s == nil || s.krx == nil || !s.krx.Enabled() {
		return nil, "", fmt.Errorf("etf score: KRX key not configured")
	}
	latest, err := s.repo.LatestDate(ctx)
	if err != nil {
		return nil, "", err
	}
	back := etfBackfillDays
	if limit > 0 {
		back = limit
	}
	days := etfFetchDays(latest, time.Now().In(model.KST), back)
	fetched, rows := 0, 0
	for i, d := range days {
		if ctx.Err() != nil {
			break
		}
		if i > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(etfCallPause):
			}
		}
		got, err := s.krx.ETFDaily(ctx, strings.ReplaceAll(d, "-", ""))
		if err != nil {
			return map[string]any{"fetchedDays": fetched, "rows": rows}, "", fmt.Errorf("etf daily %s: %w", d, err)
		}
		fetched++
		if len(got) == 0 {
			continue // 휴장일이거나 아직 공개 전 — 다음 회차가 다시 묻는다.
		}
		n, err := s.repo.UpsertDaily(ctx, d, got)
		if err != nil {
			return nil, "", err
		}
		rows += n
	}

	scoreDate, err := s.repo.LatestDate(ctx)
	if err != nil || scoreDate == "" {
		return map[string]any{"fetchedDays": fetched, "rows": rows}, "ETF 지표 없음 — 점수 생략", err
	}
	window, err := s.repo.Window(ctx, scoreDate, etfWindowSessions)
	if err != nil {
		return nil, "", err
	}
	scored := computeETFScores(window, scoreDate)
	if err := s.repo.ReplaceScores(ctx, scoreDate, scored); err != nil {
		return nil, "", err
	}
	res := map[string]any{"fetchedDays": fetched, "rows": rows, "scoreDate": scoreDate, "scored": len(scored)}
	return res, fmt.Sprintf("ETF 지표 %d일 · %d행 적재 · %s 점검 %d종목", fetched, rows, scoreDate, len(scored)), nil
}

// etfFetchDays lists weekdays after latest (or back days ago) up to yesterday KST.
func etfFetchDays(latest string, now time.Time, back int) []string {
	start := now.AddDate(0, 0, -back)
	if latest != "" {
		if t, err := time.ParseInLocation("2006-01-02", latest, now.Location()); err == nil {
			start = t.AddDate(0, 0, 1)
		}
	}
	end := now.AddDate(0, 0, -1)
	var out []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		out = append(out, d.Format("2006-01-02"))
	}
	return out
}

type etfMeasure struct {
	symbol    string
	index     string
	premium   *float64
	tracking  *float64
	beta      *float64
	indexVol  *float64 // 기초지수 연 변동성(%)
	liquidity *int64
	size      *int64
}

// computeETFScores is the pure scoring step over a stored window (oldest first per symbol).
func computeETFScores(window map[string][]repository.ETFPoint, date string) []repository.ETFScoreRow {
	measures := make([]etfMeasure, 0, len(window))
	for sym, pts := range window {
		if len(pts) < etfMinSessions || pts[len(pts)-1].Date != date {
			continue
		}
		measures = append(measures, measureETF(sym, pts))
	}
	groups := map[string][]*etfMeasure{}
	for i := range measures {
		g := etfPeerGroup(measures[i].beta, measures[i].indexVol)
		groups[g] = append(groups[g], &measures[i])
	}
	all := make([]*etfMeasure, len(measures))
	for i := range measures {
		all[i] = &measures[i]
	}

	out := make([]repository.ETFScoreRow, 0, len(measures))
	for group, members := range groups {
		peers := members
		if len(members) < etfMinPeerGroupSize {
			peers = all // 집단이 너무 작으면 백분위가 뜻이 없다 — 전체와 비교한다.
		}
		prem := etfCollect(peers, func(m *etfMeasure) *float64 { return m.premium })
		te := etfCollect(peers, func(m *etfMeasure) *float64 { return m.tracking })
		liq := etfCollect(peers, func(m *etfMeasure) *float64 { return etfLog(m.liquidity) })
		size := etfCollect(peers, func(m *etfMeasure) *float64 { return etfLog(m.size) })
		for _, m := range members {
			parts := map[string]any{}
			var sum float64
			n := 0
			add := func(key string, v *float64, raw any, dist []float64, lowerBetter bool) {
				if v == nil || len(dist) == 0 {
					return
				}
				p := etfPercentile(dist, *v)
				if lowerBetter {
					p = 1 - p
				}
				parts[key] = map[string]any{"value": raw, "pct": math.Round(p * 100)}
				sum += p
				n++
			}
			add("premium", m.premium, etfRound(m.premium, 3), prem, true)
			add("tracking", m.tracking, etfRound(m.tracking, 2), te, true)
			add("liquidity", etfLog(m.liquidity), m.liquidity, liq, false)
			add("size", etfLog(m.size), m.size, size, false)
			if n < 3 {
				continue // 넷 중 셋은 있어야 점검이라 부를 수 있다.
			}
			b, _ := json.Marshal(parts)
			out = append(out, repository.ETFScoreRow{
				Symbol: m.symbol, TradeDate: date, Score: math.Round(sum/float64(n)*10000) / 100,
				PeerGroup: group, PeerCount: len(peers), PremiumPct: etfRound(m.premium, 4),
				TrackingErrorPct: etfRound(m.tracking, 3), Beta: etfRound(m.beta, 3),
				TradeValueMedian: m.liquidity, NetAssets: m.size, IndexName: m.index, Parts: b,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

func measureETF(sym string, pts []repository.ETFPoint) etfMeasure {
	m := etfMeasure{symbol: sym, index: pts[len(pts)-1].IndexName}
	last := pts[len(pts)-1]
	if last.NetAssets.Valid && last.NetAssets.Int64 > 0 {
		v := last.NetAssets.Int64
		m.size = &v
	}

	recent := pts
	if len(recent) > etfPremiumSessions {
		recent = recent[len(recent)-etfPremiumSessions:]
	}
	var premSum float64
	premN := 0
	values := make([]float64, 0, len(recent))
	for _, p := range recent {
		if p.NAV.Valid && p.NAV.Float64 > 0 {
			premSum += math.Abs(p.Close/p.NAV.Float64-1) * 100
			premN++
		}
		if p.TradeValue.Valid {
			values = append(values, float64(p.TradeValue.Int64))
		}
	}
	if premN >= etfPremiumSessions/2 {
		v := premSum / float64(premN)
		m.premium = &v
	}
	if len(values) > 0 {
		sort.Float64s(values)
		v := int64(values[len(values)/2])
		m.liquidity = &v
	}

	// NAV 수익률과 기초지수 등락률 — 같은 날(lag 0)과 하루 늦춘 날(lag 1).
	var navRet []float64
	var idxRet []*float64
	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1], pts[i]
		if !a.NAV.Valid || !b.NAV.Valid || a.NAV.Float64 <= 0 {
			navRet = append(navRet, math.NaN())
		} else {
			navRet = append(navRet, b.NAV.Float64/a.NAV.Float64-1)
		}
		if b.IndexChangeRate.Valid {
			v := b.IndexChangeRate.Float64 / 100
			idxRet = append(idxRet, &v)
		} else {
			idxRet = append(idxRet, nil)
		}
	}
	if m.index == "" {
		return m
	}
	var idxs []float64
	for _, x := range idxRet {
		if x != nil {
			idxs = append(idxs, *x)
		}
	}
	if len(idxs) >= etfMinSessions {
		_, sd := meanSD(idxs)
		v := sd * math.Sqrt(250) * 100
		m.indexVol = &v
	}
	best := math.Inf(1)
	for lag := 0; lag <= 1; lag++ {
		beta, te, ok := trackingFit(navRet, idxRet, lag)
		if ok && te < best {
			best = te
			b, t := beta, te
			m.beta, m.tracking = &b, &t
		}
	}
	return m
}

// trackingFit regresses NAV returns on index returns shifted by lag and returns
// beta and the annualised residual SD (%). Days with |residual| > 3% are dropped.
func trackingFit(navRet []float64, idxRet []*float64, lag int) (float64, float64, bool) {
	var xs, ys []float64
	for i := lag; i < len(navRet); i++ {
		x := idxRet[i-lag]
		if x == nil || math.IsNaN(navRet[i]) {
			continue
		}
		xs = append(xs, *x)
		ys = append(ys, navRet[i])
	}
	if len(xs) < etfMinSessions {
		return 0, 0, false
	}
	mx, _ := meanSD(xs)
	my, _ := meanSD(ys)
	var cov, vx float64
	for i := range xs {
		cov += (xs[i] - mx) * (ys[i] - my)
		vx += (xs[i] - mx) * (xs[i] - mx)
	}
	if vx <= 0 {
		return 0, 0, false
	}
	beta := cov / vx
	var res []float64
	for i := range xs {
		r := ys[i] - beta*xs[i]
		if math.Abs(r) <= etfResidualCap {
			res = append(res, r)
		}
	}
	if len(res) < etfMinSessions {
		return 0, 0, false
	}
	_, sd := meanSD(res)
	return beta, sd * math.Sqrt(250) * 100, true
}

func etfPeerGroup(beta, indexVol *float64) string {
	if beta != nil && (*beta > 1.5 || *beta < 0) {
		return ETFPeerLeveraged
	}
	if indexVol != nil && *indexVol < etfLowVolIndexPct {
		return ETFPeerLowVol
	}
	return ETFPeerPlain
}

func etfCollect(ms []*etfMeasure, f func(*etfMeasure) *float64) []float64 {
	out := make([]float64, 0, len(ms))
	for _, m := range ms {
		if v := f(m); v != nil {
			out = append(out, *v)
		}
	}
	sort.Float64s(out)
	return out
}

// etfPercentile is the mid-rank share of dist below v (dist sorted ascending).
func etfPercentile(dist []float64, v float64) float64 {
	lo := sort.SearchFloat64s(dist, v)
	hi := lo
	for hi < len(dist) && dist[hi] == v {
		hi++
	}
	return (float64(lo) + float64(hi-lo)/2) / float64(len(dist))
}

func etfLog(v *int64) *float64 {
	if v == nil || *v <= 0 {
		return nil
	}
	l := math.Log(float64(*v))
	return &l
}

func etfRound(v *float64, digits int) *float64 {
	if v == nil {
		return nil
	}
	p := math.Pow(10, float64(digits))
	r := math.Round(*v*p) / p
	return &r
}
