// Command worker-close-overseas is the standalone process for the overseas
// (US-listed + always-on) daily-close batch.
//
// Mirrors worker-close-domestic on the US session instead of KRX, plus
// always-on instruments (BTC) that have no single market-close event. Kept as
// a fully separate process so a Toss/FRED slowdown on this side never delays
// the domestic ingest, and vice versa.
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

	schedules := []calendar.ScheduleID{calendar.ScheduleUS, calendar.ScheduleAlways}
	assetTypes := []model.QuoteAssetType{model.AssetUS, model.AssetETF}

	go deps.Closes.RunMarketScheduler(ctx, model.BatchDailyCloseOverseas, deps.Runs, schedules, assetTypes)

	log.Println("worker-close-overseas: running (US close +20m, always-on BTC)")
	service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchDailyCloseOverseas,
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
				fmt.Sprintf("%s 이후 해외 종목 재투영 완료 (영향 행 %d)", since, affected), nil
		})
}
