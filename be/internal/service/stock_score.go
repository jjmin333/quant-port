package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// ScoreService computes the 종합 점수 for the whole universe in one batch.
//
// Design: docs/STOCK_SCORE_DESIGN.md. Two properties matter most.
//
//  1. Every raw metric becomes a rank percentile inside its peer group, so the
//     measured fat tails (ROE up to 227,162%) cannot dominate.
//  2. The composite is shrunk toward neutral by how much of the weight was
//     actually backed by data, so a symbol carrying only RSI cannot reach 90.
//
// The batch costs zero upstream API calls — every input is already in MySQL.
type ScoreService struct {
	db       *sql.DB
	scores   *repository.StockScoreRepo
	settings *ScoreSettingsStore

	// indexScores shares this batch's daily slot — see RunScheduler.
	indexScores *IndexScoreService

	// liveQuotes overlays the leaderboard's change rate while the market is
	// trading. Nil in the batch workers — they never serve the list, and the
	// scoring run itself must stay free of upstream calls.
	liveQuotes ScoreLiveQuotes
}

// ScoreLiveQuotes is the slice of QuotesService the leaderboard needs. An
// interface, not the concrete type, so the score batch does not drag the whole
// quote stack (Toss client, Redis, poller) into worker processes.
type ScoreLiveQuotes interface {
	LiveChanges(ctx context.Context, reqs []LiveChangeRequest) map[string]LiveChange
}

// SetIndexScores wires index scoring into the same daily run.
func (s *ScoreService) SetIndexScores(svc *IndexScoreService) { s.indexScores = svc }

// SetLiveQuotes wires live prices into the leaderboard read path.
func (s *ScoreService) SetLiveQuotes(q ScoreLiveQuotes) { s.liveQuotes = q }

func NewScoreService(db *sql.DB, scores *repository.StockScoreRepo, settings *ScoreSettingsStore) *ScoreService {
	return &ScoreService{db: db, scores: scores, settings: settings}
}

// scoreInput is one symbol's raw material, assembled before normalization.
type scoreInput struct {
	symbol    string
	name      string
	assetType string
	market    string

	close     float64
	closeDate string // 쓴 종가의 날짜 — 기준일보다 이를 수 있다(scoreCloseLookbackDays)
	shares    float64
	marketCap float64

	roe          *float64
	opMargin     *float64
	netMargin    *float64
	debtRatio    *float64
	currentRatio *float64
	revGrowth    *float64
	epsGrowth    *float64

	per *float64
	pbr *float64
	psr *float64
	// extUsed — EDGAR가 비운 값을 Finnhub 보조로 채웠다(2026-09-28). 판에 「|ext」가 붙는다.
	extUsed bool
	// ttmPeriod — 국내 비율을 최근 4분기 합으로 덮었을 때 그 마지막 분기(「2026Q2」). 판에 「|ttm」(stock_score_ttm.go).
	ttmPeriod string

	ti       *TechnicalIndicators
	barCount int

	// techRaw is the absolute technical reading (RSI/MACD/trend curves) before
	// cross-sectional ranking; techConf is how complete that reading was.
	techRaw  *float64
	techConf float64

	peer string
	// sentScore/sentConf — 심리·수급 축(0–100)과 그 재료의 두께(0–1). 재료가
	// 없으면 sentScore는 nil이다(sentiment_axis.go). newsCount는 inputs 기록용.
	sentScore *float64
	sentConf  float64
	newsCount int
	// flowPct — 외국인+기관 20거래일 순매수 ÷ 거래량(%). 수급 성분이 쓰였을 때만(inputs 기록용).
	flowPct *float64
	// redditSent — Reddit 성분(15..85)이 쓰였을 때만. 판에 「|reddit」가 붙는다(2026-09-29).
	redditSent *float64

	// sector is the common 11-sector key from stock_sectors ("" when unknown).
	sector string
	// sectorPeer is the finest level for fundamental metrics — "asset:sector" —
	// set only when that asset type's sector coverage passes sectorPeerMinCoverage.
	sectorPeer string
	// metricLevels records which peer level each metric was ranked in, so an
	// unexpected percentile can be traced back to its comparison set.
	metricLevels map[string]string
}

// fundamentalsApplicable mirrors model.IsFundamentalsApplicable: ETFs and ETNs
// have no financial statements, so scoring them on quality/growth/valuation
// would be meaningless. They get a technical-only score under a distinct label.
func (in *scoreInput) fundamentalsApplicable() bool {
	return in.assetType != string(model.AssetETF)
}

// PruneOldScores drops score rows past the retention window.
//
// 조회는 전부 최신 거래일 하나만 본다(MAX(trade_date) 또는 LatestScoredDate).
// 과거 점수 시계열을 읽는 기능이 없으므로 오래된 행은 용량만 차지한다 —
// 정리하지 않으면 연 7.5GB다. repository.ScoreRetentionDates 헤더 참조.
func (s *ScoreService) PruneOldScores(ctx context.Context) (int64, error) {
	if s == nil || s.scores == nil {
		return 0, nil
	}
	return s.scores.PruneOldScores(ctx, repository.ScoreRetentionDates)
}

// Recompute builds and stores scores for one trade date. Returns the
// distribution self-check so callers (batch, admin) can gate on it.
func (s *ScoreService) Recompute(ctx context.Context, tradeDate string) (model.ScoreDistribution, error) {
	var dist model.ScoreDistribution
	if s == nil || s.db == nil {
		return dist, fmt.Errorf("score service not configured")
	}
	if _, err := time.Parse("2006-01-02", tradeDate); err != nil {
		return dist, fmt.Errorf("invalid trade date %q: %w", tradeDate, err)
	}

	start := time.Now()
	inputs, err := s.loadUniverse(ctx, tradeDate)
	if err != nil {
		return dist, fmt.Errorf("load universe: %w", err)
	}
	if len(inputs) == 0 {
		return dist, fmt.Errorf("no symbols for %s", tradeDate)
	}
	log.Printf("score: loaded %d symbols for %s in %s", len(inputs), tradeDate, time.Since(start).Round(time.Millisecond))
	if n, err := s.attachDomesticTTM(ctx, inputs, tradeDate); err != nil {
		// 분기 표를 못 읽었다 — 국내는 연간 값으로 매긴다(지금까지와 같다).
		log.Printf("score: domestic TTM skipped: %v", err)
	} else {
		log.Printf("score: domestic TTM on %d symbols", n)
	}

	techStart := time.Now()
	if err := s.attachTechnicals(ctx, inputs, tradeDate); err != nil {
		return dist, fmt.Errorf("technicals: %w", err)
	}
	log.Printf("score: technicals in %s", time.Since(techStart).Round(time.Millisecond))

	for _, in := range inputs {
		in.techRaw, in.techConf = technicalAxis(in)
	}
	/*
		동종 그룹은 **어제 것을 이어 받는다** — `assignPeerGroups` 참고. 못 읽어도
		배치는 그대로 돈다(첫 실행이면 읽을 것이 없다), 그때는 예전처럼 오늘 분포로만
		정해진다.
	*/
	prevPeer, err := s.scores.PeerGroupsBefore(ctx, tradeDate)
	if err != nil {
		log.Printf("score: previous peer groups unavailable, assigning fresh: %v", err)
		prevPeer = nil
	}
	assignPeerGroups(inputs, prevPeer)
	if n, err := s.attachSentiment(ctx, inputs, tradeDate); err != nil {
		// 재료를 못 읽었다 — 심리 축만 빠지고 나머지 네 축은 그대로 매긴다.
		log.Printf("score: sentiment axis skipped: %v", err)
	} else {
		log.Printf("score: sentiment axis on %d symbols", n)
	}
	for asset, cov := range assignSectorPeers(inputs) {
		log.Printf("score: sector peers %s coverage %.1f%% → %v", asset, cov.pct*100, cov.enabled)
	}
	normalized := normalizeUniverse(inputs)

	settings := model.DefaultScoreSettings()
	if s.settings != nil {
		settings = s.settings.Get(ctx)
	}
	overrides := map[string]model.ScoreWeights{}
	if s.settings != nil {
		overrides = s.settings.SymbolOverrides(ctx)
	}

	rows := make([]repository.StockScoreRow, 0, len(inputs))
	peerScores := map[string][]float64{}
	for _, in := range inputs {
		row := buildScoreRow(in, normalized[in.symbol], settings, overrides, tradeDate)
		rows = append(rows, row)
		if row.Score.Valid {
			peerScores[in.peer] = append(peerScores[in.peer], float64(row.Score.Int64))
		}
	}

	assignPeerPercentiles(rows, peerScores)
	dist = summarizeDistribution(rows)

	if err := s.scores.UpsertBatch(ctx, rows); err != nil {
		return dist, fmt.Errorf("store: %w", err)
	}
	if n, err := s.scores.SyncDailyCloseScores(ctx, tradeDate); err != nil {
		// Non-fatal: the scores are stored and the API serves them. Only the
		// mirrored column on the 일별 종가 rows is stale.
		log.Printf("score: sync daily closes %s: %v", tradeDate, err)
	} else {
		log.Printf("score: mirrored onto %d 일별 종가 rows", n)
	}
	// 추천 목록의 거래대금 하한이 읽는 값. 못 채워도 점수는 서 있고, 랭킹 사전계산이
	// 시작할 때 한 번 더 채운다(`EnsureTradeValue`).
	if n, err := s.scores.FillAvgTradeValue(ctx, tradeDate); err != nil {
		log.Printf("score: avg trade value %s: %v", tradeDate, err)
	} else {
		log.Printf("score: avg trade value on %d rows", n)
	}
	log.Printf("score: %s — %d rows (%d scored, mean %.1f sd %.1f) in %s; healthy=%v %v",
		tradeDate, len(rows), dist.Count, dist.Mean, dist.StdDev,
		time.Since(start).Round(time.Millisecond), dist.Healthy, dist.Warnings)
	return dist, nil
}

/*
scoreCloseLookbackDays — 가치 축이 쓰는 종가를 기준일에서 며칠까지 거슬러 찾나.

**기준일 당일 종가만 붙이면 가치 축이 날마다 생겼다 사라진다** (2026-09-23 실측).
점수 배치는 00:25 KST에 돌고 미국 종가는 05:20 KST에 적재되므로, 미국 종목은 그날
종가가 대개 없다 — 그러면 PER·PBR·PSR이 통째로 빠지고 분모가 줄어 점수가 튄다.
30거래일 이력에서 가치 축의 유무가 **81,124번** 바뀌었고(하루 약 2,800종목), 축
유무가 바뀐 비교는 전체의 19%인데 **점수 절대 변동의 38%, 5점 이상 급변의 44%**를
만들었다. AAPL은 가치 축이 격일로 생겼다 사라지며 61↔73을 오갔다.

그래서 **기준일 이전의 가장 최근 종가**를 쓴다. 밸류에이션은 「지금 가격 대비」이고
그 가격은 마지막 거래일 종가다 — 휴장일이나 적재 전이라고 가격이 사라진 것이 아니다.
10일은 추석·설 연휴(최장 5영업일 안팎)를 덮고, 그보다 오래된 종가는 거래정지에 가까워
가치 평가에 쓰지 않는다. 쓴 종가의 날짜는 `inputs.closeDate`에 남긴다.
*/
const scoreCloseLookbackDays = 10

// loadUniverse pulls every active symbol with whatever fundamentals and the
// most recent close on or before tradeDate — one query, LEFT JOINed so missing
// data stays NULL rather than dropping the symbol.
func (s *ScoreService) loadUniverse(ctx context.Context, tradeDate string) ([]*scoreInput, error) {
	const q = `
		SELECT s.symbol, s.name, s.asset_type, s.market,
		       COALESCE(f.shares_outstanding, s.shares_outstanding) AS shares,
		       f.roe, f.operating_margin, f.net_profit_margin, f.debt_ratio, f.current_ratio,
		       f.revenue_growth_yoy, f.eps_growth_yoy,
		       f.eps_diluted, f.eps_basic, f.net_income, f.total_equity, f.revenue,
		       d.close_price, DATE_FORMAT(d.trade_date, '%Y-%m-%d'),
		       COALESCE(ss.sector, ''),
		       x.per, x.pbr, x.psr, x.roe, x.operating_margin, x.net_margin,
		       x.revenue_growth, x.eps_growth, x.current_ratio
		  FROM stocks s
		  LEFT JOIN stock_fundamentals f ON f.symbol = s.symbol
		  LEFT JOIN stock_sectors ss ON ss.symbol = s.symbol
		  LEFT JOIN stock_ext_metrics x ON x.symbol = s.symbol AND s.asset_type = 'us'
		        AND x.fetched_at >= DATE_SUB(?, INTERVAL 21 DAY)
		  LEFT JOIN stock_daily_closes d ON d.symbol = s.symbol AND d.trade_date = (
		    SELECT MAX(dc.trade_date) FROM stock_daily_closes dc
		     WHERE dc.symbol = s.symbol
		       AND dc.trade_date <= ?
		       AND dc.trade_date >= DATE_SUB(?, INTERVAL ? DAY))
		 WHERE s.status = 'NORMAL'`

	rows, err := s.db.QueryContext(ctx, q, tradeDate, tradeDate, tradeDate, scoreCloseLookbackDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*scoreInput
	for rows.Next() {
		var (
			in                              scoreInput
			shares, equity, revenue         sql.NullFloat64
			epsDiluted, epsBasic, netIncome sql.NullFloat64
			closePrice                      sql.NullFloat64
			closeDate                       sql.NullString
			name, assetType, market         sql.NullString
			ext                             [9]*float64
		)
		if err := rows.Scan(
			&in.symbol, &name, &assetType, &market, &shares,
			&in.roe, &in.opMargin, &in.netMargin, &in.debtRatio, &in.currentRatio,
			&in.revGrowth, &in.epsGrowth,
			&epsDiluted, &epsBasic, &netIncome, &equity, &revenue, &closePrice, &closeDate,
			&in.sector,
			&ext[0], &ext[1], &ext[2], &ext[3], &ext[4], &ext[5], &ext[6], &ext[7], &ext[8],
		); err != nil {
			return nil, err
		}
		in.name = name.String
		in.assetType = assetType.String
		in.market = market.String
		if shares.Valid {
			in.shares = shares.Float64
		}
		if closePrice.Valid {
			in.close = closePrice.Float64
			in.closeDate = closeDate.String
		}
		if in.close > 0 && in.shares > 0 {
			in.marketCap = in.close * in.shares
		}

		// Valuation multiples are derived here rather than stored: PER/PBR
		// depend on price, which moves daily. Using the latest close (not
		// symbol_last_prices, which covers only ~1.1k symbols) is what makes
		// this axis reachable for thousands of symbols instead of hundreds.
		if in.close > 0 {
			if eps, ok := scoreEPS(epsDiluted, epsBasic, netIncome, in.shares); ok && eps > 0 {
				v := in.close / eps
				in.per = &v
			}
			if in.marketCap > 0 {
				if equity.Valid && equity.Float64 > 0 {
					v := in.marketCap / equity.Float64
					in.pbr = &v
				}
				if revenue.Valid && revenue.Float64 > 0 {
					v := in.marketCap / revenue.Float64
					in.psr = &v
				}
			}
		}
		in.applyExtMetrics(ext)
		out = append(out, &in)
	}
	return out, rows.Err()
}

/*
applyExtMetrics — EDGAR가 **비운 칸만** Finnhub 보조로 채운다(2026-09-28 사용자 결정 「해외 전 종목 보조」).

순서가 중요하다: EDGAR 값이 있으면 절대 덮지 않는다 — 같은 종목의 값이 날마다 출처를 오가면
점수가 출처 차이만큼 흔들린다. 보조 값은 본국 상장 기준 비율이라 ADR(TSM·SONY)에도 맞는다.
ext 순서: per, pbr, psr, roe, operating_margin, net_margin, revenue_growth, eps_growth, current_ratio.
*/
func (in *scoreInput) applyExtMetrics(ext [9]*float64) {
	fill := func(dst **float64, v *float64) {
		if *dst == nil && v != nil {
			x := *v
			*dst = &x
			in.extUsed = true
		}
	}
	fill(&in.per, ext[0])
	fill(&in.pbr, ext[1])
	fill(&in.psr, ext[2])
	fill(&in.roe, ext[3])
	fill(&in.opMargin, ext[4])
	fill(&in.netMargin, ext[5])
	fill(&in.revGrowth, ext[6])
	fill(&in.epsGrowth, ext[7])
	fill(&in.currentRatio, ext[8])
}

/*
scoreEPS — PER의 분모.

**화면과 같은 순서로 고른다** (`fundamentals_calc.go`의 `ComputeValuationMetrics`
+ `enrichDerivedEPS`): 희석 EPS → 기본 EPS → 없으면 순이익 ÷ 발행주식수.

세 번째 갈래가 없어서 생긴 문제(2026-09-03 사용자 지적 §7): 점수 쪽은 `eps_basic`
컬럼만 봤다. 그런데 DART/EDGAR 원문에 주당순이익이 아예 없는 경우가 흔해서, 화면의
밸류에이션 탭은 순이익으로 되짚은 PER을 멀쩡히 보여 주는데 **가치 축은 그 종목의
PER을 아예 못 본 채** PBR·PSR 둘로만 매겨지고 있었다. 같은 종목·같은 화면에서 한쪽은
PER을 알고 한쪽은 모르는 상태였던 셈이다.

EPS가 음수(적자)면 PER은 뜻이 없으므로 부르는 쪽이 `> 0`으로 거른다 — 그때 가치
축은 PBR·PSR로 간다. 「PER 없음」과 「PER 나쁨」은 다른 말이다.
*/
func scoreEPS(diluted, basic, netIncome sql.NullFloat64, shares float64) (float64, bool) {
	if diluted.Valid && diluted.Float64 != 0 {
		return diluted.Float64, true
	}
	if basic.Valid && basic.Float64 != 0 {
		return basic.Float64, true
	}
	if netIncome.Valid && shares > 0 {
		return netIncome.Float64 / shares, true
	}
	return 0, false
}

// scoreTechnicalBars caps how much history is kept per symbol: SMA200 plus a
// 52-week window needs ~252 bars, so anything older cannot change the result.
const scoreTechnicalBars = 260

/*
attachTechnicals streams every daily bar once, ordered by symbol, computing
indicators as each symbol's run ends. Memory stays bounded to one symbol.

**채점하는 날짜까지만 읽는다.** 예전에는 날짜 조건이 없어 `bar_interval='1d'`인 봉을
전부 읽었다. 매일 도는 배치에서는 티가 안 났지만(오늘 채점하면 오늘까지가 맞다), 지난
날짜를 다시 계산하면 **그날 몰랐던 봉으로 그날 점수를 매겼다** — 08-27을 오늘 다시
돌리면 08-28·08-31 봉이 섞여 들어간다. 그러면 재계산 결과와 원래 저장된 값을 나란히
놓고 비교할 수가 없고, 채점 날짜라는 인자가 사실상 종가 조인에만 쓰이게 된다.

PK가 (symbol, bar_interval, bar_time)이라 조건이 붙어도 훑는 순서와 정렬은 그대로다.
*/
func (s *ScoreService) attachTechnicals(ctx context.Context, inputs []*scoreInput, tradeDate string) error {
	bySymbol := make(map[string]*scoreInput, len(inputs))
	for _, in := range inputs {
		bySymbol[in.symbol] = in
	}

	// Ordering by the raw column is safe here because 012 migrated bar_time to
	// DATETIME; the legacy string comparison path no longer applies.
	const q = `
		SELECT symbol, open_price, high_price, low_price, close_price, volume
		  FROM candle_bars
		 WHERE bar_interval = '1d'
		   AND bar_time < DATE_ADD(?, INTERVAL 1 DAY)
		 ORDER BY symbol, bar_time`

	rows, err := s.db.QueryContext(ctx, q, tradeDate)
	if err != nil {
		return err
	}
	defer rows.Close()

	current := ""
	var buf []model.Candle
	flush := func() {
		if current == "" || len(buf) == 0 {
			return
		}
		if in, ok := bySymbol[current]; ok {
			in.barCount = len(buf)
			in.ti = ComputeTechnicalIndicators(buf)
		}
		buf = buf[:0]
	}

	for rows.Next() {
		var (
			symbol                 string
			open, high, low, close sql.NullFloat64
			volume                 sql.NullInt64
		)
		if err := rows.Scan(&symbol, &open, &high, &low, &close, &volume); err != nil {
			return err
		}
		if symbol != current {
			flush()
			current = symbol
		}
		if !close.Valid || close.Float64 <= 0 {
			continue
		}
		buf = append(buf, model.Candle{
			Open:   open.Float64,
			High:   high.Float64,
			Low:    low.Float64,
			Close:  close.Float64,
			Volume: volume.Int64,
		})
		if len(buf) > scoreTechnicalBars {
			buf = buf[len(buf)-scoreTechnicalBars:]
		}
	}
	flush()
	return rows.Err()
}

/*
assignPeerGroups keys each symbol to (assetType:market:capTercile). Sector
would be the right third axis but no sector column exists in the schema, so
market cap is the best available refinement (§8 of the design doc).

**어제 속했던 그룹을 이어 받는다** (`prevPeer`). 시총은 주가만큼 매일 움직이고 분위
경계선도 그날 분포에서 다시 잡히므로, 경계 근처 종목은 아무 일이 없어도 하루걸러
cap2↔cap3를 오간다. 시총을 못 받은 날에는 분위가 통째로 빠져 `us:NASDAQ`와
`us:NASDAQ:cap1` 사이를 왕복한다. 실측 08-27→08-28: **17,913 중 9,936(55%)이 소속
변경**. 그룹이 바뀌면 백분위가 바뀌고, 백분위가 바뀌면 재무제표가 그대로여도 수익성이
5.1점, 성장이 3.9점씩 움직인다 — 종합 점수가 하루에 평균 6.2점 뛰던 몫의 큰 부분이다.

`prevPeer`가 비면(첫 실행·보관 기간 밖) 예전과 똑같이 그날 값으로만 정한다.
*/
func assignPeerGroups(inputs []*scoreInput, prevPeer map[string]string) {
	capsByMarket := map[string][]float64{}
	for _, in := range inputs {
		base := in.assetType + ":" + in.market
		if in.marketCap > 0 {
			capsByMarket[base] = append(capsByMarket[base], in.marketCap)
		}
	}
	for k := range capsByMarket {
		sort.Float64s(capsByMarket[k])
	}
	for _, in := range inputs {
		base := in.assetType + ":" + in.market
		tercile := stickyCapTercile(
			capsByMarket[base], in.marketCap, prevTercileFor(prevPeer[in.symbol], base),
		)
		if tercile == 0 {
			in.peer = base
			continue
		}
		in.peer = fmt.Sprintf("%s:cap%d", base, tercile)
	}
	collapseSmallPeers(inputs)
}

/*
prevTercileFor는 저장된 그룹 문자열에서 분위 번호를 꺼낸다.

**같은 base일 때만** 유효하다. 종목이 시장을 옮기거나(상장 이전) 자산군이 바뀌면
어제의 분위는 다른 분포에서 매긴 값이라 이어 붙일 근거가 없다. 분위가 없던 그룹
(`us:NASDAQ`)은 0을 돌려주고, 그러면 아래 stickyCapTercile이 오늘 값으로 새로 정한다.
*/
func prevTercileFor(prev, base string) int {
	if prev == "" || !strings.HasPrefix(prev, base+":cap") {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimPrefix(prev, base+":cap"))
	if err != nil || n < 1 || n > 3 {
		return 0
	}
	return n
}

/*
stickyCapTerciles는 분위를 **바꿀 이유가 분명할 때만** 바꾼다.

두 가지를 막는다:

  - **경계 근처의 왕복.** 어제 cap2였는데 오늘 계산이 cap3를 준다면, 시총이 경계를
    `capTercileHysteresis`만큼 넘어섰을 때만 옮긴다. 경계에서 1%쯤 오르내리는 종목이
    매일 다른 무리에 섞이는 일을 없앤다.
  - **시총 결측으로 인한 이탈.** 오늘 시총을 못 받았다고 어제 있던 분위를 버리면,
    그 종목은 전 시장(수천 종목)을 상대로 다시 줄 세워진다. 값이 없는 날은 어제
    자리에 그대로 둔다 — 모른다는 것이 "다른 무리"라는 뜻은 아니다.
*/
func stickyCapTercile(sortedCaps []float64, cap float64, prev int) int {
	fresh := capTercile(sortedCaps, cap)
	if prev < 1 || prev > 3 {
		return fresh
	}
	// 오늘 값을 못 정했다(시총 결측·표본 부족) → 어제 자리를 지킨다.
	if fresh == 0 {
		return prev
	}
	if fresh == prev {
		return prev
	}

	n := len(sortedCaps)
	if n < 30 {
		return prev
	}
	t1, t2 := sortedCaps[n/3], sortedCaps[2*n/3]

	// 올라가려면 넘어야 할 선을 여유만큼 넘어야 하고, 내려가려면 여유만큼 내려가야 한다.
	switch {
	case fresh > prev:
		boundary := t1
		if prev == 2 {
			boundary = t2
		}
		if cap > boundary*(1+capTercileHysteresis) {
			return fresh
		}
	case fresh < prev:
		boundary := t2
		if prev == 2 {
			boundary = t1
		}
		if cap < boundary*(1-capTercileHysteresis) {
			return fresh
		}
	}
	return prev
}

/*
분위를 옮기려면 경계에서 이만큼은 벗어나 있어야 한다.

10%는 하루 등락으로는 좀처럼 닿지 않으면서(경계 근처 왕복을 없앤다) 몇 주에 걸친
실제 체급 변화는 막지 않는 폭이다. 등급 문자(`ScoreGrade`)에 밴드를 넓게 잡아 한
점 차이로 글자가 안 바뀌게 한 것과 같은 생각이다.
*/
const capTercileHysteresis = 0.10

// minPeerSize is the smallest group whose percentiles mean anything. Below it
// the group is folded back into its market so a symbol is not ranked against
// three neighbours.
const minPeerSize = 30

func collapseSmallPeers(inputs []*scoreInput) {
	counts := map[string]int{}
	for _, in := range inputs {
		counts[in.peer]++
	}
	for _, in := range inputs {
		if counts[in.peer] >= minPeerSize {
			continue
		}
		fallback := in.assetType + ":" + in.market
		in.peer = fallback
	}
	// Second pass: a whole market may still be too small (e.g. KONEX).
	counts = map[string]int{}
	for _, in := range inputs {
		counts[in.peer]++
	}
	for _, in := range inputs {
		if counts[in.peer] < minPeerSize {
			in.peer = in.assetType
		}
	}
}

// normalizedMetrics holds one symbol's 0–100 values keyed by metric name.
type normalizedMetrics map[string]float64

// metricSpec declares how one raw field participates in scoring.
type metricSpec struct {
	name           string
	axis           model.ScoreAxis
	higherIsBetter bool
	// lo/hi bound the plausible domain. Values outside it are treated as
	// MISSING, not as extreme.
	//
	// This is the guard that winsorizing cannot provide. Under rank
	// normalization, clipping a value to p99 still leaves it ranked first, so
	// the measured ROE of 227,162% would take the top percentile — a data
	// artifact rewarded as the best company in its peer group. Values that far
	// outside the domain are broken inputs (near-zero denominators, unit
	// mix-ups), and the honest reading of a broken input is "unknown".
	lo, hi  float64
	extract func(*scoreInput) *float64
}

// value applies the domain guard, returning nil when the figure cannot be
// trusted.
func (m metricSpec) value(in *scoreInput) *float64 {
	v := m.extract(in)
	if v == nil {
		return nil
	}
	f := *v
	if math.IsNaN(f) || math.IsInf(f, 0) || f < m.lo || f > m.hi {
		return nil
	}
	return &f
}

// scoreMetrics is the full metric catalogue. Adding a metric here is the only
// change needed for it to flow into normalization, the axis average, and the
// stored inputs blob.
//
// Domains are set from the measured universe (docs/STOCK_SCORE_DESIGN.md §2.2)
// wide enough to keep every genuine reading and tight enough to reject the
// artifacts.
var scoreMetrics = []metricSpec{
	{"roe", model.AxisQuality, true, -200, 200, func(i *scoreInput) *float64 { return i.roe }},
	{"operatingMargin", model.AxisQuality, true, -500, 100, func(i *scoreInput) *float64 { return i.opMargin }},
	{"netProfitMargin", model.AxisQuality, true, -500, 100, func(i *scoreInput) *float64 { return i.netMargin }},
	// Lower debt is better, but a NEGATIVE debt ratio means negative equity
	// (자본잠식) — a severe warning that inversion would otherwise score as the
	// safest balance sheet in the group. Excluded by the domain floor.
	{"debtRatio", model.AxisQuality, false, 0, 10000, func(i *scoreInput) *float64 { return i.debtRatio }},
	{"currentRatio", model.AxisQuality, true, 0, 100, func(i *scoreInput) *float64 { return i.currentRatio }},

	{"revenueGrowthYoY", model.AxisGrowth, true, -100, 1000, func(i *scoreInput) *float64 { return i.revGrowth }},
	{"epsGrowthYoY", model.AxisGrowth, true, -1000, 1000, func(i *scoreInput) *float64 { return i.epsGrowth }},

	{"per", model.AxisValuation, false, 0, 1000, func(i *scoreInput) *float64 { return i.per }},
	{"pbr", model.AxisValuation, false, 0, 100, func(i *scoreInput) *float64 { return i.pbr }},
	{"psr", model.AxisValuation, false, 0, 100, func(i *scoreInput) *float64 { return i.psr }},

	// The technical reading is ranked like every other metric, in two stages:
	// the absolute curves (RSI peaking below overbought, MACD vs signal, price
	// vs its moving averages) decide what a good setup looks like, then the
	// rank makes it comparable across the peer group.
	//
	// Ranking it matters for the composite's shape. Left absolute, the curves
	// sit above centre by construction — a neutral RSI of 50 scores 68 — which
	// dragged the whole universe's mean to 53.1 and tripped the batch's own
	// distribution gate.
	{"technicalComposite", model.AxisTechnical, true, 0, 100, func(i *scoreInput) *float64 { return i.techRaw }},
}

// minMetricSamples is the smallest group a single metric may be ranked in.
//
// Group size alone is not enough. A peer group can hold hundreds of symbols
// while only two or three of them carry a ROE, and ranking among three gives
// percentiles of exactly 0/50/100 — which is how 삼성전자 first came out with a
// growth score of 0 despite +33% EPS growth. The threshold is checked per
// metric, on the symbols that actually have that figure.
const minMetricSamples = 20

// peerLevels is the fallback chain, finest first. A metric is ranked at the
// most specific level that has enough real samples, so a sparsely covered
// figure is compared market-wide rather than against two neighbours.
//
// **재무 지표(수익성·성장·가치)는 섹터가 맨 앞이다** (2026-09-23 사용자 결정). PER 40은
// 반도체에서 평범하고 은행에서 비정상이다 — 시총 3분위로만 줄 세우면 가치 축이 업종을
// 판별한다. 추세는 업종과 무관하므로 기존 사슬 그대로다. 섹터 그룹에 그 지표의 표본이
// `minMetricSamples`보다 적으면 아래 단계로 내려가므로, 얇은 섹터는 자연스럽게 기존
// 그룹에서 매겨진다.
func peerLevels(in *scoreInput, fundamental bool) []string {
	levels := make([]string, 0, 5)
	if fundamental && in.sectorPeer != "" {
		levels = append(levels, in.sectorPeer)
	}
	return append(levels,
		in.peer,
		in.assetType+":"+in.market,
		in.assetType,
		"*",
	)
}

/*
sectorPeerMinCoverage — 섹터 그룹을 켜는 자산유형별 문턱.

섹터 배치는 로테이션이라 며칠에 걸쳐 채워진다(sector_ingest.go). 반쯤 채워진 날
섹터 그룹을 켜면 **섹터가 있는 종목만** 섹터 안에서, 나머지는 시총 그룹에서 매겨져
같은 날 같은 지표가 두 잣대로 섞인다. 재무 지표가 있는 종목의 80%가 섹터를 가질
때까지는 그 자산유형 전체를 기존 그룹으로 둔다. 문턱을 넘는 날 점수가 한 번 크게
움직이고, 그 이동은 설명 엔진이 「비교 집단이 바뀌었다」로 보여 줄 수 있다.
*/
const sectorPeerMinCoverage = 0.8

/*
scoreMethodVersion — 점수 **산출 방식**의 판. 방식을 바꾸면 이 값을 올린다.

모든 점수 행의 `inputs.method`에 남고, 변화 감지가 두 날의 판이 다르면 그 점수 변화를
「회사에 일어난 일」에서 뺀다(change_detect.go). 2026-09-23 실측: 심리 축 원천을 LLM
요약에서 기사 라벨로 바꾸고 섹터 피어를 켠 날, 축도 비교 집단도 그대로인데 ±8 이상
움직인 종목이 614개였다 — 전부 잣대가 움직인 것이다.

섹터 피어를 쓰는지(종목별)도 판에 붙인다 — 커버리지가 문턱을 넘는 날 자동으로 켜지므로
코드를 안 바꿔도 방식이 바뀐다.
*/
// 2026-10-10 — 국내 비율 TTM(stock_score_ttm.go). TTM 종목의 값이 움직이면 남의 백분위도 움직인다.
const scoreMethodVersion = "2026-10-10"

/*
method — 판 + 종목별로 자동으로 켜지는 전환. 수급은 **적재가 닿은 날** 그 종목에 켜진다
(신규 상장·첫 백필) — 코드가 그대로여도 그 종목의 잣대가 바뀌므로 판에 붙인다.
*/
func (in *scoreInput) method() string {
	m := scoreMethodVersion
	if in.sectorPeer != "" {
		m += "|sector"
	}
	if in.flowPct != nil {
		m += "|flow"
	}
	if in.ttmPeriod != "" {
		m += "|ttm" // 분기 행이 4개 이어지는 날부터 그 종목의 잣대가 연간 → TTM으로 바뀐다
	}
	if in.extUsed {
		m += "|ext" // Finnhub 보조가 채운 날부터 그 종목의 잣대가 바뀐다
	}
	if in.redditSent != nil {
		m += "|reddit" // ApeWisdom 요약이 잡힌 날만 — 상위 150 안팎을 드나들면 판도 드나든다
	}
	return m
}

type sectorCoverage struct {
	pct     float64
	enabled bool
}

// assignSectorPeers sets sectorPeer for asset types whose sector coverage —
// among symbols that carry at least one fundamental figure — passes the gate.
func assignSectorPeers(inputs []*scoreInput) map[string]sectorCoverage {
	type tally struct{ total, withSector int }
	counts := map[string]*tally{}
	for _, in := range inputs {
		if !in.fundamentalsApplicable() || !in.hasFundamentals() {
			continue
		}
		t := counts[in.assetType]
		if t == nil {
			t = &tally{}
			counts[in.assetType] = t
		}
		t.total++
		if in.sector != "" {
			t.withSector++
		}
	}
	out := map[string]sectorCoverage{}
	for asset, t := range counts {
		pct := 0.0
		if t.total > 0 {
			pct = float64(t.withSector) / float64(t.total)
		}
		out[asset] = sectorCoverage{pct: pct, enabled: pct >= sectorPeerMinCoverage}
	}
	for _, in := range inputs {
		in.sectorPeer = ""
		if in.sector != "" && out[in.assetType].enabled {
			in.sectorPeer = in.assetType + ":" + in.sector
		}
	}
	return out
}

// hasFundamentals reports whether any quality/growth/valuation figure exists.
func (in *scoreInput) hasFundamentals() bool {
	for _, spec := range scoreMetrics {
		if spec.axis != model.AxisTechnical && spec.value(in) != nil {
			return true
		}
	}
	return false
}

// normalizeUniverse ranks every metric, choosing the finest peer level that
// holds at least minMetricSamples readings of that metric.
func normalizeUniverse(inputs []*scoreInput) map[string]normalizedMetrics {
	out := make(map[string]normalizedMetrics, len(inputs))
	bySymbol := make(map[string]*scoreInput, len(inputs))
	for _, in := range inputs {
		out[in.symbol] = normalizedMetrics{}
		in.metricLevels = map[string]string{}
		bySymbol[in.symbol] = in
	}

	for _, spec := range scoreMetrics {
		buckets := map[string][]metricSample{}
		levelsBySymbol := map[string][]string{}

		for _, in := range inputs {
			// ETFs have no financial statements; ranking them on ROE would
			// invent a comparison that does not exist.
			if spec.axis != model.AxisTechnical && !in.fundamentalsApplicable() {
				continue
			}
			v := spec.value(in)
			if v == nil {
				continue
			}
			levels := peerLevels(in, spec.axis != model.AxisTechnical)
			levelsBySymbol[in.symbol] = levels
			sample := metricSample{symbol: in.symbol, value: *v}
			for _, key := range levels {
				buckets[key] = append(buckets[key], sample)
			}
		}

		ranked := map[string]map[string]float64{}
		for key, samples := range buckets {
			// "*" is the last resort and is always ranked, however thin.
			if len(samples) < minMetricSamples && key != "*" {
				continue
			}
			ranked[key] = percentileRanks(samples, spec.higherIsBetter)
		}

		for symbol, levels := range levelsBySymbol {
			for _, key := range levels {
				r, ok := ranked[key]
				if !ok {
					continue
				}
				pct, ok := r[symbol]
				if !ok {
					continue
				}
				out[symbol][spec.name] = pct
				// Recorded so a surprising percentile can be traced to the
				// group it was actually measured against.
				if in, ok := bySymbol[symbol]; ok {
					in.metricLevels[spec.name] = key
				}
				break
			}
		}
	}
	return out
}

// technicalAxis reuses the existing indicator curves (already unit-tested) and
// reports how complete the reading was.
//
// Confidence matters because the measured universe is dominated by symbols
// holding only ~40 daily bars: without it, "RSI only" would masquerade as a
// full technical assessment.
func technicalAxis(in *scoreInput) (score *float64, confidence float64) {
	ti := in.ti
	if ti == nil || in.barCount < 15 {
		return nil, 0
	}
	var parts []scoreComponent
	if ti.RSI14 != nil {
		parts = append(parts, scoreComponent{value: rsiScore(*ti.RSI14), weight: 0.35})
	}
	if ti.MACD != nil && ti.MACDSignal != nil {
		parts = append(parts, scoreComponent{value: macdScore(*ti.MACD, *ti.MACDSignal), weight: 0.25})
	}
	if ti.LastClose != nil && ti.SMA20 != nil {
		sma50 := *ti.SMA20
		if ti.SMA50 != nil {
			sma50 = *ti.SMA50
		}
		parts = append(parts, scoreComponent{
			value:  trendScore(*ti.LastClose, *ti.SMA20, sma50),
			weight: 0.25,
		})
	}
	if ti.PctFromLow52Week != nil && ti.PctFromHigh52Week != nil && in.barCount >= 200 {
		parts = append(parts, scoreComponent{
			value:  rangePositionScore(*ti.PctFromLow52Week, *ti.PctFromHigh52Week),
			weight: 0.15,
		})
	}
	if len(parts) == 0 {
		return nil, 0
	}

	var weighted, total float64
	for _, p := range parts {
		weighted += p.value * p.weight
		total += p.weight
	}
	v := clamp01to100(weighted / total)

	switch {
	case in.barCount >= 200:
		confidence = 1.0
	case in.barCount >= 60:
		confidence = 0.9
	case in.barCount >= 35:
		confidence = 0.8
	default:
		confidence = 0.5
	}
	return &v, confidence
}

// buildScoreRow turns one symbol's normalized metrics into the stored row.
func buildScoreRow(
	in *scoreInput,
	metrics normalizedMetrics,
	settings model.ScoreSettings,
	overrides map[string]model.ScoreWeights,
	tradeDate string,
) repository.StockScoreRow {
	breakdown := model.ScoreBreakdown{}
	confidence := map[model.ScoreAxis]float64{}

	for _, axis := range []model.ScoreAxis{
		model.AxisTechnical, model.AxisQuality, model.AxisGrowth, model.AxisValuation,
	} {
		var parts []float64
		for _, spec := range scoreMetrics {
			if spec.axis != axis {
				continue
			}
			if v, ok := metrics[spec.name]; ok {
				parts = append(parts, v)
			}
		}
		if v := axisScore(parts); v != nil {
			breakdown.Set(axis, v)
			// The technical axis carries the completeness of its own reading;
			// the statement-derived axes are either present or absent.
			if axis == model.AxisTechnical {
				confidence[axis] = in.techConf
			} else {
				confidence[axis] = 1.0
			}
		}
	}
	/*
		심리·수급 — **이 배치가 전 종목에 매긴다** (2026-09-23).

		예전에는 여기서 비워 두고 AI 요약이 다녀간 종목에만 뒤늦게 얹었다 — 그날 심리
		축이 있는 종목이 9개였고, 00:25에 지워졌다가 08:00에 붙는 탓에 축이 날마다
		생겼다 사라졌다(30거래일 8,444번). 이제 재료가 표에 있다: 기사 라벨(로컬 감성
		모델, `news_article_sentiment`)과 내부자 거래(`insider_transactions`).
		`attachSentiment` 참고. 재료가 없으면 nil — 0이 아니다.
	*/
	if in.fundamentalsApplicable() && in.sentScore != nil {
		breakdown.Sentiment = in.sentScore
		confidence[model.AxisSentiment] = in.sentConf
	}

	weights := settings.Weights
	weightSource := "global"
	if w, ok := overrides[in.symbol]; ok {
		weights = w
		weightSource = "symbol"
	}

	var weighted, usedWeight, totalWeight float64
	for _, axis := range model.ScoreAxes {
		w := float64(weights.Get(axis))
		totalWeight += w
		v := breakdown.Get(axis)
		if v == nil || w <= 0 {
			continue
		}
		weighted += *v * w
		usedWeight += w
	}

	row := repository.StockScoreRow{
		Symbol:       in.symbol,
		TradeDate:    tradeDate,
		ScoreKind:    "composite",
		PeerGroup:    in.peer,
		WeightSource: weightSource,
	}
	if !in.fundamentalsApplicable() {
		row.ScoreKind = "technical_only"
	}

	if usedWeight <= 0 || totalWeight <= 0 {
		row.Breakdown = mustJSON(breakdown)
		row.Inputs = mustJSON(collectInputs(in, metrics))
		return row
	}

	// Coverage weights each available axis by how complete it was, so a
	// technical axis built from RSI alone counts for less than a full one.
	var covered float64
	for _, axis := range model.ScoreAxes {
		w := float64(weights.Get(axis))
		if breakdown.Get(axis) == nil || w <= 0 {
			continue
		}
		conf := confidence[axis]
		if conf <= 0 {
			conf = 1
		}
		covered += w * conf
	}
	coverage := covered / totalWeight
	row.Coverage = coverage

	if coverage < settings.CoverageFloor {
		row.Breakdown = mustJSON(breakdown)
		row.Inputs = mustJSON(collectInputs(in, metrics))
		return row
	}

	raw := weighted / usedWeight
	final := applyShrink(raw, shrinkFactor(coverage, settings.ShrinkFull))
	score := int(clamp01to100(final) + 0.5)

	row.Score = sql.NullInt64{Int64: int64(score), Valid: true}
	row.Grade = model.ScoreGrade(score)
	row.Breakdown = mustJSON(breakdown)
	row.Inputs = mustJSON(collectInputs(in, metrics))
	return row
}

// collectInputs records the real figures behind the score so a surprising
// number can be audited after the fact rather than argued about.
func collectInputs(in *scoreInput, metrics normalizedMetrics) map[string]any {
	out := map[string]any{
		"barCount":  in.barCount,
		"peerGroup": in.peer,
		"method":    in.method(),
	}
	if in.close > 0 {
		out["close"] = in.close
		if in.closeDate != "" {
			out["closeDate"] = in.closeDate
		}
	}
	if in.ttmPeriod != "" {
		out["ttmPeriod"] = in.ttmPeriod
	}
	if in.marketCap > 0 {
		out["marketCap"] = in.marketCap
	}
	if in.sentScore != nil && in.fundamentalsApplicable() {
		// 키 이름은 예전 요약 경로와 같다 — 설명 엔진이 이 둘을 읽는다.
		out["sentiment"] = math.Round(*in.sentScore*10) / 10
		out["sentimentConfidence"] = math.Round(in.sentConf*100) / 100
		out["newsCount"] = in.newsCount
		if in.flowPct != nil {
			out["flowNetPct"] = math.Round(*in.flowPct*100) / 100
		}
		if in.redditSent != nil {
			out["redditSentiment"] = math.Round(*in.redditSent*10) / 10
		}
	}
	raw := map[string]any{}
	rejected := map[string]any{}
	for _, spec := range scoreMetrics {
		reported := spec.extract(in)
		if reported == nil {
			continue
		}
		if spec.value(in) != nil {
			raw[spec.name] = *reported
			continue
		}
		// Kept visible rather than dropped silently: "why is this company's
		// quality score missing" should be answerable from the stored row.
		rejected[spec.name] = *reported
	}
	if len(rejected) > 0 {
		out["rejected"] = rejected
	}
	if len(raw) > 0 {
		out["raw"] = raw
	}
	if len(metrics) > 0 {
		out["percentile"] = metrics
	}
	if len(in.metricLevels) > 0 {
		out["rankedIn"] = in.metricLevels
	}
	if in.ti != nil {
		out["technical"] = in.ti
	}
	return out
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// assignPeerPercentiles ranks each scored symbol inside its own peer group so
// the UI can say "상위 14%" instead of only showing a bare number.
//
// The stored value is a 상위 N% — lower is better — and model.ReadPeerPct floors
// it at 1: the rank below puts the single best symbol of a group at the 100th
// percentile, which inverts to 0, and 「동종 상위 0%」 is not something anyone
// says. On 09-08 twenty of the top 100 US scores read that way.
func assignPeerPercentiles(rows []repository.StockScoreRow, peerScores map[string][]float64) {
	for k := range peerScores {
		sort.Float64s(peerScores[k])
	}
	for i := range rows {
		if !rows[i].Score.Valid {
			continue
		}
		scores := peerScores[rows[i].PeerGroup]
		if len(scores) < 2 {
			continue
		}
		v := float64(rows[i].Score.Int64)
		idx := sort.SearchFloat64s(scores, v)
		pct := int(float64(idx)/float64(len(scores)-1)*100 + 0.5)
		if pct > 100 {
			pct = 100
		}
		// Report as "top N%".
		top := model.ReadPeerPct(100 - pct)
		rows[i].PeerPct = sql.NullInt64{Int64: int64(top), Valid: true}
		// 「N개 중 M위」(2026-10-10, 종목 화면 A안) — 상위 %만으로는 그룹이 몇 개인지 모른다.
		// 같은 점수는 같은 순위(위에 있는 종목 수 + 1).
		above := len(scores) - sort.SearchFloat64s(scores, v+0.5)
		rows[i].Inputs = withPeerRank(rows[i].Inputs, above+1, len(scores))
	}
}

// withPeerRank adds peerRank/peerSize to the stored inputs JSON. 읽지 못하면 원래 값을 둔다.
func withPeerRank(inputs string, rank, size int) string {
	m := map[string]any{}
	if inputs != "" {
		if err := json.Unmarshal([]byte(inputs), &m); err != nil {
			return inputs
		}
	}
	m["peerRank"] = rank
	m["peerSize"] = size
	b, err := json.Marshal(m)
	if err != nil {
		return inputs
	}
	return string(b)
}

// summarizeDistribution is the batch's own gate (design §7.1). A run whose
// shape is wrong should be visible immediately, not discovered by a user.
func summarizeDistribution(rows []repository.StockScoreRow) model.ScoreDistribution {
	var dist model.ScoreDistribution
	values := make([]float64, 0, len(rows))
	dist.Min = 100
	for _, r := range rows {
		if !r.Score.Valid {
			dist.Withheld++
			continue
		}
		v := int(r.Score.Int64)
		values = append(values, float64(v))
		if v < dist.Min {
			dist.Min = v
		}
		if v > dist.Max {
			dist.Max = v
		}
		if v <= 1 {
			dist.AtFloor++
		}
		if v >= 99 {
			dist.AtCeil++
		}
	}
	dist.Count = len(values)
	if dist.Count == 0 {
		dist.Min = 0
		dist.Warnings = append(dist.Warnings, "점수가 산출된 종목이 없습니다")
		return dist
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	dist.Mean = sum / float64(dist.Count)
	var sq float64
	for _, v := range values {
		sq += (v - dist.Mean) * (v - dist.Mean)
	}
	dist.StdDev = math.Sqrt(sq / float64(dist.Count))

	if dist.Mean < 47 || dist.Mean > 53 {
		dist.Warnings = append(dist.Warnings, fmt.Sprintf("평균 %.1f — 기대 범위 50±3 이탈", dist.Mean))
	}
	if dist.StdDev < 8 || dist.StdDev > 22 {
		dist.Warnings = append(dist.Warnings, fmt.Sprintf("표준편차 %.1f — 기대 범위 8~22 이탈", dist.StdDev))
	}
	if float64(dist.AtFloor)/float64(dist.Count) > 0.005 {
		dist.Warnings = append(dist.Warnings, fmt.Sprintf("0점 부근 %d종목 — 0.5%% 초과", dist.AtFloor))
	}
	if float64(dist.AtCeil)/float64(dist.Count) > 0.005 {
		dist.Warnings = append(dist.Warnings, fmt.Sprintf("100점 부근 %d종목 — 0.5%% 초과", dist.AtCeil))
	}
	dist.Healthy = len(dist.Warnings) == 0
	return dist
}

// RecomputeIndices runs the index score pass. Split out from Recompute (stock
// side) so cmd/worker-scores can run both from its own daily loop without this
// package importing a scheduler of its own — the loop now lives in that
// worker, not here, since scoring runs as its own process (see
// docs/archive/sessions.md "배치 잡을 독립 워커 프로세스로 분리").
func (s *ScoreService) RecomputeIndices(ctx context.Context) (int, error) {
	if s == nil || s.indexScores == nil {
		return 0, nil
	}
	return s.indexScores.RecomputeAll(ctx)
}

// LatestTradeDate is the newest date the close projection holds, which is the
// date the batch should score.
func (s *ScoreService) LatestTradeDate(ctx context.Context) (string, error) {
	var d sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT DATE_FORMAT(MAX(trade_date), '%Y-%m-%d') FROM stock_daily_closes`).Scan(&d)
	if err != nil {
		return "", err
	}
	if !d.Valid || d.String == "" {
		return "", fmt.Errorf("no daily closes stored")
	}
	return d.String, nil
}

// History returns a symbol's recent scores for the trend view.
func (s *ScoreService) History(ctx context.Context, symbol string, limit int) ([]model.StockScoreView, error) {
	if s == nil || s.scores == nil {
		return nil, nil
	}
	return s.scores.History(ctx, strings.ToUpper(strings.TrimSpace(symbol)), limit)
}

// domesticAssetTypes / overseasAssetTypes split stocks.asset_type into the
// two market tabs the 종목 랭킹 list offers. ETFs trade like US names in this
// app (Toss-sourced, USD-quoted), so they group with overseas.
var (
	domesticAssetTypes = []string{string(model.AssetDomestic)}
	overseasAssetTypes = []string{string(model.AssetUS), string(model.AssetETF)}
)

// TopScores returns the score leaderboard for one market ("domestic" or
// "overseas"), scored as of the most recent date the batch actually ran —
// today's if it has run, otherwise the last date that has scores.
func (s *ScoreService) TopScores(ctx context.Context, market string, limit int) ([]model.StockScoreListItem, string, error) {
	if s == nil || s.scores == nil {
		return nil, "", fmt.Errorf("score service not configured")
	}
	tradeDate, err := s.LatestScoredDate(ctx)
	if err != nil {
		return nil, "", err
	}
	if tradeDate == "" {
		return []model.StockScoreListItem{}, "", nil
	}

	assetTypes := domesticAssetTypes
	if strings.EqualFold(market, "overseas") {
		assetTypes = overseasAssetTypes
	}

	items, err := s.scores.TopScores(ctx, tradeDate, assetTypes, limit)
	if err != nil {
		return nil, "", err
	}
	if items == nil {
		items = []model.StockScoreListItem{}
	}
	s.overlayLiveChanges(ctx, items)
	return items, tradeDate, nil
}

/*
overlayLiveChanges — 장이 열려 있는 동안에는 확정 종가 대신 지금 값을 보여준다.

랭킹 자체는 하루 한 번 도는 배치의 결과지만, 그 옆의 등락률은 「어제 이랬다」가
아니라 「지금 이렇다」여야 한다 — 화면의 다른 시세 칸이 전부 그렇게 움직인다.
애프터마켓까지 끝난 뒤에는 `LiveChanges`가 아무것도 주지 않으므로 종가가 그대로
남는다(그때는 실시간 값이라는 것이 곧 종가다).

시세를 못 구한 행은 **손대지 않는다.** 목록에서 그 종목만 조용히 어제 값으로
남는 편이, 절반은 실시간이고 절반은 빈칸인 목록보다 낫다.
*/
func (s *ScoreService) overlayLiveChanges(ctx context.Context, items []model.StockScoreListItem) {
	if s == nil || len(items) == 0 {
		return
	}
	/*
		판정은 `list_price_overlay.go` 한 곳에서 온다 — **스크리너도 같은 함수를 쓴다.**

		예전에는 이 규칙이 여기에만 있어서 같은 종목이 랭킹에서는 실시간 등락률로,
		스크리너에서는 어제 종가 등락률로 보였다 (2026-09-23 QA P0-01).
	*/
	in := make([]ListPriceInput, len(items))
	for i, it := range items {
		in[i] = ListPriceInput{
			Symbol:    it.Symbol,
			AssetType: it.AssetType,
			Close:     it.ClosePrice,
			PrevClose: it.PrevClose,
			PriceAsOf: it.PriceAsOf,
		}
	}
	for i, d := range s.ResolveListPrices(ctx, in) {
		if d.BasePrice > 0 {
			base := d.BasePrice
			items[i].BasePrice = &base
		}
		if !d.Live {
			continue
		}
		rate := d.ChangeRate
		price := d.Price
		items[i].ChangeRate = &rate
		items[i].ClosePrice = &price
		if d.Currency != "" {
			items[i].Currency = d.Currency
		}
		items[i].PriceAsOf = model.ScorePriceAsOfLive
	}
}

// LatestScoredDate is the newest date the score batch actually wrote.
// EnsureTradeValue fills inputs.avgTradeValue on tradeDate's rows that lack it
// (the recommendable-size floor reads it). Idempotent; see FillAvgTradeValue.
func (s *ScoreService) EnsureTradeValue(ctx context.Context, tradeDate string) (int64, error) {
	if s == nil || s.scores == nil {
		return 0, nil
	}
	return s.scores.FillAvgTradeValue(ctx, tradeDate)
}

func (s *ScoreService) LatestScoredDate(ctx context.Context) (string, error) {
	var d sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT DATE_FORMAT(MAX(trade_date), '%Y-%m-%d') FROM stock_scores`).Scan(&d)
	if err != nil {
		return "", err
	}
	return d.String, nil
}

// Stats proxies the repo's per-date counts for the admin dashboard.
func (s *ScoreService) Stats(ctx context.Context, tradeDate string) (total, scored int, err error) {
	if s == nil || s.scores == nil {
		return 0, 0, nil
	}
	return s.scores.Stats(ctx, tradeDate)
}

// GetMany returns the latest score for each requested symbol in one round trip.
// Symbols with nothing scored are simply absent — the caller renders "—" for
// them, same as Get returning nil.
func (s *ScoreService) GetMany(ctx context.Context, symbols []string) ([]model.StockScoreView, error) {
	if s == nil || s.scores == nil {
		return nil, fmt.Errorf("score service not configured")
	}
	views, err := s.scores.LatestBatch(ctx, symbols)
	if err != nil || len(views) == 0 {
		return nil, err
	}
	settings := model.DefaultScoreSettings()
	if s.settings != nil {
		settings = s.settings.Get(ctx)
	}
	var overrides map[string]model.ScoreWeights
	if s.settings != nil {
		overrides = s.settings.SymbolOverrides(ctx)
	}
	for i := range views {
		views[i].Weights = settings.Weights
		views[i].WeightSource = "global"
		if w, ok := overrides[strings.ToUpper(views[i].Symbol)]; ok {
			views[i].Weights = w
			views[i].WeightSource = "symbol"
		}
	}
	return views, nil
}

// Get returns one symbol's most recent score for the public API.
func (s *ScoreService) Get(ctx context.Context, symbol string) (*model.StockScoreView, error) {
	if s == nil || s.scores == nil {
		return nil, fmt.Errorf("score service not configured")
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	view, err := s.scores.Latest(ctx, symbol)
	if err != nil || view == nil {
		return nil, err
	}
	settings := model.DefaultScoreSettings()
	if s.settings != nil {
		settings = s.settings.Get(ctx)
	}
	view.Weights = settings.Weights
	view.WeightSource = "global"
	if s.settings != nil {
		if w, ok := s.settings.SymbolOverrides(ctx)[symbol]; ok {
			view.Weights = w
			view.WeightSource = "symbol"
		}
	}
	return view, nil
}

/*
ScoredDates — 점수가 남아 있는 거래일 전부, 오래된 것부터.

랭킹 이력 백필이 쓴다. `stock_scores`는 최근 30거래일만 보관하므로 이 목록은 짧고
(실측 26일), 그것이 이 기능이 첫날에 확보할 수 있는 과거의 전부다.
*/
func (s *ScoreService) ScoredDates(ctx context.Context) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("score service not configured")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT DATE_FORMAT(trade_date, '%Y-%m-%d')
		   FROM stock_scores GROUP BY trade_date ORDER BY trade_date`)
	if err != nil {
		return nil, fmt.Errorf("scored dates: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err != nil {
			return nil, err
		}
		out = append(out, date)
	}
	return out, rows.Err()
}

/*
sentimentNewsWindow · sentimentInsiderWindow — 심리 축이 보는 창.

뉴스 14일: 하루 기사로 매기면 한 건짜리 호재가 축을 흔든다. 두 주면 대형주는 수백 건,
소형주도 몇 건은 모인다. 내부자 180일: 국내 소유보고는 분기에 한 번꼴이라 짧으면 늘 빈다.
*/
const (
	sentimentNewsWindow    = 14 * 24 * time.Hour
	sentimentInsiderWindow = 180 * 24 * time.Hour
	// sentimentFlowWindowSessions — 수급은 달력이 아니라 적재된 거래일 20개(약 한 달).
	sentimentFlowWindowSessions = 20
)

// attachSentiment fills sentScore/sentConf for every symbol with material.
// It reads two aggregates (one GROUP BY each) — no per-symbol query.
func (s *ScoreService) attachSentiment(ctx context.Context, inputs []*scoreInput, tradeDate string) (int, error) {
	now := time.Now()
	news, err := repository.NewNewsSentimentRepo(s.db).SymbolSentimentSince(ctx, now.Add(-sentimentNewsWindow))
	if err != nil {
		return 0, err
	}
	insider, err := repository.NewInsiderRepo(s.db).InsiderNetSharesSince(ctx, now.Add(-sentimentInsiderWindow))
	if err != nil {
		return 0, err
	}
	/*
		수급은 국내 보통주만 있다. 못 읽어도 뉴스·내부자로 축을 매긴다 — 재료 하나가 빠진
		것이지 축이 없는 것은 아니다.
	*/
	flows, err := repository.NewInvestorFlowRepo(s.db).FlowWindow(ctx, tradeDate, sentimentFlowWindowSessions)
	if err != nil {
		log.Printf("score: investor flows unavailable, sentiment without 수급: %v", err)
		flows = nil
	}
	// Reddit(미국 종목만, 2026-09-29) — ApeWisdom 종목 요약이 있는 종목에만 성분이 붙는다.
	from, to := socialSnapWindow(tradeDate)
	reddit, err := repository.NewExternalSignalRepo(s.db).SocialInsightsBetween(ctx, from, to)
	if err != nil {
		log.Printf("score: reddit insights unavailable: %v", err)
		reddit = nil
	}
	n := 0
	for _, in := range inputs {
		if !in.fundamentalsApplicable() {
			continue
		}
		nt := news[in.symbol]
		newsV, newsC, hasNews := newsComponentFromTally(nt.Total, nt.AvgScore)
		ins := insider[in.symbol]
		insV, insC, hasIns := insiderComponentFromTotals(ins.Buy, ins.Sell, ins.Counted)
		fl := flows[in.symbol]
		flowV, flowC, hasFlow := flowComponent(fl.Net, fl.Volume, fl.Days)
		var redV, redC float64
		var hasRed bool
		if ins, found := reddit[in.symbol]; found && in.assetType == string(model.AssetUS) {
			redV, redC, hasRed = redditSentimentComponent(&ins)
		}
		v, conf, ok := sentimentAxisParts(
			sentimentPart{newsV, newsC, sentimentNewsWeight, hasNews},
			sentimentPart{insV, insC, sentimentInsiderWeight, hasIns},
			sentimentPart{flowV, flowC, sentimentFlowWeight, hasFlow},
			sentimentPart{redV, redC, sentimentRedditWeight, hasRed},
		)
		if !ok {
			continue
		}
		if hasRed {
			r := redV
			in.redditSent = &r
		}
		in.sentScore = &v
		in.sentConf = conf
		in.newsCount = nt.Total
		if hasFlow {
			pct := float64(fl.Net) / float64(fl.Volume) * 100
			in.flowPct = &pct
		}
		n++
	}
	return n, nil
}
