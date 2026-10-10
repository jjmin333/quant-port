package model

/*
AI 랭킹 백테스트 — 「이 점수를 믿고 샀으면 어떻게 됐나」에 저장된 기록으로 답한다.

# 이 파일이 지키는 세 가지

**① 미래를 보지 않는다.** D일 랭킹은 D일 종가로 계산된다. 그러므로 그 랭킹으로 살 수
있는 가장 이른 시점은 D일 종가이고, 수익은 **D+1일부터** 난다. 엔진은 리밸런싱 날의
수익률을 절대 포함하지 않는다 — 그 하루를 포함하면 어느 랭킹이든 좋아 보인다.

**② 표본이 짧으면 짧다고 말한다.** 30거래일을 연율화하면 +400%가 나온다. 이 타입들은
`AnnualizedReturn`·`Sharpe`를 **포인터**로 들고, 표본이 모자라면 비운다 — 0이나
그럴듯한 숫자를 넣지 않는다.

**③ 비용을 숨기지 않는다.** 리밸런싱마다 종목이 바뀌면 거래 비용이 든다. 엔진은
회전율을 세어 함께 내보내고, 수수료 가정은 요청이 정한다(기본 0 — 「우리가 고른
수수료로 좋아 보인다」가 되지 않도록).
*/

// BacktestRebalance is how often the portfolio is rebuilt from the ranking.
type BacktestRebalance string

const (
	RebalanceDaily   BacktestRebalance = "daily"
	RebalanceWeekly  BacktestRebalance = "weekly"
	RebalanceMonthly BacktestRebalance = "monthly"
)

var BacktestRebalances = []BacktestRebalance{RebalanceDaily, RebalanceWeekly, RebalanceMonthly}

var BacktestRebalanceLabels = map[BacktestRebalance]string{
	RebalanceDaily:   "매일",
	RebalanceWeekly:  "매주",
	RebalanceMonthly: "매월",
}

func ParseBacktestRebalance(raw string) BacktestRebalance {
	for _, r := range BacktestRebalances {
		if string(r) == raw {
			return r
		}
	}
	return RebalanceWeekly
}

// BacktestRequest is one run's settings.
type BacktestRequest struct {
	Market    string            `json:"market"`
	Axis      ScoreRankingAxis  `json:"axis"`
	TopN      int               `json:"topN"`
	Rebalance BacktestRebalance `json:"rebalance"`
	FromDate  string            `json:"fromDate"`
	ToDate    string            `json:"toDate"`
	/*
		Benchmarks — 이 실행을 견줄 지수들. 비어 있으면 시장별 기본값을 쓴다
		(`DefaultBacktestBenchmarks`). 첫 항목이 머리줄 초과수익의 기준이다.
	*/
	Benchmarks []string `json:"benchmarks,omitempty"`
	// CostBps is the one-way trading cost in basis points, applied to the part
	// of the portfolio that actually changed at each rebalance. 기본 0 —
	// 우리가 고른 수수료로 결과가 좋아 보이면 안 된다.
	CostBps float64 `json:"costBps"`
}

// BacktestPoint is one trading day on the equity curve.
type BacktestPoint struct {
	Date string `json:"date"`
	// Portfolio is an index value starting at 100.
	Portfolio float64 `json:"portfolio"`
	/*
		Benchmarks — 지수 id → 같은 100 기준 지수값.

		배열이 아니라 맵인 이유: 어떤 지수는 그날 휴장이라 값이 없는데, 배열이면 화면이
		위치로 맞춰야 하고 한 칸이 비는 순간 선이 어긋난다.
	*/
	Benchmarks map[string]float64 `json:"benchmarks,omitempty"`
	// Holdings is how many names actually produced a return that day. 보유
	// 종목 수와 다를 수 있다 — 거래정지·상장폐지로 종가가 없는 날이 있다.
	Holdings int `json:"holdings"`
}

// BacktestMetrics is one series' performance.
type BacktestMetrics struct {
	// TotalReturn is the cumulative return over the tested window (%).
	TotalReturn float64 `json:"totalReturn"`
	/*
		AnnualizedReturn — 연 환산. **표본이 짧으면 비어 있다.**

		30거래일치 +12%를 연율화하면 +166%가 된다. 그 숫자는 측정이 아니라 외삽이고,
		백테스트 화면에서 가장 쉽게 오해되는 자리다.
	*/
	AnnualizedReturn *float64 `json:"annualizedReturn,omitempty"`
	// MDD is the maximum drawdown (%, negative).
	MDD float64 `json:"mdd"`
	// Volatility is the annualized standard deviation of daily returns (%).
	Volatility *float64 `json:"volatility,omitempty"`
	/*
		Sharpe — 무위험수익률 0 기준. **표본이 짧으면 비어 있다**(연 환산과 같은 문턱).

		짧은 창은 표준편차를 작게 만들고, 작은 분모는 Sharpe를 크게 만든다. 실측
		2026-09-18 국내 · 성장 · 상위20의 28거래일 Sharpe는 4.65였다 — 운을 실력으로
		읽은 숫자이고, 이 화면에서 가장 위험한 한 칸이다.
	*/
	Sharpe *float64 `json:"sharpe,omitempty"`
	// WinRate is the share of days with a positive return (%).
	WinRate *float64 `json:"winRate,omitempty"`
	// BestDay / WorstDay are the extremes (%), for a sense of the ride.
	BestDay  *float64 `json:"bestDay,omitempty"`
	WorstDay *float64 `json:"worstDay,omitempty"`
}

// BacktestCoverage is the honesty block: what the run actually had to work with.
type BacktestCoverage struct {
	// RankingDates is how many dated snapshots the window held.
	RankingDates int `json:"rankingDates"`
	// TradingDays is how many days produced a return.
	TradingDays int `json:"tradingDays"`
	// Rebalances is how many times the portfolio was rebuilt.
	Rebalances int `json:"rebalances"`
	// AvgHoldings is the mean number of names that produced a return.
	AvgHoldings float64 `json:"avgHoldings"`
	// MissingPrices counts (day × symbol) slots dropped for want of a close.
	MissingPrices int `json:"missingPrices"`
	// Turnover is the mean share of the portfolio replaced per rebalance (%).
	Turnover *float64 `json:"turnover,omitempty"`
	/*
		Sufficient — 이 표본으로 성과를 말해도 되는가.

		거래일 수 하나로 정한다(`BacktestMinDays`). false면 화면은 숫자를 **가설로**
		표시해야 하고, 연 환산과 Sharpe는 애초에 비어 있다.
	*/
	Sufficient bool `json:"sufficient"`
	// Note is the one-line caveat shown next to the numbers.
	Note string `json:"note,omitempty"`
}

// BacktestResult is the whole answer.
type BacktestResult struct {
	Request BacktestRequest `json:"request"`
	// FromDate / ToDate are what was actually covered, which can be narrower
	// than the request — 요청한 창이 아니라 **가진 창**을 말한다.
	FromDate string `json:"fromDate"`
	ToDate   string `json:"toDate"`

	Portfolio BacktestMetrics `json:"portfolio"`
	/*
		Benchmarks — 견준 지수 전부. 첫 항목이 primary이고, 아래 `BenchmarkID`·`Alpha`는
		그 primary의 값이다(머리줄 한 칸이 쓰는 편의 필드).

		여럿인 이유는 `BacktestBenchmarkMeta` 머리글에 있다 — 국내 상위 20에 코스닥이
		섞여 있는데 KOSPI로만 재면 초과수익이 「시장을 잘못 골라서」 나온 숫자가 된다.
	*/
	Benchmarks []BacktestBenchmark `json:"benchmarks"`
	// BenchmarkID / Alpha are the primary benchmark's, for the headline.
	BenchmarkID string   `json:"benchmarkId,omitempty"`
	Alpha       *float64 `json:"alpha,omitempty"`

	Curve    []BacktestPoint  `json:"curve"`
	Coverage BacktestCoverage `json:"coverage"`
}

/*
BacktestMinDays — 이 아래로는 성과를 말하지 않는다.

40거래일은 AGENT_STATE의 TODO(「점수: 예측력 검증 — 40거래일 축적 후」)가 이미 잡아
둔 문턱이고, 두 달 남짓이라 한 번의 급등락이 전체를 정하지 않는 최소한이다. 그보다
짧아도 곡선과 누적 수익률은 그린다 — 다만 「표본 N거래일, 통계적 유의성 없음」을
함께 적는다.
*/
const BacktestMinDays = 40

// BacktestMinSharpeDays is the floor for a standard deviation to mean anything —
// 변동성(서술 통계)이 여기서 열리고, Sharpe(성과 주장)는 BacktestMinDays를 기다린다.
const BacktestMinSharpeDays = 20

// BacktestTradingDaysPerYear is the annualization factor.
const BacktestTradingDaysPerYear = 252

/*
벤치마크 — **비교할 만한 지수를 여럿** 댄다 (2026-09-18 사용자 요청).

# 왜 하나로는 모자란가

국내 상위 20에 코스닥 종목이 섞여 있는데 KOSPI 하나로만 재면, 코스닥이 그 달에 크게
움직였을 때 초과수익이 시장이 아니라 **시장을 잘못 골라서** 나온 숫자가 된다. 해외도
같다 — NASDAQ 상장 종목을 S&P 500으로만 재는 것과 나스닥 종합으로도 재는 것은
다른 질문이고, 둘 다 볼 수 있어야 답이 정직해진다.

# 왜 이 목록만인가

`marketIndexCatalog`에는 VIX · 미 국채 10년 · USD/KRW · 비트코인 · 금도 있다. 전부
일별 종가가 쌓여 있지만 **주식 바구니의 벤치마크가 아니다** — 변동성 지수나 채권
금리와 수익률을 견주면 숫자는 나오는데 뜻이 없다. 여기 있는 것은 지분형 시장 지수뿐이다.
*/
type BacktestBenchmarkMeta struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// BacktestBenchmarkCatalog is every index a backtest may be measured against,
// per market tab. 첫 항목이 **기준(primary)** 이다 — 머리줄의 초과수익이 그것이다.
var BacktestBenchmarkCatalog = map[string][]BacktestBenchmarkMeta{
	"domestic": {
		{ID: "kospi", Label: "KOSPI"},
		{ID: "kosdaq", Label: "KOSDAQ"},
	},
	"overseas": {
		{ID: "sp500", Label: "S&P 500"},
		{ID: "nasdaq", Label: "나스닥 종합"},
		{ID: "nasdaq100", Label: "나스닥 100"},
		{ID: "sox", Label: "필라델피아 반도체"},
		{ID: "dow", Label: "다우존스"},
	},
}

// DefaultBacktestBenchmarks is what a run compares against when the caller does
// not choose. 셋을 넘기지 않는다 — 곡선이 넷 이상이면 어느 선이 내 것인지 안 읽힌다.
var DefaultBacktestBenchmarks = map[string][]string{
	"domestic": {"kospi", "kosdaq"},
	"overseas": {"sp500", "nasdaq100", "sox"},
}

// BacktestBenchmarkLabel resolves an id to its Korean label, falling back to the
// id so an index added to the catalog later is still legible.
func BacktestBenchmarkLabel(market, id string) string {
	for _, m := range BacktestBenchmarkCatalog[market] {
		if m.ID == id {
			return m.Label
		}
	}
	return id
}

// IsBacktestBenchmark reports whether an id may be used for `market`.
//
// 화이트리스트다. 임의의 index_id를 받으면 VIX 대비 초과수익 같은 무의미한 숫자를
// 요청 하나로 만들어 낼 수 있다.
func IsBacktestBenchmark(market, id string) bool {
	for _, m := range BacktestBenchmarkCatalog[market] {
		if m.ID == id {
			return true
		}
	}
	return false
}

/*
BacktestBenchmark is one index's result alongside the portfolio's.

`Alpha`는 **이 벤치마크 대비** 초과수익(%p)이다. 결과의 머리줄 `Alpha`는 첫 번째
(primary) 벤치마크의 것이고, 나머지는 여기 각 행에 붙는다 — 화면이 「무엇 대비인지」를
적지 않고 숫자만 옮기면 같은 포트폴리오가 시장마다 다른 성적표를 갖게 된다.
*/
type BacktestBenchmark struct {
	ID      string          `json:"id"`
	Label   string          `json:"label"`
	Primary bool            `json:"primary"`
	Metrics BacktestMetrics `json:"metrics"`
	Alpha   float64         `json:"alpha"`
	// Days is how many of the portfolio's trading days this index also had a
	// close for. 지수마다 휴장일이 다르다 — 해외 지수는 국내 달력과 어긋난다.
	Days int `json:"days"`
}
