// Package filings is the worker-filings role (예전 단독 프로세스, 지금은 cmd/worker가 역할로 돌린다) is the standalone process for 공시·참조 데이터 적재:
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
//	02:30 섹터 분류   — DART·EDGAR 각 1호출/종목. 90일 신선도라 첫 바퀴 뒤엔 몇 분.
//	07:00 내부자 해외 — 미국 마감(05~06시) 직후. Form 4는 거래 2영업일 내 제출.
//	09:00 재무 해외   — 같은 edgar_bulk 버킷이라 내부자와 두 시간 벌린다.
//	10:00 ETF 점검    — KRX가 전 영업일 ETF 자료를 공개한 뒤. 하루 1호출.
//	19:30 공시 스윕   — DART 접수가 대체로 끝난 뒤. 하루 몇 호출로 끝난다.
//	20:00 내부자 국내 — 소유보고 접수가 장 마감 뒤에 몰린다.
//	20:30 수급       — Toss 매매동향 확정치가 저녁에 들어온 뒤. 국내 보통주 전 종목(≈23분).
//	22:00 재무 국내   — 가장 무거운 잡(종목당 20초). 다른 DART 잡이 끝난 뒤.
//
// DART를 쓰는 세 잡(공시·내부자 국내·재무 국내)은 시각을 벌려 두었지만, 수동
// 실행이 겹칠 수 있다. 그래서 시간표에만 기대지 않고 **게이트로도 막는다** —
// service.providerGate가 겹친 호출을 실패 대신 줄 세운다.
//
// 자세한 배경과 전체 시간표는 be/docs/BATCH_SCHEDULE.md.
package filings

import (
	"context"
	"log"
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
	// 섹터 분류 — DART·EDGAR를 쓰는 다른 잡이 없는 새벽. 첫 바퀴 뒤로는 몇 분이다.
	{model.BatchSectorSync, 2, 30},
	{model.BatchInsiderOverseas, 7, 0},
	{model.BatchFinancialsOverseas, 9, 0},
	{model.BatchETFScore, 10, 0},
	{model.BatchDisclosures, 19, 30},
	{model.BatchInsiderDomestic, 20, 0},
	// 수급 — Toss STOCK_TRADING_TREND 버킷(사용자 시세와 다른 버킷). 점수 배치(00:25) 전에 끝난다.
	{model.BatchInvestorFlow, 20, 30},
	{model.BatchFinancialsDomestic, 22, 0},
	// 외부 신호 (2026-09-28). 언급 스냅샷은 변화 감지(00:25) 전, Finnhub은 다른 Finnhub 배치가 없는 새벽,
	// 네이버는 장 전.
	{model.BatchSocialMentions, 0, 5},
	// Reddit 인기 글 — 언급 스냅샷 뒤. 분당 1회라 약 85분(Reddit만 쓰므로 다른 잡과 겹쳐도 된다).
	{model.BatchSocialPosts, 0, 30},
	{model.BatchExtMetrics, 3, 0},
	{model.BatchAnalystDomestic, 6, 30},
}

// slotJobs — 이 워커가 맡은 잡 전부(예약 슬롯 = 수동 큐 대상과 같다).
func slotJobs() []model.BatchJobID {
	out := make([]model.BatchJobID, 0, len(slots))
	for _, s := range slots {
		out = append(out, s.job)
	}
	return out
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

// Run starts this role's schedules and manual queues and blocks until ctx is done.
// 부트스트랩·종료 신호는 cmd/worker가 한 번만 한다(여러 역할이 한 프로세스를 나눠 쓴다).
func Run(ctx context.Context, deps *service.WorkerDeps) {
	// 이 워커의 잡은 이 프로세스만 돌린다 — 재기동 전에 끊긴 실행을 지금 닫아 만회가 다시 넣게 한다.
	service.CloseInterruptedRuns(ctx, deps.Runs, slotJobs()...)

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
	/*
		한 슬롯은 한 번만 (service.SlotGate 머리글). time.After가 몇 초 일찍 깨면 같은 슬롯을 다시 계산해
		두 번 돌았다 — 2026-09-29·30 fin_products 05:09:54 + 05:10:00(2026-10-01 발견).
	*/
	gate := service.NewSlotGate()
	for {
		at := nextSlot(time.Now(), s.hour, s.minute)
		if !service.WaitForSlot(ctx, at) {
			return
		}
		if !gate.Allow(at) {
			continue
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
	case model.BatchSectorSync:
		return deps.Sectors.Run(ctx, limit)
	case model.BatchInvestorFlow:
		return deps.InvestorFlows.Run(ctx, limit)
	case model.BatchETFScore:
		// limit = 처음 받을 때 거슬러 갈 달력일 수(기본 90).
		return deps.ETFScores.Run(ctx, limit)
	case model.BatchExtMetrics:
		return deps.ExternalSignals.RunOverseasMetrics(ctx, limit)
	case model.BatchAnalystDomestic:
		return deps.ExternalSignals.RunDomesticAnalyst(ctx, limit)
	case model.BatchSocialMentions:
		return deps.ExternalSignals.RunSocial(ctx)
	case model.BatchSocialPosts:
		return deps.ExternalSignals.RunSocialPosts(ctx, limit)
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
