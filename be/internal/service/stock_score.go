package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
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
}

// SetIndexScores wires index scoring into the same daily run.
func (s *ScoreService) SetIndexScores(svc *IndexScoreService) { s.indexScores = svc }

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

	ti       *TechnicalIndicators
	barCount int

	// techRaw is the absolute technical reading (RSI/MACD/trend curves) before
	// cross-sectional ranking; techConf is how complete that reading was.
	techRaw  *float64
	techConf float64

	peer string
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

	techStart := time.Now()
	if err := s.attachTechnicals(ctx, inputs); err != nil {
		return dist, fmt.Errorf("technicals: %w", err)
	}
	log.Printf("score: technicals in %s", time.Since(techStart).Round(time.Millisecond))

	for _, in := range inputs {
		in.techRaw, in.techConf = technicalAxis(in)
	}
	assignPeerGroups(inputs)
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
	log.Printf("score: %s — %d rows (%d scored, mean %.1f sd %.1f) in %s; healthy=%v %v",
		tradeDate, len(rows), dist.Count, dist.Mean, dist.StdDev,
		time.Since(start).Round(time.Millisecond), dist.Healthy, dist.Warnings)
	return dist, nil
}

// loadUniverse pulls every active symbol with whatever fundamentals and that
// day's close exist — one query, LEFT JOINed so missing data stays NULL rather
// than dropping the symbol.
func (s *ScoreService) loadUniverse(ctx context.Context, tradeDate string) ([]*scoreInput, error) {
	const q = `
		SELECT s.symbol, s.name, s.asset_type, s.market,
		       COALESCE(f.shares_outstanding, s.shares_outstanding) AS shares,
		       f.roe, f.operating_margin, f.net_profit_margin, f.debt_ratio, f.current_ratio,
		       f.revenue_growth_yoy, f.eps_growth_yoy,
		       f.eps_basic, f.total_equity, f.revenue,
		       d.close_price
		  FROM stocks s
		  LEFT JOIN stock_fundamentals f ON f.symbol = s.symbol
		  LEFT JOIN stock_daily_closes d ON d.symbol = s.symbol AND d.trade_date = ?
		 WHERE s.status = 'NORMAL'`

	rows, err := s.db.QueryContext(ctx, q, tradeDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*scoreInput
	for rows.Next() {
		var (
			in                          scoreInput
			shares, eps, equity, revenue sql.NullFloat64
			closePrice                  sql.NullFloat64
			name, assetType, market     sql.NullString
		)
		if err := rows.Scan(
			&in.symbol, &name, &assetType, &market, &shares,
			&in.roe, &in.opMargin, &in.netMargin, &in.debtRatio, &in.currentRatio,
			&in.revGrowth, &in.epsGrowth,
			&eps, &equity, &revenue, &closePrice,
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
		}
		if in.close > 0 && in.shares > 0 {
			in.marketCap = in.close * in.shares
		}

		// Valuation multiples are derived here rather than stored: PER/PBR
		// depend on price, which moves daily. Using that day's close (not
		// symbol_last_prices, which covers only ~1.1k symbols) is what makes
		// this axis reachable for thousands of symbols instead of hundreds.
		if in.close > 0 {
			if eps.Valid && eps.Float64 > 0 {
				v := in.close / eps.Float64
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
		out = append(out, &in)
	}
	return out, rows.Err()
}

// scoreTechnicalBars caps how much history is kept per symbol: SMA200 plus a
// 52-week window needs ~252 bars, so anything older cannot change the result.
const scoreTechnicalBars = 260

// attachTechnicals streams every daily bar once, ordered by symbol, computing
// indicators as each symbol's run ends. Memory stays bounded to one symbol.
func (s *ScoreService) attachTechnicals(ctx context.Context, inputs []*scoreInput) error {
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
		 ORDER BY symbol, bar_time`

	rows, err := s.db.QueryContext(ctx, q)
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

// assignPeerGroups keys each symbol to (assetType:market:capTercile). Sector
// would be the right third axis but no sector column exists in the schema, so
// market cap is the best available refinement (§8 of the design doc).
func assignPeerGroups(inputs []*scoreInput) {
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
		tercile := capTercile(capsByMarket[base], in.marketCap)
		if tercile == 0 {
			in.peer = base
			continue
		}
		in.peer = fmt.Sprintf("%s:cap%d", base, tercile)
	}
	collapseSmallPeers(inputs)
}

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
func peerLevels(in *scoreInput) []string {
	return []string{
		in.peer,
		in.assetType + ":" + in.market,
		in.assetType,
		"*",
	}
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
			levels := peerLevels(in)
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
	// Sentiment stays unset in batch on purpose: news sentiment comes from the
	// LLM and insider data from live DART/EDGAR calls, neither of which can run
	// across 17,901 symbols. It is applied as a bounded on-demand adjustment.

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
	}
	if in.close > 0 {
		out["close"] = in.close
	}
	if in.marketCap > 0 {
		out["marketCap"] = in.marketCap
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
		top := 100 - pct
		rows[i].PeerPct = sql.NullInt64{Int64: int64(top), Valid: true}
	}
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
// docs/AGENT_STATE.md "배치 잡을 독립 워커 프로세스로 분리").
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
	return items, tradeDate, nil
}

// LatestScoredDate is the newest date the score batch actually wrote.
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
