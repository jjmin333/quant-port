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
	"github.com/enact/quant-port/be/internal/stocklogo"
	"github.com/enact/quant-port/be/internal/toss"
)

const (
	quoteStatsCachePrefix = "quotes:stats:v3:"
)

var usEastern *time.Location

func init() {
	usEastern, _ = time.LoadLocation("America/New_York")
	if usEastern == nil {
		usEastern = time.FixedZone("EST", -5*3600)
	}
}

type quoteStatsCache struct {
	Open         float64 `json:"open"`
	High         float64 `json:"high"`
	Low          float64 `json:"low"`
	PrevClose    float64 `json:"prevClose"`
	Week52High   float64 `json:"week52High"`
	Week52Low    float64 `json:"week52Low"`
	MarketCap    float64 `json:"marketCap"`
	TradingValue float64 `json:"tradingValue"`
	Volume       int64   `json:"volume"`
}

func (s *QuotesService) buildQuoteDetailFromToss(ctx context.Context, symbol string) (model.QuoteDetail, error) {
	stock, err := s.stockInfo(ctx, symbol)
	if err != nil || stock == nil {
		return model.QuoteDetail{}, fmt.Errorf("stock %s: %w", symbol, err)
	}

	currentPrice := s.resolveBestLivePrice(ctx, symbol, 0, nil, nil)
	if currentPrice <= 0 && s.toss != nil {
		price, err := s.toss.GetPrice(ctx, symbol)
		if err == nil && price != nil {
			currentPrice = parseDecimal(price.LastPrice)
		}
	}
	if currentPrice <= 0 {
		return model.QuoteDetail{}, fmt.Errorf("price %s: unavailable", symbol)
	}

	stats := s.loadQuoteStats(ctx, symbol, stock, currentPrice)

	prevClose, ok := s.resolvePrevClose(ctx, symbol)
	if !ok || prevClose <= 0 {
		return model.QuoteDetail{}, fmt.Errorf("reference price %s: unavailable", symbol)
	}

	marketCap := stats.MarketCap
	if marketCap <= 0 && stock.SharesOutstanding != "" {
		shares := parseDecimal(stock.SharesOutstanding)
		if shares > 0 && currentPrice > 0 {
			marketCap = shares * currentPrice
		}
	}

	currency := modelCurrency(stock.Currency)
	updatedAt := time.Now().UTC().Format(time.RFC3339)

	detail := model.QuoteDetail{
		QuoteSummary: model.QuoteSummary{
			Symbol:       stock.Symbol,
			Name:         stock.Name,
			Market:       stock.Market,
			Currency:     currency,
			AssetType:    assetTypeFromMarket(stock.Market, stock.SecurityType),
			CurrentPrice: currentPrice,
			Volume:       stats.Volume,
			TradingValue: stats.TradingValue,
		},
		Open:       stats.Open,
		High:       stats.High,
		Low:        stats.Low,
		PrevClose:  prevClose,
		Week52High: stats.Week52High,
		Week52Low:  stats.Week52Low,
		MarketCap:  marketCap,
		UpdatedAt:  updatedAt,
	}
	s.applyQuoteChange(ctx, symbol, currentPrice, prevClose, &detail)

	stocklogo.ApplyToQuoteSummary(&detail.QuoteSummary)
	return detail, nil
}

func (s *QuotesService) loadQuoteStats(ctx context.Context, symbol string, stock *toss.StockInfo, currentPrice float64) quoteStatsCache {
	key := quoteStatsCachePrefix + strings.ToUpper(symbol)
	if s.redis != nil {
		if raw, err := s.redis.Get(ctx, key).Bytes(); err == nil && len(raw) > 0 {
			var cached quoteStatsCache
			if json.Unmarshal(raw, &cached) == nil && cached.PrevClose > 0 {
				return cached
			}
		}
	}

	if stats, ok := s.statsFromRawCache(ctx, symbol, stock, currentPrice); ok && stats.PrevClose > 0 {
		s.cacheSetJSON(ctx, key, stats, 60*time.Second)
		return stats
	}

	if s.stocks != nil {
		if dbStock, err := s.stocks.GetBySymbol(ctx, symbol); err == nil && dbStock != nil {
			if dbStats, _, ok := s.statsFromDBCandles(ctx, symbol, dbStock); ok && dbStats.PrevClose > 0 {
				s.cacheSetJSON(ctx, key, dbStats.quoteStatsCache, time.Duration(model.MarketDataLastGoodCacheSec)*time.Second)
				return dbStats.quoteStatsCache
			}
		}
	}

	s.triggerQuoteStatsRefresh(symbol)
	/*
		캐시가 빈 첫 요청도 닫힌 세션이면 **확정 일봉으로 바로 답한다**(DB 한 번, 상류 호출 없음).
		예전에는 여기서 빈 값을 돌려 첫 방문자(검색엔진 렌더러 포함)가 시가·고가·저가를 「—」로
		봤다. 52주 범위 등 나머지는 위에서 건 갱신이 채운다 — 그래서 이 값은 캐시하지 않는다.
	*/
	loc := kst
	if stock != nil && stock.Currency == toss.CurrencyUSD {
		loc = usEastern
	}
	if o, h, l, v, tv, ok := s.closedSessionDailyStats(ctx, symbol, s.quoteSessionStart(ctx, symbol, loc)); ok {
		return quoteStatsCache{Open: o, High: h, Low: l, Volume: v, TradingValue: tv}
	}
	return quoteStatsCache{}
}

func (s *QuotesService) fetchQuoteStats(ctx context.Context, symbol string, stock *toss.StockInfo, currentPrice float64) quoteStatsCache {
	stats := quoteStatsCache{}

	prevClose, ok := s.resolvePrevClose(ctx, symbol)
	if !ok && s.toss != nil {
		assetType := string(s.quoteAssetType(ctx, symbol))
		var cals marketCalendars
		if s.marketStore != nil {
			cals, _ = s.marketStore.EnsureCalendars(ctx)
		}
		var err error
		prevClose, err = fetchPrevClose(ctx, s.toss, symbol, assetType, cals)
		if err == nil && prevClose > 0 {
			cacheReferencePrice(ctx, s.redis, symbol, prevClose)
		}
	}
	if prevClose > 0 {
		stats.PrevClose = prevClose
	}

	open, high, low, volume, tradingValue := s.fetchIntradayStats(ctx, symbol, stock.Currency)
	stats.Open = open
	stats.High = high
	stats.Low = low
	stats.Volume = volume
	stats.TradingValue = tradingValue

	if stats.Open <= 0 && prevClose > 0 {
		stats.Open = prevClose
	}
	if stats.High <= 0 && currentPrice > 0 {
		stats.High = currentPrice
	}
	if stats.Low <= 0 && currentPrice > 0 {
		stats.Low = currentPrice
	}
	if stats.High > 0 && currentPrice > stats.High {
		stats.High = currentPrice
	}
	if stats.Low > 0 && currentPrice > 0 && currentPrice < stats.Low {
		stats.Low = currentPrice
	}

	if bars := s.ensureDailyBarsHistory(ctx, symbol, metricsDailyBarLimit); len(bars) > 0 {
		if weekHigh, weekLow, ok := week52RangeFromBars(bars); ok {
			stats.Week52High = weekHigh
			stats.Week52Low = weekLow
		}
	}

	if stock.SharesOutstanding != "" {
		shares := parseDecimal(stock.SharesOutstanding)
		if shares > 0 && currentPrice > 0 {
			stats.MarketCap = shares * currentPrice
		}
	}

	return stats
}

/*
quoteSessionStart — 시가·고가·저가를 모을 세션의 첫 순간(그 시장 시간대 0시).

**「오늘」이 아니라 현재가가 속한 세션이다** (2026-09-26 외부 방문자 점검). 예전에는
벽시계의 오늘 0시였다. 장이 닫힌 날(주말·휴장)과 평일 개장 전에는 오늘 봉이 하나도
없으니 아래 채움 규칙이 돌아 **시가 = 전일 종가, 고가 = 저가 = 현재가, 거래량 0**이라는
있지도 않은 하루가 만들어졌다(실측 추석 연휴 삼성전자: 시가 277,500 · 고가·저가 286,500
— 실제 9/23은 282,500 · 286,500 · 280,750). 캐시가 빈 순간에는 0이라 화면이 「—」였다.

`priceSessionDate`는 등락률의 기준가(직전 종가)를 고르는 쪽이 이미 쓰는 판정이다 —
장이 돌면 오늘, 닫혀 있으면 마지막으로 닫힌 세션. 같은 날짜를 쓰면 시가·고가·저가와
등락률이 같은 하루를 말한다.
*/
func (s *QuotesService) quoteSessionStart(ctx context.Context, symbol string, loc *time.Location) time.Time {
	if d, err := time.ParseInLocation("2006-01-02", s.priceSessionDate(ctx, symbol), loc); err == nil {
		return d
	}
	return startOfDay(time.Now().In(loc))
}

/*
closedSessionDailyStats — **장이 닫혀 있으면** 그 세션의 확정 일봉을 그대로 쓴다.

1분봉 합산은 장중에는 맞지만 닫힌 뒤에는 틀린다: 저장·캐시된 1분봉은 최근 480개(8시간)라
NXT까지 12시간을 도는 국내는 **정오 이후만** 남아 시가가 그 시각의 가격이 되고(추석 연휴
삼성전자: 시가 283,000 · 실제 282,500), 미국은 확장 시간대 봉만 있어 거래량이 **6만 주**로
나왔다(실제 정규장 2,164만 주). 확정 일봉에는 그런 구멍이 없다.

일봉 날짜는 저장 규약대로 bar_time의 날짜가 그 시장의 거래일이다(`prevCloseFromDailyBars`와 같다).
*/
func (s *QuotesService) closedSessionDailyStats(ctx context.Context, symbol string, sessionStart time.Time) (
	open, high, low float64, volume int64, tradingValue float64, ok bool,
) {
	c, found := s.closedSessionDailyBar(ctx, symbol, sessionStart)
	if !found || c.Open <= 0 || c.High <= 0 || c.Low <= 0 {
		return
	}
	return c.Open, c.High, c.Low, c.Volume, c.Close * float64(c.Volume), true
}

// closedSessionDailyBar — 장이 닫혀 있을 때 그 세션(sessionStart 날짜)의 확정 일봉.
func (s *QuotesService) closedSessionDailyBar(ctx context.Context, symbol string, sessionStart time.Time) (model.Candle, bool) {
	if s == nil || s.candles == nil || sessionStart.IsZero() {
		return model.Candle{}, false
	}
	if !s.sessionClosedNow(ctx, symbol) {
		return model.Candle{}, false
	}
	daily, _, err := s.candles.ListBefore(ctx, symbol, model.Interval1d, "", 3)
	if err != nil {
		return model.Candle{}, false
	}
	want := sessionStart.Format("2006-01-02")
	for i := len(daily) - 1; i >= 0; i-- {
		if parseQuoteCandleTime(daily[i].Time).In(kst).Format("2006-01-02") == want {
			return daily[i], true
		}
	}
	return model.Candle{}, false
}

// sessionClosedNow — 이 종목의 시장이 지금 닫혀 있는가(서버 달력 기준).
func (s *QuotesService) sessionClosedNow(ctx context.Context, symbol string) bool {
	var cals marketCalendars
	if s.marketStore != nil {
		cals, _ = s.marketStore.EnsureCalendars(ctx)
	}
	return marketClockAt(string(s.quoteAssetType(ctx, symbol)), cals, time.Now()).Closed()
}

// fetchIntradayStats computes the current session's open/high/low/volume/tradingValue.
// Tries DB-stored 1m bars first — refreshRawCandles already keeps these warm
// for every tracked symbol via its own delta fetch, so this can usually
// avoid hitting Toss at all. Only symbols with no tracked history yet (cold
// start) fall through to a live GetCandles(Count:480) call.
func (s *QuotesService) fetchIntradayStats(ctx context.Context, symbol string, currency toss.Currency) (
	open, high, low float64, volume int64, tradingValue float64,
) {
	loc := kst
	if currency == toss.CurrencyUSD {
		loc = usEastern
	}
	today := s.quoteSessionStart(ctx, symbol, loc)
	if o, h, l, v, tv, ok := s.closedSessionDailyStats(ctx, symbol, today); ok {
		return o, h, l, v, tv
	}

	if bars := s.listIntradayBarsFromDB(ctx, symbol, 480); len(bars) > 0 {
		if o, h, l, v, tv, ok := sumIntradayBarsFrom(bars, today); ok {
			return o, h, l, v, tv
		}
	}

	page, err := s.toss.GetCandles(ctx, toss.GetCandlesParams{
		Symbol:   symbol,
		Interval: toss.CandleInterval1m,
		Count:    480,
	})
	if err != nil || page == nil || len(page.Candles) == 0 {
		return s.fallbackDailySessionStats(ctx, symbol, today)
	}

	var first bool
	for _, c := range page.Candles {
		t := parseQuoteCandleTime(c.Timestamp)
		if t.Before(today) {
			continue
		}
		o := parseDecimal(c.OpenPrice)
		h := parseDecimal(c.HighPrice)
		l := parseDecimal(c.LowPrice)
		v := parseDecimalInt(c.Volume)
		closePx := parseDecimal(c.ClosePrice)

		if !first {
			open = o
			high = h
			low = l
			first = true
		} else {
			if h > high {
				high = h
			}
			if l < low || low == 0 {
				low = l
			}
		}
		volume += v
		tradingValue += closePx * float64(v)
	}

	if !first {
		return s.fallbackDailySessionStats(ctx, symbol, today)
	}
	return open, high, low, volume, tradingValue
}

// sumIntradayBarsFrom reduces DB-stored 1m bars to today's session stats.
// ok is false when none of the bars fall on/after `today`, signaling the
// caller should fall through to a live Toss fetch instead of reporting an
// all-zero session.
func sumIntradayBarsFrom(bars []model.Candle, today time.Time) (
	open, high, low float64, volume int64, tradingValue float64, ok bool,
) {
	for _, c := range bars {
		if parseQuoteCandleTime(c.Time).Before(today) {
			continue
		}
		if !ok {
			open, high, low = c.Open, c.High, c.Low
			ok = true
		} else {
			if c.High > high {
				high = c.High
			}
			if c.Low < low || low == 0 {
				low = c.Low
			}
		}
		volume += c.Volume
		tradingValue += c.Close * float64(c.Volume)
	}
	return
}

func (s *QuotesService) fallbackDailySessionStats(ctx context.Context, symbol string, today time.Time) (
	open, high, low float64, volume int64, tradingValue float64,
) {
	adjusted := true
	page, err := s.toss.GetCandles(ctx, toss.GetCandlesParams{
		Symbol:   symbol,
		Interval: toss.CandleInterval1d,
		Count:    2,
		Adjusted: &adjusted,
	})
	if err != nil || page == nil || len(page.Candles) == 0 {
		return 0, 0, 0, 0, 0
	}
	c := page.Candles[0]
	t := parseQuoteCandleTime(c.Timestamp)
	if startOfDay(t).Before(today.AddDate(0, 0, -1)) {
		return 0, 0, 0, 0, 0
	}
	open = parseDecimal(c.OpenPrice)
	high = parseDecimal(c.HighPrice)
	low = parseDecimal(c.LowPrice)
	volume = parseDecimalInt(c.Volume)
	closePx := parseDecimal(c.ClosePrice)
	tradingValue = closePx * float64(volume)
	return open, high, low, volume, tradingValue
}

func (s *QuotesService) statsFromRawCache(ctx context.Context, symbol string, stock *toss.StockInfo, currentPrice float64) (quoteStatsCache, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	// 291KB짜리 배열이라 짧은 기억을 거쳐 읽는다 (`quotes_raw_tail_memo.go`).
	// 아래 반복은 전부 읽기라 공유 슬라이스를 써도 안전하다.
	raw1m, ok := loadRawCandles(ctx, s.redis, fmt.Sprintf(mdRaw1mKeyFmt, sym))
	if !ok {
		return quoteStatsCache{}, false
	}

	stats := quoteStatsCache{}
	loc := kst
	if stock != nil && stock.Currency == toss.CurrencyUSD {
		loc = usEastern
	}
	today := s.quoteSessionStart(ctx, symbol, loc)

	var first bool
	if o, h, l, v, tv, ok := s.closedSessionDailyStats(ctx, sym, today); ok {
		stats.Open, stats.High, stats.Low, stats.Volume, stats.TradingValue = o, h, l, v, tv
		raw1m = nil // 아래 1분봉 합산을 건너뛴다 — 닫힌 세션은 일봉이 정답이다.
	}
	for _, c := range raw1m {
		t := parseQuoteCandleTime(c.Time)
		if t.Before(today) {
			continue
		}
		if !first {
			stats.Open = c.Open
			stats.High = c.High
			stats.Low = c.Low
			first = true
		} else {
			if c.High > stats.High {
				stats.High = c.High
			}
			if c.Low < stats.Low || stats.Low == 0 {
				stats.Low = c.Low
			}
		}
		stats.Volume += c.Volume
		stats.TradingValue += c.Close * float64(c.Volume)
	}

	// 일봉 원본은 아래 52주 고저 계산이 그대로 쓴다 — 존재 확인만으로 대체할 수 없다.
	raw1d, hasRaw1d := loadRawCandles(ctx, s.redis, fmt.Sprintf(mdRaw1dKeyFmt, sym))
	if hasRaw1d {
		if ref, ok := s.resolvePrevClose(ctx, sym); ok {
			stats.PrevClose = ref
		}
	}

	if stats.Open <= 0 && stats.PrevClose > 0 {
		stats.Open = stats.PrevClose
	}
	if stats.High <= 0 && currentPrice > 0 {
		stats.High = currentPrice
	}
	if stats.Low <= 0 && currentPrice > 0 {
		stats.Low = currentPrice
	}
	if stats.High > 0 && currentPrice > stats.High {
		stats.High = currentPrice
	}
	if stats.Low > 0 && currentPrice > 0 && currentPrice < stats.Low {
		stats.Low = currentPrice
	}

	if h, l, ok := week52RangeFromBars(raw1d); ok {
		stats.Week52High, stats.Week52Low = h, l
	}

	if stock != nil && stock.SharesOutstanding != "" {
		shares := parseDecimal(stock.SharesOutstanding)
		if shares > 0 && currentPrice > 0 {
			stats.MarketCap = shares * currentPrice
		}
	}

	return stats, stats.PrevClose > 0
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// prevCloseFromDailyCandles returns the prior session close from daily bars.
// Bars may be ascending (DB) or descending (Toss tail) — always sort by time first.
func prevCloseFromDailyCandles(bars []model.Candle) (float64, bool) {
	if len(bars) == 0 {
		return 0, false
	}
	sorted := append([]model.Candle(nil), bars...)
	sort.Slice(sorted, func(i, j int) bool {
		return parseQuoteCandleTime(sorted[i].Time).Before(parseQuoteCandleTime(sorted[j].Time))
	})
	if len(sorted) >= 2 {
		return sorted[len(sorted)-2].Close, true
	}
	return sorted[len(sorted)-1].Close, true
}

func (s *QuotesService) enrichQuoteSummaries(ctx context.Context, items []model.QuoteSummary) {
	if !s.TossEnabled() || len(items) == 0 {
		return
	}
	for i := range items {
		stock, err := s.toss.GetStock(ctx, items[i].Symbol)
		if err != nil || stock == nil {
			continue
		}
		stats := s.loadQuoteStats(ctx, items[i].Symbol, stock, items[i].CurrentPrice)
		prevClose := stats.PrevClose
		if ref, ok := s.resolvePrevClose(ctx, items[i].Symbol); ok {
			prevClose = ref
		}
		if items[i].CurrentPrice > 0 {
			items[i].Change, items[i].ChangeRate = computeQuoteChange(items[i].CurrentPrice, prevClose)
		}
		items[i].Volume = stats.Volume
		items[i].TradingValue = stats.TradingValue
		items[i].Name = stock.Name
		items[i].Market = stock.Market
		items[i].AssetType = assetTypeFromMarket(stock.Market, stock.SecurityType)
		items[i].Currency = modelCurrency(stock.Currency)
	}
}

func (s *QuotesService) indexChangeFromCandles(ctx context.Context, indicatorSymbol string) (change, changeRate float64, ok bool) {
	page, err := s.toss.GetMarketIndicatorCandles(ctx, toss.GetMarketIndicatorCandlesParams{
		Symbol:   indicatorSymbol,
		Interval: toss.CandleInterval1d,
		Count:    2,
	})
	if err != nil || page == nil || len(page.Candles) == 0 {
		return 0, 0, false
	}
	last := parseDecimal(page.Candles[0].ClosePrice)
	if len(page.Candles) >= 2 {
		prev := parseDecimal(page.Candles[1].ClosePrice)
		if prev > 0 {
			change = last - prev
			changeRate = (change / prev) * 100
			return change, changeRate, true
		}
	}
	return 0, 0, false
}

func roundIndexValue(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}
