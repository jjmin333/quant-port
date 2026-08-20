package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

// Scheduled AI 요약 batch.
//
// One run per market per trading day, finishing before that market opens (see
// stock_summary_schedule.go). Targets come from 마켓 레이더's 1일 기준 top 100
// minus ETF/ETN/ETP (see llm_batch_targets.go).
//
// Everything here exists to keep a few hundred sequential LLM calls inside a
// free tier that caps requests *and* tokens, per minute and per day:
//
//   - the work list is cut to the remaining request allowance before the
//     first call, instead of discovering the ceiling one 429 at a time;
//   - spacing adapts to the observed token cost per symbol, so a prompt that
//     grows slows the run down rather than tripping the per-minute cap;
//   - the run stops at the first budget or deadline breach and carries the
//     untouched symbols to the next run, which starts with them.

const (
	// llmBatchRunKey holds one market-day's record; llmBatchHistoryKey is a
	// capped newest-first list for the admin page.
	llmBatchRunKeyPrefix  = "llm:batch:v1:run:"
	llmBatchHistoryPrefix = "llm:batch:v1:history:"
	llmBatchCarryPrefix   = "llm:batch:v1:carryover:"
	llmBatchAvgTokensKey  = "llm:batch:v1:avg_tokens"

	llmBatchRunTTL     = 8 * 24 * time.Hour
	llmBatchHistoryMax = 14

	// llmBatchDefaultTokens seeds the per-call token estimate before any
	// call has been observed. Deliberately above the ~2.6K a real AI 요약
	// costs, so the first run of a fresh deployment under-books rather than
	// overshoots.
	llmBatchDefaultTokens = 3_500

	// llmBatchTokenPacingRatio is the share of the per-minute token ceiling
	// a run is allowed to occupy. The remaining headroom absorbs concurrent
	// news translation and any manual admin refresh.
	llmBatchTokenPacingRatio = 0.6
)

func llmBatchRunKey(market toss.MarketCountry, tradeDate string) string {
	return llmBatchRunKeyPrefix + string(market) + ":" + tradeDate
}

func llmBatchHistoryKey(market toss.MarketCountry) string {
	return llmBatchHistoryPrefix + string(market)
}

func llmBatchCarryKey(market toss.MarketCountry) string {
	return llmBatchCarryPrefix + string(market)
}

// RunScheduler drives the per-market batches until ctx is cancelled.
//
// It deliberately does not sweep on startup: a restart must not re-spend the
// day's quota. What it does do is resume a window it woke up inside — if the
// process comes back at 07:40 and today's KR run has not been recorded, that
// run still happens, because nextBatchWindow returns a start time of "now"
// for an in-progress window with no record.
func (s *StockSummaryService) RunScheduler(ctx context.Context) {
	if s.settings == nil {
		return
	}

	for {
		settings := s.settings.Get(ctx)
		window, ok := nextBatchWindow(time.Now(), settings.DailyCap, func(market toss.MarketCountry, tradeDate string) bool {
			return s.batchAlreadyRan(ctx, market, tradeDate)
		})
		if !ok {
			// No trading day within the lookahead (should not happen with
			// real calendars). Re-check in an hour rather than spinning.
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Hour):
				continue
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(window.startAt)):
		}

		s.runMarketBatch(ctx, window)
	}
}

// scheduleFor looks up the fixed schedule for a market.
func scheduleFor(market toss.MarketCountry) (batchSchedule, bool) {
	for _, sched := range batchSchedules {
		if sched.market == market {
			return sched, true
		}
	}
	return batchSchedule{}, false
}

// manualBatchWindow is how long a manual run is given to finish — generous
// compared to the scheduled path's pre-open deadline, since there is no open
// bell to beat and the caller (배치 관리) is watching interactively.
const manualBatchWindow = 2 * time.Hour

// RunBatchNow executes one market's scheduled AI 요약 batch immediately,
// ignoring the pre-open window and the "already ran today" guard — the
// explicit manual trigger from 배치 관리 (/admin/batch), distinct from the
// arbitrary-symbol "수동 생성" on /admin/llm (stock_summary_manual.go). It
// shares the scheduled path's Redis run/history keys, so /admin/llm reflects
// a manual run the same way it reflects a scheduled one, and it shares the
// same LLM quota — running it after the schedule already ran today spends
// the quota again.
//
// Unlike the scheduled path, a manual trigger is not tied to today being a
// trading day: an admin re-running a failed batch, or catching up over a
// holiday, still has a closed session's data to work with. When today is
// closed, the run is pinned to the last completed close (calendar.LastSessionCloseAt)
// instead of being refused — 대상 selection reads whatever 마켓 레이더 last
// stored, which on a closed session is that same last session, so the run
// analyses it on a 종가(closing price) basis rather than intraday data.
func (s *StockSummaryService) RunBatchNow(ctx context.Context, market toss.MarketCountry) (map[string]any, string, error) {
	sched, ok := scheduleFor(market)
	if !ok {
		return nil, "", fmt.Errorf("알 수 없는 시장: %s", market)
	}
	now := time.Now()

	window := batchRunWindow{schedule: sched, startAt: now, deadline: now.Add(manualBatchWindow)}
	if !sched.isTradingDay(now) {
		window.asOf = calendar.LastSessionCloseAt(sched.calendar, now)
	}
	run, err := s.runMarketBatch(ctx, window)
	if err != nil {
		return nil, "", err
	}
	msg := fmt.Sprintf("%s %s: 대상 %d · 성공 %d · 실패 %d · 건너뜀 %d (%d토큰, stop=%s)",
		market, run.TradeDate, run.Planned, run.Succeeded, run.Failed, run.Skipped, run.TokensUsed, run.StopReason)
	return map[string]any{"run": run}, msg, nil
}

// batchAlreadyRan reports whether this market-day already has a record.
func (s *StockSummaryService) batchAlreadyRan(ctx context.Context, market toss.MarketCountry, tradeDate string) bool {
	if s.redis == nil {
		// Without Redis the run record has nowhere to live, so fall back to
		// process-local memory: a restart may repeat a run, which the budget
		// guard still bounds.
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.lastBatchDate[market] == tradeDate
	}
	n, err := s.redis.Exists(ctx, llmBatchRunKey(market, tradeDate)).Result()
	return err == nil && n > 0
}

// beginMarketRun claims a market for the duration of one run, refusing a
// second concurrent run (schedule vs. manual, or two manual clicks) from
// racing on the same Redis run/history keys and budget check.
func (s *StockSummaryService) beginMarketRun(market toss.MarketCountry) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runningMarkets == nil {
		s.runningMarkets = map[toss.MarketCountry]bool{}
	}
	if s.runningMarkets[market] {
		return false
	}
	s.runningMarkets[market] = true
	return true
}

func (s *StockSummaryService) endMarketRun(market toss.MarketCountry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.runningMarkets, market)
}

// runMarketBatch executes one market's daily run, whether reached from the
// schedule loop above or from an admin's manual trigger (RunBatchNow). The
// returned run is nil only when nothing was attempted (not a trading day, LLM
// disabled, or another run for the same market already in flight) — err
// distinguishes that from a run that was attempted and recorded but failed to
// even select targets.
func (s *StockSummaryService) runMarketBatch(ctx context.Context, window batchRunWindow) (*model.LLMBatchRun, error) {
	sched := window.schedule
	now := time.Now()
	day := now
	if !window.asOf.IsZero() {
		// A manual trigger already resolved this to the last completed
		// close; nothing left to validate against wall-clock time.
		day = window.asOf
	}
	tradeDate := sched.tradeDateOn(day)

	if window.asOf.IsZero() && !sched.isTradingDay(day) {
		// The window was scheduled off a trading day, but the clock may have
		// crossed midnight while waiting. Nothing to analyse on a closed
		// session — skip without recording, so the next real session runs.
		log.Printf("llm batch %s: %s is not a trading day — skipped", sched.market, tradeDate)
		return nil, fmt.Errorf("%s 시장은 %s에 휴장입니다", sched.market, tradeDate)
	}
	if s.llm == nil || !s.llm.Enabled() {
		log.Printf("llm batch %s: llm disabled — skipped", sched.market)
		return nil, fmt.Errorf("LLM이 비활성화되어 있습니다")
	}
	if !s.beginMarketRun(sched.market) {
		return nil, fmt.Errorf("%s 시장 배치가 이미 실행 중입니다", sched.market)
	}
	defer s.endMarketRun(sched.market)

	settings := s.settings.Get(ctx)
	run := &model.LLMBatchRun{
		Market:     string(sched.market),
		TradeDate:  tradeDate,
		StartedAt:  now.Format(time.RFC3339),
		DeadlineAt: window.deadline.Format(time.RFC3339),
		Running:    true,
		Symbols:    []model.LLMBatchSymbolResult{},
	}

	targets, err := s.batchTargets(ctx, sched.market, settings)
	if err != nil {
		log.Printf("llm batch %s: select targets: %v", sched.market, err)
		run.Running = false
		run.StopReason = "select_failed"
		run.FinishedAt = time.Now().Format(time.RFC3339)
		s.storeBatchRun(ctx, sched.market, tradeDate, run)
		return nil, fmt.Errorf("대상 선정 실패: %w", err)
	}
	run.Considered = targets.Considered
	run.Excluded = targets.Excluded

	planned, stopReason := s.applyBudget(ctx, targets.Targets, settings, run)
	run.Planned = len(planned)
	log.Printf("llm batch %s %s: %d ranked, %d after filters, %d planned (calls left %d, tokens left %d)",
		sched.market, tradeDate, targets.Considered, len(targets.Targets), len(planned),
		run.CallsRemaining, run.TokensRemaining)

	s.storeBatchRun(ctx, sched.market, tradeDate, run)
	remaining := s.executeBatch(ctx, window, planned, run, stopReason)
	s.setCarryover(ctx, sched.market, remaining)

	run.Running = false
	run.Carryover = remaining
	run.FinishedAt = time.Now().Format(time.RFC3339)
	s.storeBatchRun(ctx, sched.market, tradeDate, run)
	s.pushBatchHistory(ctx, sched.market, run)

	s.mu.Lock()
	if s.lastBatchDate == nil {
		s.lastBatchDate = map[toss.MarketCountry]string{}
	}
	s.lastBatchDate[sched.market] = tradeDate
	s.mu.Unlock()

	log.Printf("llm batch %s %s: done — %d ok, %d failed, %d skipped, %d tokens, stop=%s, carryover=%d",
		sched.market, tradeDate, run.Succeeded, run.Failed, run.Skipped, run.TokensUsed, run.StopReason, len(remaining))
	return run, nil
}

// batchTargets resolves the run's symbol list: carry-over first (they were
// cut from a previous run and are already a day stale), then the pinned
// always-include symbols, then the ranking.
func (s *StockSummaryService) batchTargets(
	ctx context.Context,
	market toss.MarketCountry,
	settings model.LLMSettings,
) (model.LLMBatchSelection, error) {
	pinned := s.pinnedForMarket(ctx, market, settings.PilotSymbols)

	if settings.TargetModeOrDefault() == model.LLMTargetModeManual {
		catalog, err := s.catalogFor(ctx, pinned)
		if err != nil {
			return model.LLMBatchSelection{}, err
		}
		return model.LLMBatchSelection{
			Market:  string(market),
			Targets: prependPinned(pinned, catalog, nil),
		}, nil
	}

	selection, err := s.selectBatchTargets(ctx, market, pinned)
	if err != nil {
		return model.LLMBatchSelection{}, err
	}
	selection.Targets = s.prependCarryover(ctx, market, selection.Targets)
	return selection, nil
}

// pinnedForMarket keeps only the always-include symbols belonging to this
// market — the settings list is one flat list across both.
func (s *StockSummaryService) pinnedForMarket(ctx context.Context, market toss.MarketCountry, pinned []string) []string {
	if len(pinned) == 0 {
		return nil
	}
	catalog, err := s.catalogFor(ctx, pinned)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(pinned))
	for _, symbol := range pinned {
		stock, ok := catalog[strings.ToUpper(strings.TrimSpace(symbol))]
		if !ok {
			continue
		}
		if marketForStock(stock) == market {
			out = append(out, stock.Symbol)
		}
	}
	return out
}

// marketForStock maps a catalog row onto the ranking market it belongs to.
func marketForStock(stock model.Stock) toss.MarketCountry {
	if ResolveFundamentalsAssetType(&stock) == model.AssetDomestic {
		return toss.MarketKR
	}
	return toss.MarketUS
}

// applyBudget trims the target list to what today's remaining provider
// allowance and the admin's DailyCap permit, and records why it stopped where
// it did. Symbols cut here never reach the LLM, so they cost nothing.
func (s *StockSummaryService) applyBudget(
	ctx context.Context,
	targets []model.LLMBatchTarget,
	settings model.LLMSettings,
	run *model.LLMBatchRun,
) ([]model.LLMBatchTarget, string) {
	head := s.llm.Headroom(ctx)
	avgTokens := s.batchAvgTokens(ctx)

	limit := len(targets)
	stopReason := model.LLMBatchStopCompleted

	if settings.DailyCap > 0 && limit > settings.DailyCap {
		limit = settings.DailyCap
		stopReason = model.LLMBatchStopQuota
	}

	// Calls: whatever the day has left, minus the reserve held for news
	// briefings and manual refreshes.
	callBudget := int64(-1)
	if head.Calls >= 0 {
		callBudget = head.Calls - int64(settings.QuotaReserve)
		if callBudget < 0 {
			callBudget = 0
		}
		if int64(limit) > callBudget {
			limit = int(callBudget)
			stopReason = model.LLMBatchStopQuota
		}
	}
	run.CallsRemaining = callBudget

	// Tokens: same idea, converted to a symbol count through the observed
	// average cost per call.
	tokenBudget := int64(-1)
	if head.DayTokens >= 0 {
		tokenBudget = head.DayTokens - settings.TokenReserve
		if tokenBudget < 0 {
			tokenBudget = 0
		}
		affordable := tokenBudget / avgTokens
		if int64(limit) > affordable {
			limit = int(affordable)
			stopReason = model.LLMBatchStopTokens
		}
	}
	run.TokensRemaining = tokenBudget

	if limit >= len(targets) {
		return targets, model.LLMBatchStopCompleted
	}
	if limit < 0 {
		limit = 0
	}
	if run.Excluded == nil {
		run.Excluded = map[string]int{}
	}
	run.Excluded[model.LLMBatchExcludeOverBudget] += len(targets) - limit
	return targets[:limit], stopReason
}

// executeBatch walks the planned targets sequentially and returns the ones it
// never got to, for carry-over.
func (s *StockSummaryService) executeBatch(
	ctx context.Context,
	window batchRunWindow,
	planned []model.LLMBatchTarget,
	run *model.LLMBatchRun,
	cutReason string,
) []string {
	run.StopReason = cutReason
	spacing := s.batchSpacing(ctx)

	for i, target := range planned {
		if ctx.Err() != nil {
			run.StopReason = model.LLMBatchStopShutdown
			return remainingSymbols(planned[i:])
		}
		// Stop before the call that would run past the market open rather
		// than after it: a summary that lands post-open is worth less than
		// the quota it cost.
		if time.Now().Add(spacing).After(window.deadline) {
			run.StopReason = model.LLMBatchStopDeadline
			return remainingSymbols(planned[i:])
		}
		if reason, ok := s.batchBudgetExhausted(ctx); ok {
			run.StopReason = reason
			return remainingSymbols(planned[i:])
		}

		if i > 0 {
			select {
			case <-ctx.Done():
				run.StopReason = model.LLMBatchStopShutdown
				return remainingSymbols(planned[i:])
			case <-time.After(spacing):
			}
		}

		result := model.LLMBatchSymbolResult{Symbol: target.Symbol, Name: target.Name}
		view, stats, err := s.refreshSymbol(ctx, target.Symbol, true)
		switch {
		case err != nil:
			result.Status = model.LLMBatchSymbolError
			result.Error = err.Error()
			run.Failed++
		case stats.llmErr != nil:
			// The summary was still rebuilt and cached, just without the LLM
			// narrative — worth reporting separately from a hard failure.
			result.Status = model.LLMBatchSymbolError
			result.Source = view.Source
			result.Error = stats.llmErr.Error()
			run.Failed++
		default:
			result.Status = model.LLMBatchSymbolDone
			result.Source = view.Source
			result.Tokens = stats.llmTokens
			run.Succeeded++
		}

		if stats.llmTokens > 0 {
			run.TokensUsed += stats.llmTokens
			spacing = s.recordBatchTokens(ctx, stats.llmTokens)
		}
		run.Symbols = append(run.Symbols, result)
		s.storeBatchRun(ctx, window.schedule.market, run.TradeDate, run)
	}

	return nil
}

// batchBudgetExhausted reports whether the day's remaining allowance has run
// out mid-run — a concurrent translation burst can consume it while the batch
// is walking its list.
func (s *StockSummaryService) batchBudgetExhausted(ctx context.Context) (string, bool) {
	head := s.llm.Headroom(ctx)
	if head.Calls == 0 {
		return model.LLMBatchStopQuota, true
	}
	if head.DayTokens >= 0 && head.DayTokens < s.batchAvgTokens(ctx) {
		return model.LLMBatchStopTokens, true
	}
	return "", false
}

// batchSpacing is the gap between calls: never below the provider's own
// minimum interval, and stretched further when the observed token cost would
// otherwise push the run past its share of the per-minute token ceiling.
func (s *StockSummaryService) batchSpacing(ctx context.Context) time.Duration {
	spacing := llmRefreshSpacing
	limits := s.llm.Limits()
	if limits.TokensPerMinute <= 0 {
		return spacing
	}
	allowance := float64(limits.TokensPerMinute) * llmBatchTokenPacingRatio
	if allowance <= 0 {
		return spacing
	}
	perCall := float64(s.batchAvgTokens(ctx))
	paced := time.Duration(perCall / allowance * float64(time.Minute))
	if paced > spacing {
		return paced
	}
	return spacing
}

// batchAvgTokens is the running estimate of one AI 요약 call's token cost,
// seeded with a conservative constant and refined from real usage.
func (s *StockSummaryService) batchAvgTokens(ctx context.Context) int64 {
	if s.redis == nil {
		return llmBatchDefaultTokens
	}
	v, err := s.redis.Get(ctx, llmBatchAvgTokensKey).Int64()
	if err != nil || v <= 0 {
		return llmBatchDefaultTokens
	}
	return v
}

// recordBatchTokens folds one observed call into the running average and
// returns the spacing to use for the next call.
func (s *StockSummaryService) recordBatchTokens(ctx context.Context, tokens int64) time.Duration {
	if s.redis != nil && tokens > 0 {
		// Exponential moving average, weight 1/4 on the newest sample: fast
		// enough to react to a prompt change, damped enough that one
		// unusually long news list does not re-pace the whole run.
		prev := s.batchAvgTokens(ctx)
		next := (prev*3 + tokens) / 4
		_ = s.redis.Set(ctx, llmBatchAvgTokensKey, next, 30*24*time.Hour).Err()
	}
	return s.batchSpacing(ctx)
}

func remainingSymbols(targets []model.LLMBatchTarget) []string {
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		out = append(out, t.Symbol)
	}
	return out
}

// prependCarryover puts symbols a previous run could not reach at the head of
// this run's list, ahead of the ranking, and clears the queue.
func (s *StockSummaryService) prependCarryover(
	ctx context.Context,
	market toss.MarketCountry,
	targets []model.LLMBatchTarget,
) []model.LLMBatchTarget {
	if s.redis == nil {
		return targets
	}
	raw, err := s.redis.Get(ctx, llmBatchCarryKey(market)).Bytes()
	if err != nil || len(raw) == 0 {
		return targets
	}
	var symbols []string
	if json.Unmarshal(raw, &symbols) != nil || len(symbols) == 0 {
		return targets
	}
	_ = s.redis.Del(ctx, llmBatchCarryKey(market)).Err()

	bySymbol := make(map[string]model.LLMBatchTarget, len(targets))
	for _, t := range targets {
		bySymbol[t.Symbol] = t
	}
	out := make([]model.LLMBatchTarget, 0, len(targets))
	seen := make(map[string]bool, len(targets))
	for _, symbol := range symbols {
		target, ok := bySymbol[symbol]
		if !ok {
			// Dropped out of today's ranking — it is no longer among the
			// most-traded names, so it does not earn a call.
			continue
		}
		seen[symbol] = true
		out = append(out, target)
	}
	for _, t := range targets {
		if !seen[t.Symbol] {
			out = append(out, t)
		}
	}
	return out
}

func (s *StockSummaryService) setCarryover(ctx context.Context, market toss.MarketCountry, symbols []string) {
	if s.redis == nil {
		return
	}
	if len(symbols) == 0 {
		_ = s.redis.Del(ctx, llmBatchCarryKey(market)).Err()
		return
	}
	if payload, err := json.Marshal(symbols); err == nil {
		_ = s.redis.Set(ctx, llmBatchCarryKey(market), payload, llmBatchRunTTL).Err()
	}
}

func (s *StockSummaryService) storeBatchRun(ctx context.Context, market toss.MarketCountry, tradeDate string, run *model.LLMBatchRun) {
	if s.redis == nil || run == nil {
		return
	}
	payload, err := json.Marshal(run)
	if err != nil {
		return
	}
	_ = s.redis.Set(ctx, llmBatchRunKey(market, tradeDate), payload, llmBatchRunTTL).Err()
}

func (s *StockSummaryService) pushBatchHistory(ctx context.Context, market toss.MarketCountry, run *model.LLMBatchRun) {
	if s.redis == nil || run == nil {
		return
	}
	// The per-symbol detail is only interesting for the run in flight; the
	// history list keeps the totals so the page stays small.
	summary := *run
	summary.Symbols = nil
	payload, err := json.Marshal(summary)
	if err != nil {
		return
	}
	pipe := s.redis.Pipeline()
	pipe.LPush(ctx, llmBatchHistoryKey(market), payload)
	pipe.LTrim(ctx, llmBatchHistoryKey(market), 0, llmBatchHistoryMax-1)
	pipe.Expire(ctx, llmBatchHistoryKey(market), llmBatchRunTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("llm batch %s: history push: %v", market, err)
	}
}

// BatchStatus reports the current/most recent run per market plus the targets
// the next run would pick right now — read-only, never triggers generation.
func (s *StockSummaryService) BatchStatus(ctx context.Context) model.LLMBatchStatus {
	if s.settings == nil {
		return model.LLMBatchStatus{Markets: []model.LLMBatchMarketStatus{}}
	}
	settings := s.settings.Get(ctx)
	out := model.LLMBatchStatus{
		TargetMode: settings.TargetModeOrDefault(),
		DailyCap:   settings.DailyCap,
		Markets:    make([]model.LLMBatchMarketStatus, 0, len(batchSchedules)),
	}
	head := s.llm.Headroom(ctx)
	out.CallsRemaining = head.Calls
	out.TokensRemaining = head.DayTokens
	out.AvgTokensPerCall = s.batchAvgTokens(ctx)

	now := time.Now()
	for _, sched := range batchSchedules {
		status := model.LLMBatchMarketStatus{Market: string(sched.market)}
		window, ok := nextBatchWindow(now, settings.DailyCap, func(market toss.MarketCountry, tradeDate string) bool {
			// Only this market matters for its own "next run" line.
			return market == sched.market && s.batchAlreadyRan(ctx, market, tradeDate)
		})
		if ok && window.schedule.market == sched.market {
			status.NextRunAt = window.startAt.Format(time.RFC3339)
			status.NextDeadlineAt = window.deadline.Format(time.RFC3339)
		}
		status.LastRun = s.lastBatchRun(ctx, sched.market)
		status.History = s.batchHistory(ctx, sched.market)
		out.Markets = append(out.Markets, status)
	}
	return out
}

// lastBatchRun reads today's record, falling back to the newest history entry
// when today has not run yet.
func (s *StockSummaryService) lastBatchRun(ctx context.Context, market toss.MarketCountry) *model.LLMBatchRun {
	if s.redis == nil {
		return nil
	}
	for _, sched := range batchSchedules {
		if sched.market != market {
			continue
		}
		now := time.Now()
		for dayOffset := 0; dayOffset >= -2; dayOffset-- {
			tradeDate := sched.tradeDateOn(now.AddDate(0, 0, dayOffset))
			raw, err := s.redis.Get(ctx, llmBatchRunKey(market, tradeDate)).Bytes()
			if err != nil || len(raw) == 0 {
				continue
			}
			var run model.LLMBatchRun
			if json.Unmarshal(raw, &run) == nil {
				return &run
			}
		}
	}
	return nil
}

func (s *StockSummaryService) batchHistory(ctx context.Context, market toss.MarketCountry) []model.LLMBatchRun {
	if s.redis == nil {
		return nil
	}
	raw, err := s.redis.LRange(ctx, llmBatchHistoryKey(market), 0, llmBatchHistoryMax-1).Result()
	if err != nil {
		return nil
	}
	out := make([]model.LLMBatchRun, 0, len(raw))
	for _, item := range raw {
		var run model.LLMBatchRun
		if json.Unmarshal([]byte(item), &run) == nil {
			out = append(out, run)
		}
	}
	return out
}

// PreviewBatchTargets resolves what a run would pick for a market right now,
// without running anything — the admin page's "오늘 대상" section.
func (s *StockSummaryService) PreviewBatchTargets(ctx context.Context, market toss.MarketCountry) (model.LLMBatchSelection, error) {
	if s.settings == nil {
		return model.LLMBatchSelection{Market: string(market)}, nil
	}
	settings := s.settings.Get(ctx)
	// A preview must not consume the carry-over queue the next real run
	// depends on, so resolve targets without draining it.
	pinned := s.pinnedForMarket(ctx, market, settings.PilotSymbols)
	if settings.TargetModeOrDefault() == model.LLMTargetModeManual {
		catalog, err := s.catalogFor(ctx, pinned)
		if err != nil {
			return model.LLMBatchSelection{}, err
		}
		return model.LLMBatchSelection{Market: string(market), Targets: prependPinned(pinned, catalog, nil)}, nil
	}
	return s.selectBatchTargets(ctx, market, pinned)
}
