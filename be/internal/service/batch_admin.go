package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/metrics"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

// BatchAdminService is the API process's view of the batch system: read-only
// status/coverage, plus enqueueing manual runs.
//
// It deliberately does NOT execute any job itself. Each job runs in its own
// worker process (cmd/worker-*), polling batch_job_runs for rows queued here.
// That split is the whole point: a batch that hangs, leaks memory, or gets
// rate-limited for minutes can only ever affect its own small container, never
// the process serving live user requests.
type BatchAdminService struct {
	runs        *repository.BatchJobRunRepo
	dailyCloses *DailyCloseService
	scores      *ScoreService
	// llm supplies the AI 요약 batch's own coverage (last run per market) —
	// its execution still lives in the API process (RunBatchNow), unlike the
	// other jobs' dedicated worker processes, because it shares that
	// process's fundamentals/news/LLM client dependency graph.
	llm *StockSummaryService
	// research supplies the 리서치 콜 verdict job's coverage. Set separately
	// (SetResearch) so adding it did not churn every NewBatchAdminService call.
	research *ResearchService
	// earnings supplies 실적 발표 적재 커버리지. 이게 없으면 관리자 화면에서
	// "배치가 성공했지만 0건"과 "실제로 채워졌다"를 구분할 수 없다.
	earnings *repository.EarningsEventRepo
	// syncState supplies per-symbol 적재 커버리지(내부자·재무·경고). 등급제
	// 로테이션이라 "몇 %를 돌았고 가장 오래된 종목이 언제 것인가"가 유일한
	// 진행 지표다.
	syncState *repository.SyncStateRepo
	// disclosures supplies 공시 스윕 커버리지 — 여기만 종목이 아니라 날짜가 축이다.
	disclosures *repository.DisclosureRepo
	// health judges 「돌았어야 했는데 안 돌았다」. 워커의 워치독과 같은 판정기를
	// 쓰되 여기서는 **읽는 순간 다시 계산한다** — 화면이 15분 묵은 판정을 보여줄
	// 이유가 없다(batch_health.go 머리글).
	health *BatchHealthService
}

// SetHealth wires the batch health judge.
func (s *BatchAdminService) SetHealth(h *BatchHealthService) { s.health = h }

/*
Health — 지금 이 순간의 판정과, **감시 자체가 살아 있는지**.

워치독이 죽으면 판정이 조용히 멈추는데 화면은 그걸 알 방법이 없다. 그래서 저장된
보고서의 나이를 함께 돌려준다 — 내용이 아니라 나이만 쓴다.
*/
func (s *BatchAdminService) Health(ctx context.Context) model.BatchHealthView {
	now := time.Now()
	view := model.BatchHealthView{
		Report:        model.BatchHealthReport{ComputedAt: now, Jobs: []model.BatchJobHealth{}},
		WatchdogStale: true,
	}
	if s == nil || s.health == nil {
		return view
	}
	report, err := s.health.Report(ctx, now)
	if err != nil {
		log.Printf("batch health: %v", err)
		return view
	}
	view.Report = report
	watchdogAge := int64(-1)
	if age, ok := s.health.WatchdogAge(ctx, now); ok {
		seconds := int64(age.Seconds())
		view.WatchdogAgeSeconds = &seconds
		view.WatchdogStale = age > batchHealthStaleAfter
		watchdogAge = seconds
	}
	// 화면 밖으로도 내보낸다 — 콘솔을 아무도 안 열어도 외부 모니터가 볼 수 있게.
	metrics.SetBatchHealth(report.Unhealthy, watchdogAge)
	return view
}

// SetResearch wires the 리서치 콜 verdict coverage source.
func (s *BatchAdminService) SetResearch(r *ResearchService) { s.research = r }

// SetEarnings wires the 실적 발표 적재 coverage source.
func (s *BatchAdminService) SetEarnings(r *repository.EarningsEventRepo) { s.earnings = r }

// SetIngestCoverage wires the per-symbol 적재 상태와 공시 스윕 커버리지 소스.
func (s *BatchAdminService) SetIngestCoverage(sync *repository.SyncStateRepo, disclosures *repository.DisclosureRepo) {
	s.syncState = sync
	s.disclosures = disclosures
}

func NewBatchAdminService(
	runs *repository.BatchJobRunRepo,
	dailyCloses *DailyCloseService,
	scores *ScoreService,
	llmSummary *StockSummaryService,
) *BatchAdminService {
	return &BatchAdminService{runs: runs, dailyCloses: dailyCloses, scores: scores, llm: llmSummary}
}

// batchJobCatalog is the single source of truth for what the console offers.
// Schedule/worker text is descriptive only — the actual trigger lives in the
// corresponding cmd/worker-* binary.
var batchJobCatalog = []model.BatchJobSpec{
	{
		ID:           model.BatchDailyCloseDomestic,
		Category:     model.CategoryDerive,
		Label:        "일별 종가 — 국내",
		Purpose:      "KRX 종목·KOSPI/KOSDAQ/원달러 환율의 일별 종가를 candle_bars에서 투영합니다.",
		Cost:         model.CostFree,
		Schedule:     "KRX 마감 +20분 (worker-close-domestic)",
		AcceptsSince: true,
	},
	{
		ID:           model.BatchDailyCloseOverseas,
		Category:     model.CategoryDerive,
		Label:        "일별 종가 — 해외",
		Purpose:      "미국 상장 종목·나스닥/S&P/다우/VIX/금/비트코인의 일별 종가를 candle_bars에서 투영합니다.",
		Cost:         model.CostFree,
		Schedule:     "US 마감 +20분 (worker-close-overseas)",
		AcceptsSince: true,
	},
	{
		ID:       model.BatchTossUniverse,
		Category: model.CategoryIngest,
		Label:    "토스 유니버스",
		Purpose: "토스가 거래를 지원하는 종목 집합을 마켓 단위로 받아(하루 7호출) 카탈로그에 표시합니다. " +
			"그 집합에 없는 종목은 검색·목록에서 숨고, 수집 배치의 대상에서도 빠집니다 " +
			"— 예전에는 그 사실을 종목마다 404를 맞아 가며 배웠습니다(경고 스윕 호출의 19%). " +
			"같은 응답의 한국어 종목명으로 뉴스 검색 별칭도 갱신합니다(알파벳 A · 알파벳 C).",
		Cost:     model.CostFree,
		Schedule: "매일 01:00 KST (worker-filings)",
	},
	{
		ID:       model.BatchStockScores,
		Category: model.CategoryDerive,
		Label:    "종목·지수 종합 점수",
		Purpose:  "전 종목·전 지수 종합 점수를 재계산하고 일별 종가 행에 미러링합니다. 분포 검증까지 수행합니다.",
		Cost:     model.CostFree,
		Schedule: "매일 00:25 KST (worker-scores)",
	},
	{
		ID:       model.BatchDataRetention,
		Category: model.CategoryDerive,
		Label:    "만료 데이터 정리",
		Purpose: "쓰이지 않는데 계속 쌓이는 행을 지웁니다. 현재 대상은 만료된 리프레시 토큰입니다 " +
			"— 로그인·갱신마다 한 행이 생기고 7일마다 회전하므로 활성 사용자당 연 365행씩 늘어납니다. " +
			"만료 후 7일이 지난 것만 지웁니다(재발급 직후의 「없는 토큰」과 「만료된 토큰」을 구분해야 합니다).",
		Cost:     model.CostFree,
		Schedule: "매일 04:30 KST (worker-scores)",
	},
	{
		ID:       model.BatchCandleRollup,
		Category: model.CategoryDerive,
		Label:    "캔들 계층 롤업",
		Purpose: "오래된 인트라데이 봉의 해상도를 낮춰 보관합니다 " +
			"(1분→3일→5분→7일→10분→14일→30분→30일→60분→60일 만료). " +
			"삭제가 아니라 접기라 차트는 계속 그려지고 Toss 재호출도 늘지 않습니다. " +
			"되돌릴 수 없습니다 — 접힌 1분봉은 사라집니다.",
		Cost:     model.CostFree,
		Schedule: "매일 05:30 KST (worker-scores)",
	},
	{
		ID:           model.BatchIndexCloses,
		Category:     model.CategoryIngest,
		Label:        "지수 종가 이력 백필",
		Purpose:      "지수 일별 종가 이력을 Toss·FRED에서 백필합니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "수동 (worker-index)",
		AcceptsSince: true,
		Note:         "Toss·FRED를 호출합니다. btc·gold는 날짜 있는 이력 소스가 없어 백필 대상이 아닙니다.",
	},
	{
		ID:           model.BatchDailyCloseFetchStale,
		Category:     model.CategoryIngest,
		Label:        "일별 종가 — 누락 세션 채우기",
		Purpose:      "종가가 있으나 최신 세션이 빠진 종목을 Toss에서 다시 받아옵니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "수동 (worker-fetch)",
		AcceptsSince: true, AcceptsLimit: true,
		Note: "Toss 캔들 한도(초당 5회)를 앱과 공유합니다. 중단해도 진행분은 보존됩니다.",
	},
	{
		ID:           model.BatchDailyCloseFetchMissing,
		Category:     model.CategoryIngest,
		Label:        "일별 종가 — 미보유 종목 채우기",
		Purpose:      "종가가 전혀 없는 종목을 Toss에서 받아옵니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "수동 (worker-fetch)",
		AcceptsSince: true, AcceptsLimit: true,
		Note: "상장폐지·우선주 등 조회 불가 코드가 많아 실패율이 높습니다(정상).",
	},
	{
		ID:           model.BatchDailyCloseFetchHistory,
		Category:     model.CategoryIngest,
		Label:        "일별 종가 — 과거 구간 채우기",
		Purpose:      "종가가 있으나 기준일까지 닿지 않은 종목의 Toss 1d 캔들을 페이지네이션으로 받아옵니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "수동 (worker-fetch)",
		AcceptsSince: true, AcceptsLimit: true,
		Note: "실적 반응 백필용. market=us 로 US+ETF만 제한 가능.",
	},
	{
		ID:       model.BatchLLMSummaryDomestic,
		Category: model.CategoryLLM,
		Label:    "AI 요약 배치 — 국내",
		Purpose:  "마켓 레이더 1일 기준 상위 종목(파생상품 제외)의 AI 요약을 지금 즉시 생성합니다.",
		Cost:     model.CostRateLimited,
		Schedule: "KRX 마감 후 자동 (16:30~18:30 KST) · 수동 실행 가능",
		Note:     "LLM 일일/분당 호출·토큰 한도를 앱의 온디맨드 조회·/admin/llm 수동 생성과 공유합니다. 오늘 이미 자동 실행되었어도 다시 실행되며 한도를 다시 소모합니다. 휴장일에는 대상이 없어 실패로 기록됩니다.",
	},
	{
		ID:       model.BatchLLMSummaryOverseas,
		Category: model.CategoryLLM,
		Label:    "AI 요약 배치 — 해외",
		Purpose:  "마켓 레이더 1일 기준 상위 종목(파생상품 제외)의 AI 요약을 지금 즉시 생성합니다.",
		Cost:     model.CostRateLimited,
		Schedule: "US 마감 후 자동 (18:00~20:00 ET) · 수동 실행 가능",
		Note:     "LLM 일일/분당 호출·토큰 한도를 앱의 온디맨드 조회·/admin/llm 수동 생성과 공유합니다. 오늘 이미 자동 실행되었어도 다시 실행되며 한도를 다시 소모합니다. 휴장일에는 대상이 없어 실패로 기록됩니다.",
	},
	{
		ID:       model.BatchNewsBriefingDomestic,
		Category: model.CategoryLLM,
		Label:    "AI 뉴스 브리핑 — 국내",
		Purpose:  "국내주식 뉴스 화면의 AI 시장 브리핑을 지금 즉시 다시 생성합니다.",
		Cost:     model.CostRateLimited,
		Schedule: "장중 30분 · 그 외 1시간 자동 · 수동 실행 가능",
		Note:     "자동 실행은 직전 생성 이후 새 기사가 없으면 건너뜁니다. 수동 실행은 새 기사가 없어도 무조건 생성하며 LLM 한도를 소모합니다. LLM이 꺼져 있으면 규칙 기반 브리핑으로 저장됩니다.",
	},
	{
		ID:       model.BatchNewsBriefingOverseas,
		Category: model.CategoryLLM,
		Label:    "AI 뉴스 브리핑 — 해외",
		Purpose:  "해외주식 뉴스 화면의 AI 시장 브리핑을 지금 즉시 다시 생성합니다.",
		Cost:     model.CostRateLimited,
		Schedule: "장중 30분 · 그 외 1시간 자동 · 수동 실행 가능",
		Note:     "자동 실행은 직전 생성 이후 새 기사가 없으면 건너뜁니다. 수동 실행은 새 기사가 없어도 무조건 생성하며 LLM 한도를 소모합니다. LLM이 꺼져 있으면 규칙 기반 브리핑으로 저장됩니다.",
	},
	{
		ID:       model.BatchIndexNotes,
		Category: model.CategoryLLM,
		Label:    "AI 지수 해설",
		Purpose:  "지수 13종이 각각 어떤 국면인지 한 문단으로 다시 씁니다.",
		Cost:     model.CostRateLimited,
		Schedule: "매일 00:25 자동 (점수 배치 직후, worker-scores) · 수동 실행 가능",
		Note:     "지수당 1콜(최대 13콜)입니다. 자동 실행은 같은 거래일에 이미 해설이 있는 지수를 건너뛰지만, 수동 실행은 프롬프트를 고친 뒤 다시 보기 위한 것이라 무조건 덮어쓰며 한도를 다시 소모합니다. 숫자는 규칙이 계산해 넘기고 모델은 문장만 씁니다 — LLM이 꺼져 있으면 지표 나열 문장이 그대로 남습니다.",
	},
	{
		ID:       model.BatchWatchdog,
		Category: model.CategoryDerive,
		Label:    "배치 감시",
		Purpose: "모든 배치가 제 시각에 돌았는지 판정해 batch_health에 남깁니다. " +
			"원장은 실행된 것만 알기 때문에, 실행되지 않은 것은 기대 시각을 따로 계산해야 보입니다 " +
			"— 휴장이라 안 돈 것과 죽어서 안 돈 것을 가르는 것이 이 잡의 전부입니다.",
		Cost:     model.CostFree,
		Schedule: "15분마다 (worker-scores)",
		Note:     "DB만 읽고 판정만 합니다. 늦은 잡은 로그에 한 줄씩 남고, 관리자 화면 상단 배너로도 나갑니다. 이 잡 자신도 감시 대상입니다 — 마지막 판정이 낡으면 감시가 멈춘 것입니다. 오래된 원장 행(90일 초과) 정리도 여기서 함께 합니다.",
	},
	{
		ID:       model.BatchResearchVerdict,
		Category: model.CategoryDerive,
		Label:    "투자 리포트 판정",
		Purpose:  "만기가 지났거나 목표가·손절가에 도달한 투자 리포트를 저장된 종가로 판정하고 알파를 계산합니다.",
		Cost:     model.CostFree,
		Schedule: "일별 종가 배치 이후 (worker-research)",
		Note:     "저장된 종가만 읽으므로 외부 호출이 없습니다. 일별 종가가 아직 안 들어온 날 실행하면 그날 만기 리포트는 다음 실행으로 넘어갑니다.",
	},
	{
		ID:           model.BatchEarningsDomestic,
		Category:     model.CategoryIngest,
		Label:        "실적 발표 — 국내",
		Purpose:      "FnGuide 잠정실적발표(발표일 · 컨센서스 추정치 · 발표치)를 월 단위로 받아 earnings_events에 적재합니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "시즌 10:00–20:00 /2h · 비시즌 18:30 KST (worker-earnings, 기동 catch-up)",
		AcceptsSince: true,
		Note:         "시즌(1–2·4–5·7–8·10–11월)에는 당월을 2시간마다 다시 받습니다 — FnGuide가 이미 지난 날짜에 개별·발표치를 나중에 붙이기 때문입니다. 20시와 비시즌 18:30만 전월을 겹칩니다(정정). 기준일을 주면 그 달부터 지금까지를 월별로 받습니다. 월당 1호출이라 시즌에도 하루 7회입니다.",
	},
	{
		ID:       model.BatchEarningsOverseas,
		Category: model.CategoryIngest,
		Label:    "실적 발표 — 해외",
		Purpose:  "Finnhub 실적 캘린더(예정일 · 장전/장후 · EPS·매출 추정치와 실적)를 받아 earnings_events에 적재합니다.",
		Cost:     model.CostRateLimited,
		Schedule: "매일 06:00 전체 창 + 5분 당일 폴링 (worker-earnings, 기동 catch-up)",
		Note:     "06:00은 과거 49일 + 향후 76일(7일 조각 18회)로 일정 정정과 먼 미래를 받습니다. 00:00부터 5분마다 KST 어제~오늘만 다시 받아 장전·장후 실적치를 따라갑니다(1회). 전체 창이 도는 동안 5분 폴링은 건너뜁니다. 워커가 06:00 이후에 뜨면 그날 전체 창을 기동 직후 한 번 메웁니다. 우리 종목 카탈로그에 없는 심볼은 버립니다. Finnhub가 실적치를 안 준 지난 「예정」은 DB에 남기고, 화면은 KST 어제보다 오래된 예정만 숨깁니다(8/21 결정 — 어제까지는 미국 세션 시차).",
	},
	{
		ID:           model.BatchInsiderDomestic,
		Category:     model.CategoryIngest,
		Label:        "내부자 거래 — 국내",
		Purpose:      "DART 임원ㆍ주요주주 소유보고를 종목별로 받아 insider_transactions에 적재합니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 20:00 KST (worker-filings)",
		AcceptsLimit: true,
		Note:         "한 번에 도는 종목 수를 limit으로 조절합니다(기본 1,500). 대상은 등급제입니다 — 관심·보유 종목 중 하루 이상 지난 것이 먼저, 그다음 한 번도 안 받은 종목, 마지막으로 오래된 순입니다. 종목당 DART 1호출이고 종목 간 1.1초를 둡니다.",
	},
	{
		ID:           model.BatchInsiderOverseas,
		Category:     model.CategoryIngest,
		Label:        "내부자 거래 — 해외",
		Purpose:      "SEC Form 4(보통주 거래)를 종목별로 받아 insider_transactions에 적재합니다.",
		Cost:         model.CostRateLimited,
		AcceptsLimit: true,
		Schedule:     "매일 07:00 KST (worker-filings)",
		Note:         "증분입니다 — 이미 저장한 accession은 다시 내려받지 않으므로 평시 종목당 1~2호출입니다. 첫 적재만 종목당 최대 21호출(목록 1 + 공시 20)입니다. 조회 경로와 분리된 edgar_bulk 버킷(분당 200회 페이스)을 씁니다.",
	},
	{
		ID:           model.BatchFinancialsDomestic,
		Category:     model.CategoryIngest,
		Label:        "분기 재무 — 국내",
		Purpose:      "DART 분기 재무를 받아 financial_periods에 적재합니다. 재무제표 상세 모달과 실적 탭 「분기 실적」이 이 표를 함께 읽습니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 22:00 KST (worker-filings)",
		AcceptsLimit: true,
		Note:         "종목당 DART 호출이 20회를 넘습니다(연도 4벌 × 보고서 4종 + 누적 조회). 1초 간격이라 종목 하나에 20초 이상 걸리므로 기본 대상은 400종목(약 2.5시간)입니다. 분기값과 누적값을 **둘 다** 담습니다 — 상세 모달은 분기, 국내 실적 탭은 누적을 봅니다.",
	},
	{
		ID:           model.BatchFinancialsOverseas,
		Category:     model.CategoryIngest,
		Label:        "분기 재무 — 해외",
		Purpose:      "SEC EDGAR companyfacts에서 분기 재무를 받아 financial_periods에 적재합니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 09:00 KST (worker-filings)",
		AcceptsLimit: true,
		Note:         "종목당 companyfacts 1호출이라 국내보다 훨씬 쌉니다. edgar_bulk 버킷을 내부자 거래 배치와 공유하되 시각을 두 시간 벌려 둡니다.",
	},
	{
		ID:           model.BatchDisclosures,
		Category:     model.CategoryIngest,
		Label:        "공시 — 시장 전체 스윕",
		Purpose:      "DART 접수 공시를 날짜 단위로 통째로 받아 stock_disclosures에 적재합니다. 「오늘의 공시」와 국내 실적 탭의 최근 공시가 이 표를 읽습니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 19:30 KST (worker-filings)",
		AcceptsLimit: true,
		Note:         "종목별이 아니라 **날짜별**입니다 — corp_code 없이 부르면 그날 접수분 전체가 페이지로 옵니다(실측 397건/일 = 4호출). 종목별로 돌면 4,400호출이 드는 일입니다. 「최대 종목」칸은 여기서 **거슬러 받을 일수**로 쓰입니다(기본 1일, 놓친 날 메우기용).",
	},
	{
		ID:           model.BatchWarnings,
		Category:     model.CategoryIngest,
		Label:        "종목 경고 — 전 종목",
		Purpose:      "투자주의·투자경고·거래정지 등 지정 상태를 Toss에서 받아 stock_warnings에 적재합니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 01:30 KST (worker-filings)",
		AcceptsLimit: true,
		Note:         "여기만 등급제가 아니라 **전 종목 매일**입니다 — 경고는 하루 단위로 붙고 풀리므로 로테이션으로 며칠에 한 번 보면 이미 풀린 경고를 계속 달게 됩니다. 종목당 Toss 1호출, 250ms 간격이라 약 17,900종목에 2시간 안팎입니다. 간격은 실측으로 정했습니다 — Toss 경고 API는 초당 5회 제한이고, 120ms(초당 8회)로 돌렸더니 429가 났습니다. 429는 한 번 기다렸다 재시도합니다. 갱신은 종목별 **교체**라 해제된 지정이 남지 않습니다.",
	},
}

func (s *BatchAdminService) earningsCoverage(ctx context.Context, market string) map[string]any {
	if s.earnings == nil {
		return nil
	}
	rows, symbols, latest, err := s.earnings.Coverage(ctx, market)
	if err != nil {
		return nil
	}
	return map[string]any{"rows": rows, "symbols": symbols, "latestEventDate": latest}
}

func (s *BatchAdminService) syncCoverage(ctx context.Context, kind string, assetTypes ...string) map[string]any {
	if s.syncState == nil {
		return nil
	}
	out, err := s.syncState.Coverage(ctx, kind, assetTypes)
	if err != nil {
		return nil
	}
	return out
}

func (s *BatchAdminService) disclosureCoverage(ctx context.Context) map[string]any {
	if s.disclosures == nil {
		return nil
	}
	latest, total, err := s.disclosures.LatestDate(ctx)
	if err != nil {
		return nil
	}
	out := map[string]any{"rows": total}
	if latest != "" {
		out["latestFilingDate"] = latest
	}
	return out
}

func batchJobSpec(id model.BatchJobID) (model.BatchJobSpec, bool) {
	for _, spec := range batchJobCatalog {
		if spec.ID == id {
			return spec, true
		}
	}
	return model.BatchJobSpec{}, false
}

// BatchJobParams are the optional knobs a run accepts.
type BatchJobParams struct {
	Since  string
	Limit  int
	Market string
}

func (p BatchJobParams) validate() error {
	if p.Since != "" {
		if _, err := time.Parse("2006-01-02", p.Since); err != nil {
			return fmt.Errorf("잘못된 기준일 %q", p.Since)
		}
	}
	if p.Limit < 0 {
		return fmt.Errorf("limit은 0 이상이어야 합니다")
	}
	if p.Market != "" {
		if _, err := ParseDailyCloseMarket(p.Market); err != nil {
			return err
		}
	}
	return nil
}

func (p BatchJobParams) toMap() map[string]any {
	m := map[string]any{}
	if p.Since != "" {
		m["since"] = p.Since
	}
	if p.Limit > 0 {
		m["limit"] = p.Limit
	}
	if p.Market != "" {
		m["market"] = p.Market
	}
	return m
}

// Catalog returns every job with its latest run, current coverage, and judged
// health (see batch_expectation.go — 그 판정이 「휴장이라 안 돎」과 「죽어서 안 돎」을
// 가른다).
func (s *BatchAdminService) Catalog(ctx context.Context) []model.BatchJobStatus {
	now := time.Now()
	// 판정은 한 번의 질의로 모아 온다 — 잡마다 따로 물으면 화면 한 번에 40여 개다.
	var ledger map[string]repository.BatchLedgerRow
	if s.runs != nil {
		if rows, err := s.runs.LedgerRows(ctx, batchLedgerLookback); err == nil {
			ledger = rows
		}
	}

	out := make([]model.BatchJobStatus, 0, len(batchJobCatalog))
	for _, spec := range batchJobCatalog {
		status := model.BatchJobStatus{Spec: spec}
		if s.runs != nil {
			if last, err := s.runs.Latest(ctx, string(spec.ID)); err == nil {
				status.LastRun = last
				status.Running = last != nil && (last.Status == "running" || last.Status == "queued")
			}
		}
		if ledger != nil {
			health, expectedAt, _ := judgeBatchHealth(batchExpectations[spec.ID], ledger[string(spec.ID)], now)
			status.Health = health
			status.ExpectedAt = expectedAt
		}
		status.Coverage = s.coverageFor(ctx, spec.ID)
		out = append(out, status)
	}
	return out
}

// coverageFor answers "how complete is this job's output right now", which is
// the question an operator actually has when deciding whether to re-run.
func (s *BatchAdminService) coverageFor(ctx context.Context, id model.BatchJobID) map[string]any {
	if s.dailyCloses == nil {
		return nil
	}
	switch id {
	case model.BatchDailyCloseDomestic:
		symbols, total, latest, err := s.dailyCloses.CoverageSummary(ctx, model.AssetDomestic)
		if err != nil {
			return nil
		}
		return map[string]any{"symbols": symbols, "activeSymbols": total, "latestTradeDate": latest}
	case model.BatchDailyCloseOverseas:
		symbols, total, latest, err := s.dailyCloses.CoverageSummary(ctx, model.AssetUS, model.AssetETF)
		if err != nil {
			return nil
		}
		return map[string]any{"symbols": symbols, "activeSymbols": total, "latestTradeDate": latest}
	case model.BatchDailyCloseFetchStale, model.BatchDailyCloseFetchMissing, model.BatchDailyCloseFetchHistory:
		symbols, total, latest, err := s.dailyCloses.CoverageSummary(ctx)
		if err != nil {
			return nil
		}
		return map[string]any{"symbols": symbols, "activeSymbols": total, "latestTradeDate": latest}
	case model.BatchStockScores:
		if s.scores == nil {
			return nil
		}
		date, err := s.scores.LatestScoredDate(ctx)
		if err != nil || date == "" {
			return nil
		}
		rows, scored, err := s.scores.Stats(ctx, date)
		if err != nil {
			return nil
		}
		return map[string]any{"tradeDate": date, "rows": rows, "scored": scored}
	case model.BatchIndexCloses:
		indices, rows, latest, err := s.dailyCloses.IndexCoverageSummary(ctx)
		if err != nil {
			return nil
		}
		return map[string]any{"indices": indices, "rows": rows, "latestTradeDate": latest}
	case model.BatchLLMSummaryDomestic:
		return s.llmBatchCoverage(ctx, toss.MarketKR)
	case model.BatchLLMSummaryOverseas:
		return s.llmBatchCoverage(ctx, toss.MarketUS)
	case model.BatchEarningsDomestic:
		return s.earningsCoverage(ctx, string(model.AssetDomestic))
	case model.BatchEarningsOverseas:
		return s.earningsCoverage(ctx, string(model.AssetUS))
	case model.BatchInsiderDomestic:
		return s.syncCoverage(ctx, model.SyncKindInsiderDART, string(model.AssetDomestic))
	case model.BatchInsiderOverseas:
		return s.syncCoverage(ctx, model.SyncKindInsiderEdgar, string(model.AssetUS))
	case model.BatchFinancialsDomestic:
		return s.syncCoverage(ctx, model.SyncKindFinancialsDART, string(model.AssetDomestic))
	case model.BatchFinancialsOverseas:
		return s.syncCoverage(ctx, model.SyncKindFinancialsSEC, string(model.AssetUS))
	case model.BatchWarnings:
		return s.syncCoverage(ctx, model.SyncKindWarnings,
			string(model.AssetDomestic), string(model.AssetUS), string(model.AssetETF))
	case model.BatchDisclosures:
		return s.disclosureCoverage(ctx)
	case model.BatchResearchVerdict:
		if s.research == nil {
			return nil
		}
		counts, err := s.research.StatusCounts(ctx)
		if err != nil {
			return nil
		}
		return counts
	}
	return nil
}

// llmBatchCoverage reports the last recorded run for one market's AI 요약
// batch — schedule- and manual-triggered runs both land here since they
// share the same Redis record (see StockSummaryService.RunBatchNow).
func (s *BatchAdminService) llmBatchCoverage(ctx context.Context, market toss.MarketCountry) map[string]any {
	if s.llm == nil {
		return nil
	}
	status := s.llm.BatchStatus(ctx)
	for _, m := range status.Markets {
		if m.Market != string(market) {
			continue
		}
		out := map[string]any{}
		if m.LastRun != nil {
			out["tradeDate"] = m.LastRun.TradeDate
			out["considered"] = m.LastRun.Considered
			out["planned"] = m.LastRun.Planned
			out["succeeded"] = m.LastRun.Succeeded
			out["failed"] = m.LastRun.Failed
			out["skipped"] = m.LastRun.Skipped
		}
		return out
	}
	return nil
}

// History returns recent runs across all jobs.
func (s *BatchAdminService) History(ctx context.Context, limit int) ([]model.BatchJobRun, error) {
	if s.runs == nil {
		return nil, nil
	}
	return s.runs.Recent(ctx, limit)
}

// ErrBatchBusy is returned when the job already has a queued or running row —
// piling up duplicate requests would just make the worker's backlog longer,
// not the job finish sooner.
var ErrBatchBusy = fmt.Errorf("이미 대기 중이거나 실행 중인 작업입니다")

// Trigger enqueues a manual run. The corresponding worker process claims and
// executes it — this call never runs the job itself.
func (s *BatchAdminService) Trigger(ctx context.Context, id model.BatchJobID, params BatchJobParams) error {
	if _, ok := batchJobSpec(id); !ok {
		return fmt.Errorf("알 수 없는 배치: %s", id)
	}
	if err := params.validate(); err != nil {
		return err
	}
	if s.runs == nil {
		return fmt.Errorf("batch run store unavailable")
	}
	busy, err := s.runs.HasPending(ctx, string(id))
	if err != nil {
		return err
	}
	if busy {
		return ErrBatchBusy
	}
	_, err = s.runs.Enqueue(ctx, string(id), params.toMap())
	return err
}
