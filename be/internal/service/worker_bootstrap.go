package service

import (
	"context"
	"fmt"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/marketdata/fnguide"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

// WorkerDeps is the common set of connections every worker process needs.
// Each worker binary is intentionally thin: it wires just the services its one
// job requires and runs a single scheduling/polling loop — see cmd/worker-*.
type WorkerDeps struct {
	Cfg       *config.Config
	MySQL     *repository.MySQL
	Redis     *repository.Redis
	Toss      *toss.Client // nil when Toss credentials are not configured
	Runs      *repository.BatchJobRunRepo
	Closes    *DailyCloseService
	Scores    *ScoreService
	Indices   *IndexScoreService
	IndexFill *IndexBackfillService
	Bars      *repository.CandleBarRepo
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

	return &WorkerDeps{
		Cfg: cfg, MySQL: mysql, Redis: redisConn, Toss: tossClient,
		Runs:      repository.NewBatchJobRunRepo(mysql.DB()),
		Closes:    closes,
		Scores:    scores,
		Indices:   indexScores,
		IndexFill: indexFill,
		Bars:      bars,
		Research:  research,
		Earnings:  earnings,
		Insider:   insider,
		Reference: reference,
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
