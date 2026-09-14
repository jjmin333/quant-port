package service

import (
	"context"
	"fmt"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/marketdata/fnguide"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

// WorkerDeps is the common set of connections every worker process needs.
// Each worker binary is intentionally thin: it wires just the services its one
// job requires and runs a single scheduling/polling loop — see cmd/worker-*.
type WorkerDeps struct {
	Cfg   *config.Config
	MySQL *repository.MySQL
	Redis *repository.Redis
	Toss  *toss.Client // nil when Toss credentials are not configured
	Runs  *repository.BatchJobRunRepo
	// RefreshTokens는 만료 토큰 정리 배치가 쓴다. 이 표에는 삭제 코드가 없어
	// 활성 사용자당 연 365행씩 쌓이고 있었다(2026-08-23 실측 2,145행 만료 잔존).
	RefreshTokens *repository.RefreshTokenRepo
	Closes        *DailyCloseService
	Scores        *ScoreService
	// ScoreRankings precomputes AI 추천 랭킹의 축·기간 조합. 점수 배치 직후에
	// 돌아야 한다 — 그 결과를 그대로 재료로 쓴다.
	ScoreRankings *ScoreRankingService
	Indices       *IndexScoreService
	IndexFill     *IndexBackfillService
	Bars          *repository.CandleBarRepo
	// Research judges 리서치 콜 against stored closes. The verdict path needs
	// no quote/score/news collaborator — those only matter when publishing.
	Research *ResearchService
	// Earnings ingests 실적 발표 이벤트 (FnGuide 국내 · Finnhub 해외). API
	// 프로세스에는 이 서비스가 없다 — 두 소스 호출은 워커에만 있어야 한다.
	Earnings *EarningsIngestService
	// Insider ingests 내부자 거래 (DART 국내 · SEC EDGAR 미국). 같은 이유로
	// 워커 전용이다 — 이 호출들이 요청 경로에 있던 것이 이 기능의 원래 문제였다.
	Insider *InsiderIngestService
	// Reference ingests 분기 재무·공시·종목 경고. 조회 시점 외부 호출을 없앤
	// 나머지 세 경로가 여기로 왔다.
	Reference *ReferenceIngestService
	// TossUniverse는 「토스가 어떤 종목을 취급하는가」를 하루 7호출로 확정한다.
	// 그 판정이 다른 잡의 대상 선정과 화면 노출을 함께 정한다.
	TossUniverse *TossUniverseService
	// Health judges whether every batch ran on time. DB만 읽는다 —
	// 감시가 감시 대상보다 무거우면 안 된다(batch_health.go 머리글).
	Health *BatchHealthService
}

// workerSchemaWait bounds how long a worker waits for the api container to
// finish migrating before giving up. Generous on purpose: a cold deploy runs
// every migration for the first time, and the worker's restart policy retries
// anyway — failing fast here would just add churn.
const workerSchemaWait = 3 * time.Minute

// BootstrapWorker connects to MySQL/Redis, waits for the schema the api process
// migrates, and assembles the services shared across worker binaries. Callers
// close deps.MySQL/deps.Redis on shutdown.
func BootstrapWorker(ctx context.Context) (*WorkerDeps, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	mysql, err := repository.NewMySQL(cfg.MySQLDSN)
	if err != nil {
		return nil, fmt.Errorf("mysql: %w", err)
	}
	// 워커는 마이그레이션을 하지 않는다 — api 프로세스 하나만 마이그레이터다.
	// 자세한 배경은 repository.VerifySchema 주석 참조.
	if err := repository.VerifySchema(ctx, mysql.DB(), workerSchemaWait); err != nil {
		mysql.Close()
		return nil, fmt.Errorf("schema: %w", err)
	}

	redisConn, err := repository.NewRedis(cfg.RedisURL)
	if err != nil {
		mysql.Close()
		return nil, fmt.Errorf("redis: %w", err)
	}

	var tossClient *toss.Client
	if cfg.Toss.Enabled() {
		tossClient, err = toss.NewClient(cfg.Toss, toss.NewTokenStore(redisConn.Client()))
		if err != nil {
			mysql.Close()
			redisConn.Close()
			return nil, fmt.Errorf("toss: %w", err)
		}
		tossClient.StartTokenRefresh(ctx)
	}

	stockRepo := repository.NewStockRepo(mysql.DB())
	indexCloseRepo := repository.NewIndexDailyCloseRepo(mysql.DB())

	// Read-only index store: List() falls back to the Redis snapshot (written
	// by the API process's live poller) and then MySQL, so this never needs to
	// call Run() itself or hold Toss/FRED/etc clients — those are only used by
	// the refresh path, which workers do not touch.
	indexReader := NewMarketIndexStore(nil, nil, nil, redisConn.Client(), repository.NewMarketIndexRepo(mysql.DB()))

	closes := NewDailyCloseService(
		mysql.DB(),
		repository.NewStockDailyCloseRepo(mysql.DB()),
		indexCloseRepo,
		stockRepo,
		indexReader,
	)
	scoreSettings := NewScoreSettingsStore(repository.NewScoreSettingsRepo(mysql.DB()))
	scores := NewScoreService(mysql.DB(), repository.NewStockScoreRepo(mysql.DB()), scoreSettings)
	indexScores := NewIndexScoreService(mysql.DB(), indexCloseRepo)
	scores.SetIndexScores(indexScores)
	scoreRankings := NewScoreRankingService(
		repository.NewStockScoreRankingRepo(mysql.DB()),
		scores,
	)

	var indexFill *IndexBackfillService
	if tossClient != nil {
		indexFill = NewIndexBackfillService(mysql.DB(), indexCloseRepo, tossClient, cfg)
	}

	bars := repository.NewCandleBarRepo(mysql.DB())
	research := NewResearchService(
		repository.NewResearchRepo(mysql.DB()),
		repository.NewStockDailyCloseRepo(mysql.DB()),
		indexCloseRepo,
		stockRepo,
		bars,
	)

	// 워커의 외부 호출도 API 프로세스와 같은 apibudget(Redis 공유)에서 센다 —
	// 그래야 관리자 화면의 사용량이 사실이다.
	apiBudget := apibudget.NewManager(redisConn.Client())

	/*
		지수 해설용 모델 풀.

		워커에 LLM이 붙는 것은 여기가 처음이다 — 그동안 모델을 부르는 곳은 API
		프로세스의 뉴스 브리핑뿐이었다. 지수 해설은 **배치에서만** 도는 값이라
		(하루 열세 콜) 요청 경로에 둘 이유가 없고, 워커에 두면 조회가 아무리 늘어도
		지출이 그대로다. 키가 없으면 풀이 비활성이고, 그때는 지수마다 규칙 문장이
		그대로 남는다 — `index_llm_note.go` 참고.
	*/
	indexScores.SetLLM(llm.NewPoolFromKeys(llm.Config{
		Provider: cfg.LLM.Provider,
		BaseURL:  cfg.LLM.BaseURL,
		Model:    cfg.LLM.Model,
	}, cfg.LLM.KeyKR, cfg.LLM.KeyUS, apiBudget))

	earnings := NewEarningsIngestService(
		repository.NewEarningsEventRepo(mysql.DB()),
		stockRepo,
		fnguide.New(fnguide.Config{
			Enabled: cfg.FnGuide.Enabled,
			BaseURL: cfg.FnGuide.BaseURL,
		}, apiBudget),
		// 배치 전용 버킷 — 라이브 ETF 시세 폴러의 5분 간격에 걸려 하루 한 번
		// 도는 배치가 실패하지 않게 한다(첫 실행이 실제로 그렇게 죽었다).
		NewEarningsClient(EarningsConfig{FinnhubAPIKey: cfg.Finnhub.APIKey}, apiBudget).UseBatchBudget(),
	)

	// 공시 기반 적재 4종(내부자·재무·공시·경고). EDGAR는 전용 버킷으로 옮긴다 —
	// 요청 경로용 8/min으로는 수천 종목을 도는 배치가 두 번째 종목에서 멈춘다.
	//
	// dartGate 하나를 네 잡이 나눠 쓴다. DART 한도(1초 간격)는 provider 단위라,
	// 게이트가 없으면 겹친 두 잡이 서로를 "min interval not elapsed"로 실패시킨다.
	// 게이트는 그 판정을 실패 대신 **기다림**으로 바꾼다.
	dartGate := newProviderGate(apiBudget.LimitsFor(apibudget.ProviderDART).MinInterval)
	dartClient := NewDartClient(DartConfig{APIKey: cfg.Dart.APIKey}, apiBudget)
	edgarBatch := NewEdgarClient(EdgarConfig{UserAgent: cfg.Edgar.UserAgent}, apiBudget).UseBatchBudget()
	syncState := repository.NewSyncStateRepo(mysql.DB())
	// 구멍 메우기가 「Toss가 모른다」를 기억하게 한다 — 배선이 없으면 예전처럼
	// 매 회차 같은 종목을 다시 묻는다(DailyCloseService.SetSyncState 머리글).
	closes.SetSyncState(syncState)

	insider := NewInsiderIngestService(
		repository.NewInsiderRepo(mysql.DB()),
		syncState,
		dartClient,
		edgarBatch,
		dartGate,
	)
	reference := NewReferenceIngestService(
		repository.NewFinancialPeriodRepo(mysql.DB()),
		repository.NewDisclosureRepo(mysql.DB()),
		repository.NewWarningRepo(mysql.DB()),
		syncState,
		stockRepo,
		dartClient,
		edgarBatch,
		tossClient,
		dartGate,
	)

	runs := repository.NewBatchJobRunRepo(mysql.DB())
	// 워치독이 판정을 사건 원장에도 남긴다 — 15분마다 같은 로그를 찍는 대신
	// (kind, subject)당 한 행을 열고 닫는다(model.Incident 머리글).
	incidents := repository.NewOpsIncidentRepo(mysql.DB())
	health := NewBatchHealthService(runs, repository.NewBatchHealthRepo(mysql.DB()))
	health.SetIncidents(incidents)
	/*
		한도 소진을 원장에 남긴다. **워커에도 배선한다** — 2026-09-10에 DART를 다 쓴
		것은 api였지만 그 벽에 부딪힌 것은 워커의 배치 셋이었다. 둘 중 하나만 알면
		「누가 썼나」와 「누가 막혔나」가 서로 다른 표에 흩어진다.
	*/
	quotaReporter := NewQuotaIncidentReporter(apiBudget, incidents)
	quotaReporter.Install()
	health.SetQuotaReporter(quotaReporter)
	return &WorkerDeps{
		Cfg: cfg, MySQL: mysql, Redis: redisConn, Toss: tossClient,
		Runs:          runs,
		Health:        health,
		RefreshTokens: repository.NewRefreshTokenRepo(mysql.DB()),
		Closes:        closes,
		Scores:        scores,
		ScoreRankings: scoreRankings,
		Indices:       indexScores,
		IndexFill:     indexFill,
		Bars:          bars,
		Research:      research,
		Earnings:      earnings,
		Insider:       insider,
		Reference:     reference,
		TossUniverse:  NewTossUniverseService(tossClient, repository.NewTossUniverseRepo(mysql.DB())),
	}, nil
}

// Close releases the shared connections.
func (d *WorkerDeps) Close() {
	if d == nil {
		return
	}
	if d.Redis != nil {
		d.Redis.Close()
	}
	if d.MySQL != nil {
		d.MySQL.Close()
	}
}
