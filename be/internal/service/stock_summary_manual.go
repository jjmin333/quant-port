package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// Manual AI 요약 regeneration — the escape hatch from the scheduled
// per-market batch. An admin picks arbitrary symbols on /admin/llm and gets
// them rebuilt right away.
//
// Deliberately different from the scheduled path in two ways:
//   - any symbol qualifies, batch target or not, because "임의의 종목을 지금
//     즉시" is the whole point; the apibudget quota guard still applies, so
//     this can't run away with the free tier
//   - progress is tracked per symbol so the page can report what actually
//     happened (llm vs rule fallback, errors) instead of just "queued"
//
// One job at a time, process-local. Multi-instance deployments would need the
// job state in Redis — noted alongside the poller leader-election TODO.

var (
	// ErrManualRefreshBusy — a previous manual run is still in flight.
	ErrManualRefreshBusy = errors.New("이미 실행 중인 수동 생성 작업이 있습니다")
	// ErrManualRefreshNoSymbols — nothing valid left after normalization.
	ErrManualRefreshNoSymbols = errors.New("생성할 종목을 1개 이상 입력하세요")
)

// maxManualRefreshSymbols bounds one manual run. Matches the pilot list cap:
// each symbol costs one LLM call plus llmRefreshSpacing of wall clock.
const maxManualRefreshSymbols = 10

// TriggerManualRefresh starts a background regeneration for the given symbols
// and returns the job's initial state. It never blocks on the LLM — callers
// poll ManualRefreshStatus.
func (s *StockSummaryService) TriggerManualRefresh(symbols []string) (model.LLMManualRefreshJob, error) {
	normalized := normalizeManualSymbols(symbols)
	if len(normalized) == 0 {
		return model.LLMManualRefreshJob{}, ErrManualRefreshNoSymbols
	}

	s.mu.Lock()
	if s.manualJob != nil && s.manualJob.Running {
		running := *s.manualJob
		s.mu.Unlock()
		return running, ErrManualRefreshBusy
	}
	now := time.Now()
	job := &model.LLMManualRefreshJob{
		ID:        now.Format("20060102T150405"),
		Running:   true,
		StartedAt: now.Format(time.RFC3339),
		Symbols:   make([]model.LLMRefreshSymbolState, 0, len(normalized)),
	}
	for _, symbol := range normalized {
		job.Symbols = append(job.Symbols, model.LLMRefreshSymbolState{
			Symbol: symbol,
			Status: model.LLMRefreshPending,
		})
	}
	s.manualJob = job
	snapshot := cloneManualJob(job)
	s.mu.Unlock()

	go s.runManualRefresh(s.backgroundContext(), job.ID, normalized)
	return snapshot, nil
}

// ManualRefreshStatus returns the most recent manual run's state, or an empty
// job if none has run since startup.
func (s *StockSummaryService) ManualRefreshStatus() model.LLMManualRefreshJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.manualJob == nil {
		return model.LLMManualRefreshJob{Symbols: []model.LLMRefreshSymbolState{}}
	}
	return cloneManualJob(s.manualJob)
}

func (s *StockSummaryService) runManualRefresh(ctx context.Context, jobID string, symbols []string) {
	for i, symbol := range symbols {
		if ctx.Err() != nil {
			s.updateManualSymbol(jobID, symbol, model.LLMRefreshSymbolState{
				Symbol: symbol,
				Status: model.LLMRefreshError,
				Error:  "서버 종료로 중단됨",
			})
			break
		}
		if i > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(llmRefreshSpacing):
			}
		}

		s.updateManualSymbol(jobID, symbol, model.LLMRefreshSymbolState{
			Symbol: symbol,
			Status: model.LLMRefreshRunning,
		})

		view, _, err := s.refreshSymbol(ctx, symbol, true)
		if err != nil {
			log.Printf("stock summary manual refresh %s: %v", symbol, err)
			s.updateManualSymbol(jobID, symbol, model.LLMRefreshSymbolState{
				Symbol: symbol,
				Status: model.LLMRefreshError,
				Error:  err.Error(),
			})
			continue
		}
		s.updateManualSymbol(jobID, symbol, model.LLMRefreshSymbolState{
			Symbol:      symbol,
			Status:      model.LLMRefreshDone,
			Source:      view.Source,
			GeneratedAt: view.GeneratedAt,
		})
	}

	s.mu.Lock()
	if s.manualJob != nil && s.manualJob.ID == jobID {
		s.manualJob.Running = false
		s.manualJob.FinishedAt = time.Now().Format(time.RFC3339)
	}
	s.mu.Unlock()
}

func (s *StockSummaryService) updateManualSymbol(jobID, symbol string, state model.LLMRefreshSymbolState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.manualJob == nil || s.manualJob.ID != jobID {
		return
	}
	for i := range s.manualJob.Symbols {
		if s.manualJob.Symbols[i].Symbol == symbol {
			s.manualJob.Symbols[i] = state
			return
		}
	}
}

func cloneManualJob(job *model.LLMManualRefreshJob) model.LLMManualRefreshJob {
	out := *job
	out.Symbols = append([]model.LLMRefreshSymbolState(nil), job.Symbols...)
	return out
}

func normalizeManualSymbols(symbols []string) []string {
	seen := make(map[string]bool, len(symbols))
	out := make([]string, 0, len(symbols))
	for _, raw := range symbols {
		symbol := strings.ToUpper(strings.TrimSpace(raw))
		if symbol == "" || seen[symbol] {
			continue
		}
		seen[symbol] = true
		out = append(out, symbol)
		if len(out) == maxManualRefreshSymbols {
			break
		}
	}
	return out
}
