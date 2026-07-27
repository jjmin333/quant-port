package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

const portfolioStatsCachePrefix = "portfolio:stats:v8:"
const portfolioPurchaseCachePrefix = "portfolio:purchase:v2:"

var kst = time.FixedZone("KST", 9*3600)

type holdingStatsCache struct {
	PrevClose              float64 `json:"prevClose"`
	RegularClose           float64 `json:"regularClose"`
	MarketSession          string  `json:"marketSession"`
	PurchaseDate           string  `json:"purchaseDate"`
	SincePurchaseHigh     float64 `json:"sincePurchaseHigh"`
	SincePurchaseHighDate string  `json:"sincePurchaseHighDate"`
	SincePurchaseLow      float64 `json:"sincePurchaseLow"`
	SincePurchaseLowDate  string  `json:"sincePurchaseLowDate"`
}

func (s *PortfolioService) enrichHoldingsStats(
	ctx context.Context,
	client *toss.Client,
	accounts []toss.Account,
	holdings []model.UnifiedHolding,
) {
	cals := s.loadMarketCalendars(ctx, client)
	now := time.Now()

	for i := range holdings {
		stats := s.loadOrFetchHoldingStats(ctx, client, accounts, &holdings[i], cals, now)
		applyHoldingStats(&holdings[i], stats)
		time.Sleep(120 * time.Millisecond)
	}
}

func applyHoldingStats(h *model.UnifiedHolding, stats holdingStatsCache) {
	h.PrevClose = stats.PrevClose
	h.RegularClose = stats.RegularClose
	h.MarketSession = stats.MarketSession
	h.PurchaseDate = stats.PurchaseDate
	h.SincePurchaseHigh = stats.SincePurchaseHigh
	h.SincePurchaseHighDate = stats.SincePurchaseHighDate
	h.SincePurchaseLow = stats.SincePurchaseLow
	h.SincePurchaseLowDate = stats.SincePurchaseLowDate

	if h.AvgPrice > 0 && h.SincePurchaseHigh > 0 && h.SincePurchaseHigh < h.AvgPrice {
		h.SincePurchaseHigh = h.AvgPrice
	}

	updateSessionBreakdown(h)
}

func (s *PortfolioService) loadHoldingStatsCache(ctx context.Context, symbol string) holdingStatsCache {
	if s.redis == nil {
		return holdingStatsCache{}
	}
	cacheKey := portfolioStatsCachePrefix + strings.ToUpper(strings.TrimSpace(symbol))
	raw, err := s.redis.Get(ctx, cacheKey).Bytes()
	if err != nil || len(raw) == 0 {
		return holdingStatsCache{}
	}
	var stats holdingStatsCache
	if json.Unmarshal(raw, &stats) != nil || stats.PrevClose <= 0 {
		return holdingStatsCache{}
	}
	return stats
}

func (s *PortfolioService) loadOrFetchHoldingStats(
	ctx context.Context,
	client *toss.Client,
	accounts []toss.Account,
	h *model.UnifiedHolding,
	cals marketCalendars,
	now time.Time,
) holdingStatsCache {
	if cached := s.loadHoldingStatsCache(ctx, h.Symbol); cached.PrevClose > 0 {
		return cached
	}

	stats, err := s.fetchHoldingStats(ctx, client, accounts, h, cals, now)
	if err != nil || stats.PrevClose <= 0 {
		return holdingStatsCache{}
	}

	if s.redis != nil {
		if payload, err := json.Marshal(stats); err == nil {
			cacheKey := portfolioStatsCachePrefix + h.Symbol
			_ = s.redis.Set(ctx, cacheKey, payload, time.Duration(model.PortfolioStatsCacheSec)*time.Second).Err()
		}
	}
	return stats
}

type orderFill struct {
	at    time.Time
	side  toss.OrderSide
	qty   float64
	price float64 // per-share fill price
}

func (s *PortfolioService) fetchHoldingStats(
	ctx context.Context,
	client *toss.Client,
	accounts []toss.Account,
	h *model.UnifiedHolding,
	cals marketCalendars,
	now time.Time,
) (holdingStatsCache, error) {
	stats := holdingStatsCache{}

	prevClose, err := fetchPrevClose(ctx, client, h.Symbol)
	if err != nil {
		return stats, err
	}
	stats.PrevClose = prevClose

	session := detectMarketSession(h.AssetType, cals, now)
	stats.MarketSession = session
	regularClose, _ := fetchTodayRegularClose(ctx, client, h.Symbol, h.AssetType, cals, now, session)
	stats.RegularClose = regularClose

	fills, err := s.collectOrderFills(ctx, client, accounts, h.Symbol)
	if err != nil || len(fills) == 0 {
		return stats, nil
	}

	holdStart, ok := inferCurrentHoldStart(fills, h.TotalShares)
	if !ok {
		return stats, nil
	}
	stats.PurchaseDate = formatDateKST(holdStart)

	buyFills := currentHoldBuyFills(fills, h.TotalShares)

	high, highAt, low, lowAt, err := scanSinceHoldPriceRange(
		ctx,
		client,
		h.Symbol,
		holdStart,
		h.AvgPrice,
		h.CurrentPrice,
		buyFills,
	)
	if err != nil && high == 0 {
		return stats, nil
	}
	if high > 0 {
		stats.SincePurchaseHigh = normalizePerShareExtreme(high, h.AvgPrice, h.TotalShares)
		stats.SincePurchaseHighDate = formatStatTimestamp(highAt)
	}
	if low > 0 {
		stats.SincePurchaseLow = normalizePerShareExtreme(low, h.AvgPrice, h.TotalShares)
		stats.SincePurchaseLowDate = formatStatTimestamp(lowAt)
	}
	return stats, nil
}

func fetchPrevClose(ctx context.Context, client *toss.Client, symbol string) (float64, error) {
	adjusted := true
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*200) * time.Millisecond)
		}
		page, err := client.GetCandles(ctx, toss.GetCandlesParams{
			Symbol:   symbol,
			Interval: toss.CandleInterval1d,
			Count:    2,
			Adjusted: &adjusted,
		})
		if err != nil {
			lastErr = err
			continue
		}
		if page == nil || len(page.Candles) == 0 {
			lastErr = fmt.Errorf("no candles for %s", symbol)
			continue
		}
		if len(page.Candles) >= 2 {
			return parseDecimal(page.Candles[1].ClosePrice), nil
		}
		return parseDecimal(page.Candles[0].ClosePrice), nil
	}
	return 0, lastErr
}

// scanSinceHoldPriceRange measures high/low from exact purchase time.
// Strategy: 1m candles for purchase day (from fill time) and today; daily candles for full days in between.
func scanSinceHoldPriceRange(
	ctx context.Context,
	client *toss.Client,
	symbol string,
	since time.Time,
	avgPrice float64,
	currentPrice float64,
	buyFills []orderFill,
) (high float64, highAt time.Time, low float64, lowAt time.Time, err error) {
	var tracker priceRangeTracker
	now := time.Now()

	tracker.applyPurchaseBaselines(since, avgPrice, buyFills)

	if currentPrice > 0 {
		tracker.consider(now, currentPrice)
	}

	sinceLocal := since.In(kst)
	nowLocal := now.In(kst)
	purchaseDay := startOfDayKST(sinceLocal)
	today := startOfDayKST(nowLocal)

	if today.Equal(purchaseDay) {
		candles, fetchErr := collectCandlesPaginated(ctx, client, symbol, toss.CandleInterval1m, since, now)
		if fetchErr != nil {
			err = fetchErr
		}
		tracker.considerCandlesSince(candles, since, time.Minute)
	} else {
		endPurchaseDay := endOfDayKST(purchaseDay)
		if candles, fetchErr := collectCandlesPaginated(ctx, client, symbol, toss.CandleInterval1m, since, endPurchaseDay); fetchErr != nil {
			err = fetchErr
		} else {
			tracker.considerCandlesSince(candles, since, time.Minute)
		}

		middleFrom := purchaseDay.AddDate(0, 0, 1)
		middleTo := today.AddDate(0, 0, -1)
		if !middleFrom.After(middleTo) {
			if candles, fetchErr := collectDailyCandlesBetween(ctx, client, symbol, middleFrom, middleTo); fetchErr != nil && err == nil {
				err = fetchErr
			} else {
				tracker.considerCandles(candles)
			}
		}

		if candles, fetchErr := collectCandlesPaginated(ctx, client, symbol, toss.CandleInterval1m, today, now); fetchErr != nil && err == nil {
			err = fetchErr
		} else {
			tracker.considerCandlesSince(candles, today, time.Minute)
		}
	}

	tracker.applyPurchaseBaselines(since, avgPrice, buyFills)

	if tracker.high == 0 || tracker.low == 0 {
		return 0, time.Time{}, 0, time.Time{}, err
	}
	return tracker.high, tracker.highAt, tracker.low, tracker.lowAt, err
}

type priceRangeTracker struct {
	high, low     float64
	highAt, lowAt time.Time
}

// applyPurchaseBaselines ensures post-hold high is at least what the holder paid.
func (t *priceRangeTracker) applyPurchaseBaselines(since time.Time, avgPrice float64, buyFills []orderFill) {
	if avgPrice > 0 {
		t.consider(since, avgPrice)
	}
	for _, f := range buyFills {
		if f.price > 0 {
			t.consider(f.at, f.price)
		}
	}
}

func currentHoldBuyFills(fills []orderFill, currentShares float64) []orderFill {
	const eps = 1e-4
	remaining := currentShares
	out := make([]orderFill, 0)

	for i := len(fills) - 1; i >= 0; i-- {
		f := fills[i]
		if f.side != toss.OrderSideBuy {
			continue
		}
		take := math.Min(remaining, f.qty)
		if take <= eps {
			continue
		}
		out = append(out, f)
		remaining -= take
		if remaining <= eps {
			break
		}
	}
	return out
}

func orderFillPrice(o toss.Order) float64 {
	if o.Execution.AverageFilledPrice != nil {
		if v := parseDecimal(*o.Execution.AverageFilledPrice); v > 0 {
			return v
		}
	}
	if o.Price != nil {
		if v := parseDecimal(*o.Price); v > 0 {
			return v
		}
	}
	if o.Execution.FilledAmount != nil {
		amt := parseDecimal(*o.Execution.FilledAmount)
		qty := parseDecimal(o.Execution.FilledQuantity)
		if qty > 0 && amt > 0 {
			return amt / qty
		}
	}
	return 0
}

func (t *priceRangeTracker) consider(at time.Time, price float64) {
	if price <= 0 {
		return
	}
	if price > t.high || t.high == 0 {
		t.high = price
		t.highAt = at
	}
	if price < t.low || t.low == 0 {
		t.low = price
		t.lowAt = at
	}
}

func (t *priceRangeTracker) considerOHLC(at time.Time, hi, lo float64) {
	if hi > t.high || t.high == 0 {
		t.high = hi
		t.highAt = at
	}
	if lo > 0 && (lo < t.low || t.low == 0) {
		t.low = lo
		t.lowAt = at
	}
}

func (t *priceRangeTracker) considerCandles(candles []toss.Candle) {
	t.considerCandlesSince(candles, time.Time{}, 0)
}

// considerCandlesSince applies OHLC after purchase time. For the candle containing
// the fill, only the close is used so pre-fill highs in the same minute are excluded.
func (t *priceRangeTracker) considerCandlesSince(candles []toss.Candle, since time.Time, interval time.Duration) {
	for _, c := range candles {
		ts, ok := parseCandleTime(c.Timestamp)
		if !ok {
			continue
		}
		if since.IsZero() || interval <= 0 {
			t.considerOHLC(ts, parseDecimal(c.HighPrice), parseDecimal(c.LowPrice))
			continue
		}

		candleEnd := ts.Add(interval)
		if !since.Before(candleEnd) {
			continue
		}
		if since.After(ts) {
			close := parseDecimal(c.ClosePrice)
			if close > 0 {
				t.consider(since, close)
			}
			continue
		}
		t.considerOHLC(ts, parseDecimal(c.HighPrice), parseDecimal(c.LowPrice))
	}
}

// resolvePerSharePrice prefers broker per-share quotes but corrects totals when qty > 1.
func resolvePerSharePrice(quoted, positionTotal, qty float64) float64 {
	if quoted <= 0 {
		if qty > 0 && positionTotal > 0 {
			return positionTotal / qty
		}
		return 0
	}
	if qty > 1 && positionTotal > 0 {
		perShareFromTotal := positionTotal / qty
		if math.Abs(quoted-positionTotal) <= positionTotal*0.02 {
			return perShareFromTotal
		}
		if quoted > positionTotal {
			return perShareFromTotal
		}
	}
	return quoted
}

// normalizePerShareExtreme converts position totals back to per-share when detected.
func normalizePerShareExtreme(value, avgPrice, totalShares float64) float64 {
	if value <= 0 || totalShares <= 1 || avgPrice <= 0 {
		return value
	}
	ratio := value / avgPrice
	if ratio >= totalShares*0.85 && ratio <= totalShares*1.15 {
		return value / totalShares
	}
	return value
}

func collectDailyCandlesBetween(
	ctx context.Context,
	client *toss.Client,
	symbol string,
	fromDay, toDay time.Time,
) ([]toss.Candle, error) {
	fromDay = startOfDayKST(fromDay)
	toDay = startOfDayKST(toDay)
	until := endOfDayKST(toDay)

	candles, err := collectCandlesPaginated(ctx, client, symbol, toss.CandleInterval1d, fromDay, until)
	if err != nil {
		return nil, err
	}

	filtered := make([]toss.Candle, 0, len(candles))
	for _, c := range candles {
		ts, ok := parseCandleTime(c.Timestamp)
		if !ok {
			continue
		}
		day := startOfDayKST(ts)
		if day.Before(fromDay) || day.After(toDay) {
			continue
		}
		filtered = append(filtered, c)
	}
	return filtered, nil
}

func collectCandlesPaginated(
	ctx context.Context,
	client *toss.Client,
	symbol string,
	interval toss.CandleInterval,
	since, until time.Time,
) ([]toss.Candle, error) {
	adjusted := true
	var collected []toss.Candle
	before := ""

	for page := 0; page < 30; page++ {
		params := toss.GetCandlesParams{
			Symbol:   symbol,
			Interval: interval,
			Count:    200,
			Adjusted: &adjusted,
		}
		if before != "" {
			params.Before = before
		}

		pageResp, err := fetchCandlesWithRetry(ctx, client, params)
		if err != nil {
			if len(collected) > 0 {
				return collected, nil
			}
			return collected, err
		}
		if pageResp == nil || len(pageResp.Candles) == 0 {
			break
		}

		done := false
		for _, c := range pageResp.Candles {
			ts, ok := parseCandleTime(c.Timestamp)
			if !ok {
				continue
			}
			if !until.IsZero() && ts.After(until) {
				continue
			}
			if ts.Before(since) {
				done = true
				break
			}
			collected = append(collected, c)
		}

		if done || pageResp.NextBefore == nil {
			break
		}
		before = *pageResp.NextBefore
		time.Sleep(120 * time.Millisecond)
	}
	return collected, nil
}

func fetchCandlesWithRetry(ctx context.Context, client *toss.Client, params toss.GetCandlesParams) (*toss.CandlePageResponse, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*300) * time.Millisecond)
		}
		pageResp, err := client.GetCandles(ctx, params)
		if err == nil {
			return pageResp, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func parseCandleTime(raw string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func startOfDayKST(t time.Time) time.Time {
	d := t.In(kst)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, kst)
}

func endOfDayKST(t time.Time) time.Time {
	return startOfDayKST(t).AddDate(0, 0, 1).Add(-time.Nanosecond)
}

func formatStatTimestamp(t time.Time) string {
	return t.In(kst).Format("2006-01-02T15:04:05+09:00")
}

func updateSinceHoldExtremes(h *model.UnifiedHolding) {
	if h.CurrentPrice <= 0 {
		return
	}
	now := formatStatTimestamp(time.Now())
	price := normalizePerShareExtreme(h.CurrentPrice, h.AvgPrice, h.TotalShares)

	if h.SincePurchaseHigh == 0 || price > h.SincePurchaseHigh {
		h.SincePurchaseHigh = price
		h.SincePurchaseHighDate = now
	}
	if h.AvgPrice > 0 && h.SincePurchaseHigh > 0 && h.SincePurchaseHigh < h.AvgPrice {
		h.SincePurchaseHigh = h.AvgPrice
	}
	if h.SincePurchaseLow == 0 || price < h.SincePurchaseLow {
		h.SincePurchaseLow = price
		h.SincePurchaseLowDate = now
	}
}

func (s *PortfolioService) resolveCurrentHoldStartDate(
	ctx context.Context,
	client *toss.Client,
	accounts []toss.Account,
	symbol string,
	currentShares float64,
) (time.Time, bool) {
	if currentShares <= 0 {
		return time.Time{}, false
	}

	cacheKey := fmt.Sprintf("%s%s:%.6f", portfolioPurchaseCachePrefix, symbol, currentShares)
	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Result(); err == nil && cached != "" {
			if t, err := time.Parse(time.RFC3339, cached); err == nil {
				return t, true
			}
		}
	}

	fills, err := s.collectOrderFills(ctx, client, accounts, symbol)
	if err != nil || len(fills) == 0 {
		return time.Time{}, false
	}

	start, ok := inferCurrentHoldStart(fills, currentShares)
	if ok && s.redis != nil {
		_ = s.redis.Set(ctx, cacheKey, start.Format(time.RFC3339), 24*time.Hour).Err()
	}
	return start, ok
}

func (s *PortfolioService) collectOrderFills(
	ctx context.Context,
	client *toss.Client,
	accounts []toss.Account,
	symbol string,
) ([]orderFill, error) {
	var fills []orderFill

	for _, acc := range accounts {
		cursor := ""
		for pages := 0; pages < 20; pages++ {
			page, err := client.GetOrders(ctx, acc.AccountSeq, toss.GetOrdersParams{
				Status: toss.OrderLifecycleClosed,
				Symbol: symbol,
				Limit:  100,
				Cursor: cursor,
			})
			if err != nil || page == nil {
				break
			}

			for _, o := range page.Orders {
				qty := parseDecimal(o.Execution.FilledQuantity)
				if qty <= 0 {
					continue
				}
				if !orderHasFill(o.Status) {
					continue
				}

				at, ok := orderFillTime(o)
				if !ok {
					continue
				}

				fills = append(fills, orderFill{
					at:    at,
					side:  o.Side,
					qty:   qty,
					price: orderFillPrice(o),
				})
			}

			if !page.HasNext || page.NextCursor == nil {
				break
			}
			cursor = *page.NextCursor
		}
	}

	sort.Slice(fills, func(i, j int) bool {
		if fills[i].at.Equal(fills[j].at) {
			// Process sells before buys at the same timestamp so a same-day round-trip resets correctly.
			if fills[i].side == fills[j].side {
				return false
			}
			return fills[i].side == toss.OrderSideSell
		}
		return fills[i].at.Before(fills[j].at)
	})

	return fills, nil
}

func orderHasFill(status toss.OrderStatus) bool {
	switch status {
	case toss.OrderStatusFilled,
		toss.OrderStatusPartialFilled,
		toss.OrderStatusCanceled,
		toss.OrderStatusRejected,
		toss.OrderStatusReplaced:
		return true
	default:
		return false
	}
}

func orderFillTime(o toss.Order) (time.Time, bool) {
	if o.Execution.FilledAt != nil && *o.Execution.FilledAt != "" {
		if t, err := time.Parse(time.RFC3339, *o.Execution.FilledAt); err == nil {
			return t, true
		}
	}
	if o.OrderedAt != "" {
		if t, err := time.Parse(time.RFC3339, o.OrderedAt); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// inferCurrentHoldStart finds when the current open position began.
// Forward simulation handles sell-then-rebuy; backward lot matching is the fallback.
func inferCurrentHoldStart(fills []orderFill, currentShares float64) (time.Time, bool) {
	const eps = 1e-4

	if start, ok := forwardHoldStart(fills, eps); ok {
		if qtyMatch(runningQtyAfterFills(fills), currentShares, eps) {
			return start, true
		}
	}

	return backwardHoldStart(fills, currentShares, eps)
}

func forwardHoldStart(fills []orderFill, eps float64) (time.Time, bool) {
	var (
		running     float64
		periodStart time.Time
		found       bool
	)

	for _, f := range fills {
		switch f.side {
		case toss.OrderSideBuy:
			if running <= eps {
				periodStart = f.at
				found = true
			}
			running += f.qty
		case toss.OrderSideSell:
			running -= f.qty
			if running <= eps {
				running = 0
				found = false
			}
		}
	}

	if !found || running <= eps {
		return time.Time{}, false
	}
	return periodStart, true
}

func backwardHoldStart(fills []orderFill, currentShares float64, eps float64) (time.Time, bool) {
	remaining := currentShares
	var start time.Time
	found := false

	for i := len(fills) - 1; i >= 0; i-- {
		f := fills[i]
		if f.side != toss.OrderSideBuy {
			continue
		}
		take := math.Min(remaining, f.qty)
		if take <= eps {
			continue
		}
		start = f.at
		found = true
		remaining -= take
		if remaining <= eps {
			return start, true
		}
	}

	return start, found && remaining <= eps
}

func runningQtyAfterFills(fills []orderFill) float64 {
	var running float64
	for _, f := range fills {
		switch f.side {
		case toss.OrderSideBuy:
			running += f.qty
		case toss.OrderSideSell:
			running -= f.qty
		}
	}
	return running
}

func qtyMatch(a, b, eps float64) bool {
	if a <= 0 || b <= 0 {
		return false
	}
	diff := math.Abs(a - b)
	tolerance := math.Max(eps, math.Min(a, b)*0.02) // allow 2% for fractional / rounding drift
	return diff <= tolerance
}

func formatDateKST(t time.Time) string {
	return t.In(kst).Format("2006-01-02")
}
