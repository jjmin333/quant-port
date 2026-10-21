// Package fetch is the worker-fetch role (예전 단독 프로세스, 지금은 cmd/worker가 역할로 돌린다) is the standalone process for the rate-limited
// stock daily-close fetch jobs: missing symbols, stale sessions, and history gaps.
//
// All three share Toss's per-second candle budget with the live app, and running
// them as a single-process queue (rather than inside the API or alongside the
// close workers) guarantees they never run concurrently with each other and
// never compete with a live request for CPU or memory in the same process.
package fetch

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
)

/*
fetchDelay paces requests under Toss's shared candle limit (5/sec app-wide).

400ms(2.5/s)는 **예산의 절반만 쓰겠다**는 뜻이다. 250ms일 때는 4/s라 한도의 80%를
이 잡이 가져갔고, 마감 후 구멍 메우기가 전 종목을 도는 동안 API 로그가
`/api/v1/candles group=MARKET_DATA_CHART primary exhausted`로 덮였다 — 차트를 보는
사람이 SECONDARY 키로 밀린다.

느려지는 대신 잃는 것은 없다. 이 잡은 재개 가능하고(매 실행이 남은 대상을 다시
질의한다) 결과는 다음 세션 전까지만 있으면 된다. 국내 4,000종목 ≈ 27분,
해외 13,500종목 ≈ 90분이고 둘 다 마감 직후 한가한 구간에 돈다.
*/
const fetchDelay = 400 * time.Millisecond

type fetchKind int

const (
	fetchStale fetchKind = iota
	fetchMissing
	fetchHistory
	// fetchRefresh — 그날 종가가 있는 국내 종목을 NXT 마감 뒤 전부 다시 받는다(QP-21).
	fetchRefresh
)

// Run starts this role's schedules and manual queues and blocks until ctx is done.
// 부트스트랩·종료 신호는 cmd/worker가 한 번만 한다(여러 역할이 한 프로세스를 나눠 쓴다).
func Run(ctx context.Context, deps *service.WorkerDeps) {
	// 이 워커의 잡은 이 프로세스만 돌린다 — 재기동 전에 끊긴 실행을 지금 닫아 만회가 다시 넣게 한다.
	service.CloseInterruptedRuns(ctx, deps.Runs,
		model.BatchDailyCloseFetchHistory, model.BatchDailyCloseFetchMissing, model.BatchDailyCloseFetchStale,
		model.BatchDailyCloseRefreshDomestic)

	if deps.Toss == nil {
		log.Println("worker-fetch: Toss not configured — queue will accumulate until credentials are set")
	}

	log.Println("worker-fetch: running (manual queue only, fetch-stale + fetch-missing + fetch-history)")

	// Three job ids share one Toss budget, so a mutex serializes the actual work
	// even though each has its own queue-poll loop.
	var tossMu sync.Mutex
	go service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchDailyCloseFetchStale,
		handler(deps, fetchStale, &tossMu))
	go service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchDailyCloseRefreshDomestic,
		handler(deps, fetchRefresh, &tossMu))
	go service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchDailyCloseFetchMissing,
		handler(deps, fetchMissing, &tossMu))
	service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchDailyCloseFetchHistory,
		handler(deps, fetchHistory, &tossMu))
}

func handler(deps *service.WorkerDeps, kind fetchKind, tossMu *sync.Mutex) service.BatchWorkerHandler {
	return func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
		tossMu.Lock()
		defer tossMu.Unlock()
		if deps.Toss == nil {
			return nil, "", fmt.Errorf("Toss 자격증명이 없어 실행할 수 없습니다")
		}
		since, _ := params["since"].(string)
		if since == "" {
			since = service.DailyCloseBackfillStart
		}
		limit := 500
		if v, ok := params["limit"].(float64); ok && v > 0 {
			limit = int(v)
		}
		market, _ := params["market"].(string)
		assetTypes, err := service.ParseDailyCloseMarket(market)
		if err != nil {
			return nil, "", err
		}

		var (
			res       service.FetchMissingResult
			staleAsOf string
		)
		switch kind {
		case fetchStale:
			/*
				`asOf`는 **메우려는 세션의 날짜**다. 마감 후 자동 큐잉이 방금 끝난
				세션 날짜를 실어 보낸다 — 저장된 종가에서 유도하면 안 된다.

				`SymbolsStaleBefore`의 조건이 `newest < asOf`인데, 저장된 최신 종가를
				기준으로 삼으면 **지금 채우려는 그 세션이 기준이 되지 못한다**: 아무도
				그날 종가를 저장하지 않았으니 기준은 늘 한 세션 뒤에 머물고, 그날
				종가가 없는 종목이 조건에서 통째로 빠진다(실측 08-31 국내: 대상
				4,287 → 516). 파라미터가 없는 수동 실행만 예전 동작으로 떨어진다.
			*/
			asOf, _ := params["asOf"].(string)
			if asOf == "" {
				var e error
				if asOf, e = deps.Closes.LatestStoredTradeDate(ctx); e != nil || asOf == "" {
					return nil, "", fmt.Errorf("최신 거래일을 확인할 수 없습니다")
				}
			}
			res, err = deps.Closes.FetchStaleCloses(ctx, deps.Toss, deps.Bars, asOf, since, limit, fetchDelay, 0, assetTypes...)
			staleAsOf = asOf
		case fetchRefresh:
			asOf, _ := params["asOf"].(string)
			if asOf == "" {
				return nil, "", fmt.Errorf("asOf(세션 날짜)가 필요합니다")
			}
			res, since, err = deps.Closes.FetchRefreshCloses(ctx, deps.Toss, deps.Bars, asOf, fetchDelay, assetTypes...)
			staleAsOf = asOf
		case fetchMissing:
			res, err = deps.Closes.FetchMissingCloses(ctx, deps.Toss, deps.Bars, since, limit, fetchDelay, 0, assetTypes...)
		case fetchHistory:
			res, err = deps.Closes.FetchHistoryCloses(ctx, deps.Toss, deps.Bars, since, limit, fetchDelay, 0, assetTypes...)
		}
		if err != nil {
			return nil, "", err
		}
		// Project what was just fetched so the store reflects it immediately.
		// 구간을 나눠 돈다 — 한 문장으로 여덟 달치를 투영하면 전 종목 재수집 직후
		// readTimeout에 잘린다(BackfillStocksFromCandleBarsChunked 주석의 실측).
		if _, e := deps.Closes.BackfillStocksFromCandleBarsChunked(ctx, since, assetTypes...); e != nil {
			log.Printf("worker-fetch: project after fetch: %v", e)
		}
		out := map[string]any{
			"attempted": res.Attempted, "filled": res.Filled,
			"noData": res.Empty, "failed": res.Failed, "remainingBefore": res.Remaining,
			"tossUnknown": res.Unknown,
		}
		msg := fmt.Sprintf("시도 %d · 채움 %d · 데이터없음 %d · 실패 %d (실행 전 대상 %d)",
			res.Attempted, res.Filled, res.Empty, res.Failed, res.Remaining)
		// 「실패」와 「상대가 모름」은 다른 것이다 — 갈라 적지 않아서 해외 회차가
		// 매번 「실패 3,254」를 적고도 고장인지 정상인지 알 수 없었다.
		if res.Unknown > 0 {
			msg += fmt.Sprintf(" · 그중 미취급 %d(30일 제외)", res.Unknown)
		}

		/*
			**끝나고 나서 그 세션이 실제로 덮였는지 센다.**

			위 숫자들은 「무엇을 시도했나」만 말한다 — 대상 선정이 어긋나면 시도도
			채움도 성공처럼 보이면서 그날 종가는 비어 있다(08-31 국내가 정확히 그랬다:
			「시도 516 · 채움 391」로 done, 실제 커버리지는 4,287 중 391).

			커버리지는 결과를 직접 재므로 그 종류의 회귀가 배치 화면에 바로 드러난다.
		*/
		if (kind == fetchStale || kind == fetchRefresh) && staleAsOf != "" {
			have, total, e := deps.Closes.SessionCloseCoverage(ctx, staleAsOf, assetTypes...)
			if e != nil {
				log.Printf("worker-fetch: coverage %s: %v", staleAsOf, e)
			} else {
				out["coverageDate"] = staleAsOf
				out["coverageHave"] = have
				out["coverageTotal"] = total
				msg += fmt.Sprintf(" · %s 종가 보유 %d/%d", staleAsOf, have, total)
				if total > 0 && have*10 < total*9 {
					log.Printf("worker-fetch: WARNING %s 종가 커버리지 %d/%d (90%% 미만)", staleAsOf, have, total)
				}
			}
		}
		return out, msg, nil
	}
}
