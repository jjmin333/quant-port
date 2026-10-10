package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
AI 랭킹 백테스트 — 「이 점수를 믿고 샀으면 어떻게 됐나」.

설계 원칙 셋은 `model/score_backtest.go` 머리글에 있다(미래를 보지 않는다 · 표본이
짧으면 짧다고 말한다 · 비용을 숨기지 않는다). 여기서는 그중 첫째가 코드의 어디에
있는지만 짚는다.

# 미래를 보지 않는 자리

D일 랭킹은 **D일 종가로** 계산된다. 그 랭킹을 보고 살 수 있는 가장 이른 순간이 D일
종가이므로, 수익은 D+1일부터 난다. 그래서 `holdingsFor`는 **오늘보다 엄격히 이전의**
리밸런싱 날만 본다(`<`, `<=`가 아니다). 이 부등호 하나가 백테스트의 정직성을 정한다 —
`<=`로 두면 「오늘 오른 종목을 오늘 아침에 고른」 것이 되어 어떤 랭킹이든 좋아 보인다.

# 비용

읽기는 두 번이다. 랭킹 구성 한 번(얇은 표), 종가 한 번(`ClosesSinceMulti`, 종목마다
개별 하한이 아니라 공통 창이므로 한 번에 묶인다). 나머지는 전부 메모리 산수다.
*/

// ScoreBacktestService runs backtests over the stored ranking history.
type ScoreBacktestService struct {
	history *repository.ScoreRankingHistoryRepo
	closes  *repository.StockDailyCloseRepo
	indexes *repository.IndexDailyCloseRepo
}

func NewScoreBacktestService(
	history *repository.ScoreRankingHistoryRepo,
	closes *repository.StockDailyCloseRepo,
	indexes *repository.IndexDailyCloseRepo,
) *ScoreBacktestService {
	return &ScoreBacktestService{history: history, closes: closes, indexes: indexes}
}

// backtestMaxTopN mirrors the snapshot depth — asking for more than was stored
// would silently return a shallower basket than the label promises.
const backtestMaxTopN = repository.ScoreRankingHistoryDepth

// Run executes one backtest. Returns (nil, nil) when the history holds nothing
// for the requested window — an empty table is a normal state on day one.
func (s *ScoreBacktestService) Run(
	ctx context.Context, req model.BacktestRequest,
) (*model.BacktestResult, error) {
	if s == nil || s.history == nil || s.closes == nil {
		return nil, fmt.Errorf("backtest service not configured")
	}
	req = normalizeBacktestRequest(req)

	constituents, err := s.history.Constituents(
		ctx, req.Market, string(req.Axis), req.FromDate, req.ToDate, req.TopN)
	if err != nil {
		return nil, err
	}
	if len(constituents) == 0 {
		return nil, nil
	}

	rankingDates := btSortedDates(constituents)
	rebalanceDates := pickRebalanceDates(rankingDates, req.Rebalance)

	// 종가는 **첫 리밸런싱 날부터** 필요하다. 그날 종가가 다음 날 수익률의 분모다.
	since := rebalanceDates[0]
	symbols := uniqueSymbols(constituents)

	reqs := make([]repository.SeriesRequest, 0, len(symbols))
	for _, sym := range symbols {
		reqs = append(reqs, repository.SeriesRequest{Key: sym, Since: since})
	}
	series, err := s.closes.ClosesSinceMulti(ctx, reqs)
	if err != nil {
		return nil, err
	}

	closesBySymbol := make(map[string]map[string]float64, len(series))
	for sym, points := range series {
		byDate := make(map[string]float64, len(points))
		for _, p := range points {
			if p.Close > 0 {
				byDate[p.Date] = p.Close
			}
		}
		closesBySymbol[strings.ToUpper(sym)] = byDate
	}

	/*
		지수도 **한 번에** 읽는다 — `ClosesSinceMulti`가 종목에 쓰는 것과 같은 함수다.
		벤치마크가 셋이라고 조회가 셋이 되면, 이 화면을 여는 비용이 벤치마크 수에
		비례해 늘어난다.
	*/
	benchmarks := map[string]map[string]float64{}
	if s.indexes != nil && len(req.Benchmarks) > 0 {
		reqs := make([]repository.SeriesRequest, 0, len(req.Benchmarks))
		for _, id := range req.Benchmarks {
			reqs = append(reqs, repository.SeriesRequest{Key: id, Since: since})
		}
		if idx, err := s.indexes.ClosesSinceMulti(ctx, reqs); err == nil {
			for _, id := range req.Benchmarks {
				byDate := make(map[string]float64)
				for _, p := range idx[strings.ToLower(id)] {
					if p.Close > 0 {
						byDate[p.Date] = p.Close
					}
				}
				if len(byDate) > 0 {
					benchmarks[id] = byDate
				}
			}
		}
	}

	return runBacktest(req, constituents, rebalanceDates, closesBySymbol, benchmarks), nil
}

func normalizeBacktestRequest(req model.BacktestRequest) model.BacktestRequest {
	if req.Market != "overseas" {
		req.Market = "domestic"
	}
	if req.Axis == "" {
		req.Axis = model.RankAxisComposite
	}
	if req.TopN <= 0 {
		req.TopN = 20
	}
	if req.TopN > backtestMaxTopN {
		req.TopN = backtestMaxTopN
	}
	if req.Rebalance == "" {
		req.Rebalance = model.RebalanceWeekly
	}
	if req.CostBps < 0 {
		req.CostBps = 0
	}
	if req.FromDate == "" {
		req.FromDate = "1900-01-01"
	}
	if req.ToDate == "" {
		req.ToDate = "9999-12-31"
	}

	/*
		벤치마크는 **화이트리스트**를 지난 것만 남는다.

		임의의 index_id를 받으면 「VIX 대비 초과수익」 같은 숫자를 요청 하나로 만들 수
		있다. 숫자는 나오지만 뜻이 없고, 화면에 한 번 나가면 사람은 그것을 성적으로 읽는다.
	*/
	allowed := make([]string, 0, len(req.Benchmarks))
	seen := map[string]struct{}{}
	for _, id := range req.Benchmarks {
		id = strings.ToLower(strings.TrimSpace(id))
		if !model.IsBacktestBenchmark(req.Market, id) {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		allowed = append(allowed, id)
	}
	if len(allowed) == 0 {
		allowed = model.DefaultBacktestBenchmarks[req.Market]
	}
	req.Benchmarks = allowed
	return req
}

/*
runBacktest — 순수 계산부. DB도 시계도 없다.

시험이 이 함수를 직접 부른다(`score_backtest_test.go`) — 미래를 보지 않는다는 성질은
「그런 쿼리를 안 짰다」로는 증명되지 않고, **미래에 폭등하는 종목을 심어 두고 그날
수익이 안 잡히는지** 재야 증명된다.
*/
func runBacktest(
	req model.BacktestRequest,
	constituents map[string][]repository.ScoreRankingHistoryRow,
	rebalanceDates []string,
	closes map[string]map[string]float64,
	benchmarks map[string]map[string]float64,
) *model.BacktestResult {
	/*
		달력은 **종가가 있는 날**이다.

		랭킹 날짜를 쓰면 점수 배치가 쉰 날에 구멍이 생기고(실측 27일 중 16일만 점수가
		있었다), 그 구멍을 「시장이 없었다」로 읽으면 그 사이의 등락이 통째로 사라진다.
		종가는 시장이 열린 모든 날에 있다.

		**벤치마크의 달력은 쓰지 않는다** — 해외 지수는 국내 달력과 어긋나고, 지수에
		맞추면 포트폴리오가 실제로 움직인 날이 빠진다. 지수는 이 달력 위에서 잰다.
	*/
	calendar := tradingCalendar(closes, rebalanceDates[0], req.ToDate)
	if len(calendar) < 2 {
		return emptyBacktestResult(req, rebalanceDates)
	}

	holdingsAt := make(map[string][]string, len(rebalanceDates))
	for _, date := range rebalanceDates {
		rows := constituents[date]
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			if len(names) >= req.TopN {
				break
			}
			names = append(names, strings.ToUpper(row.Symbol))
		}
		holdingsAt[date] = names
	}

	// 벤치마크는 포트폴리오와 **같은 달력** 위에서 따로 굴린다.
	tracks := make(map[string]*benchmarkTrack, len(benchmarks))
	for _, id := range req.Benchmarks {
		if closesByDate, ok := benchmarks[id]; ok {
			tracks[id] = &benchmarkTrack{closes: closesByDate, index: 100, peak: 100}
		}
	}

	var (
		index        = 100.0
		peak         = 100.0
		mdd          float64
		daily        []float64
		curve        = make([]model.BacktestPoint, 0, len(calendar))
		missing      int
		holdingsSum  int
		holdingDays  int
		turnovers    []float64
		prevHoldings []string
		rebalances   int
	)

	primaryID := ""
	if len(req.Benchmarks) > 0 {
		primaryID = req.Benchmarks[0]
	}

	curve = append(curve, model.BacktestPoint{
		Date:       calendar[0],
		Portfolio:  100,
		Benchmarks: startingBenchmarkPoints(req.Benchmarks, tracks),
	})

	for i := 1; i < len(calendar); i++ {
		prev, today := calendar[i-1], calendar[i]

		// **부등호 하나가 정직성을 정한다** — 오늘보다 엄격히 이전의 리밸런싱만 본다.
		rebalanceDate := latestOnOrBefore(rebalanceDates, prev)
		if rebalanceDate == "" {
			continue
		}
		names := holdingsAt[rebalanceDate]
		if len(names) == 0 {
			continue
		}
		if !sameNames(prevHoldings, names) {
			if prevHoldings != nil {
				turnovers = append(turnovers, turnoverBetween(prevHoldings, names))
			}
			prevHoldings = names
			rebalances++
		}

		var sum float64
		var counted int
		for _, sym := range names {
			byDate := closes[sym]
			p0, ok0 := byDate[prev]
			p1, ok1 := byDate[today]
			if !ok0 || !ok1 || p0 <= 0 {
				// 거래정지·상장폐지·데이터 공백. **0%로 세지 않는다** — 그러면
				// 없는 종목이 「보합」으로 수익률을 희석한다.
				missing++
				continue
			}
			sum += p1/p0 - 1
			counted++
		}
		if counted == 0 {
			continue
		}
		ret := sum / float64(counted)

		// 거래 비용은 **바뀐 비중만큼만** 문다. 같은 종목을 계속 들고 있으면 비용이 없다.
		if len(turnovers) > 0 && req.CostBps > 0 && today == nextAfter(calendar, rebalanceDate) {
			ret -= turnovers[len(turnovers)-1] * req.CostBps / 10000
		}

		index *= 1 + ret
		daily = append(daily, ret)
		holdingsSum += counted
		holdingDays++
		if index > peak {
			peak = index
		}
		if dd := index/peak - 1; dd < mdd {
			mdd = dd
		}

		for _, track := range tracks {
			track.step(prev, today)
		}

		curve = append(curve, model.BacktestPoint{
			Date:       today,
			Portfolio:  btRound2(index),
			Benchmarks: benchmarkPoints(req.Benchmarks, tracks),
			Holdings:   counted,
		})
	}

	out := &model.BacktestResult{
		Request:     req,
		FromDate:    calendar[0],
		ToDate:      curve[len(curve)-1].Date,
		BenchmarkID: primaryID,
		Curve:       curve,
		Portfolio:   summarizeBacktest(index, mdd, daily),
		Benchmarks:  []model.BacktestBenchmark{},
	}

	for _, id := range req.Benchmarks {
		track, ok := tracks[id]
		if !ok || len(track.daily) == 0 {
			// 지수 이력이 그 창에 없으면 **행을 만들지 않는다** — 0%짜리 벤치마크가
			// 서 있으면 포트폴리오가 시장을 이긴 것처럼 읽힌다.
			continue
		}
		m := summarizeBacktest(track.index, track.mdd, track.daily)
		row := model.BacktestBenchmark{
			ID:      id,
			Label:   model.BacktestBenchmarkLabel(req.Market, id),
			Primary: id == primaryID,
			Metrics: m,
			Alpha:   btRound2(out.Portfolio.TotalReturn - m.TotalReturn),
			Days:    len(track.daily),
		}
		out.Benchmarks = append(out.Benchmarks, row)
		if row.Primary {
			alpha := row.Alpha
			out.Alpha = &alpha
		}
	}
	// primary의 이력이 없으면 머리줄 기준을 **실제로 잰 첫 번째**로 옮긴다.
	if out.Alpha == nil && len(out.Benchmarks) > 0 {
		out.Benchmarks[0].Primary = true
		out.BenchmarkID = out.Benchmarks[0].ID
		alpha := out.Benchmarks[0].Alpha
		out.Alpha = &alpha
	}

	out.Coverage = model.BacktestCoverage{
		RankingDates:  len(constituents),
		TradingDays:   holdingDays,
		Rebalances:    rebalances,
		MissingPrices: missing,
		Sufficient:    holdingDays >= model.BacktestMinDays,
	}
	if holdingDays > 0 {
		out.Coverage.AvgHoldings = btRound2(float64(holdingsSum) / float64(holdingDays))
	}
	if len(turnovers) > 0 {
		t := btRound2(mean(turnovers) * 100)
		out.Coverage.Turnover = &t
	}
	out.Coverage.Note = backtestNote(out.Coverage)
	return out
}

/*
benchmarkTrack — 한 지수를 포트폴리오와 **같은 달력** 위에서 굴린 결과.

지수마다 휴장일이 다르다(해외 지수는 국내 달력과 어긋나고, FRED 계열은 미국 공휴일에
빈다). 그 날은 **건너뛴다** — 0%로 세면 지수가 실제보다 덜 움직인 것이 되고, 그만큼
포트폴리오의 초과수익이 부풀려진다.
*/
type benchmarkTrack struct {
	closes map[string]float64
	index  float64
	peak   float64
	mdd    float64
	daily  []float64
}

func (t *benchmarkTrack) step(prev, today string) {
	p0, ok0 := t.closes[prev]
	p1, ok1 := t.closes[today]
	if !ok0 || !ok1 || p0 <= 0 {
		return
	}
	ret := p1/p0 - 1
	t.index *= 1 + ret
	t.daily = append(t.daily, ret)
	if t.index > t.peak {
		t.peak = t.index
	}
	if dd := t.index/t.peak - 1; dd < t.mdd {
		t.mdd = dd
	}
}

func startingBenchmarkPoints(ids []string, tracks map[string]*benchmarkTrack) map[string]float64 {
	if len(tracks) == 0 {
		return nil
	}
	out := make(map[string]float64, len(tracks))
	for _, id := range ids {
		if _, ok := tracks[id]; ok {
			out[id] = 100
		}
	}
	return out
}

func benchmarkPoints(ids []string, tracks map[string]*benchmarkTrack) map[string]float64 {
	if len(tracks) == 0 {
		return nil
	}
	out := make(map[string]float64, len(tracks))
	for _, id := range ids {
		if t, ok := tracks[id]; ok {
			out[id] = btRound2(t.index)
		}
	}
	return out
}

func summarizeBacktest(index, mdd float64, daily []float64) model.BacktestMetrics {
	m := model.BacktestMetrics{
		TotalReturn: btRound2((index/100 - 1) * 100),
		MDD:         btRound2(mdd * 100),
	}
	n := len(daily)
	if n == 0 {
		return m
	}

	wins := 0
	best, worst := daily[0], daily[0]
	for _, r := range daily {
		if r > 0 {
			wins++
		}
		best = math.Max(best, r)
		worst = math.Min(worst, r)
	}
	winRate := btRound2(float64(wins) / float64(n) * 100)
	m.WinRate = &winRate
	b, w := btRound2(best*100), btRound2(worst*100)
	m.BestDay, m.WorstDay = &b, &w

	/*
		변동성은 **서술 통계**라 20거래일부터 낸다 — 「이 바구니가 얼마나 흔들렸나」는
		짧은 창에서도 사실이다.

		Sharpe는 다르다. 28거래일로 잰 값이 4.65로 나오는데(실측 2026-09-18, 국내 ·
		성장 · 상위20), 그건 **운을 실력으로 읽은 숫자**다. 분모가 짧으면 표준편차가
		작게 나오고 Sharpe는 그만큼 커진다. 연 환산과 같은 문턱을 쓴다 — 안내문이
		약속한 바로 그 숫자여야 한다.
	*/
	if n >= model.BacktestMinSharpeDays {
		sd := stdDev(daily)
		if sd > 0 {
			vol := btRound2(sd * math.Sqrt(model.BacktestTradingDaysPerYear) * 100)
			m.Volatility = &vol
			if n >= model.BacktestMinDays {
				sharpe := btRound2(mean(daily) / sd * math.Sqrt(model.BacktestTradingDaysPerYear))
				m.Sharpe = &sharpe
			}
		}
	}
	/*
		연 환산은 **표본이 충분할 때만**. 30거래일치 +12%를 연율화하면 +166%가 되는데,
		그건 측정이 아니라 외삽이다 (`model.BacktestMinDays` 머리글).
	*/
	if n >= model.BacktestMinDays {
		years := float64(n) / model.BacktestTradingDaysPerYear
		if years > 0 && index > 0 {
			ann := btRound2((math.Pow(index/100, 1/years) - 1) * 100)
			m.AnnualizedReturn = &ann
		}
	}
	return m
}

func backtestNote(c model.BacktestCoverage) string {
	if c.TradingDays == 0 {
		return "수익률을 계산할 수 있는 거래일이 없습니다."
	}
	if !c.Sufficient {
		return fmt.Sprintf(
			"표본이 %d거래일로 짧아 통계적 유의성을 말할 수 없습니다. 연 환산 수익률과 Sharpe는 %d거래일부터 계산합니다.",
			c.TradingDays, model.BacktestMinDays)
	}
	return ""
}

func emptyBacktestResult(
	req model.BacktestRequest, rebalanceDates []string,
) *model.BacktestResult {
	return &model.BacktestResult{
		Request:    req,
		FromDate:   rebalanceDates[0],
		ToDate:     rebalanceDates[len(rebalanceDates)-1],
		Curve:      []model.BacktestPoint{},
		Benchmarks: []model.BacktestBenchmark{},
		Coverage: model.BacktestCoverage{
			RankingDates: len(rebalanceDates),
			Note:         "수익률을 계산할 수 있는 거래일이 없습니다.",
		},
	}
}

/* -------------------------------------------------------------------------- */
/* 달력 · 리밸런싱                                                             */
/* -------------------------------------------------------------------------- */

// tradingCalendar is every date any holding has a close for, inside the window.
func tradingCalendar(closes map[string]map[string]float64, from, to string) []string {
	seen := map[string]struct{}{}
	for _, byDate := range closes {
		for date := range byDate {
			if date < from || date > to {
				continue
			}
			seen[date] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for date := range seen {
		out = append(out, date)
	}
	sort.Strings(out)
	return out
}

/*
pickRebalanceDates — 랭킹이 있는 날 중에서 주기에 맞는 날만 고른다.

주·월의 경계는 **첫 랭킹 날**이다. 「매주 월요일」로 못박지 않는 이유: 점수 배치가
매일 도는 것이 아니라(실측 27일 중 16일) 특정 요일을 고르면 그 요일이 빈 주는 통째로
리밸런싱을 건너뛴다. 「그 주에 처음 랭킹이 나온 날」은 어느 주에나 있다.
*/
func pickRebalanceDates(rankingDates []string, period model.BacktestRebalance) []string {
	if len(rankingDates) == 0 {
		return nil
	}
	if period == model.RebalanceDaily {
		return rankingDates
	}
	out := make([]string, 0, len(rankingDates))
	lastBucket := ""
	for _, date := range rankingDates {
		bucket := bucketOf(date, period)
		if bucket == lastBucket {
			continue
		}
		lastBucket = bucket
		out = append(out, date)
	}
	return out
}

func bucketOf(date string, period model.BacktestRebalance) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	if period == model.RebalanceMonthly {
		return t.Format("2006-01")
	}
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

// latestOnOrBefore is the last rebalance date at or before `date`. The caller
// passes YESTERDAY, which is what keeps the run free of look-ahead.
func latestOnOrBefore(dates []string, date string) string {
	i := sort.SearchStrings(dates, date)
	if i < len(dates) && dates[i] == date {
		return dates[i]
	}
	if i == 0 {
		return ""
	}
	return dates[i-1]
}

func nextAfter(calendar []string, date string) string {
	i := sort.SearchStrings(calendar, date)
	if i < len(calendar) && calendar[i] == date {
		i++
	}
	if i >= len(calendar) {
		return ""
	}
	return calendar[i]
}

/* -------------------------------------------------------------------------- */
/* 작은 계산기들                                                               */
/* -------------------------------------------------------------------------- */

func btSortedDates[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func uniqueSymbols(constituents map[string][]repository.ScoreRankingHistoryRow) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, rows := range constituents {
		for _, row := range rows {
			sym := strings.ToUpper(row.Symbol)
			if _, dup := seen[sym]; dup {
				continue
			}
			seen[sym] = struct{}{}
			out = append(out, sym)
		}
	}
	sort.Strings(out)
	return out
}

func sameNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// turnoverBetween is the share of the basket that changed (0~1).
func turnoverBetween(prev, next []string) float64 {
	if len(next) == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(prev))
	for _, s := range prev {
		set[s] = struct{}{}
	}
	kept := 0
	for _, s := range next {
		if _, ok := set[s]; ok {
			kept++
		}
	}
	return 1 - float64(kept)/float64(len(next))
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// stdDev is the sample standard deviation (n−1). 표본이라 n−1이다 — n으로 나누면
// 짧은 창에서 변동성이 체계적으로 작게 나오고, Sharpe는 그만큼 커진다.
func stdDev(xs []float64) float64 {
	n := len(xs)
	if n < 2 {
		return 0
	}
	m := mean(xs)
	var sum float64
	for _, x := range xs {
		d := x - m
		sum += d * d
	}
	return math.Sqrt(sum / float64(n-1))
}

func btRound2(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*100) / 100
}
