// Package closedomestic is the worker-close-domestic role (예전 단독 프로세스, 지금은 cmd/worker가 역할로 돌린다) is the standalone process for the domestic
// (KRX) daily-close batch.
//
// It owns two things, independent of every other worker: the KRX-close
// schedule trigger (+ the continuously-quoted KRW/USD rate, on weekdays), and
// the queue for manually requested domestic re-projections. A slow or stuck
// run here can never delay or block the overseas worker, and a crash here
// never touches the request-serving API.
package closedomestic

import (
	"context"
	"fmt"
	"log"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
)

// Run starts this role's schedules and manual queues and blocks until ctx is done.
// 부트스트랩·종료 신호는 cmd/worker가 한 번만 한다(여러 역할이 한 프로세스를 나눠 쓴다).
func Run(ctx context.Context, deps *service.WorkerDeps) {
	// 이 워커의 잡은 이 프로세스만 돌린다 — 재기동 전에 끊긴 실행을 지금 닫아 만회가 다시 넣게 한다.
	service.CloseInterruptedRuns(ctx, deps.Runs, model.BatchDailyCloseDomestic)

	schedules := []calendar.ScheduleID{calendar.ScheduleKRX, calendar.ScheduleWeekdays}
	assetTypes := []model.QuoteAssetType{model.AssetDomestic}

	go deps.Closes.RunMarketScheduler(ctx, model.BatchDailyCloseDomestic, deps.Runs, schedules, assetTypes)

	log.Println("worker-close-domestic: running (KRX close +20m, weekday FX)")
	service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchDailyCloseDomestic,
		func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
			since, _ := params["since"].(string)
			if since == "" {
				since = service.DailyCloseBackfillStart
			}
			affected, err := deps.Closes.BackfillStocksFromCandleBars(ctx, since, assetTypes...)
			if err != nil {
				return nil, "", err
			}
			return map[string]any{"affectedRows": affected},
				fmt.Sprintf("%s 이후 국내 종목 재투영 완료 (영향 행 %d)", since, affected), nil
		})
}
