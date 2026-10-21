package service

import (
	"context"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

/*
국내 일봉 **NXT 마감 뒤 재적재** (2026-10-21 사용자 결정, 티켓 QP-21 ①).

15:50 적재(`daily_close_domestic` → `daily_close_fetch_stale`)는 Toss 일봉이 **통합(KRX+NXT)** 기준인데
15:40부터 NXT 애프터마켓이 돌아, 그 시각의 체결가가 그날 종가로 굳었다. 실측 10-21: 삼성전자 DB 222,500 ·
KRX 223,500 · 통합 최종가 224,000, 국내 240종목이 등락 0%. 이 프로젝트의 종가는 「시간외까지 마감된 값」
(`computeQuoteChange` 머리글의 결정)이라, NXT가 닫힌 뒤 20:10에 그날 종가가 있는 국내 종목을 **전부 다시**
받아 덮어쓴다. 종목당 최근 40봉을 받으므로 지난 며칠의 스냅샷 종가도 함께 고쳐진다.

15:50 적재는 그대로 둔다 — 장 마감 직후 화면이 전날 종가에 머물지 않게 하는 임시값이고, 20:10이 최종값이다.
호출 수는 국내 약 4,300 × 1(캔들 한도를 앱과 공유, `fetchDelay`로 간격)이 하루 한 번 더 든다.
*/

// nxtFinalRefreshHour·Minute — KST. NXT 애프터마켓이 20:00에 닫힌다.
const (
	nxtFinalRefreshHour   = 20
	nxtFinalRefreshMinute = 10
	// 재적재 뒤 일봉 투영을 다시 하는 범위 — 받은 40봉 중 최근 것만 다시 셈해도 스냅샷 구간을 덮는다.
	nxtRefreshProjectDays = 10
)

// nxtRefreshSlot is day's 20:10 KST.
func nxtRefreshSlot(day time.Time) time.Time {
	d := day.In(calendar.LocationFor(calendar.ScheduleKRX))
	return time.Date(d.Year(), d.Month(), d.Day(), nxtFinalRefreshHour, nxtFinalRefreshMinute, 0, 0, d.Location())
}

// nextNXTRefresh is the first KRX-trading-day 20:10 strictly after now.
func nextNXTRefresh(now time.Time) time.Time {
	local := now.In(calendar.LocationFor(calendar.ScheduleKRX))
	for i := 0; i < 15; i++ {
		day := local.AddDate(0, 0, i)
		if !calendar.IsTradingDay(calendar.ScheduleKRX, day) {
			continue
		}
		if at := nxtRefreshSlot(day); at.After(now) {
			return at
		}
	}
	return now.Add(24 * time.Hour)
}

// SymbolsWithCloseOn lists active symbols that have a stored close on asOf.
func (s *DailyCloseService) SymbolsWithCloseOn(ctx context.Context, asOf string, assetTypes ...model.QuoteAssetType) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	filter, filterArgs := assetTypeFilter(assetTypes)
	args := append([]any{asOf}, filterArgs...)
	rows, err := s.db.QueryContext(ctx, `
		SELECT st.symbol FROM stocks st
		  JOIN stock_daily_closes d ON d.symbol = st.symbol AND d.trade_date = ?
		 WHERE st.status = 'NORMAL'`+filter+s.tossUnknownFilter()+`
		 ORDER BY st.symbol`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var sym string
		if err := rows.Scan(&sym); err != nil {
			return nil, err
		}
		out = append(out, sym)
	}
	return out, rows.Err()
}

// FetchRefreshCloses re-fetches every symbol that has a close on asOf and
// overwrites candle_bars + the projection. Returns the projection start it used.
func (s *DailyCloseService) FetchRefreshCloses(
	ctx context.Context, client *toss.Client, bars *repository.CandleBarRepo,
	asOf string, delay time.Duration, assetTypes ...model.QuoteAssetType,
) (FetchMissingResult, string, error) {
	var res FetchMissingResult
	day, err := time.ParseInLocation("2006-01-02", asOf, calendar.LocationFor(calendar.ScheduleKRX))
	if err != nil {
		return res, "", err
	}
	since := day.AddDate(0, 0, -nxtRefreshProjectDays)
	if s == nil || client == nil || bars == nil {
		return res, since.Format("2006-01-02"), nil
	}
	symbols, err := s.SymbolsWithCloseOn(ctx, asOf, assetTypes...)
	if err != nil {
		return res, "", err
	}
	res.Remaining = len(symbols)
	res, err = s.fetchSymbols(ctx, client, bars, symbols, since, delay, 500, res)
	return res, since.Format("2006-01-02"), err
}

// RunNXTFinalRefresh queues `daily_close_refresh_domestic` at 20:10 KST on
// KRX trading days (worker-fetch runs it). Blocks until ctx is done.
func (s *DailyCloseService) RunNXTFinalRefresh(ctx context.Context, runs *repository.BatchJobRunRepo) {
	if runs == nil {
		return
	}
	/*
		만회: 오늘 20:10이 지났는데 그 뒤 성공한 실행이 없으면 지금 넣는다. 성공 시각으로 가리므로
		저녁에 재배포를 여러 번 해도 전 종목 재수집이 한 번만 돈다(메모리 `new_scheduled_job_runs_on_first_deploy`).
	*/
	now := time.Now()
	if calendar.IsTradingDay(calendar.ScheduleKRX, now.In(calendar.LocationFor(calendar.ScheduleKRX))) {
		if slot := nxtRefreshSlot(now); now.After(slot) {
			last, err := runs.LastSuccess(ctx, string(model.BatchDailyCloseRefreshDomestic))
			if err == nil && (last == nil || last.FinishedAt == nil || last.FinishedAt.Before(slot)) {
				enqueueNXTRefresh(ctx, runs, slot)
			}
		}
	}
	gate := NewSlotGate()
	for {
		next := nextNXTRefresh(time.Now())
		if !WaitForSlot(ctx, next) {
			return
		}
		if gate.Allow(next) {
			enqueueNXTRefresh(ctx, runs, next)
		}
	}
}

func enqueueNXTRefresh(ctx context.Context, runs *repository.BatchJobRunRepo, slot time.Time) {
	id := string(model.BatchDailyCloseRefreshDomestic)
	if pending, err := runs.HasPending(ctx, id); err != nil || pending {
		return
	}
	asOf := slot.In(calendar.LocationFor(calendar.ScheduleKRX)).Format("2006-01-02")
	if _, err := runs.EnqueueAs(ctx, id, repository.TriggerSchedule, map[string]any{"market": "domestic", "asOf": asOf}); err != nil {
		log.Printf("daily close: NXT refresh enqueue: %v", err)
		return
	}
	log.Printf("daily close: queued NXT final refresh (asOf=%s)", asOf)
}
