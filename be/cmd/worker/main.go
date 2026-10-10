// Command worker runs one or more batch roles in a single process (2026-10-01 사용자 결정 —
// 워커 컨테이너 9개를 성격별로 합친다). 어떤 역할을 돌릴지는 WORKER_ROLES(쉼표 목록)가 정한다.
//
//	worker-market  close-domestic,close-overseas,fetch,index   시세·종가·지수
//	worker-stock   scores,filings,earnings,research,growth      주식 데이터·점수·감시
//	worker-asset   asset                                        부동산·금리·공매·금융상품 + 점수·AI
//
// LLM 배치(종목 요약·뉴스 브리핑·실적 관전·스크리너 사전·Reddit 요약·리포트 검토)는 의존 그래프가
// api와 같아 api 바이너리를 SERVER_ROLE=batch로 띄운다(worker-content).
//
// 역할은 예전 단독 프로세스와 같은 일을 같은 방식으로 한다 — 부트스트랩(MySQL·Redis·Toss·서비스
// 묶음)과 종료 신호만 한 번으로 모았다. 역할마다 고루틴 하나이고, 잡 사이의 겹침은 원래대로
// 각 잡의 claim 락과 provider 게이트가 막는다.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/internal/workers/asset"
	"github.com/enact/quant-port/be/internal/workers/closedomestic"
	"github.com/enact/quant-port/be/internal/workers/closeoverseas"
	"github.com/enact/quant-port/be/internal/workers/earnings"
	"github.com/enact/quant-port/be/internal/workers/fetch"
	"github.com/enact/quant-port/be/internal/workers/filings"
	"github.com/enact/quant-port/be/internal/workers/growth"
	"github.com/enact/quant-port/be/internal/workers/index"
	"github.com/enact/quant-port/be/internal/workers/research"
	"github.com/enact/quant-port/be/internal/workers/scores"
)

var roles = map[string]func(context.Context, *service.WorkerDeps){
	"close-domestic": closedomestic.Run,
	"close-overseas": closeoverseas.Run,
	"fetch":          fetch.Run,
	"index":          index.Run,
	"scores":         scores.Run,
	"filings":        filings.Run,
	"earnings":       earnings.Run,
	"research":       research.Run,
	"growth":         growth.Run,
	"asset":          asset.Run,
}

func main() {
	var picked []string
	for _, r := range strings.Split(os.Getenv("WORKER_ROLES"), ",") {
		if r = strings.TrimSpace(r); r == "" {
			continue
		}
		if _, ok := roles[r]; !ok {
			log.Fatalf("worker: unknown role %q", r)
		}
		picked = append(picked, r)
	}
	if len(picked) == 0 {
		log.Fatal("worker: WORKER_ROLES is empty")
	}

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

	var wg sync.WaitGroup
	for _, r := range picked {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			roles[name](ctx, deps)
			log.Printf("worker: role %s stopped", name)
		}(r)
	}
	log.Printf("worker: running roles %s", strings.Join(picked, ","))
	wg.Wait()
}
