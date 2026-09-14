package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

var kst = time.FixedZone("KST", 9*3600)

type marketCalendars struct {
	us *toss.UsMarketCalendarResponse
	kr *toss.KrMarketCalendarResponse
}

func detectMarketSession(assetType string, cals marketCalendars, now time.Time) string {
	if assetType == "us" || assetType == "etf" {
		if cals.us != nil {
			return detectUsSessionAcrossDays(cals.us, now)
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

/*
detectUsSessionAcrossDays — 미국 세션은 KST 날짜 경계를 넘는다.

Toss 달력의 `today`는 **KST 기준 오늘**이다. 그런데 미국 정규장은 KST로
22:30~05:00이라 자정을 넘는다. 그래서 KST 00:00을 지나면 `today`가 다음 날로
넘어가고, 그 `regularMarket`은 **오늘 밤에 시작할** 세션을 가리킨다 — 지금
돌아가고 있는 세션은 `previousBusinessDay` 쪽에 들어 있다.

`today`만 보면 매 정규장의 뒷부분 4시간 30분(KST 00:00~05:00 = ET 11:00~16:00)이
통째로 "closed"로 판정된다. 실측(2026-08-25 01:31 KST = ET 12:31, 정규장
한복판): AAPL marketSession=closed. 그 판정은 폴러의 filterLiveSessionSymbols를
타고 1분봉 적재를 멈춰 세워 차트가 그 자리에 얼어붙는다.

세션 창은 오프셋이 붙은 절대 시각이라 두 날짜를 다 봐도 오탐이 없다 — now가
실제로 그 구간 안에 있을 때만 맞는다. 미래 방향(`nextBusinessDay`)은 볼 필요가
없다 — KST 자정 전에는 `today`가 이미 그날 밤 세션을 담고 있다.
*/
func detectUsSessionAcrossDays(cal *toss.UsMarketCalendarResponse, now time.Time) string {
	if cal == nil {
		return "closed"
	}
	if session := detectUsSession(cal.Today, now); session != "closed" {
		return session
	}
	return detectUsSession(cal.PreviousBusinessDay, now)
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
	// 291KB짜리 배열에서 종가 하나를 고르는 자리라 짧은 기억을 거친다
	// (`quotes_raw_tail_memo.go`). 아래 반복은 읽기뿐이다.
	candles, ok := loadRawCandles(ctx, rdb, fmt.Sprintf(mdRaw1mKeyFmt, sym))
	if !ok {
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

func previousRegularSessionBounds(assetType string, cals marketCalendars, now time.Time) (start, end time.Time, ok bool) {
	_ = now
	if assetType == "us" || assetType == "etf" {
		if cals.us == nil || cals.us.PreviousBusinessDay.RegularMarket == nil {
			return time.Time{}, time.Time{}, false
		}
		start, ok1 := parseCandleTime(cals.us.PreviousBusinessDay.RegularMarket.StartTime)
		end, ok2 := parseCandleTime(cals.us.PreviousBusinessDay.RegularMarket.EndTime)
		return start, end, ok1 && ok2
	}
	if cals.kr == nil || cals.kr.PreviousBusinessDay.Integrated == nil || cals.kr.PreviousBusinessDay.Integrated.RegularMarket == nil {
		return time.Time{}, time.Time{}, false
	}
	start, ok1 := parseCandleTime(cals.kr.PreviousBusinessDay.Integrated.RegularMarket.StartTime)
	end, ok2 := parseCandleTime(cals.kr.PreviousBusinessDay.Integrated.RegularMarket.EndTime)
	return start, end, ok1 && ok2
}

func krPreviousRegularSessionFallback(now time.Time) (start, end time.Time, ok bool) {
	d := now.In(kst)
	prev := d.AddDate(0, 0, -1)
	for prev.Weekday() == time.Saturday || prev.Weekday() == time.Sunday {
		prev = prev.AddDate(0, 0, -1)
	}
	start = time.Date(prev.Year(), prev.Month(), prev.Day(), 9, 0, 0, 0, kst)
	end = time.Date(prev.Year(), prev.Month(), prev.Day(), 15, 30, 0, 0, kst)
	return start, end, true
}

func fetchPrevClose(ctx context.Context, client *toss.Client, symbol, assetType string, cals marketCalendars) (float64, error) {
	if client == nil {
		return 0, fmt.Errorf("no toss client")
	}
	now := time.Now()
	start, end, ok := previousRegularSessionBounds(assetType, cals, now)
	if !ok && (assetType == "domestic" || assetType == "bond" || assetType == "cash") {
		start, end, ok = krPreviousRegularSessionFallback(now)
	}
	if ok {
		candles, err := collectCandlesPaginated(ctx, client, symbol, toss.CandleInterval1m, start, end)
		if err == nil {
			var last float64
			var lastTs time.Time
			for _, c := range candles {
				ts, tsOk := parseCandleTime(c.Timestamp)
				if !tsOk || ts.After(end) {
					continue
				}
				v := parseDecimal(c.ClosePrice)
				if v > 0 && (lastTs.IsZero() || !ts.Before(lastTs)) {
					last = v
					lastTs = ts
				}
			}
			if last > 0 {
				return last, nil
			}
		}
	}

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
