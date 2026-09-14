package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
)

// candleRollupChunkSize — 한 트랜잭션이 다루는 심볼 수.
//
// 최초 백로그는 1분봉만 1,390만 행이다. 통째로 돌리면 트랜잭션 하나가 그 전부를
// 잡아 redo와 undo가 터진다. 심볼로 쪼개면 PK가 (symbol, bar_interval, bar_time)
// 이라 각 청크가 정확한 범위 스캔이 된다.
const candleRollupChunkSize = 100

// RollupOptions tunes one rollup run.
type RollupOptions struct {
	// DryRun이면 아무것도 쓰지 않고 만들어질 봉 수만 센다.
	DryRun bool
	// ChunkSize는 0이면 candleRollupChunkSize.
	ChunkSize int
	// MaxSymbolsPerStep은 0이면 무제한. 최초 백로그를 나눠 돌릴 때 쓴다.
	MaxSymbolsPerStep int
	// OnlySymbols가 비어 있지 않으면 그 종목만 접는다. 되돌릴 수 없는 작업이라
	// 전량 실행 전에 한두 종목으로 결과를 확인하기 위한 카나리아 경로다.
	OnlySymbols []string
}

// targets narrows the discovered symbol list to OnlySymbols when set.
func (o RollupOptions) targets(found []string) []string {
	if len(o.OnlySymbols) == 0 {
		return found
	}
	want := make(map[string]struct{}, len(o.OnlySymbols))
	for _, s := range o.OnlySymbols {
		want[s] = struct{}{}
	}
	out := make([]string, 0, len(o.OnlySymbols))
	for _, s := range found {
		if _, ok := want[s]; ok {
			out = append(out, s)
		}
	}
	return out
}

// RollupStepSummary reports one cascade step.
type RollupStepSummary struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Cutoff   string `json:"cutoff"`
	Symbols  int    `json:"symbols"`
	Inserted int64  `json:"inserted"`
	Deleted  int64  `json:"deleted"`
}

// RollupSummary is the whole cascade's result.
type RollupSummary struct {
	Steps    []RollupStepSummary `json:"steps"`
	Pruned   int64               `json:"pruned"`
	Elapsed  string              `json:"elapsed"`
	DryRun   bool                `json:"dryRun"`
	Complete bool                `json:"complete"`
}

// CandleRollupService folds aged intraday bars into coarser tiers.
type CandleRollupService struct {
	candles *repository.CandleBarRepo
}

func NewCandleRollupService(candles *repository.CandleBarRepo) *CandleRollupService {
	return &CandleRollupService{candles: candles}
}

/*
rollupSteps — intradayTiers에서 캐스케이드를 유도한다.

계층표가 유일한 진실이다. 여기서 단계를 따로 적으면 조회 경로(collectTieredBars)와
적재 경로가 갈라져, 접어 놓고 못 읽는 구간이 생긴다.
*/
func rollupSteps() []repository.RollupStep {
	out := make([]repository.RollupStep, 0, len(intradayTiers)-1)
	for i := 0; i+1 < len(intradayTiers); i++ {
		cur, next := intradayTiers[i], intradayTiers[i+1]
		out = append(out, repository.RollupStep{
			From:    cur.interval,
			To:      next.interval,
			Minutes: next.minutes,
			AgeDays: cur.ageDays,
		})
	}
	return out
}

/*
Run — 캐스케이드를 **고운 계층부터** 한 번에 돌린다.

순서가 중요하다. 1m→5m를 먼저 끝내야 그 5분봉이 같은 실행에서 5m→10m으로
이어진다. 반대로 돌리면 데이터가 하루에 한 칸씩만 내려가 수렴이 느리다.

마지막 계층(60분봉)은 접지 않고 **지운다** — 1일봉은 이미 더 정확하게, 더 길게
있으므로 접으면 그것을 덮어써 배치 입력까지 오염된다. repository.PruneIntraday
헤더 참조.
*/
func (s *CandleRollupService) Run(ctx context.Context, opts RollupOptions) (RollupSummary, error) {
	started := time.Now()
	summary := RollupSummary{DryRun: opts.DryRun, Complete: true}
	if s == nil || s.candles == nil {
		return summary, fmt.Errorf("candle rollup: repository unavailable")
	}

	chunk := opts.ChunkSize
	if chunk <= 0 {
		chunk = candleRollupChunkSize
	}

	for _, step := range rollupSteps() {
		cutoff, err := s.candles.RollupCutoff(ctx, step)
		if err != nil {
			return summary, err
		}
		found, err := s.candles.RollupSymbols(ctx, step.From, cutoff, opts.MaxSymbolsPerStep)
		if err != nil {
			return summary, err
		}
		if opts.MaxSymbolsPerStep > 0 && len(found) >= opts.MaxSymbolsPerStep {
			summary.Complete = false
		}
		symbols := opts.targets(found)

		st := RollupStepSummary{From: string(step.From), To: string(step.To), Cutoff: cutoff, Symbols: len(symbols)}
		for i := 0; i < len(symbols); i += chunk {
			end := min(i+chunk, len(symbols))
			res, err := s.candles.RollupChunk(ctx, step, cutoff, symbols[i:end], opts.DryRun)
			if err != nil {
				return summary, err
			}
			st.Inserted += res.Inserted
			st.Deleted += res.Deleted

			select {
			case <-ctx.Done():
				summary.Complete = false
				summary.Steps = append(summary.Steps, st)
				summary.Elapsed = time.Since(started).Round(time.Second).String()
				return summary, ctx.Err()
			default:
			}
		}
		if st.Symbols > 0 {
			log.Printf("candle rollup: %s -> %s (< %s): %d종목 %d봉 생성 %d봉 삭제",
				step.From, step.To, cutoff, st.Symbols, st.Inserted, st.Deleted)
		}
		summary.Steps = append(summary.Steps, st)
	}

	// 마지막 계층 만료.
	last := intradayTiers[len(intradayTiers)-1]
	pruneStep := repository.RollupStep{From: last.interval, Minutes: last.minutes, AgeDays: last.ageDays}
	cutoff, err := s.candles.RollupCutoff(ctx, pruneStep)
	if err != nil {
		return summary, err
	}
	foundLast, err := s.candles.RollupSymbols(ctx, last.interval, cutoff, opts.MaxSymbolsPerStep)
	if err != nil {
		return summary, err
	}
	symbols := opts.targets(foundLast)
	for i := 0; i < len(symbols); i += chunk {
		end := min(i+chunk, len(symbols))
		n, err := s.candles.PruneIntraday(ctx, last.interval, cutoff, symbols[i:end], opts.DryRun)
		if err != nil {
			return summary, err
		}
		summary.Pruned += n
	}
	if len(symbols) > 0 {
		log.Printf("candle rollup: prune %s (< %s): %d종목 %d봉 삭제", last.interval, cutoff, len(symbols), summary.Pruned)
	}

	summary.Elapsed = time.Since(started).Round(time.Second).String()
	return summary, nil
}

// Message renders the admin-facing one-liner for batch_job_runs.
func (r RollupSummary) Message() string {
	var ins, del int64
	for _, s := range r.Steps {
		ins += s.Inserted
		del += s.Deleted
	}
	prefix := ""
	if r.DryRun {
		prefix = "[모의] "
	}
	msg := fmt.Sprintf("%s%d봉 생성 · %d봉 접힘 · %d봉 만료 (%s)", prefix, ins, del, r.Pruned, r.Elapsed)
	if !r.Complete {
		msg += " — 상한에 걸려 남은 분량 있음"
	}
	return msg
}
