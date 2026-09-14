// Command worker-filings is the standalone process for 공시·참조 데이터 적재:
// 내부자 거래, 분기 재무, 공시 스윕, 종목 경고.
//
// # 왜 별 프로세스인가
//
// 이 데이터들은 원래 **사용자 요청이 곧 외부 호출**이었다. 종목 탭을 누르면 EDGAR에
// 종목당 4호출, 재무제표 모달을 열면 DART에 종목당 16호출, 「오늘의 공시」는 보유
// 종목 수만큼 DART를 불렀다. 전부 앱이 공유하는 좁은 한도를 통과해야 했으니,
// 사용자가 늘수록 데이터가 사라지는 구조였다.
//
// 적재는 여기, 조회는 API 프로세스. 둘은 표를 통해서만 만난다.
//
// # 시각 배치 (KST)
//
//	01:30 종목 경고   — 전 종목. KRX 마감 뒤이고 다른 배치가 없는 시간대.
//	07:00 내부자 해외 — 미국 마감(05~06시) 직후. Form 4는 거래 2영업일 내 제출.
//	09:00 재무 해외   — 같은 edgar_bulk 버킷이라 내부자와 두 시간 벌린다.
//	19:30 공시 스윕   — DART 접수가 대체로 끝난 뒤. 하루 몇 호출로 끝난다.
//	20:00 내부자 국내 — 소유보고 접수가 장 마감 뒤에 몰린다.
//	22:00 재무 국내   — 가장 무거운 잡(종목당 20초). 다른 DART 잡이 끝난 뒤.
//
// DART를 쓰는 세 잡(공시·내부자 국내·재무 국내)은 시각을 벌려 두었지만, 수동
// 실행이 겹칠 수 있다. 그래서 시간표에만 기대지 않고 **게이트로도 막는다** —
// service.providerGate가 겹친 호출을 실패 대신 줄 세운다.
//
// 자세한 배경과 전체 시간표는 be/docs/BATCH_SCHEDULE.md.
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

// slot is one scheduled job: when it fires (KST) and what it runs.
type slot struct {
	job    model.BatchJobID
	hour   int
	minute int
}

var slots = []slot{
	/*
		유니버스가 **가장 먼저** 돈다 (01:00).

		그날의 다른 잡들이 이 판정을 근거로 대상을 고른다 — 경고 스윕(01:30)과
		종가 구멍 메우기가 「토스가 모르는 종목」을 빼고 시작할 수 있어야 한다.
		거꾸로 두면 하루치 404를 그대로 치르고 다음 날부터 줄어든다.

		호출이 일곱뿐이라 30분 여유는 넉넉하다(마켓 사이 2초 간격 + 응답 시간).
	*/
	{model.BatchTossUniverse, 1, 0},
	{model.BatchWarnings, 1, 30},
	{model.BatchInsiderOverseas, 7, 0},
	{model.BatchFinancialsOverseas, 9, 0},
	{model.BatchDisclosures, 19, 30},
	{model.BatchInsiderDomestic, 20, 0},
	{model.BatchFinancialsDomestic, 22, 0},
}

func kstLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.FixedZone("KST", 9*3600)
	}
	return loc
}

// nextSlot is the next daily HH:MM KST strictly after now.
func nextSlot(now time.Time, hour, minute int) time.Time {
	loc := kstLocation()
	local := now.In(loc)
	at := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, loc)
	if !at.After(local) {
		at = at.AddDate(0, 0, 1)
	}
	return at
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

	for _, s := range slots {
		go runSchedule(ctx, deps, s)
	}

	// 수동 실행 큐는 잡마다 하나씩. 한 폴러로 합치지 않는 이유는
	// RunBatchQueueWorker가 잡 하나를 맡는 계약이고, 섞으면 한쪽의 긴 실행이
	// 다른 쪽 클릭을 막기 때문이다.
	for _, s := range slots[:len(slots)-1] {
		go runQueue(ctx, deps, s.job)
	}

	log.Printf("worker-filings: running (%d jobs + manual queue)", len(slots))
	runQueue(ctx, deps, slots[len(slots)-1].job)
}

func runSchedule(ctx context.Context, deps *service.WorkerDeps, s slot) {
	for {
		wait := time.Until(nextSlot(time.Now(), s.hour, s.minute))
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		service.RunScheduledJob(ctx, deps.Runs, s.job,
			func(ctx context.Context) (map[string]any, string, error) {
				// 예약 실행은 항상 기본 한도다. 더 크게 돌리는 것은 관리자가
				// limit을 주고 수동으로 한다.
				return dispatch(ctx, deps, s.job, 0)
			})
	}
}

func runQueue(ctx context.Context, deps *service.WorkerDeps, job model.BatchJobID) {
	service.RunBatchQueueWorker(ctx, deps.Runs, job,
		func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
			return dispatch(ctx, deps, job, limitFromParams(params))
		})
}

// dispatch is the one place that maps a job id to its ingest call.
func dispatch(ctx context.Context, deps *service.WorkerDeps, job model.BatchJobID, limit int) (map[string]any, string, error) {
	switch job {
	case model.BatchInsiderOverseas:
		return deps.Insider.IngestOverseas(ctx, limit)
	case model.BatchInsiderDomestic:
		return deps.Insider.IngestDomestic(ctx, limit)
	case model.BatchFinancialsOverseas:
		return deps.Reference.IngestFinancialsOverseas(ctx, limit)
	case model.BatchFinancialsDomestic:
		return deps.Reference.IngestFinancialsDomestic(ctx, limit)
	case model.BatchDisclosures:
		// 공시 스윕에서 limit은 종목 수가 아니라 **거슬러 받을 일수**다.
		// 종목 축이 없는 유일한 잡이라 같은 칸을 다른 뜻으로 쓴다.
		return deps.Reference.IngestDisclosures(ctx, limit)
	case model.BatchWarnings:
		return deps.Reference.IngestWarnings(ctx, limit)
	case model.BatchTossUniverse:
		// limit 은 쓰지 않는다 — 마켓 일곱을 다 받아야 반영되는 잡이라
		// 「몇 개만」이 성립하지 않는다(TossUniverseService.Run 머리글).
		return deps.TossUniverse.Run(ctx)
	}
	return nil, "", nil
}

// limitFromParams reads the admin console's optional 숫자 knob.
func limitFromParams(params map[string]any) int {
	raw, ok := params["limit"]
	if !ok {
		return 0
	}
	switch v := raw.(type) {
	case float64: // JSON round-trip
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}
