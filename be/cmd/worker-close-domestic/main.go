// Command worker-close-domestic is the standalone process for the domestic
// (KRX) daily-close batch.
//
// It owns two things, independent of every other worker: the KRX-close
// schedule trigger (+ the continuously-quoted KRW/USD rate, on weekdays), and
// the queue for manually requested domestic re-projections. A slow or stuck
// run here can never delay or block the overseas worker, and a crash here
// never touches the request-serving API.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
)

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
