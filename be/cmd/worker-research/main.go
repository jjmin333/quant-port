// Command worker-research is the standalone process for the 리서치 콜 verdict
// batch.
//
// It only reads stored closes, so it costs no upstream API budget and cannot
// be rate-limited. What it does need is ordering: the daily-close workers must
// have landed the session first, or a call maturing today would be judged
// against yesterday's price. Hence the schedule below sits well after both
// market close workers, and any call it cannot resolve simply stays open for
// the next run rather than being resolved with stale data.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
)

// verdictHourKST / verdictMinuteKST place the daily slot after both close
// workers have written the newest session:
//
//	KRX  close 15:30 KST + 20m  → ~15:50 KST (previous KST day)
//	US   close 16:00 ET + 20m   → ~05:20–06:20 KST (this morning, DST-dependent)
//
// 07:00 KST clears the later of the two with room to spare. Running earlier
// would judge a call maturing today against yesterday's US close.
const verdictHourKST = 7

const verdictMinuteKST = 0

func kstLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.FixedZone("KST", 9*3600)
	}
	return loc
}

// nextRun is the next verdict slot strictly after now.
func nextRun(now time.Time) time.Time {
	loc := kstLocation()
	local := now.In(loc)
	slot := time.Date(local.Year(), local.Month(), local.Day(), verdictHourKST, verdictMinuteKST, 0, 0, loc)
	if !slot.After(local) {
		slot = slot.AddDate(0, 0, 1)
	}
	return slot
}

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

	log.Println("worker-research: running (daily 07:00 KST + manual queue)")
	service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchResearchVerdict,
		func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
			return deps.Research.RunVerdicts(ctx)
		})
}

func runSchedule(ctx context.Context, deps *service.WorkerDeps) {
	for {
		wait := time.Until(nextRun(time.Now()))
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		service.RunScheduledJob(ctx, deps.Runs, model.BatchResearchVerdict,
			func(ctx context.Context) (map[string]any, string, error) {
				return deps.Research.RunVerdicts(ctx)
			})
	}
}
