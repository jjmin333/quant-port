package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/handler"
	"github.com/enact/quant-port/be/internal/import/resolver"
	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/media"
	"github.com/enact/quant-port/be/internal/metrics"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/safego"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/internal/stocklogo"
	"github.com/enact/quant-port/be/internal/toss"
)

type Server struct {
	cfg   *config.Config
	http  *http.Server
	mysql *repository.MySQL
	redis *repository.Redis
	toss  *toss.Client
	auth  *service.AuthService
	users *repository.UserRepo

	// bgCancel stops every background poller/scheduler started by New.
	bgCancel context.CancelFunc
	// bgDone closes when bgCancel runs. Middleware that owns a goroutine (the
	// rate limiter's bucket sweeper) takes this so it stops with the rest.
	bgDone <-chan struct{}

	health           *handler.HealthHandler
	authHandler      *handler.AuthHandler
	quotes           *handler.QuotesHandler
	market           *handler.MarketHandler
	watchlist        *handler.WatchlistHandler
	journal          *handler.JournalHandler
	stamps           *handler.StampHandler
	sample           *handler.SampleHandler
	research         *handler.ResearchHandler
	userHoldings     *handler.UserHoldingsHandler
	holdingTrades    *handler.HoldingTradesHandler
	priceTargets     *handler.PriceTargetHandler
	userCashBalances *handler.UserCashBalancesHandler
	importJobs       *handler.ImportHandler
	mediaFiles       *handler.MediaHandler
	fundamentals     *handler.FundamentalsHandler
	earnings         *handler.EarningsHandler
	earningsCalendar *handler.EarningsCalendarHandler
	financialHistory *handler.FinancialHistoryHandler
	insider          *handler.InsiderHandler
	stockSummary     *handler.StockSummaryHandler
	dailyCloses      *handler.DailyCloseHandler
	scores           *handler.ScoreHandler
	news             *handler.NewsHandler
	admin            *handler.AdminHandler

	importJobsSvc *service.ImportJobService
}

func New(cfg *config.Config) (*Server, error) {
	// bgCtx drives every background poller/scheduler started below so Shutdown
	// can stop them instead of leaving them running against closed pools.
	bgCtx, bgCancel := context.WithCancel(context.Background())
	fail := func(err error) (*Server, error) {
		bgCancel()
		return nil, err
	}

	mysql, err := repository.NewMySQL(cfg.MySQLDSN)
	if err != nil {
		return fail(fmt.Errorf("mysql: %w", err))
	}

	migrationsDir := filepath.Join("migrations")
	if err := repository.EnsureSchema(bgCtx, mysql.DB(), migrationsDir); err != nil {
		mysql.Close()
		return fail(fmt.Errorf("mysql schema: %w", err))
	}

	manifestPath := os.Getenv("LOGO_MANIFEST_PATH")
	if manifestPath == "" {
		manifestPath = filepath.Join("..", "fe", "public", "logos", "manifest.json")
	}
	if err := stocklogo.LoadManifest(manifestPath); err != nil {
		log.Printf("stock logos: manifest load: %v", err)
	} else if m := stocklogo.GetManifest(); m != nil && len(m.Entries) > 0 {
		log.Printf("stock logos: manifest loaded (%d entries)", len(m.Entries))
	}

	redis, err := repository.NewRedis(cfg.RedisURL)
	if err != nil {
		mysql.Close()
		return fail(fmt.Errorf("redis: %w", err))
	}
	service.PurgeLegacyTerminalCacheKeys(bgCtx, redis.Client())

	userRepo := repository.NewUserRepo(mysql.DB())
	refreshRepo := repository.NewRefreshTokenRepo(mysql.DB())

	authSvc, err := service.NewAuthService(cfg.Auth, userRepo, refreshRepo)
	if err != nil {
		mysql.Close()
		redis.Close()
		return fail(fmt.Errorf("auth: %w", err))
	}

	var tossClient *toss.Client
	if cfg.Toss.Enabled() {
		tossClient, err = toss.NewClient(cfg.Toss, toss.NewTokenStore(redis.Client()))
		if err != nil {
			mysql.Close()
			redis.Close()
			return fail(fmt.Errorf("toss client: %w", err))
		}
		log.Println("toss open api global client enabled")
		tossClient.StartTokenRefresh(bgCtx)
	} else {
		log.Println("toss open api global client disabled")
	}

	watchlistRepo := repository.NewWatchlistRepo(mysql.DB())
	userHoldingsRepo := repository.NewUserHoldingsRepo(mysql.DB())
	userCashBalancesRepo := repository.NewUserCashBalancesRepo(mysql.DB())
	candleBarRepo := repository.NewCandleBarRepo(mysql.DB())
	quoteTerminalRepo := repository.NewQuoteTerminalRepo(mysql.DB())
	stockRepo := repository.NewStockRepo(mysql.DB())
	marketDataSettingsRepo := repository.NewMarketDataSettingsRepo(mysql.DB())
	marketDataSettings := service.NewMarketDataSettingsStore(marketDataSettingsRepo)

	fxRateRepo := repository.NewFxRateRepo(mysql.DB())
	fxSvc := service.NewExchangeRateService(tossClient, redis.Client(), fxRateRepo)
	go fxSvc.Run(bgCtx)

	rankingSnapshotRepo := repository.NewRankingSnapshotRepo(mysql.DB())
	marketRankingsRepo := repository.NewMarketRankingsRepo(mysql.DB())
	marketSvc := service.NewMarketService(tossClient, redis.Client(), fxSvc, stockRepo, rankingSnapshotRepo, marketRankingsRepo)
	marketSvc.SetDataSettings(marketDataSettings)
	marketSvc.SetWarnings(repository.NewWarningRepo(mysql.DB()))
	if n := marketSvc.WarmRankingsCache(bgCtx); n == 0 {
		log.Println("market rankings: no persisted rankings in DB/Redis yet")
	}
	go marketSvc.RunRankingsPoller(bgCtx)
	apiBudget := apibudget.NewManager(redis.Client())
	apiBudget.SetLimits(apibudget.ProviderOpenAI, llmLimits(cfg.LLM, apiBudget.LimitsFor(apibudget.ProviderOpenAI)))
	dartClient := service.NewDartClient(service.DartConfig{APIKey: cfg.Dart.APIKey}, apiBudget)
	earningsClient := service.NewEarningsClient(service.EarningsConfig{FinnhubAPIKey: cfg.Finnhub.APIKey}, apiBudget)
	marketStore := service.NewMarketDataStore(tossClient, redis.Client(), earningsClient, dartClient, userHoldingsRepo, watchlistRepo, quoteTerminalRepo)
	marketStore.SetDisclosures(repository.NewDisclosureRepo(mysql.DB()))
	go marketStore.Run(bgCtx)

	marketClients := marketdata.NewClients(cfg.MarketData, apiBudget, redis.Client())
	indexRepo := repository.NewMarketIndexRepo(mysql.DB())
	indexStore := service.NewMarketIndexStore(tossClient, marketClients, fxSvc, redis.Client(), indexRepo)
	indexStore.SetMarketStore(marketStore)
	quotesSvc := service.NewQuotesService(tossClient, marketClients, fxSvc, watchlistRepo, stockRepo, redis.Client(), candleBarRepo, quoteTerminalRepo)
	userHoldingsSvc := service.NewUserHoldingsService(userHoldingsRepo, redis.Client())
	userCashBalancesSvc := service.NewUserCashBalancesService(userCashBalancesRepo)
	holdingTradesRepo := repository.NewHoldingTradesRepo(mysql.DB())
	userHoldingsSvc.SetTradesRepo(holdingTradesRepo)
	holdingTradesSvc := service.NewHoldingTradesService(
		holdingTradesRepo,
		userHoldingsSvc,
		repository.NewStockDailyCloseRepo(mysql.DB()),
		fxSvc,
	)
	quotesSvc.SetUserHoldings(userHoldingsSvc)
	quotesSvc.SetIndexStore(indexStore)
	go indexStore.Run(bgCtx)
	quotesSvc.SetMarketStore(marketStore)
	quotesSvc.SetRankingsRepo(marketRankingsRepo)
	marketSvc.SetMarketStore(marketStore)
	marketSvc.SetQuotesService(quotesSvc)

	newsHTTP := &http.Client{Timeout: 15 * time.Second}
	naverNews := marketdata.NewNaverNewsClient(cfg.News.Naver.ClientID, cfg.News.Naver.ClientSecret, newsHTTP, apiBudget)
	kakaoNews := marketdata.NewKakaoNewsClient(cfg.News.Kakao.RestAPIKey, newsHTTP, apiBudget)
	rssNews := marketdata.NewRSSNewsClient(newsHTTP)
	finnhubNews := marketdata.NewFinnhubNewsClient(cfg.News.Finnhub.APIKey, newsHTTP, apiBudget)
	rssUSNews := marketdata.NewUSRSSNewsClient(newsHTTP)
	googleNews := marketdata.NewGoogleNewsRSSClient(newsHTTP)
	papagoNews := marketdata.NewPapagoClient(cfg.News.Papago.ClientID, cfg.News.Papago.ClientSecret, newsHTTP, apiBudget)
	newsRepo := repository.NewNewsRepo(mysql.DB())
	keywordRepo := repository.NewNewsKeywordRepo(mysql.DB())
	pollSettingsRepo := repository.NewNewsPollSettingsRepo(mysql.DB())
	readerTransRepo := repository.NewNewsReaderTranslationRepo(mysql.DB())
	llmClient := llm.NewClient(llm.Config{
		Provider: cfg.LLM.Provider,
		APIKey:   cfg.LLM.APIKey,
		BaseURL:  cfg.LLM.BaseURL,
		Model:    cfg.LLM.Model,
	}, apiBudget)
	newsStore := service.NewNewsStore(cfg.News, naverNews, kakaoNews, rssNews, finnhubNews, rssUSNews, googleNews, papagoNews, newsRepo, keywordRepo, pollSettingsRepo, stockRepo, watchlistRepo, userHoldingsRepo, redis.Client())
	newsStore.SetLLM(llmClient)
	go newsStore.Run(bgCtx)
	newsReader := service.NewNewsReaderService(redis.Client(), papagoNews, readerTransRepo, newsRepo)

	newsBriefingSvc := service.NewNewsBriefingService(
		newsStore, indexStore, quotesSvc, stockRepo,
		repository.NewNewsBriefingRepo(mysql.DB()),
	)
	newsBriefingSvc.SetLLM(llmClient)

	watchlistSvc := service.NewWatchlistService(watchlistRepo)
	watchlistSvc.SetQuotes(quotesSvc)
	journalRepo := repository.NewJournalNoteRepo(mysql.DB())
	journalSvc := service.NewJournalService(journalRepo)
	// A note stamps the linked stock's price at save time; a target stamps it at
	// registration. Both read only, and both stay usable when these are down.
	journalSvc.SetQuotes(quotesSvc)
	journalSvc.SetDailyCloses(repository.NewStockDailyCloseRepo(mysql.DB()))
	priceTargetRepo := repository.NewPriceTargetRepo(mysql.DB())
	priceTargetSvc := service.NewPriceTargetService(priceTargetRepo)
	priceTargetSvc.SetQuotes(quotesSvc)
	priceTargetSvc.SetDailyCloses(repository.NewStockDailyCloseRepo(mysql.DB()))
	// Reach/peak come from the same daily bars stamps use for early-hit checks.
	priceTargetSvc.SetCandles(candleBarRepo)
	importJobSvc := service.NewImportJobService(
		redis.Client(),
		userHoldingsSvc,
		watchlistSvc,
		resolver.New(stockRepo),
		cfg.Import.TempDir,
	)

	edgarClient := service.NewEdgarClient(service.EdgarConfig{UserAgent: cfg.Edgar.UserAgent}, apiBudget)
	fundamentalsRepo := repository.NewFundamentalsRepo(mysql.DB())
	fundamentalsSvc := service.NewFundamentalsService(dartClient, edgarClient, tossClient, stockRepo, fundamentalsRepo)
	fundamentalsSvc.SetMarketStore(marketStore)
	// 실적 이벤트 표는 워커가 채우고 API는 읽기만 한다 — FnGuide·Finnhub 호출은
	// 이 프로세스에 없다(조회 경로에서 외부 HTTP 금지).
	earningsEventRepo := repository.NewEarningsEventRepo(mysql.DB())
	earningsCalendarSvc := service.NewEarningsCalendarService(earningsEventRepo, redis.Client())
	newsBriefingSvc.SetEarnings(earningsCalendarSvc)
	newsBriefingSvc.SetToss(tossClient)
	newsBriefingSvc.SetRedis(redis.Client())
	go newsBriefingSvc.Run(bgCtx)
	// 이 세 표(분기 재무·공시·적재 상태)가 조회 시점 외부 호출을 없앤 결과다.
	// 채우는 쪽은 worker-filings, 읽는 쪽은 여기뿐이다.
	financialPeriodRepo := repository.NewFinancialPeriodRepo(mysql.DB())
	disclosureRepo := repository.NewDisclosureRepo(mysql.DB())
	syncStateRepo := repository.NewSyncStateRepo(mysql.DB())
	earningsTabSvc := service.NewEarningsTabService(stockRepo, financialPeriodRepo, disclosureRepo)
	// 종목 실적 탭도 같은 표를 1순위로 읽는다. 캘린더와 탭이 다른 소스를 보면
	// 같은 발표에 대해 서로 다른 서프라이즈를 말하게 된다.
	earningsTabSvc.SetEvents(earningsEventRepo, earningsCalendarSvc)
	financialHistorySvc := service.NewFinancialHistoryService(financialPeriodRepo, syncStateRepo, stockRepo)
	// 내부자 거래도 실적 이벤트와 같다 — 표는 worker-filings가 채우고 API는
	// 읽기만 한다. 예전에는 이 서비스가 요청마다 EDGAR/DART를 직접 불렀고,
	// 그게 동시 접속에서 빈 화면이 나오던 원인이었다.
	insiderRepo := repository.NewInsiderRepo(mysql.DB())
	insiderTabSvc := service.NewInsiderTabService(insiderRepo, syncStateRepo, stockRepo)
	llmSettingsRepo := repository.NewLLMSettingsRepo(mysql.DB())
	llmSettingsStore := service.NewLLMSettingsStore(llmSettingsRepo)
	stockSummarySvc := service.NewStockSummaryService(
		fundamentalsSvc, financialHistorySvc, earningsTabSvc, insiderTabSvc,
		newsStore, candleBarRepo, stockRepo, llmClient, redis.Client(),
		llmSettingsStore,
	)
	// The scheduled AI 요약 batch draws its targets from 마켓 레이더's stored
	// 1일 기준 rankings — no live Toss call, just what the poller already saved.
	stockSummarySvc.SetRankings(marketRankingsRepo)
	dailyCloseSvc := service.NewDailyCloseService(
		mysql.DB(),
		repository.NewStockDailyCloseRepo(mysql.DB()),
		repository.NewIndexDailyCloseRepo(mysql.DB()),
		stockRepo,
		indexStore,
	)
	stockSummarySvc.SetDailyCloses(dailyCloseSvc)
	// No in-process scheduler here anymore — see cmd/worker-close-domestic and
	// cmd/worker-close-overseas. dailyCloseSvc stays alive in this process only
	// for read paths (AttachAnalysis, coverage queries, the daily-closes API).

	scoreSettingsStore := service.NewScoreSettingsStore(repository.NewScoreSettingsRepo(mysql.DB()))
	scoreSvc := service.NewScoreService(
		mysql.DB(),
		repository.NewStockScoreRepo(mysql.DB()),
		scoreSettingsStore,
	)
	indexCloseRepo := repository.NewIndexDailyCloseRepo(mysql.DB())
	indexScoreSvc := service.NewIndexScoreService(mysql.DB(), indexCloseRepo)
	scoreSvc.SetIndexScores(indexScoreSvc)
	// Scoring, too, now runs in its own process — see cmd/worker-scores.
	stockSummarySvc.SetScores(scoreSvc)

	// Stamps read from everything above but write nothing back — the snapshot is
	// assembled from whatever is already cached/persisted (docs/STAMP_DESIGN.md).
	stampSvc := service.NewStampService(
		repository.NewStampRepo(mysql.DB()),
		repository.NewStockDailyCloseRepo(mysql.DB()),
		indexCloseRepo,
		stockRepo,
		candleBarRepo,
	)
	stampSvc.SetQuotes(quotesSvc)
	stampSvc.SetScores(scoreSvc)
	stampSvc.SetNews(newsStore)
	stampSvc.SetIndexStore(indexStore)
	stampSvc.SetMarketStore(marketStore)
	stampSvc.SetHoldings(userHoldingsSvc)

	// Research calls reuse the same read-only snapshot sources as stamps, plus
	// the rule-based briefing for the auto-generated draft (no LLM involved).
	researchSvc := service.NewResearchService(
		repository.NewResearchRepo(mysql.DB()),
		repository.NewStockDailyCloseRepo(mysql.DB()),
		indexCloseRepo,
		stockRepo,
		candleBarRepo,
	)
	researchSvc.SetQuotes(quotesSvc)
	researchSvc.SetScores(scoreSvc)
	researchSvc.SetNews(newsStore)
	researchSvc.SetIndexStore(indexStore)
	researchSvc.SetMarketStore(marketStore)
	researchSvc.SetStockSummary(stockSummarySvc)

	batchRunRepo := repository.NewBatchJobRunRepo(mysql.DB())
	// A run left "running" by a restart would otherwise show as permanently in
	// flight and block every later trigger. Only affects runs the API process
	// itself might have orphaned (there are none any more) or a stale row from
	// before this split — kept as a safety net.
	if err := batchRunRepo.MarkStaleRunsFailed(bgCtx); err != nil {
		log.Printf("batch runs: clear stale: %v", err)
	}
	batchAdminSvc := service.NewBatchAdminService(batchRunRepo, dailyCloseSvc, scoreSvc, stockSummarySvc)
	batchAdminSvc.SetResearch(researchSvc)
	batchAdminSvc.SetEarnings(earningsEventRepo)
	batchAdminSvc.SetIngestCoverage(syncStateRepo, disclosureRepo)

	stockSummarySvc.Start(bgCtx)
	// The AI 요약 batch has no dedicated worker process (see BatchAdminService
	// for why — its dependency graph is this process's), so 배치 관리's manual
	// trigger is served by a queue poller running right here, the same
	// abstraction cmd/worker-* uses, just sharing this process instead of one
	// of its own.
	safego.GoCtx(bgCtx, "batch-llm-domestic", func(ctx context.Context) {
		service.RunBatchQueueWorker(ctx, batchRunRepo, model.BatchLLMSummaryDomestic,
			func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
				return stockSummarySvc.RunBatchNow(ctx, toss.MarketKR)
			})
	})
	safego.GoCtx(bgCtx, "batch-llm-overseas", func(ctx context.Context) {
		service.RunBatchQueueWorker(ctx, batchRunRepo, model.BatchLLMSummaryOverseas,
			func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
				return stockSummarySvc.RunBatchNow(ctx, toss.MarketUS)
			})
	})
	// 뉴스 브리핑도 API 프로세스 안에서 돈다 — NewsStore·시세·LLM 클라이언트를
	// 그대로 쓰기 때문에 전용 워커로 뗄 수 없다.
	safego.GoCtx(bgCtx, "batch-news-briefing-domestic", func(ctx context.Context) {
		service.RunBatchQueueWorker(ctx, batchRunRepo, model.BatchNewsBriefingDomestic,
			func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
				return newsBriefingSvc.RunBatchNow(ctx, "domestic")
			})
	})
	safego.GoCtx(bgCtx, "batch-news-briefing-overseas", func(ctx context.Context) {
		service.RunBatchQueueWorker(ctx, batchRunRepo, model.BatchNewsBriefingOverseas,
			func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
				return newsBriefingSvc.RunBatchNow(ctx, "us")
			})
	})
	go service.RunReferenceDataPoller(bgCtx, marketDataSettings, marketSvc, fundamentalsSvc)
	safego.GoCtx(bgCtx, "startup-repair-asset-types", func(ctx context.Context) {
		ctx, cancel := context.WithTimeout(bgCtx, 5*time.Minute)
		defer cancel()
		time.Sleep(15 * time.Second)
		if n, err := marketSvc.RepairDomesticAssetTypes(ctx); err != nil {
			log.Printf("startup repair asset types: %v", err)
		} else if n > 0 {
			log.Printf("startup repair asset types: %d symbols", n)
		}
		// Bulk DART refresh disabled at startup — apibudget + rate limits caused mass failed overwrites.
		// Use: go run ./cmd/seed-fundamentals or wait for scheduled reference_data_poller.
	})
	// User uploads (리포트 본문 이미지가 첫 사용처). A store that cannot be
	// created is not fatal — the app runs fine without uploads, and failing
	// startup over a media directory would take the whole API down for a
	// feature that is one editor button.
	var mediaSvc *service.MediaService
	if store, err := media.NewLocalStore(cfg.Media.Dir); err != nil {
		log.Printf("media store disabled: %v", err)
	} else {
		mediaSvc = service.NewMediaService(store, cfg.Media.MaxUploadBytes, cfg.Media.PublicBaseURL)
		log.Printf("media store: %s (max %d MB)", store.BaseDir(), cfg.Media.MaxUploadBytes>>20)
	}

	// All collaborators are wired at this point, so it is safe to start the
	// quote poller (it reads marketStore / indexStore / userHoldings).
	quotesSvc.StartPolling(bgCtx)

	s := &Server{
		cfg:              cfg,
		bgCancel:         bgCancel,
		bgDone:           bgCtx.Done(),
		mysql:            mysql,
		redis:            redis,
		toss:             tossClient,
		auth:             authSvc,
		users:            userRepo,
		importJobsSvc:    importJobSvc,
		health:           handler.NewHealthHandler(mysql, redis, apiBudget),
		authHandler:      handler.NewAuthHandler(authSvc),
		quotes:           handler.NewQuotesHandler(quotesSvc),
		market:           handler.NewMarketHandler(marketSvc, marketDataSettings),
		watchlist:        handler.NewWatchlistHandler(watchlistSvc),
		journal:          handler.NewJournalHandler(journalSvc),
		stamps:           handler.NewStampHandler(stampSvc),
		sample:           handler.NewSampleHandler(),
		research:         handler.NewResearchHandler(researchSvc),
		userHoldings:     handler.NewUserHoldingsHandler(userHoldingsSvc),
		holdingTrades:    handler.NewHoldingTradesHandler(holdingTradesSvc),
		priceTargets:     handler.NewPriceTargetHandler(priceTargetSvc),
		userCashBalances: handler.NewUserCashBalancesHandler(userCashBalancesSvc),
		importJobs:       handler.NewImportHandler(importJobSvc),
		mediaFiles:       handler.NewMediaHandler(mediaSvc),
		fundamentals:     handler.NewFundamentalsHandler(fundamentalsSvc),
		earnings:         handler.NewEarningsHandler(earningsTabSvc),
		earningsCalendar: handler.NewEarningsCalendarHandler(earningsCalendarSvc),
		financialHistory: handler.NewFinancialHistoryHandler(financialHistorySvc),
		insider:          handler.NewInsiderHandler(insiderTabSvc),
		stockSummary:     handler.NewStockSummaryHandler(stockSummarySvc),
		dailyCloses:      handler.NewDailyCloseHandler(dailyCloseSvc),
		scores:           handler.NewScoreHandler(scoreSvc, indexScoreSvc),
		news:             handler.NewNewsHandler(newsStore, newsReader, newsBriefingSvc),
		admin: handler.NewAdminHandler(
			keywordRepo, newsStore, apiBudget, marketDataSettings,
			service.NewCacheAdminService(redis.Client()),
			service.NewDBAdminService(mysql.DB()),
			stockSummarySvc,
			llmSettingsStore,
			scoreSvc,
			scoreSettingsStore,
			batchAdminSvc,
		),
	}

	s.http = &http.Server{
		Addr:         cfg.Addr(),
		Handler:      s.routes(),
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s, nil
}

func (s *Server) Run() error {
	return s.http.ListenAndServe()
}

// llmLimits folds the LLM_* env overrides onto the compiled-in provider
// quotas. Free-tier ceilings move without notice and differ per model and per
// account, and the AI 요약 batch sizes itself directly from them — so they
// have to be correctable from the environment rather than only by a release.
// Zero keeps the compiled-in default; negative clears that ceiling entirely
// (a paid tier, where a self-imposed cap is just a self-inflicted outage).
func llmLimits(cfg config.LLMConfig, base apibudget.Limits) apibudget.Limits {
	out := base
	if cfg.DailyQuota != 0 {
		out.DailyQuota = max(cfg.DailyQuota, 0)
	}
	if cfg.PerMinute != 0 {
		out.PerMinute = max(cfg.PerMinute, 0)
	}
	if cfg.TokensPerMinute != 0 {
		out.TokensPerMinute = max(cfg.TokensPerMinute, 0)
	}
	if cfg.TokensPerDay != 0 {
		out.TokensPerDay = max(cfg.TokensPerDay, 0)
	}
	return out
}

// Shutdown drains in-flight HTTP requests first, cancels background workers,
// waits for screenshot import jobs, then closes the MySQL/Redis pools.
func (s *Server) Shutdown(ctx context.Context) error {
	log.Printf("shutdown: draining HTTP listeners…")
	err := s.http.Shutdown(ctx)

	if s.bgCancel != nil {
		log.Printf("shutdown: stopping background workers…")
		s.bgCancel()
	}

	if s.importJobsSvc != nil {
		log.Printf("shutdown: waiting for import jobs…")
		s.importJobsSvc.WaitIdle(ctx)
	}

	if s.mysql != nil {
		_ = s.mysql.Close()
	}
	if s.redis != nil {
		_ = s.redis.Close()
	}
	return err
}

// redisPoolStats adapts go-redis's pool counters for the metrics endpoint,
// keeping the driver import out of internal/metrics.
func (s *Server) redisPoolStats() metrics.RedisPoolStats {
	if s == nil || s.redis == nil || s.redis.Client() == nil {
		return metrics.RedisPoolStats{}
	}
	st := s.redis.Client().PoolStats()
	if st == nil {
		return metrics.RedisPoolStats{}
	}
	return metrics.RedisPoolStats{
		TotalConns: st.TotalConns,
		IdleConns:  st.IdleConns,
		StaleConns: st.StaleConns,
	}
}
