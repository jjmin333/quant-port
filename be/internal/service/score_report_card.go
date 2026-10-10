package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"

	"github.com/enact/quant-port/be/internal/repository"
)

/*
ScoreReportCardService — AI 점수 **성적표**: 축마다 「점수가 높을수록 그 뒤에 더 올랐나」.

# 무엇을 재나

점수일마다 축 점수와 **그 뒤 5거래일 수익률**의 스피어만 순위상관(IC)을 재서
`score_ic_daily`에 쌓는다(점수 배치 꼬리). 조회는 쌓인 값을 요약만 한다.

# 정직하게 말하는 것 (2026-09-23 사용자 결정 — 지금 공개, 경고를 크게)

  - 표본이 짧으면 **짧다고 말한다.** 날짜 수와 t값을 함께 내고, |t| < 2이거나 20일 미만이면
    「통계적으로 의미 없음」이다.
  - **산출 방식이 바뀐 날을 표시한다**(`scoreMethodChanges`). 그 앞뒤 IC는 같은 잣대가 아니다.
  - 날짜별 IC는 서로 겹친다(5일 수익률 창이 이어져 있다) — t값은 그만큼 낙관적이다. 화면이
    이 사실을 한 줄로 적는다.
*/
type ScoreReportCardService struct {
	repo *repository.ScoreICRepo
}

func NewScoreReportCardService(repo *repository.ScoreICRepo) *ScoreReportCardService {
	return &ScoreReportCardService{repo: repo}
}

const (
	scoreICHorizon = 5
	scoreICTopN    = 20
	scoreICMinN    = 30 // 이보다 적은 종목으로 잰 날은 남기지 않는다
	scoreICMinDays = 20 // 요약이 「의미 있을 수 있다」를 말하는 최소 날짜 수
)

var scoreICAxes = []string{"composite", "technical", "quality", "growth", "valuation", "sentiment"}

/*
scoreMethodChanges — 점수 산출 방식이 바뀐 날. 성적표가 구분선을 긋는다.

**맨 끝 항목의 날짜는 `scoreMethodVersion`과 같아야 한다** — 시험이 지킨다
(score_report_card_test.go). 방식을 바꾸면 둘을 함께 고친다.
*/
var scoreMethodChanges = []struct {
	Date string `json:"date"`
	Note string `json:"note"`
}{
	{"2026-09-23", "재무 3축을 섹터 안에서 비교 · 심리 축을 기사 감성 라벨로 전 종목 계산 · 가치 축에 최근 종가 사용"},
	{"2026-09-24", "심리·수급 축에 국내 외국인·기관 20거래일 순매수(거래량 대비)를 더함"},
	{"2026-10-10", "국내 PER·PBR·PSR·ROE·이익률·성장률을 최근 4분기 합(TTM)으로 — 연간 사업보고서 기준에서 바꿈"},
}

// Compute measures every stored score date that has forward data but no row yet.
func (s *ScoreReportCardService) Compute(ctx context.Context) (map[string]any, string, error) {
	if s == nil || s.repo == nil {
		return nil, "", fmt.Errorf("score report card not configured")
	}
	dates, err := s.repo.HistoryDates(ctx)
	if err != nil {
		return nil, "", err
	}
	done, err := s.repo.MeasuredDates(ctx, scoreICHorizon)
	if err != nil {
		return nil, "", err
	}
	measured, pending := 0, 0
	for _, d := range dates {
		if done[d] {
			continue
		}
		var rows []repository.ScoreICRow
		for _, market := range []string{"domestic", "us"} {
			fwd, err := s.repo.ForwardRows(ctx, d, market, scoreICHorizon)
			if err != nil {
				return nil, "", err
			}
			if len(fwd) < scoreICMinN {
				continue
			}
			rows = append(rows, measureIC(d, market, fwd)...)
		}
		if len(rows) == 0 {
			// 아직 앞으로의 종가가 없다(최근 날짜) — 다음 회차가 잰다.
			pending++
			continue
		}
		if err := s.repo.Upsert(ctx, rows); err != nil {
			return nil, "", err
		}
		measured++
	}
	msg := fmt.Sprintf("성적표 %d일 새로 계산 · 아직 결과가 없는 날 %d (5거래일 뒤에 잰다)", measured, pending)
	return map[string]any{"measured": measured, "pending": pending}, msg, nil
}

func measureIC(date, market string, rows []repository.ScoreForwardRow) []repository.ScoreICRow {
	out := make([]repository.ScoreICRow, 0, len(scoreICAxes))
	for _, axis := range scoreICAxes {
		var xs, ys []float64
		for _, r := range rows {
			v := r.Score
			if axis != "composite" {
				v = r.Axes[axis]
			}
			if !v.Valid {
				continue
			}
			xs = append(xs, v.Float64)
			ys = append(ys, r.Return)
		}
		row := repository.ScoreICRow{TradeDate: date, Market: market, Axis: axis, Horizon: scoreICHorizon, N: len(xs)}
		if len(xs) >= scoreICMinN {
			row.IC = sql.NullFloat64{Float64: round4(spearman(xs, ys)), Valid: true}
			excess, hit := topNStats(xs, ys, scoreICTopN)
			row.TopExcess = sql.NullFloat64{Float64: round4(excess), Valid: true}
			row.TopHit = sql.NullFloat64{Float64: round4(hit), Valid: true}
		}
		out = append(out, row)
	}
	return out
}

// spearman is the Pearson correlation of average ranks (ties share a rank).
func spearman(xs, ys []float64) float64 {
	rx, ry := averageRanks(xs), averageRanks(ys)
	n := float64(len(rx))
	var mx, my float64
	for i := range rx {
		mx += rx[i]
		my += ry[i]
	}
	mx /= n
	my /= n
	var cov, vx, vy float64
	for i := range rx {
		dx, dy := rx[i]-mx, ry[i]-my
		cov += dx * dy
		vx += dx * dx
		vy += dy * dy
	}
	if vx == 0 || vy == 0 {
		return 0
	}
	return cov / math.Sqrt(vx*vy)
}

func averageRanks(v []float64) []float64 {
	idx := make([]int, len(v))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return v[idx[a]] < v[idx[b]] })
	ranks := make([]float64, len(v))
	for i := 0; i < len(idx); {
		j := i
		for j+1 < len(idx) && v[idx[j+1]] == v[idx[i]] {
			j++
		}
		avg := float64(i+j)/2 + 1
		for k := i; k <= j; k++ {
			ranks[idx[k]] = avg
		}
		i = j + 1
	}
	return ranks
}

// topNStats: mean return of the top-N by score minus the universe mean, and
// the share of those N that beat the universe median.
func topNStats(xs, ys []float64, n int) (excess, hit float64) {
	idx := make([]int, len(xs))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return xs[idx[a]] > xs[idx[b]] })
	if n > len(idx) {
		n = len(idx)
	}
	sorted := append([]float64(nil), ys...)
	sort.Float64s(sorted)
	median := sorted[len(sorted)/2]
	var all, top float64
	for _, y := range ys {
		all += y
	}
	all /= float64(len(ys))
	beat := 0
	for _, i := range idx[:n] {
		top += ys[i]
		if ys[i] > median {
			beat++
		}
	}
	return top/float64(n) - all, float64(beat) / float64(n)
}

// ScoreReportAxis summarises one axis over the stored dates.
type ScoreReportAxis struct {
	Axis          string             `json:"axis"`
	Days          int                `json:"days"`
	MeanIC        *float64           `json:"meanIC,omitempty"`
	TStat         *float64           `json:"tStat,omitempty"`
	PositiveShare *float64           `json:"positiveShare,omitempty"`
	MeanTopExcess *float64           `json:"meanTopExcess,omitempty"`
	MeanTopHit    *float64           `json:"meanTopHit,omitempty"`
	Significant   bool               `json:"significant"`
	Series        []ScoreReportDaily `json:"series"`
}

// ScoreReportDaily is one date's IC for the chart.
type ScoreReportDaily struct {
	Date string  `json:"date"`
	IC   float64 `json:"ic"`
}

// Report summarises stored rows for one market.
func (s *ScoreReportCardService) Report(ctx context.Context, market string) (map[string]any, error) {
	rows, err := s.repo.List(ctx, market, scoreICHorizon)
	if err != nil {
		return nil, err
	}
	by := map[string][]repository.ScoreICRow{}
	for _, r := range rows {
		by[r.Axis] = append(by[r.Axis], r)
	}
	axes := make([]ScoreReportAxis, 0, len(scoreICAxes))
	for _, axis := range scoreICAxes {
		axes = append(axes, summarizeAxis(axis, by[axis]))
	}
	return map[string]any{
		"market":        market,
		"horizon":       scoreICHorizon,
		"topN":          scoreICTopN,
		"minDays":       scoreICMinDays,
		"axes":          axes,
		"methodChanges": scoreMethodChanges,
	}, nil
}

func summarizeAxis(axis string, rows []repository.ScoreICRow) ScoreReportAxis {
	out := ScoreReportAxis{Axis: axis, Series: []ScoreReportDaily{}}
	var ics, excess, hits []float64
	for _, r := range rows {
		if !r.IC.Valid {
			continue
		}
		ics = append(ics, r.IC.Float64)
		out.Series = append(out.Series, ScoreReportDaily{Date: r.TradeDate, IC: r.IC.Float64})
		if r.TopExcess.Valid {
			excess = append(excess, r.TopExcess.Float64)
		}
		if r.TopHit.Valid {
			hits = append(hits, r.TopHit.Float64)
		}
	}
	out.Days = len(ics)
	if len(ics) == 0 {
		return out
	}
	mean, sd := meanSD(ics)
	m := round4(mean)
	out.MeanIC = &m
	pos := 0
	for _, v := range ics {
		if v > 0 {
			pos++
		}
	}
	ps := round4(float64(pos) / float64(len(ics)))
	out.PositiveShare = &ps
	if len(ics) >= 2 && sd > 0 {
		/*
			**겹치는 창을 보정한다.** 날짜별 IC는 5거래일 수익률로 재므로 이웃한 날끼리 4일을
			공유한다 — 26일치가 사실상 독립 표본 5개쯤이다. 날짜 수로 t를 내면 약 √5배
			부풀려진다(2026-09-23 첫 계산: 추세 축 t −2.8 → 보정 −1.3). 유효 표본을
			날짜 수 ÷ 창 길이로 잡는다 — 정교한 뉴이-웨스트는 아니지만 방향이 맞고 설명이 쉽다.
		*/
		effective := math.Max(1, float64(len(ics))/float64(scoreICHorizon))
		t := round4(mean / (sd / math.Sqrt(effective)))
		out.TStat = &t
		out.Significant = len(ics) >= scoreICMinDays && math.Abs(t) >= 2
	}
	if len(excess) > 0 {
		e, _ := meanSD(excess)
		e = round4(e)
		out.MeanTopExcess = &e
	}
	if len(hits) > 0 {
		h, _ := meanSD(hits)
		h = round4(h)
		out.MeanTopHit = &h
	}
	return out
}

func meanSD(v []float64) (float64, float64) {
	var sum float64
	for _, x := range v {
		sum += x
	}
	mean := sum / float64(len(v))
	if len(v) < 2 {
		return mean, 0
	}
	var ss float64
	for _, x := range v {
		ss += (x - mean) * (x - mean)
	}
	return mean, math.Sqrt(ss / float64(len(v)-1))
}

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }
