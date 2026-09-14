// Command worker-scores is the standalone process for the daily 종합 점수
// batch (stocks + indices).
//
// DB-only — no upstream API calls — so it does not compete with the fetch
// worker's Toss budget. Runs once daily at a fixed KST slot, reading whatever
// the two close workers most recently wrote; kept separate from them so a slow
// or stuck score recompute cannot delay either market's close ingestion.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
)

// nextScheduledRefresh is the next 00:00:00 KST strictly after now. Scoring
// runs a bit after that (see scoreDelay) so the close workers have had time to
// land the newest session first.
func nextScheduledRefresh(now time.Time) time.Time {
	loc := kstLocation()
	local := now.In(loc)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	if !midnight.After(local) {
		midnight = midnight.AddDate(0, 0, 1)
	}
	return midnight
}

func kstLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.FixedZone("KST", 9*3600)
	}
	return loc
}

const scoreDelay = 25 * time.Minute

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deps, err := service.BootstrapWorker(ctx)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	defer deps.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-stop
		log.Printf("shutdown: received %s", sig)
		cancel()
	}()

	go runSchedule(ctx, deps)
	go runRollupSchedule(ctx, deps)
	go runRetentionSchedule(ctx, deps)
	go runWatchdogSchedule(ctx, deps)

	go service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchDataRetention,
		func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
			return purgeExpired(ctx, deps)
		})

	// 수동 실행 큐는 잡마다 하나씩 — RunBatchQueueWorker가 잡 하나를 맡는 계약이라
	// 섞으면 롤업의 긴 실행이 점수 재계산 클릭을 막는다.
	go service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchCandleRollup,
		func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
			return rollup(ctx, deps, params)
		})

	/*
		지수 해설은 **점수 배치 안에서 자동으로** 돌지만(아래 `recompute`), 손으로 다시
		돌릴 길을 따로 둔다. 프롬프트를 고친 뒤 문장만 다시 보고 싶을 때 전 종목 재계산
		17,915행을 끌고 올 이유가 없다. 수동 실행은 `force`라 같은 날 것도 덮어쓴다 —
		그게 이 버튼을 누르는 유일한 이유다.
	*/
	go service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchIndexNotes,
		func(ctx context.Context, _ map[string]any) (map[string]any, string, error) {
			res, err := deps.Indices.GenerateNotes(ctx, true)
			if err != nil {
				return nil, "", err
			}
			msg := fmt.Sprintf("지수 해설 %d건 생성 · 건너뜀 %d · 실패 %d (토큰 %d)",
				res.Generated, res.Skipped, res.Failed, res.Tokens)
			return map[string]any{"notes": res}, msg, nil
		})

	log.Println("worker-scores: running (점수+지수해설 00:25 · 만료 정리 04:30 · 캔들 롤업 05:30 KST + manual queue)")
	service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchStockScores,
		func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
			return recompute(ctx, deps)
		})
}

func runSchedule(ctx context.Context, deps *service.WorkerDeps) {
	// 한 슬롯은 한 번만 — 실측으로 빠른 예약 잡들이 매일 두 번 돌았다
	// (service.SlotGate 머리글).
	gate := service.NewSlotGate()
	for {
		slot := nextScheduledRefresh(time.Now()).Add(scoreDelay)
		if !service.WaitForSlot(ctx, slot) {
			return
		}
		if !gate.Allow(slot) {
			continue
		}
		service.RunScheduledJob(ctx, deps.Runs, model.BatchStockScores,
			func(ctx context.Context) (map[string]any, string, error) {
				return recompute(ctx, deps)
			})
	}
}

// 캔들 롤업 슬롯 — 05:30 KST.
//
// 미국장 마감(05:00 KST) 직후이고 06:00 해외 실적 배치보다 앞선다. 두 시장 모두
// 닫혀 있어 1분봉 쓰기가 없는 유일한 창이다 — 롤업이 지우는 구간에 폴러가 동시에
// 쓰면 그 봉은 다음 실행에서 다시 접힐 뿐이지만, 굳이 겹칠 이유가 없다.
const (
	rollupHour   = 5
	rollupMinute = 30
)

func nextRollup(now time.Time) time.Time {
	loc := kstLocation()
	local := now.In(loc)
	at := time.Date(local.Year(), local.Month(), local.Day(), rollupHour, rollupMinute, 0, 0, loc)
	if !at.After(local) {
		at = at.AddDate(0, 0, 1)
	}
	return at
}

func runRollupSchedule(ctx context.Context, deps *service.WorkerDeps) {
	gate := service.NewSlotGate()
	for {
		slot := nextRollup(time.Now())
		if !service.WaitForSlot(ctx, slot) {
			return
		}
		if !gate.Allow(slot) {
			continue
		}
		service.RunScheduledJob(ctx, deps.Runs, model.BatchCandleRollup,
			func(ctx context.Context) (map[string]any, string, error) {
				return rollup(ctx, deps, nil)
			})
	}
}

// 만료 데이터 정리 슬롯 — 04:30 KST.
//
// 05:30 캔들 롤업보다 앞이고 두 시장 모두 닫혀 있다. 가벼운 잡이라 순서만
// 지키면 되고, 롤업이 무거우니 그 앞에 둔다.
const (
	retentionHour   = 4
	retentionMinute = 30
)

func nextRetention(now time.Time) time.Time {
	loc := kstLocation()
	local := now.In(loc)
	at := time.Date(local.Year(), local.Month(), local.Day(), retentionHour, retentionMinute, 0, 0, loc)
	if !at.After(local) {
		at = at.AddDate(0, 0, 1)
	}
	return at
}

func runRetentionSchedule(ctx context.Context, deps *service.WorkerDeps) {
	gate := service.NewSlotGate()
	for {
		slot := nextRetention(time.Now())
		if !service.WaitForSlot(ctx, slot) {
			return
		}
		if !gate.Allow(slot) {
			continue
		}
		service.RunScheduledJob(ctx, deps.Runs, model.BatchDataRetention,
			func(ctx context.Context) (map[string]any, string, error) {
				return purgeExpired(ctx, deps)
			})
	}
}

/*
runWatchdogSchedule — 15분마다 「모든 배치가 제때 돌았나」를 판정한다.

이 워커에 얹은 이유: 여기가 가장 한가하고(하루 세 슬롯), DB만 읽는 잡이라 무엇과도
다투지 않으며, **API와 독립**이기 때문이다. API 안에 두면 API가 죽을 때 감시도 함께
죽는다. 반대로 API는 이 잡이 남긴 보고서의 나이를 읽어 이 잡의 죽음을 본다.

기동 직후 한 번 돈다 — 재기동은 무언가 잘못됐을 때 가장 자주 일어나는 일이고, 그
순간이 판정이 가장 필요한 때다.
*/
func runWatchdogSchedule(ctx context.Context, deps *service.WorkerDeps) {
	if deps.Health == nil {
		return
	}
	runOnce := func() {
		service.RunScheduledJob(ctx, deps.Runs, model.BatchWatchdog,
			func(ctx context.Context) (map[string]any, string, error) {
				return deps.Health.RunWatchdog(ctx)
			})
	}
	runOnce()

	ticker := time.NewTicker(service.BatchWatchdogInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runOnce()
		}
	}
}

// purgeExpired drops rows that nothing reads any more.
//
// 지금은 리프레시 토큰 하나지만, 「쓰이지 않는데 쌓이는 것」이 또 나오면 여기에
// 붙인다 — 잡을 늘리는 것보다 한 슬롯에서 함께 도는 편이 운영이 단순하다.
func purgeExpired(ctx context.Context, deps *service.WorkerDeps) (map[string]any, string, error) {
	tokens, err := deps.RefreshTokens.PurgeExpired(ctx, 0)
	if err != nil {
		return nil, "", err
	}
	return map[string]any{"refreshTokens": tokens},
		fmt.Sprintf("만료 리프레시 토큰 %d행 정리", tokens), nil
}

// rollup folds aged intraday bars one tier coarser.
//
// 관리자 수동 실행은 params로 dryRun·maxSymbols를 줄 수 있다. 되돌릴 수 없는
// 작업이라 「무엇이 접힐지」를 먼저 볼 수 있어야 한다.
func rollup(ctx context.Context, deps *service.WorkerDeps, params map[string]any) (map[string]any, string, error) {
	opts := service.RollupOptions{}
	if v, ok := params["dryRun"].(bool); ok {
		opts.DryRun = v
	}
	if v, ok := params["maxSymbols"].(float64); ok && v > 0 {
		opts.MaxSymbolsPerStep = int(v)
	}

	summary, err := service.NewCandleRollupService(deps.Bars).Run(ctx, opts)
	if err != nil {
		return nil, "", err
	}
	return map[string]any{"rollup": summary}, summary.Message(), nil
}

func recompute(ctx context.Context, deps *service.WorkerDeps) (map[string]any, string, error) {
	tradeDate, err := deps.Scores.LatestTradeDate(ctx)
	if err != nil {
		return nil, "", err
	}
	dist, err := deps.Scores.Recompute(ctx, tradeDate)
	if err != nil {
		return nil, "", err
	}
	// Indices ride the same daily slot: they read the closes the close
	// workers already persisted, so this costs no upstream call either.
	indexCount, err := deps.Scores.RecomputeIndices(ctx)
	if err != nil {
		log.Printf("worker-scores: index recompute: %v", err)
	}

	/*
		지수 한 줄 해설. **점수를 매긴 직후에** 부른다 — 해설이 붙는 행이 방금 위에서
		갱신된 그 행이라, 순서가 뒤집히면 어제 행에 오늘 문장을 쓰게 된다.

		하루 열세 콜이고, 같은 거래일에 이미 있는 지수는 건너뛴다. 실패해도 배치를
		멈추지 않는다 — 규칙 문장이 그 자리에 그대로 있으므로 잃는 것은 문장뿐이다.
	*/
	notes, err := deps.Indices.GenerateNotes(ctx, false)
	if err != nil {
		log.Printf("worker-scores: index notes: %v", err)
	}

	/*
		AI 추천 랭킹의 축·기간 조합을 다시 만든다 — **정리(prune)보다 먼저**다.

		이 계산은 최근 30일 창까지 훑으므로, 보관 기간을 넘긴 행을 지운 뒤에 돌리면
		1개월 랭킹이 남은 날짜만 보고 매겨진다. 순서가 곧 정확도다.

		실측 한 판 2 시장 × (기간 4회 훑기 + 표시용 조회 1회) — 개발 PC에서 1개월
		창 하나가 국내 2.08초 · 해외 1.30초다. 실패해도 배치를 멈추지 않는다:
		랭킹은 어제 것이 그대로 남고, 점수 자체는 이미 갱신됐다.
	*/
	if deps.ScoreRankings != nil {
		if err := deps.ScoreRankings.ComputeAll(ctx); err != nil {
			log.Printf("worker-scores: score rankings: %v", err)
		}
	}

	// 오래된 점수는 아무도 읽지 않는다 — 조회는 전부 최신 하나만 본다.
	// 정리하지 않으면 하루 17,906행씩 연 7.5GB로 자란다
	// (repository.ScoreRetentionDates 헤더 참조).
	pruned, err := deps.Scores.PruneOldScores(ctx)
	if err != nil {
		log.Printf("worker-scores: prune old scores: %v", err)
	}

	msg := fmt.Sprintf("%s 기준 %d종목 + 지수 %d개 산출 (평균 %.1f, 표준편차 %.1f)",
		tradeDate, dist.Count, indexCount, dist.Mean, dist.StdDev)
	if notes.Generated > 0 || notes.Failed > 0 {
		msg += fmt.Sprintf(" · 지수 해설 %d건 생성", notes.Generated)
		if notes.Failed > 0 {
			msg += fmt.Sprintf(" (실패 %d)", notes.Failed)
		}
	}
	if pruned > 0 {
		msg += fmt.Sprintf(" · 오래된 점수 %d행 정리", pruned)
	}
	if !dist.Healthy {
		msg += fmt.Sprintf(" — 분포 경고: %v", dist.Warnings)
	}
	return map[string]any{
		"tradeDate":    tradeDate,
		"distribution": dist,
		"indexScores":  indexCount,
		"indexNotes":   notes,
	}, msg, nil
}
