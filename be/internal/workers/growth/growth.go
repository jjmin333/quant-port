// Package growth is the worker-growth role (예전 단독 프로세스, 지금은 cmd/worker가 역할로 돌린다) is the standalone process for 검색 유입 감시.
//
// It crawls the public sitemap, pulls Search Console / GA4 / Bing figures when
// credentials are configured, counts sign-up and activation activity from
// MySQL, and writes a daily report (plus a weekly one on Mondays). It calls no
// LLM and writes only the growth_* tables — see internal/growth.
//
// It is its own container rather than a job on an existing worker because it
// is the only process that reads Google credentials, and because a slow or
// blocked crawl must not hold up the market-data jobs sharing a worker.
package growth

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/growth"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
)

// 07:30 KST — 리서치 판정(07:00) 뒤, 사람이 아침에 볼 수 있을 때. Search Console은 어차피
// 2~3일 늦으므로 시각은 크롤과 보고서가 정한다.
const (
	runHourKST   = 7
	runMinuteKST = 30
	schemaWait   = 3 * time.Minute
)

func kst() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.FixedZone("KST", 9*3600)
	}
	return loc
}

func nextRun(now time.Time) time.Time {
	local := now.In(kst())
	slot := time.Date(local.Year(), local.Month(), local.Day(), runHourKST, runMinuteKST, 0, 0, kst())
	if !slot.After(local) {
		slot = slot.AddDate(0, 0, 1)
	}
	return slot
}

// Run starts this role (공유 부트스트랩의 DB를 쓴다) and blocks until ctx is done.
func Run(ctx context.Context, deps *service.WorkerDeps) {
	mysql := deps.MySQL
	// 마이그레이터는 api 하나뿐이다 — 공용 표와 이 워커의 표가 보일 때까지 기다린다.
	if err := repository.VerifySchema(ctx, mysql.DB(), schemaWait); err != nil {
		log.Printf("worker-growth: schema: %v", err)
		return
	}
	if err := waitForTable(ctx, mysql.DB(), "growth_reports", schemaWait); err != nil {
		log.Printf("worker-growth: schema: %v", err)
		return
	}

	runs := repository.NewBatchJobRunRepo(mysql.DB())
	svc := growth.New(mysql.DB(), growth.LoadConfig())
	service.CloseInterruptedRuns(ctx, runs, model.BatchGrowthAudit)

	go runSchedule(ctx, runs, svc)

	log.Println("worker-growth: running (daily 07:30 KST · weekly report on Mondays · manual queue)")
	service.RunBatchQueueWorker(ctx, runs, model.BatchGrowthAudit,
		func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
			// limit=1 → 요일과 상관없이 주간 보고서도.
			weekly := false
			if v, ok := params["limit"].(float64); ok && v >= 1 {
				weekly = true
			}
			return svc.Run(ctx, weekly)
		})
}

func runSchedule(ctx context.Context, runs *repository.BatchJobRunRepo, svc *growth.Service) {
	// 한 슬롯은 한 번만(service.SlotGate 머리글).
	gate := service.NewSlotGate()
	for {
		slot := nextRun(time.Now())
		if !service.WaitForSlot(ctx, slot) {
			return
		}
		if !gate.Allow(slot) {
			continue
		}
		service.RunScheduledJob(ctx, runs, model.BatchGrowthAudit,
			func(ctx context.Context) (map[string]any, string, error) {
				return svc.Run(ctx, false)
			})
	}
}

func waitForTable(ctx context.Context, db *sql.DB, table string, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	for {
		var n int
		err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`, table).Scan(&n)
		if err == nil && n > 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("table %s not ready after %s — is the api container running its migrations?", table, wait)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}
