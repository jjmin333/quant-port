package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

type marketCalendars struct {
	us *toss.UsMarketCalendarResponse
	kr *toss.KrMarketCalendarResponse
}

func (s *PortfolioService) loadMarketCalendars(ctx context.Context, _ *toss.Client) marketCalendars {
	if s.marketStore != nil {
		if cals, ok := s.marketStore.EnsureCalendars(ctx); ok {
			return cals
		}
	}
	return marketCalendars{}
}

func detectMarketSession(assetType string, cals marketCalendars, now time.Time) string {
	if assetType == "us" || assetType == "etf" {
		if cals.us != nil {
			return detectUsSession(cals.us.Today, now)
		}
		return "closed"
	}
	if assetType == "domestic" || assetType == "bond" || assetType == "cash" {
		if cals.kr != nil && cals.kr.Today.Integrated != nil {
			return detectKrSession(*cals.kr.Today.Integrated, now)
		}
		return "closed"
	}
	if cals.kr != nil && cals.kr.Today.Integrated != nil {
		return detectKrSession(*cals.kr.Today.Integrated, now)
	}
	return "regular"
}

func detectUsSession(day toss.UsMarketDay, now time.Time) string {
	if inSessionWindow(now, day.RegularMarket) {
		return "regular"
	}
	if inSessionWindow(now, day.AfterMarket) {
		return "after"
	}
	if inSessionWindow(now, day.PreMarket) {
		return "pre"
	}
	if inSessionWindow(now, day.DayMarket) {
		return "day"
	}
	return "closed"
}

func detectKrSession(integrated toss.IntegratedHour, now time.Time) string {
	if inSessionWindow(now, integrated.RegularMarket) {
		return "regular"
	}
	if inSessionWindow(now, integrated.AfterMarket) {
		return "after"
	}
	if inSessionWindow(now, integrated.PreMarket) {
		return "pre"
	}
	return "closed"
}

func inSessionWindow(now time.Time, session interface{}) bool {
	switch s := session.(type) {
	case *toss.UsRegularMarketSession:
		if s == nil {
			return false
		}
		return inTimeWindow(now, s.StartTime, s.EndTime)
	case *toss.UsAfterMarketSession:
		if s == nil {
			return false
		}
		return inTimeWindow(now, s.StartTime, s.EndTime)
	case *toss.UsPreMarketSession:
		if s == nil {
			return false
		}
		return inTimeWindow(now, s.StartTime, s.EndTime)
	case *toss.UsDayMarketSession:
		if s == nil {
			return false
		}
		return inTimeWindow(now, s.StartTime, s.EndTime)
	case *toss.RegularMarketSession:
		if s == nil {
			return false
		}
		return inTimeWindow(now, s.StartTime, s.EndTime)
	case *toss.AfterMarketSession:
		if s == nil {
			return false
		}
		return inTimeWindow(now, s.StartTime, s.EndTime)
	case *toss.PreMarketSession:
		if s == nil {
			return false
		}
		return inTimeWindow(now, s.StartTime, s.EndTime)
	default:
		return false
	}
}

func inTimeWindow(now time.Time, startRaw, endRaw string) bool {
	start, ok1 := parseCandleTime(startRaw)
	end, ok2 := parseCandleTime(endRaw)
	if !ok1 || !ok2 {
		return false
	}
	return !now.Before(start) && !now.After(end)
}

func regularSessionBounds(assetType string, cals marketCalendars) (start, end time.Time, ok bool) {
	if assetType == "us" || assetType == "etf" {
		if cals.us == nil || cals.us.Today.RegularMarket == nil {
			return time.Time{}, time.Time{}, false
		}
		start, ok1 := parseCandleTime(cals.us.Today.RegularMarket.StartTime)
		end, ok2 := parseCandleTime(cals.us.Today.RegularMarket.EndTime)
		return start, end, ok1 && ok2
	}
	if cals.kr == nil || cals.kr.Today.Integrated == nil || cals.kr.Today.Integrated.RegularMarket == nil {
		return time.Time{}, time.Time{}, false
	}
	start, ok1 := parseCandleTime(cals.kr.Today.Integrated.RegularMarket.StartTime)
	end, ok2 := parseCandleTime(cals.kr.Today.Integrated.RegularMarket.EndTime)
	return start, end, ok1 && ok2
}

func fetchRegularSessionClose(
	ctx context.Context,
	client *toss.Client,
	symbol string,
	assetType string,
	cals marketCalendars,
	now time.Time,
) (float64, bool) {
	regularStart, regularEnd, ok := regularSessionBounds(assetType, cals)
	if !ok || now.Before(regularEnd) {
		return 0, false
	}

	candles, err := collectCandlesPaginated(ctx, client, symbol, toss.CandleInterval1m, regularStart, regularEnd)
	if err == nil && len(candles) > 0 {
		var lastClose float64
		var lastTs time.Time
		for _, c := range candles {
			ts, tsOk := parseCandleTime(c.Timestamp)
			if !tsOk || ts.After(regularEnd) {
				continue
			}
			closePrice := parseDecimal(c.ClosePrice)
			if closePrice > 0 && (lastTs.IsZero() || !ts.Before(lastTs)) {
				lastClose = closePrice
				lastTs = ts
			}
		}
		if lastClose > 0 {
			return lastClose, true
		}
	}

	return fetchRegularCloseDailyFallback(ctx, client, symbol, now)
}

func fetchRegularCloseDailyFallback(ctx context.Context, client *toss.Client, symbol string, now time.Time) (float64, bool) {
	adjusted := true
	page, err := client.GetCandles(ctx, toss.GetCandlesParams{
		Symbol:   symbol,
		Interval: toss.CandleInterval1d,
		Count:    1,
		Adjusted: &adjusted,
	})
	if err != nil || page == nil || len(page.Candles) == 0 {
		return 0, false
	}
	closePrice := parseDecimal(page.Candles[0].ClosePrice)
	if closePrice <= 0 {
		return 0, false
	}
	ts, ok := parseCandleTime(page.Candles[0].Timestamp)
	if !ok {
		return closePrice, true
	}
	if startOfDayKST(ts).Equal(startOfDayKST(now)) {
		return closePrice, true
	}
	return 0, false
}

func fetchTodayRegularClose(
	ctx context.Context,
	client *toss.Client,
	symbol string,
	assetType string,
	cals marketCalendars,
	now time.Time,
	session string,
) (float64, bool) {
	if session == "regular" || session == "pre" || session == "day" {
		return 0, false
	}
	return fetchRegularSessionClose(ctx, client, symbol, assetType, cals, now)
}

func refreshHoldingSession(
	ctx context.Context,
	rdb *redis.Client,
	h *model.UnifiedHolding,
	cals marketCalendars,
	now time.Time,
	statsLoader func(string) holdingStatsCache,
	requestStatsRefresh func(string),
) {
	session := detectMarketSession(h.AssetType, cals, now)
	h.MarketSession = session

	if statsLoader != nil {
		stats := statsLoader(h.Symbol)
		if stats.PrevClose > 0 {
			h.PrevClose = stats.PrevClose
		}
		if stats.RegularClose > 0 {
			h.RegularClose = stats.RegularClose
		}
		if stats.MarketSession != "" {
			h.MarketSession = stats.MarketSession
		}
	}

	if h.RegularClose <= 0 {
		if rc, ok := regularCloseFromRawCandles(ctx, rdb, h.Symbol, h.AssetType, cals, now); ok {
			h.RegularClose = rc
		}
	}

	if session != "after" && (session != "closed" || h.RegularClose > 0) {
		return
	}
	if session == "after" && h.RegularClose > 0 && h.RegularClose != h.CurrentPrice {
		return
	}

	if h.RegularClose <= 0 && requestStatsRefresh != nil {
		requestStatsRefresh(h.Symbol)
	}
}

func regularCloseFromRawCandles(
	ctx context.Context,
	rdb *redis.Client,
	symbol string,
	assetType string,
	cals marketCalendars,
	now time.Time,
) (float64, bool) {
	if rdb == nil {
		return 0, false
	}
	regularStart, regularEnd, ok := regularSessionBounds(assetType, cals)
	if !ok || now.Before(regularEnd) {
		return 0, false
	}

	sym := strings.ToUpper(strings.TrimSpace(symbol))
	raw, err := rdb.Get(ctx, fmt.Sprintf(mdRaw1mKeyFmt, sym)).Bytes()
	if err != nil || len(raw) == 0 {
		return 0, false
	}
	var candles []model.Candle
	if json.Unmarshal(raw, &candles) != nil || len(candles) == 0 {
		return 0, false
	}

	var lastClose float64
	var lastTs time.Time
	for _, c := range candles {
		ts, tsOk := parseCandleTime(c.Time)
		if !tsOk || ts.Before(regularStart) || ts.After(regularEnd) {
			continue
		}
		if c.Close > 0 && (lastTs.IsZero() || !ts.Before(lastTs)) {
			lastClose = c.Close
			lastTs = ts
		}
	}
	if lastClose > 0 {
		return lastClose, true
	}
	return 0, false
}

func updateSessionBreakdown(h *model.UnifiedHolding) {
	if h.PrevClose <= 0 || h.CurrentPrice <= 0 {
		h.RegularChange = 0
		h.RegularChangeRate = 0
		h.ExtendedChange = 0
		h.ExtendedChangeRate = 0
		h.SessionChange = 0
		h.SessionChangeRate = 0
		return
	}

	session := h.MarketSession
	if session == "" {
		session = "regular"
	}

	prevClose := h.PrevClose
	current := h.CurrentPrice

	switch session {
	case "after":
		regularClose := h.RegularClose
		if regularClose <= 0 {
			regularClose = current
		}
		h.RegularChange = regularClose - prevClose
		h.RegularChangeRate = (h.RegularChange / prevClose) * 100
		h.ExtendedChange = current - regularClose
		if regularClose > 0 {
			h.ExtendedChangeRate = (h.ExtendedChange / regularClose) * 100
		}

	case "pre", "day":
		h.RegularChange = 0
		h.RegularChangeRate = 0
		h.ExtendedChange = current - prevClose
		h.ExtendedChangeRate = (h.ExtendedChange / prevClose) * 100

	case "regular":
		h.RegularChange = current - prevClose
		h.RegularChangeRate = (h.RegularChange / prevClose) * 100
		h.ExtendedChange = 0
		h.ExtendedChangeRate = 0

	default: // closed
		regularClose := h.RegularClose
		if regularClose > 0 {
			h.RegularChange = regularClose - prevClose
			h.RegularChangeRate = (h.RegularChange / prevClose) * 100
			h.ExtendedChange = current - regularClose
			h.ExtendedChangeRate = (h.ExtendedChange / regularClose) * 100
		} else {
			h.RegularChange = current - prevClose
			h.RegularChangeRate = (h.RegularChange / prevClose) * 100
			h.ExtendedChange = 0
			h.ExtendedChangeRate = 0
		}
	}

	h.SessionChange = current - prevClose
	h.SessionChangeRate = (h.SessionChange / prevClose) * 100
}

func applySessionMeta(h *model.UnifiedHolding, prevClose, regularClose float64, session string) {
	h.PrevClose = prevClose
	h.RegularClose = regularClose
	h.MarketSession = session
	updateSessionBreakdown(h)
}
