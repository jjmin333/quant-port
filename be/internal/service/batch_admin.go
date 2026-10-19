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
	// coverage caches coverageFor results — see batch_admin_coverage.go.
	coverage batchCoverageCache
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
		Schedule:     "KRX 마감 +20분 (worker-market)",
		AcceptsSince: true,
	},
	{
		ID:           model.BatchDailyCloseOverseas,
		Category:     model.CategoryDerive,
		Label:        "일별 종가 — 해외",
		Purpose:      "미국 상장 종목·나스닥/S&P/다우/VIX/금/비트코인의 일별 종가를 candle_bars에서 투영합니다.",
		Cost:         model.CostFree,
		Schedule:     "US 마감 +20분 (worker-market)",
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
		Schedule: "매일 01:00 KST (worker-stock)",
	},
	{
		ID:           model.BatchSectorSync,
		Category:     model.CategoryIngest,
		Label:        "섹터 분류",
		Purpose:      "종목별 산업 코드(국내 DART 기업개황의 KSIC · 미국 SEC의 SIC)를 받아 공통 섹터 11개로 옮겨 stock_sectors에 적재합니다. 점수 피어그룹과 포트폴리오 섹터 노출의 재료입니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 02:30 KST (worker-stock)",
		AcceptsLimit: true,
		Note:         "limit은 국내·미국 각각에 적용됩니다(기본 국내 1,000 · 미국 3,000). 종목당 1호출이고, 90일 안에 받은 종목은 건너뜁니다 — 첫 바퀴(국내 약 3일 · 미국 약 2일) 뒤에는 신규 상장과 90일 지난 종목만 받습니다. 옮길 수 없는 코드는 섹터를 비워 둡니다(짐작하지 않습니다).",
	},
	{
		ID:           model.BatchETFScore,
		Category:     model.CategoryIngest,
		Label:        "국내 ETF 점검",
		Purpose:      "KRX ETF 일별 자료(NAV·순자산·거래대금·기초지수)를 적재하고, 같은 비교 집단(일반 / 채권·금리 / 레버리지·인버스) 안에서 괴리율·추적오차·유동성·규모를 백분위로 매겨 ETF 점검 점수를 냅니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 10:00 KST (worker-stock)",
		AcceptsLimit: true,
		Note:         "KRX는 다음 영업일에 공개합니다. 마지막 적재일 다음 날부터 어제까지 평일마다 1호출(7초 간격)이고, 처음에는 limit(달력일, 기본 90)만큼 거슬러 받습니다. 총보수는 소스가 없어 점수에 없습니다.",
	},
	{
		ID:           model.BatchInvestorFlow,
		Category:     model.CategoryIngest,
		Label:        "수급(투자자별 순매수)",
		Purpose:      "국내 보통주의 외국인·기관·개인 순매수 주식 수를 Toss 매매동향(STOCK_TRADING_TREND 버킷)에서 받아 stock_investor_flows에 적재합니다. 심리·수급 축의 수급 재료입니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 20:30 KST (worker-stock)",
		AcceptsLimit: true,
		Note:         "limit은 종목 수입니다(기본 전 종목 약 2,760). 종목당 1호출 · 0.5초 간격이고, 처음 보는 종목은 한 호출로 60일을 받습니다. 사용자 시세(MARKET_DATA)와 다른 버킷이라 화면 시세와 다투지 않습니다. 확정치(개인 값이 있는 기록)만 저장합니다.",
	},
	{
		ID:           model.BatchExtMetrics,
		Category:     model.CategoryIngest,
		Label:        "해외 지표 보조 · 애널리스트 의견",
		Purpose:      "Finnhub /stock/metric(PER·PBR·PSR·ROE·이익률·성장률, 본국 상장 기준)과 추천 추이(강력매수~강력매도 명수)를 받습니다. 지표는 EDGAR 값이 없을 때만(20-F 현지 통화 ADR 등) 점수·화면에 씁니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 03:00 KST (worker-stock)",
		AcceptsLimit: true,
		Note:         "limit은 종목 수(기본 1,500, 관심·보유 먼저 → 오래된 순). 종목당 최대 2호출 · 3초 간격(finnhub_metrics 20/분)이라 약 2.5시간, 한 바퀴 약 5일. 목표가는 무료 등급에 없어 받지 않습니다.",
	},
	{
		ID:           model.BatchAnalystDomestic,
		Category:     model.CategoryIngest,
		Label:        "국내 목표가 · 투자의견",
		Purpose:      "네이버 증권 컨센서스(평균 목표가 · 투자의견 1~5)를 국내 보통주 전 종목에 대해 받습니다. 공개 API가 아니라 한도를 스스로 낮게 둡니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 06:30 KST (worker-stock)",
		AcceptsLimit: true,
		Note:         "limit은 종목 수(기본 전 종목 약 2,700). 종목당 1호출 · 1.2초 간격 ≈ 55분. 애널리스트가 없는 종목은 「내역 없음」으로 남습니다.",
	},
	{
		ID:       model.BatchSocialMentions,
		Category: model.CategoryIngest,
		Label:    "Reddit 종목 언급",
		Purpose:  "ApeWisdom 공개 API에서 Reddit 주식 서브레딧의 종목별 24시간 언급 수·전일 언급·추천 수·순위를 받아 하루 한 줄로 저장합니다. 오늘의 변화 「레딧」과 종목 커뮤니티 탭이 읽습니다.",
		Cost:     model.CostRateLimited,
		Schedule: "매일 00:05 KST (worker-stock)",
		Note:     "전 종목 목록 7페이지 + 서브레딧 9개 + 언급 상위 150종목의 종목 페이지(감성·언급 사용자·키워드) — 3초 간격 약 9분. 영어 낱말과 같은 티커(ON·ALL·AI 등)와 카탈로그 밖 티커는 버립니다. 감성은 점수 심리 축의 Reddit 성분(미국 종목, 작은 가중치)이 됩니다.",
	},
	{
		ID:       model.BatchFinProducts,
		Category: model.CategoryIngest,
		Label:    "금융상품 공시 (예·적금·대출)",
		Purpose:  "금감원 「금융상품 한눈에」 Open API에서 정기예금·적금·주택담보대출·전세자금대출·개인신용대출 공시와 금융회사 홈페이지를 받습니다. 금융상품 메뉴가 읽습니다.",
		Cost:     model.CostRateLimited,
		Schedule: "매일 05:10 KST (worker-asset)",
		Note:     "상품 5종 × 권역 5 + 회사 — 약 60호출 · 1초 간격. 키(FSS_FINLIFE_API_KEY)가 없으면 실패로 남습니다. 모든 권역이 성공한 종류만 이번에 안 보인 상품을 숨깁니다(지우지 않음).",
	},
	{
		ID:           model.BatchReTrades,
		Category:     model.CategoryIngest,
		Label:        "부동산 실거래 (국토부)",
		Purpose:      "국토부 실거래가 12종(아파트·오피스텔·연립·단독 매매/전월세, 분양권, 토지, 상업업무용, 공장창고)을 13개월 창으로 받고, 끝나면 모든 사용자 자산을 다시 평가합니다. 순자산·부동산 시세 화면이 읽습니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 03:30 KST (worker-asset)",
		AcceptsLimit: true,
		Note:         "지역 tier all(전국 시군구)은 아파트만, full(서울 25구 + 사용자가 등록한 지역)은 12종. 최근 3개월은 매일, 3~5개월 전은 주 1회. limit은 이번 회차 호출 상한(기본 6,000) — 첫 바퀴는 며칠에 나눠 찹니다. 전국 시군구 목록은 행안부 법정동코드 API(활용신청 필요), 안 되면 서울만.",
	},
	{
		ID:       model.BatchAssetScores,
		Category: model.CategoryDerive,
		Label:    "자산 점수 (부동산·금융상품·공매)",
		Purpose:  "아파트 단지(모멘텀·거래·전세가율·고점 위치), 시군구(주간 지수 4·12주·거래량·전세), 예적금·대출(금리·우대 의존·가입 제한·권역), 공매 회차(할인·유찰·주의 표지) 점수를 매기고 금융상품 공시월 금리 이력을 남깁니다.",
		Cost:     model.CostFree,
		Schedule: "매일 09:50 KST (worker-asset)",
		Note:     "DB만. 점수는 같은 묶음(시도·상품 종류·시도×용도) 안 백분위의 가중 평균 — 추천이 아니라 시장 지표입니다.",
	},
	{
		ID:       model.BatchAssetAI,
		Category: model.CategoryDerive,
		Label:    "자산 AI 해설",
		Purpose:  "금리 한 줄, 시군구·단지 시장 해설, 예적금 우대조건 구조화. 자산 전용 Gemini 키(GEMINI_SUB_API_KEY)만 씁니다.",
		Cost:     model.CostRateLimited,
		Schedule: "매일 10:20 KST (worker-asset)",
		Note:     "여러 항목을 한 콜에 묶고(지역·단지 6개, 우대조건 8개) 입력이 같으면 다시 부르지 않습니다. 키가 없거나 기능 버킷(llm_feat_asset)이 차면 규칙 문장.",
	},
	{
		ID:       model.BatchReIndex,
		Category: model.CategoryIngest,
		Label:    "부동산 지수 (한국부동산원)",
		Purpose:  "R-ONE 통계 11표 — 주간 아파트 매매·전세지수, 실거래가격지수, 월간 매매·월세지수, 전월세 전환율, 오피스텔 지수, 오피스·상가 임대가격지수·공실률.",
		Cost:     model.CostRateLimited,
		Schedule: "매일 06:00 KST (worker-asset)",
		Note:     "처음엔 표마다 전체 이력, 그 뒤엔 마지막 시점(정정 반영)부터. 끝나면 지역마다 주간 지수의 지역 id를 맞춰 둡니다(평가의 지수 보정).",
	},
	{
		ID:       model.BatchEcosRates,
		Category: model.CategoryIngest,
		Label:    "금리 (한국은행 ECOS)",
		Purpose:  "기준금리, 국고채 1~30년, 회사채 AA-·BBB-, CD·CP·KOFR, 예금·적금·주담대·전세·신용대출 평균금리. 순자산 금리 카드와 채권 평가가 읽습니다.",
		Cost:     model.CostRateLimited,
		Schedule: "매일 09:20 KST (worker-asset)",
		Note:     "처음엔 일별 5년·월별 2009년부터, 그 뒤엔 마지막 시점 1주(월은 3개월) 앞부터 다시 받습니다.",
	},
	{
		ID:           model.BatchOnbid,
		Category:     model.CategoryIngest,
		Label:        "부동산 공매 (캠코 온비드)",
		Purpose:      "앞으로 30일 안에 개찰하는 부동산 공매 공고 → 회차별 물건(감정가·최저입찰가·유찰횟수·상태) → 낙찰 회차의 낙찰가. 공매 화면이 읽습니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 07:40 KST (worker-asset)",
		AcceptsLimit: true,
		Note:         "온비드 API는 오퍼레이션당 개발계정 일 1,000. limit은 공고상세 호출 상한(기본 900). 입찰이 끝났는데 결과가 빈 공고는 다시 받습니다.",
	},
	{
		ID:           model.BatchSocialPosts,
		Category:     model.CategoryIngest,
		Label:        "Reddit 인기 글",
		Purpose:      "언급 상위 종목의 이번 주 인기 Reddit 글(제목·앞부분·링크)을 검색 RSS로 받고 제목·앞부분을 번역해 둡니다. 종목 커뮤니티 탭이 읽습니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 00:30 KST (worker-stock)",
		AcceptsLimit: true,
		Note:         "Reddit RSS가 분당 1회라 하루 약 80종목(약 85분). 상위 40종목은 매일, 나머지는 사흘마다. limit은 종목 수.",
	},
	{
		ID:       model.BatchStockScores,
		Category: model.CategoryDerive,
		Label:    "종목·지수 종합 점수",
		Purpose:  "전 종목·전 지수 종합 점수를 재계산하고 일별 종가 행에 미러링합니다. 분포 검증까지 수행합니다.",
		Cost:     model.CostFree,
		Schedule: "매일 00:25 KST (worker-stock)",
	},
	{
		ID:       model.BatchDataRetention,
		Category: model.CategoryDerive,
		Label:    "만료 데이터 정리",
		Purpose: "쓰이지 않는데 계속 쌓이는 행을 지웁니다. 현재 대상은 만료된 리프레시 토큰입니다 " +
			"— 로그인·갱신마다 한 행이 생기고 7일마다 회전하므로 활성 사용자당 연 365행씩 늘어납니다. " +
			"만료 후 7일이 지난 것만 지웁니다(재발급 직후의 「없는 토큰」과 「만료된 토큰」을 구분해야 합니다).",
		Cost:     model.CostFree,
		Schedule: "매일 04:30 KST (worker-stock)",
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
		Schedule: "매일 05:30 KST (worker-stock)",
	},
	{
		ID:           model.BatchIndexCloses,
		Category:     model.CategoryIngest,
		Label:        "지수 종가 이력 백필",
		Purpose:      "지수 일별 종가 이력을 Toss·FRED에서 백필합니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "수동 (worker-market)",
		AcceptsSince: true,
		Note:         "Toss·FRED를 호출합니다. btc·gold는 날짜 있는 이력 소스가 없어 백필 대상이 아닙니다.",
	},
	{
		ID:           model.BatchDailyCloseFetchStale,
		Category:     model.CategoryIngest,
		Label:        "일별 종가 — 누락 세션 채우기",
		Purpose:      "종가가 있으나 최신 세션이 빠진 종목을 Toss에서 다시 받아옵니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "수동 (worker-market)",
		AcceptsSince: true, AcceptsLimit: true,
		Note: "Toss 캔들 한도(초당 5회)를 앱과 공유합니다. 중단해도 진행분은 보존됩니다.",
	},
	{
		ID:           model.BatchDailyCloseFetchMissing,
		Category:     model.CategoryIngest,
		Label:        "일별 종가 — 미보유 종목 채우기",
		Purpose:      "종가가 전혀 없는 종목을 Toss에서 받아옵니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "수동 (worker-market)",
		AcceptsSince: true, AcceptsLimit: true,
		Note: "상장폐지·우선주 등 조회 불가 코드가 많아 실패율이 높습니다(정상).",
	},
	{
		ID:           model.BatchDailyCloseFetchHistory,
		Category:     model.CategoryIngest,
		Label:        "일별 종가 — 과거 구간 채우기",
		Purpose:      "종가가 있으나 기준일까지 닿지 않은 종목의 Toss 1d 캔들을 페이지네이션으로 받아옵니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "수동 (worker-market)",
		AcceptsSince: true, AcceptsLimit: true,
		Note: "실적 반응 백필용. market=us 로 US+ETF만 제한 가능.",
	},
	{
		ID:       model.BatchLLMSummaryDomestic,
		Category: model.CategoryLLM,
		Label:    "AI 요약 배치 — 국내",
		Purpose:  "마켓 레이더 1일 기준 상위 종목(파생상품 제외)의 AI 요약을 지금 즉시 생성합니다.",
		Cost:     model.CostRateLimited,
		Schedule: "KRX 마감 후 자동 (16:30~18:30 KST) · 수동 실행 가능 (worker-content)",
		Note:     "LLM 일일/분당 호출·토큰 한도를 앱의 온디맨드 조회·/admin/llm 수동 생성과 공유합니다. 오늘 이미 자동 실행되었어도 다시 실행되며 한도를 다시 소모합니다. 휴장일에는 대상이 없어 실패로 기록됩니다.",
	},
	{
		ID:       model.BatchLLMSummaryOverseas,
		Category: model.CategoryLLM,
		Label:    "AI 요약 배치 — 해외",
		Purpose:  "마켓 레이더 1일 기준 상위 종목(파생상품 제외)의 AI 요약을 지금 즉시 생성합니다.",
		Cost:     model.CostRateLimited,
		Schedule: "US 마감 후 자동 (18:00~20:00 ET) · 수동 실행 가능 (worker-content)",
		Note:     "LLM 일일/분당 호출·토큰 한도를 앱의 온디맨드 조회·/admin/llm 수동 생성과 공유합니다. 오늘 이미 자동 실행되었어도 다시 실행되며 한도를 다시 소모합니다. 휴장일에는 대상이 없어 실패로 기록됩니다.",
	},
	{
		ID:       model.BatchEarningsPreviewDomestic,
		Category: model.CategoryLLM,
		Label:    "실적 관전 포인트 — 국내",
		Purpose:  "향후 7일 안에 실적을 발표하는 국내 종목(보유·관심 먼저, 최대 30)의 「볼 것」 요약과, 끝에서 국내 실적 화면 맨 위 「AI 실적 요약」을 만듭니다.",
		Cost:     model.CostRateLimited,
		Schedule: "매일 07:10 · 18:50 KST 자동 · 수동 실행 가능 (api)",
		Note:     "기능 버킷 llm_feat_earnings(하루 60콜·12만 토큰)과 키 버킷을 모두 씁니다. 같은 사실표면 다시 만들지 않고, AI 글은 3일간 유지합니다(발표 전날 한 번 새로). 한도가 없으면 규칙 문장으로 저장됩니다.",
	},
	{
		ID:       model.BatchEarningsPreviewOverseas,
		Category: model.CategoryLLM,
		Label:    "실적 관전 포인트 — 해외",
		Purpose:  "향후 7일 안에 실적을 발표하는 해외 종목(보유·관심 먼저, 최대 30)의 「볼 것」 요약과, 끝에서 해외 실적 화면 맨 위 「AI 실적 요약」을 만듭니다.",
		Cost:     model.CostRateLimited,
		Schedule: "매일 07:10 · 21:30 KST 자동 · 수동 실행 가능 (api)",
		Note:     "기능 버킷 llm_feat_earnings를 국내와 나눠 씁니다. 한도가 없으면 규칙 문장으로 저장됩니다.",
	},
	{
		ID:       model.BatchSocialSummary,
		Category: model.CategoryLLM,
		Label:    "Reddit 커뮤니티 AI 요약",
		Purpose:  "언급 상위 40종목(해외)의 커뮤니티 탭 「AI 핵심 요약」 — 레딧 여론의 쟁점·분위기·근거를 씁니다. 커뮤니티 점수는 규칙 값입니다.",
		Cost:     model.CostRateLimited,
		Schedule: "매일 04:20 KST 자동 · 수동 실행 가능 (worker-content)",
		Note:     "기능 버킷 llm_feat_social(하루 50콜·15만 토큰)과 키 버킷을 모두 씁니다. 같은 스냅샷 날짜의 AI 글이 있으면 건너뜁니다. 한도가 없으면 규칙 문장으로 저장됩니다.",
	},
	{
		ID:       model.BatchResearchReview,
		Category: model.CategoryDerive,
		Label:    "리포트 검토 노트",
		Purpose:  "최근 30일 공개 리포트 중 새로 게시·수정된 것(한 번에 10건)의 주장과 데이터 대조 노트를 만듭니다.",
		Cost:     model.CostFree,
		Schedule: "30분마다(:05·:35) 자동 · 수동 실행 가능 (worker-content)",
		Note:     "LLM을 쓰지 않습니다(2026-10-12) — 점수 방향·목표가 거리·이후 가격으로 만든 규칙 노트만 저장하고, 본문을 고치면 다시 만듭니다.",
	},
	{
		ID:       model.BatchNewsBriefingDomestic,
		Category: model.CategoryLLM,
		Label:    "AI 뉴스 브리핑 — 국내",
		Purpose:  "국내주식 뉴스 화면의 AI 시장 브리핑을 지금 즉시 다시 생성합니다.",
		Cost:     model.CostRateLimited,
		Schedule: "장중 30분 · 그 외 1시간 자동 · 수동 실행 가능 (worker-content)",
		Note:     "자동 실행은 직전 생성 이후 새 기사가 없으면 건너뜁니다. 수동 실행은 새 기사가 없어도 무조건 생성하며 LLM 한도를 소모합니다. LLM이 꺼져 있으면 규칙 기반 브리핑으로 저장됩니다.",
	},
	{
		ID:       model.BatchNewsBriefingOverseas,
		Category: model.CategoryLLM,
		Label:    "AI 뉴스 브리핑 — 해외",
		Purpose:  "해외주식 뉴스 화면의 AI 시장 브리핑을 지금 즉시 다시 생성합니다.",
		Cost:     model.CostRateLimited,
		Schedule: "장중 30분 · 그 외 1시간 자동 · 수동 실행 가능 (worker-content)",
		Note:     "자동 실행은 직전 생성 이후 새 기사가 없으면 건너뜁니다. 수동 실행은 새 기사가 없어도 무조건 생성하며 LLM 한도를 소모합니다. LLM이 꺼져 있으면 규칙 기반 브리핑으로 저장됩니다.",
	},
	{
		ID:       model.BatchScoreReportCard,
		Category: model.CategoryDerive,
		Label:    "AI 점수 성적표",
		Purpose:  "점수일마다 축 점수와 그 뒤 5거래일 수익률의 순위상관(IC)과 상위 20종목 초과수익을 재서 score_ic_daily에 쌓습니다. AI 랭킹 화면의 성적표가 읽습니다.",
		Cost:     model.CostFree,
		Schedule: "매일 00:25 점수 배치의 꼬리 (worker-stock) · 수동 실행 가능",
		Note:     "DB만 읽습니다. 미래를 보지 않습니다 — 수익률은 점수일 다음 세션부터 셉니다. 이미 잰 날짜는 건너뛰고, 아직 5거래일이 지나지 않은 날은 다음 회차가 잽니다.",
	},
	{
		ID:       model.BatchChangeDetect,
		Category: model.CategoryDerive,
		Label:    "변화 감지",
		Purpose:  "그날 전 종목의 급등락(종목별 변동성 기준) · 거래량 급증 · AI 점수 급변 · 뉴스 사건 · 주요 공시 · 실적 · 내부자 대량 매매를 규칙으로 골라 「변화」 메뉴에 올립니다.",
		Cost:     model.CostFree,
		Schedule: "매일 00:25 점수 배치의 마지막 단계 (worker-stock) · 수동 실행 가능",
		Note:     "DB만 읽습니다. 축이 생기거나 사라져서 바뀐 점수와, 종목의 15% 이상이 한꺼번에 뛴 날(비교 기준이 바뀐 날)의 점수 변화는 올리지 않습니다. 같은 날 사건도 기사도 없는 급등락은 「설명 불가」로 표시합니다. 휴장일 회차는 이미 올린 사건을 다시 올리지 않습니다.",
	},
	{
		ID:           model.BatchNewsSentiment,
		Category:     model.CategoryDerive,
		Label:        "뉴스 감성 라벨",
		Purpose:      "종목이 붙은 기사 제목에 감성 점수(로컬 모델 — 한국어 KR-FinBert-SC · 영어 FinBERT)와 사건 종류(목표가·실적·수주·증자 등, 규칙)를 매겨 news_article_sentiment에 둡니다. 점수 배치가 이 라벨로 전 종목 심리 축을 매깁니다.",
		Cost:         model.CostFree,
		Schedule:     "매일 00:25 점수 배치의 첫 단계 (worker-stock) · 수동 실행 가능",
		AcceptsLimit: true,
		Note:         "limit은 거슬러 볼 **일수**입니다(기본 3 · 최대 30). 이미 라벨이 있는 기사는 건너뛰므로 여러 번 눌러도 안전합니다. LLM을 부르지 않습니다 — 분류기 컨테이너(sentiment)가 없으면 0건으로 끝나고 심리 축은 「없음」으로 남습니다.",
	},
	{
		ID:       model.BatchIndexNotes,
		Category: model.CategoryLLM,
		Label:    "AI 지수 해설",
		Purpose:  "지수 13종이 각각 어떤 국면인지 한 문단으로 다시 씁니다.",
		Cost:     model.CostRateLimited,
		Schedule: "매일 00:25 자동 (점수 배치 직후, worker-stock) · 16:30 만회 · 수동 실행 가능",
		Note:     "지수당 1콜(최대 13콜)입니다. 자동 실행은 같은 거래일에 이미 해설이 있는 지수를 건너뛰지만, 수동 실행은 프롬프트를 고친 뒤 다시 보기 위한 것이라 무조건 덮어쓰며 한도를 다시 소모합니다. 16:30 만회 슬롯은 00:25에 못 채운 칸만 채웁니다 — Gemini 무료 한도가 태평양 자정(16:00 KST)에 리셋된 직후라 00:25보다 예산이 넉넉합니다. 숫자는 규칙이 계산해 넘기고 모델은 문장만 씁니다 — LLM이 꺼져 있으면 지표 나열 문장이 그대로 남습니다.",
	},
	{
		ID:       model.BatchWatchdog,
		Category: model.CategoryDerive,
		Label:    "배치 감시",
		Purpose: "모든 배치가 제 시각에 돌았는지 판정해 batch_health에 남깁니다. " +
			"원장은 실행된 것만 알기 때문에, 실행되지 않은 것은 기대 시각을 따로 계산해야 보입니다 " +
			"— 휴장이라 안 돈 것과 죽어서 안 돈 것을 가르는 것이 이 잡의 전부입니다.",
		Cost:     model.CostFree,
		Schedule: "15분마다 (worker-stock)",
		Note:     "DB만 읽고 판정만 합니다. 늦은 잡은 로그에 한 줄씩 남고, 관리자 화면 상단 배너로도 나갑니다. 이 잡 자신도 감시 대상입니다 — 마지막 판정이 낡으면 감시가 멈춘 것입니다. 오래된 원장 행(90일 초과) 정리도 여기서 함께 합니다.",
	},
	{
		ID:       model.BatchGrowthAudit,
		Category: model.CategoryDerive,
		Label:    "검색 유입 감시",
		Purpose: "사이트맵 URL을 모두 열어 색인 문제를 찾고, Search Console · GA4 · Bing · 가입·활동 지표를 모아 " +
			"일간 보고서를 만듭니다. 월요일에는 Top 20 기회가 담긴 주간 보고서를 함께 만듭니다.",
		Cost:         model.CostFree,
		Schedule:     "매일 07:30 KST (worker-stock) · 월요일 주간 보고서 · 수동 실행 가능",
		Note:         "LLM을 부르지 않고 growth_* 표와 보고서 파일만 씁니다 — 코드·점수·사용자 데이터는 바꾸지 않습니다. 크롤은 우리 사이트를 0.5초 간격 2개씩 엽니다(약 10분). Google·Bing 자격 증명이 없으면 그 소스는 건너뛰고 보고서에 「미설정」으로 남습니다. limit에 1을 주면 요일과 상관없이 주간 보고서도 만듭니다.",
		AcceptsLimit: true,
	},
	{
		ID:       model.BatchBotDrafts,
		Category: model.CategoryDerive,
		Label:    "봇 초안 (커뮤니티·리포트)",
		Purpose:  "그 시각의 브리핑 「많이 언급된 종목」·오늘의 변화·실적 프리뷰에서 종목을 가중 랜덤으로 골라 커뮤니티 초안 5개와 투자 리포트 초안 2개를 만들고, 초안마다 뽑은 작성자 유형(한 줄 글~컬럼)대로 AI가 다시 씁니다.",
		Cost:     model.CostRateLimited,
		Schedule: "KST 00·03·06·09·12·15·18·21시 (worker-stock) · 수동 실행 가능",
		Note:     "템플릿(저장된 AI 요약·브리핑·점수)을 먼저 만들고, 자산 전용 LLM 키(기능 버킷 llm_feat_bot, 하루 80콜·30만 토큰)로 다시 씁니다. 출력에 사실에 없는 숫자·권유 표현이 있으면 버리고 템플릿을 둡니다. 503이면 다음 회차가 12시간 안의 템플릿을 이어 씁니다. 초안은 바로 올라가지 않고 관리자 「봇 초안」 화면에서 고쳐서 게시 계정으로 올립니다. 게시하지 않은 초안은 24시간 뒤 지웁니다. 최근 24시간에 초안을 만든 종목은 다시 고르지 않습니다.",
	},
	{
		ID:       model.BatchResearchVerdict,
		Category: model.CategoryDerive,
		Label:    "투자 리포트 판정",
		Purpose:  "만기가 지났거나 목표가·손절가에 도달한 투자 리포트를 저장된 종가로 판정하고 알파를 계산합니다.",
		Cost:     model.CostFree,
		Schedule: "일별 종가 배치 이후 (worker-stock)",
		Note:     "저장된 종가만 읽으므로 외부 호출이 없습니다. 일별 종가가 아직 안 들어온 날 실행하면 그날 만기 리포트는 다음 실행으로 넘어갑니다.",
	},
	{
		ID:           model.BatchEarningsDomestic,
		Category:     model.CategoryIngest,
		Label:        "실적 발표 — 국내",
		Purpose:      "FnGuide 잠정실적발표(발표일 · 컨센서스 추정치 · 발표치)를 월 단위로 받아 earnings_events에 적재합니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "시즌 10:00–20:00 /2h · 비시즌 18:30 KST (worker-stock, 기동 catch-up)",
		AcceptsSince: true,
		Note:         "시즌(1–2·4–5·7–8·10–11월)에는 당월을 2시간마다 다시 받습니다 — FnGuide가 이미 지난 날짜에 개별·발표치를 나중에 붙이기 때문입니다. 20시와 비시즌 18:30만 전월을 겹칩니다(정정). 기준일을 주면 그 달부터 지금까지를 월별로 받습니다. 월당 1호출이라 시즌에도 하루 7회입니다.",
	},
	{
		ID:       model.BatchEarningsOverseas,
		Category: model.CategoryIngest,
		Label:    "실적 발표 — 해외",
		Purpose:  "Finnhub 실적 캘린더(예정일 · 장전/장후 · EPS·매출 추정치와 실적)를 받아 earnings_events에 적재합니다.",
		Cost:     model.CostRateLimited,
		Schedule: "매일 06:00 전체 창 + 5분 당일 폴링 (worker-stock, 기동 catch-up)",
		Note:     "06:00은 과거 49일 + 향후 76일(7일 조각 18회)로 일정 정정과 먼 미래를 받습니다. 00:00부터 5분마다 KST 어제~오늘만 다시 받아 장전·장후 실적치를 따라갑니다(1회). 전체 창이 도는 동안 5분 폴링은 건너뜁니다. 워커가 06:00 이후에 뜨면 그날 전체 창을 기동 직후 한 번 메웁니다. 우리 종목 카탈로그에 없는 심볼은 버립니다. Finnhub가 실적치를 안 준 지난 「예정」은 DB에 남기고, 화면은 KST 어제보다 오래된 예정만 숨깁니다(8/21 결정 — 어제까지는 미국 세션 시차).",
	},
	{
		ID:           model.BatchInsiderDomestic,
		Category:     model.CategoryIngest,
		Label:        "내부자 거래 — 국내",
		Purpose:      "DART 임원ㆍ주요주주 소유보고를 종목별로 받아 insider_transactions에 적재합니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 20:00 KST (worker-stock)",
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
		Schedule:     "매일 07:00 KST (worker-stock)",
		Note:         "증분입니다 — 이미 저장한 accession은 다시 내려받지 않으므로 평시 종목당 1~2호출입니다. 첫 적재만 종목당 최대 21호출(목록 1 + 공시 20)입니다. 조회 경로와 분리된 edgar_bulk 버킷(분당 200회 페이스)을 씁니다.",
	},
	{
		ID:           model.BatchFinancialsDomestic,
		Category:     model.CategoryIngest,
		Label:        "분기 재무 — 국내",
		Purpose:      "DART 분기 재무를 받아 financial_periods에 적재합니다. 재무제표 상세 모달과 실적 탭 「분기 실적」이 이 표를 함께 읽습니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 22:00 KST (worker-stock)",
		AcceptsLimit: true,
		Note:         "종목당 DART 호출이 20회를 넘습니다(연도 4벌 × 보고서 4종 + 누적 조회). 1초 간격이라 종목 하나에 20초 이상 걸리므로 기본 대상은 400종목(약 2.5시간)입니다. 분기값과 누적값을 **둘 다** 담습니다 — 상세 모달은 분기, 국내 실적 탭은 누적을 봅니다.",
	},
	{
		ID:           model.BatchFinancialsOverseas,
		Category:     model.CategoryIngest,
		Label:        "분기 재무 — 해외",
		Purpose:      "SEC EDGAR companyfacts에서 분기 재무를 받아 financial_periods에 적재합니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 09:00 KST (worker-stock)",
		AcceptsLimit: true,
		Note:         "종목당 companyfacts 1호출이라 국내보다 훨씬 쌉니다. edgar_bulk 버킷을 내부자 거래 배치와 공유하되 시각을 두 시간 벌려 둡니다.",
	},
	{
		ID:           model.BatchDisclosures,
		Category:     model.CategoryIngest,
		Label:        "공시 — 시장 전체 스윕",
		Purpose:      "DART 접수 공시를 날짜 단위로 통째로 받아 stock_disclosures에 적재합니다. 「오늘의 공시」와 국내 실적 탭의 최근 공시가 이 표를 읽습니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 19:30 KST (worker-stock)",
		AcceptsLimit: true,
		Note:         "종목별이 아니라 **날짜별**입니다 — corp_code 없이 부르면 그날 접수분 전체가 페이지로 옵니다(실측 397건/일 = 4호출). 종목별로 돌면 4,400호출이 드는 일입니다. 「최대 종목」칸은 여기서 **거슬러 받을 일수**로 쓰입니다(기본 1일, 놓친 날 메우기용).",
	},
	{
		ID:           model.BatchWarnings,
		Category:     model.CategoryIngest,
		Label:        "종목 경고 — 전 종목",
		Purpose:      "투자주의·투자경고·거래정지 등 지정 상태를 Toss에서 받아 stock_warnings에 적재합니다.",
		Cost:         model.CostRateLimited,
		Schedule:     "매일 01:30 KST (worker-stock)",
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
//
// 커버리지는 캐시에서 기다리지 않고 싣는다(batch_admin_coverage.go). 아직 한 번도
// 세지 못했으면 coveragePending이 참이다.
func (s *BatchAdminService) Catalog(ctx context.Context) (jobs []model.BatchJobStatus, coveragePending bool) {
	now := time.Now()
	coverage, coveragePending := s.CoverageSnapshot()
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
		status.Coverage = coverage[spec.ID]
		out = append(out, status)
	}
	return out, coveragePending
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
	case model.BatchSectorSync:
		return map[string]any{
			"domestic": s.syncCoverage(ctx, model.SyncKindSectorDART, string(model.AssetDomestic)),
			"us":       s.syncCoverage(ctx, model.SyncKindSectorSEC, string(model.AssetUS)),
		}
	case model.BatchInsiderDomestic:
		return s.syncCoverage(ctx, model.SyncKindInsiderDART, string(model.AssetDomestic))
	case model.BatchInvestorFlow:
		return s.syncCoverage(ctx, model.SyncKindInvestorFlow, string(model.AssetDomestic))
	case model.BatchExtMetrics:
		return s.syncCoverage(ctx, model.SyncKindExtMetrics, string(model.AssetUS))
	case model.BatchAnalystDomestic:
		return s.syncCoverage(ctx, model.SyncKindAnalystNaver, string(model.AssetDomestic))
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
