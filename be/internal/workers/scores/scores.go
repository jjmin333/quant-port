// Package scores is the worker-scores role (예전 단독 프로세스, 지금은 cmd/worker가 역할로 돌린다) is the standalone process for the daily 종합 점수
// batch (stocks + indices).
//
// DB-only — no upstream API calls — so it does not compete with the fetch
// worker's Toss budget. Runs once daily at a fixed KST slot, reading whatever
// the two close workers most recently wrote; kept separate from them so a slow
// or stuck score recompute cannot delay either market's close ingestion.
package scores

import (
	"context"
	"fmt"
	"log"
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

// Run starts this role's schedules and manual queues and blocks until ctx is done.
// 부트스트랩·종료 신호는 cmd/worker가 한 번만 한다(여러 역할이 한 프로세스를 나눠 쓴다).
func Run(ctx context.Context, deps *service.WorkerDeps) {
	// 이 워커의 잡은 이 프로세스만 돌린다 — 재기동 전에 끊긴 실행을 지금 닫아 만회가 다시 넣게 한다.
	service.CloseInterruptedRuns(ctx, deps.Runs, model.BatchCandleRollup, model.BatchChangeDetect, model.BatchDataRetention, model.BatchIndexNotes, model.BatchNewsSentiment, model.BatchScoreReportCard, model.BatchStockScores, model.BatchWatchdog)

	go runSchedule(ctx, deps)
	go runIndexNoteCatchupSchedule(ctx, deps)
	go runRollupSchedule(ctx, deps)
	go runRetentionSchedule(ctx, deps)
	go runWatchdogSchedule(ctx, deps)

	// 성적표 수동 실행 — 빠진 날짜를 채운다.
	go service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchScoreReportCard,
		func(ctx context.Context, _ map[string]any) (map[string]any, string, error) {
			return deps.ReportCard.Compute(ctx)
		})

	// 변화 감지 수동 실행 — 가장 최근 점수일로 다시 만든다. params.date(YYYY-MM-DD)를 주면
	// 그날을 다시 만든다 — 감지 규칙을 고친 뒤 이미 저장된 날을 새 규칙으로 갈아 끼울 때.
	go service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchChangeDetect,
		func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
			if d, ok := params["date"].(string); ok {
				if _, err := time.Parse("2006-01-02", d); err != nil {
					return nil, "", fmt.Errorf("change detect date %q: %w", d, err)
				}
				return deps.Changes.Run(ctx, d)
			}
			tradeDate, err := deps.Scores.LatestScoredDate(ctx)
			if err != nil {
				return nil, "", err
			}
			return deps.Changes.Run(ctx, tradeDate)
		})

	// 뉴스 감성 라벨 수동 실행 — limit은 거슬러 볼 일수(과거 채우기).
	go service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchNewsSentiment,
		func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
			days := 0
			if v, ok := params["limit"].(float64); ok {
				days = int(v)
			}
			return deps.NewsSentiment.Run(ctx, days)
		})

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
			return map[string]any{"notes": res}, indexNoteMessage(res), nil
		})

	log.Println("worker-scores: running (점수+지수해설 00:25 · 만료 정리 04:30 · 캔들 롤업 05:30 · 지수해설 만회 16:30 KST + manual queue)")
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

/*
지수 해설 만회 슬롯 — 16:30 KST (2026-09-16).

**00:25에 실패한 해설을 다음 날 00:25까지 끌고 가지 않는다.** 그 회차가 실패하면
화면이 읽는 행(점수가 앉은 행)에 문단이 없고, 사용자에게는 그것이 「지수 LLM이 안
돈다」로 보인다 — 실제로 하루를 그렇게 보냈다(`service/index_llm_note.go` 머리글).

**왜 16:30인가.** 이 배치가 쓰는 Gemini 무료 한도는 태평양 자정에 리셋된다(= 16:00
KST). 00:25는 그 날치의 8시간째이고, 바로 앞 17:10에 국내 종목 요약 배치가 하루
토큰의 70%를 가져간다 — 지수 해설은 언제나 **남은 것으로** 부르고 있었다. 16:30은
리셋 직후이자 17:10보다 앞이라, 만회분이 가장 넉넉한 창에서 돈다.

**부르는 값이 없으면 공짜다.** `GenerateNotes(force=false)`는 해설이 이미 있는
지수를 SELECT 한 번으로 건너뛰므로, 00:25가 멀쩡했던 날 이 슬롯은 열세 번의 읽기로
끝난다. 그래서 「실패했을 때만 돈다」는 조건을 따로 들 이유가 없다 — 조건을 들면
그 조건이 또 하나의 틀릴 자리가 된다.

`force`는 쓰지 않는다. 만회는 **빈 칸을 채우는 일**이지 오늘 것을 다시 쓰는 일이
아니다(다시 쓰기는 관리자 수동 실행의 몫이다).
*/
const (
	indexNoteCatchupHour   = 16
	indexNoteCatchupMinute = 30
)

func nextIndexNoteCatchup(now time.Time) time.Time {
	loc := kstLocation()
	local := now.In(loc)
	at := time.Date(local.Year(), local.Month(), local.Day(),
		indexNoteCatchupHour, indexNoteCatchupMinute, 0, 0, loc)
	if !at.After(local) {
		at = at.AddDate(0, 0, 1)
	}
	return at
}

func runIndexNoteCatchupSchedule(ctx context.Context, deps *service.WorkerDeps) {
	// 한 슬롯은 한 번만 — `WaitForSlot`이 몇 밀리초 이르게 깨면 같은 슬롯이 두 번
	// 돈다(service.SlotGate 머리글).
	gate := service.NewSlotGate()
	for {
		slot := nextIndexNoteCatchup(time.Now())
		if !service.WaitForSlot(ctx, slot) {
			return
		}
		if !gate.Allow(slot) {
			continue
		}
		service.RunScheduledJob(ctx, deps.Runs, model.BatchIndexNotes,
			func(ctx context.Context) (map[string]any, string, error) {
				res, err := deps.Indices.GenerateNotes(ctx, false)
				if err != nil {
					return nil, "", err
				}
				return map[string]any{"notes": res}, indexNoteMessage(res), nil
			})
	}
}

/*
indexNoteMessage — 배치 기록 한 줄.

**「예산이 닫혔다」와 「모델이 실패했다」를 가른다.** 앞은 부른 시각을 의심할 일이고
뒤는 코드나 프롬프트를 의심할 일인데, 예전 문구는 둘 다 「실패 N」이라 기록만 보고는
구분할 수 없었다.
*/
func indexNoteMessage(res service.IndexNoteResult) string {
	msg := fmt.Sprintf("지수 해설 %d건 생성 · 건너뜀 %d · 실패 %d (토큰 %d)",
		res.Generated, res.Skipped, res.Failed, res.Tokens)
	if res.HaltReason != "" {
		msg += fmt.Sprintf(" — 예산이 막아 남은 지수는 부르지 않았다: %s", res.HaltReason)
	}
	return msg
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
	/*
		기사 감성 라벨을 **먼저** 붙인다 — 점수가 그 라벨로 심리 축을 매긴다. 원장에도
		자기 이름으로 한 줄 남긴다(관리자 화면에서 「몇 건 붙었나」가 보이게). 실패해도
		점수는 계속 간다: 라벨이 없는 종목의 심리 축이 「없음」으로 남을 뿐이다.
	*/
	if deps.NewsSentiment != nil {
		service.RunScheduledJob(ctx, deps.Runs, model.BatchNewsSentiment,
			func(ctx context.Context) (map[string]any, string, error) {
				return deps.NewsSentiment.Run(ctx, 0)
			})
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

	/*
		오늘 랭킹 구성을 얇은 표에 남긴다 — **정리(prune)보다 먼저**, 같은 이유다.

		`stock_scores`는 30거래일에서 잘리므로 여기서 남기지 않으면 그날의 상위
		목록은 영영 사라진다. 백테스트가 읽는 과거는 이 한 줄이 만든다.

		백필도 여기서 부른다 — 이 기능이 처음 켜지는 날 표가 비어 있으면 백테스트는
		0거래일로 시작하는데, 점수 표에는 보관 기간만큼 과거가 이미 들어 있다.
		이미 채운 날짜는 건너뛰므로 평소에는 SELECT 한 번으로 끝난다.

		실패해도 배치를 멈추지 않는다: 점수와 화면 랭킹은 이미 갱신됐고, 빠진 날짜는
		다음 회차의 백필이 메운다.
	*/
	if deps.RankingHistory != nil {
		if err := deps.RankingHistory.SnapshotLatest(ctx); err != nil {
			log.Printf("worker-scores: ranking history snapshot: %v", err)
		}
		if filled, err := deps.RankingHistory.Backfill(ctx); err != nil {
			log.Printf("worker-scores: ranking history backfill: %v", err)
		} else if filled > 0 {
			log.Printf("worker-scores: ranking history backfilled %d dates", filled)
		}
	}

	/*
		변화 감지 — 점수 이력(위 스냅샷)이 오늘 날짜로 들어간 **뒤에** 돈다. 점수 급변을
		그 이력의 전날과 비교하기 때문이다. 실패해도 점수는 이미 끝났다.
	*/
	if deps.Changes != nil {
		service.RunScheduledJob(ctx, deps.Runs, model.BatchChangeDetect,
			func(ctx context.Context) (map[string]any, string, error) {
				return deps.Changes.Run(ctx, tradeDate)
			})
	}
	// 성적표 — 점수 이력에 새로 5거래일이 지난 날짜가 생기면 그날을 잰다.
	if deps.ReportCard != nil {
		service.RunScheduledJob(ctx, deps.Runs, model.BatchScoreReportCard,
			func(ctx context.Context) (map[string]any, string, error) {
				return deps.ReportCard.Compute(ctx)
			})
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
		// 예산이 닫혀 멈춘 것은 실패와 다른 사건이다 — 16:30 만회 슬롯이 이어받는다.
		if notes.HaltReason != "" {
			msg += fmt.Sprintf(" — 예산 중단: %s", notes.HaltReason)
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
