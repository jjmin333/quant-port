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
	"github.com/enact/quant-port/be/internal/translate"
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
	clientError      *handler.ClientErrorHandler

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
		// RESTConfig는 웹소켓이 켜져 있으면 SECONDARY를 뺀다 — 두 용도가 같은 앱을
		// 쓰면 서로의 토큰을 죽인다(config.RESTConfig 헤더).
		tossClient, err = toss.NewClient(cfg.Toss.RESTConfig(), toss.NewTokenStore(redis.Client()))
		if err != nil {
			mysql.Close()
			redis.Close()
			return fail(fmt.Errorf("toss client: %w", err))
		}
		if tossClient.HasSecondary() {
			log.Println("toss open api global client enabled (primary + secondary failover)")
		} else {
			log.Println("toss open api global client enabled")
		}
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
	// Run은 아래에서 시작한다 — 세션 판정을 붙인 뒤여야 첫 회차부터 올바른 주기가
	// 되고, 고루틴이 먼저 뜨면 SetSessionProbe와 경쟁한다.

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
	llmQuota := llmLimits(cfg.LLM, apiBudget.LimitsFor(apibudget.ProviderOpenAI))
	apiBudget.SetLimits(apibudget.ProviderOpenAI, llmQuota)
	apiBudget.SetLimits(apibudget.ProviderLLMKR, llmQuota)
	apiBudget.SetLimits(apibudget.ProviderLLMUS, llmQuota)
	dartClient := service.NewDartClient(service.DartConfig{APIKey: cfg.Dart.APIKey}, apiBudget)
	earningsClient := service.NewEarningsClient(service.EarningsConfig{FinnhubAPIKey: cfg.Finnhub.APIKey}, apiBudget)
	marketStore := service.NewMarketDataStore(tossClient, redis.Client(), earningsClient, dartClient, quoteTerminalRepo)
	marketStore.SetDisclosures(repository.NewDisclosureRepo(mysql.DB()))
	go marketStore.Run(bgCtx)
	// TTL 없는 키 비중을 감시한다 — maxmemory에 닿으면 volatile-lru가 TTL 있는
	// 캐시만 버리므로 이 비율이 곧 실질 캐시 용량의 잠식률이다.
	safego.GoCtx(bgCtx, "redis-census", func(ctx context.Context) {
		service.RunRedisCensus(ctx, redis.Client())
	})

	marketClients := marketdata.NewClients(cfg.MarketData, apiBudget, redis.Client())
	indexRepo := repository.NewMarketIndexRepo(mysql.DB())
	indexStore := service.NewMarketIndexStore(tossClient, marketClients, fxSvc, redis.Client(), indexRepo)
	indexStore.SetMarketStore(marketStore)
	// 원/달러·SOXX·QQQ는 실시간 응답에 이력이 없어 차트·등락률이 비어 나온다.
	// 종가 표에는 매일 쌓이고 있으므로 거기서 메운다(market_index_history.go).
	indexStore.SetDailyCloses(repository.NewIndexDailyCloseRepo(mysql.DB()))
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
	// 없는 심볼로 매매를 기록하지 않는다 (service/symbol_catalog.go).
	holdingTradesSvc.SetCatalog(stockRepo)
	/*
		환율 주기를 세션에 맞춘다 (2026-09-11 사용자 요청 7 — 15초는 하루 5,760호출이고
		실측 변동은 분당 0.011%였다). 판정은 거래소 달력을 가진 quotesSvc가 한다.
	*/
	fxSvc.SetSessionProbe(func(ctx context.Context) bool {
		for _, assetType := range []model.QuoteAssetType{model.AssetDomestic, model.AssetUS} {
			// 캐시만 읽는다 — 매 회차 도는 자리다(MarketClockCachedFor 머리글).
			if !quotesSvc.MarketClockCachedFor(ctx, string(assetType)).Closed() {
				return true
			}
		}
		return false
	})
	go fxSvc.Run(bgCtx)

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
	newsRepo := repository.NewNewsRepo(mysql.DB())
	keywordRepo := repository.NewNewsKeywordRepo(mysql.DB())
	pollSettingsRepo := repository.NewNewsPollSettingsRepo(mysql.DB())
	readerTransRepo := repository.NewNewsReaderTranslationRepo(mysql.DB())
	llmPool := llm.NewPoolFromKeys(llm.Config{
		Provider: cfg.LLM.Provider,
		BaseURL:  cfg.LLM.BaseURL,
		Model:    cfg.LLM.Model,
	}, cfg.LLM.KeyKR, cfg.LLM.KeyUS, apiBudget)
	newsStore := service.NewNewsStore(cfg.News, naverNews, kakaoNews, rssNews, finnhubNews, rssUSNews, googleNews, newsRepo, keywordRepo, pollSettingsRepo, stockRepo, watchlistRepo, userHoldingsRepo, redis.Client())
	newsStore.SetLLM(llmPool)

	/*
		해외 제목 번역기를 붙인다 — 순서는 TRANSLATE_PROVIDER가 정한다.

		**무엇이 실제로 붙었는지 로그에 찍는다.** 설정 오타 하나로 엔진이 하나도 안
		붙어도 화면은 똑같아 보인다 — 해외 제목이 영어로 남을 뿐이라, 「원래 영어인
		기사」와 구분되지 않는다. 계기가 없으면 아무 일도 안 하면서 잘 도는 것처럼
		보인다는 것을 DART 배치에서 이미 한 번 겪었다.
	*/
	translatorChain := translate.Build(translate.ParseOptions(os.Getenv), apiBudget)
	newsStore.SetTranslator(translatorChain)
	log.Printf("news translate: engines=%s", translatorChain.Name())
	if !translatorChain.Available() {
		log.Print("news translate: **아무 엔진도 못 붙었다** — 해외 제목이 영어로 남는다")
	}
	go newsStore.Run(bgCtx)
	newsReader := service.NewNewsReaderService(redis.Client(), readerTransRepo, newsRepo)

	newsBriefingSvc := service.NewNewsBriefingService(
		newsStore, indexStore, quotesSvc,
		repository.NewNewsBriefingRepo(mysql.DB()),
	)
	newsBriefingSvc.SetLLM(llmPool)

	watchlistSvc := service.NewWatchlistService(watchlistRepo)
	watchlistSvc.SetQuotes(quotesSvc)
	watchlistSvc.SetCatalog(stockRepo)
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
	earningsSurpriseRepo := repository.NewEarningsSurpriseHistoryRepo(mysql.DB())
	earningsCalendarSvc := service.NewEarningsCalendarService(
		earningsEventRepo, earningsSurpriseRepo, redis.Client())
	newsBriefingSvc.SetEarnings(earningsCalendarSvc)
	newsBriefingSvc.SetToss(tossClient)
	newsBriefingSvc.SetRedis(redis.Client())
	// 스케줄러 기동은 배선이 **다 끝난 뒤**다(아래 SetBatchRuns 다음). 여기서 띄우면
	// 기동 직후 첫 슬롯이 원장 배선 전에 돌고, 세터와 goroutine이 같은 필드를
	// 동시에 만진다.
	// 이 세 표(분기 재무·공시·적재 상태)가 조회 시점 외부 호출을 없앤 결과다.
	// 채우는 쪽은 worker-filings, 읽는 쪽은 여기뿐이다.
	financialPeriodRepo := repository.NewFinancialPeriodRepo(mysql.DB())
	disclosureRepo := repository.NewDisclosureRepo(mysql.DB())
	syncStateRepo := repository.NewSyncStateRepo(mysql.DB())
	// 지표 갱신은 다른 종목 단위 적재와 **같은 로테이션 커서·같은 예산**을 쓴다.
	// 이 배선이 없으면 RefreshRotation은 아무것도 하지 않는다 — 그 함수 머리글의
	// 2026-09-10 사고(전 종목 스윕이 DART 하루치를 통째로 씀) 참조.
	fundamentalsSvc.SetRotation(syncStateRepo, apiBudget)
	earningsTabSvc := service.NewEarningsTabService(stockRepo, financialPeriodRepo, disclosureRepo)
	// 종목 실적 탭도 같은 표를 1순위로 읽는다. 캘린더와 탭이 다른 소스를 보면
	// 같은 발표에 대해 서로 다른 서프라이즈를 말하게 된다.
	earningsTabSvc.SetEvents(earningsEventRepo, earningsCalendarSvc)
	financialHistorySvc := service.NewFinancialHistoryService(financialPeriodRepo, syncStateRepo, stockRepo)
	// 분기별 PER·PBR을 실제로 계산하려면 그 시점 종가가 필요하다 — 일봉이 5년치
	// 있으므로(실측 2021-10까지) 지어낸 추세선을 쓸 이유가 없다.
	financialHistorySvc.SetValuationSources(candleBarRepo, fundamentalsRepo)
	// 내부자 거래도 실적 이벤트와 같다 — 표는 worker-filings가 채우고 API는
	// 읽기만 한다. 예전에는 이 서비스가 요청마다 EDGAR/DART를 직접 불렀고,
	// 그게 동시 접속에서 빈 화면이 나오던 원인이었다.
	insiderRepo := repository.NewInsiderRepo(mysql.DB())
	insiderTabSvc := service.NewInsiderTabService(insiderRepo, syncStateRepo, stockRepo)
	llmSettingsRepo := repository.NewLLMSettingsRepo(mysql.DB())
	llmSettingsStore := service.NewLLMSettingsStore(llmSettingsRepo)
	stockSummarySvc := service.NewStockSummaryService(
		fundamentalsSvc, financialHistorySvc, earningsTabSvc, insiderTabSvc,
		newsStore, candleBarRepo, stockRepo, llmPool, redis.Client(),
		llmSettingsStore,
	)
	// The scheduled AI 요약 batch draws its targets from 마켓 레이더's stored
	// 1일 기준 rankings — no live Toss call, just what the poller already saved.
	stockSummarySvc.SetRankings(marketRankingsRepo)
	/*
		LLM이 쓴 글은 **캐시 밖에도 한 벌** 남긴다 (migrations/065).

		캐시(24시간)만으로 이어 받던 동안에는 하루 동안 아무도 열지 않은 종목의 글이
		사라졌다. 개요 탭이 그 글이 없으면 카드를 아예 그리지 않으므로, 그 손실이 이제
		「어제 있던 카드가 오늘 없다」로 보인다.
	*/
	stockSummarySvc.SetNarratives(repository.NewStockSummaryNarrativeRepo(mysql.DB()))
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
	// 랭킹의 등락률은 장중에 실시간 시세로 덮인다 — 읽기 경로에서만이고, 점수를
	// 계산하는 worker-scores에는 이 배선이 없다(그 배치는 업스트림을 부르지 않는다).
	scoreSvc.SetLiveQuotes(quotesSvc)
	// Scoring, too, now runs in its own process — see cmd/worker-scores.
	stockSummarySvc.SetScores(scoreSvc)

	/*
		AI 추천 랭킹은 **저장된 목록을 읽기만** 한다. 축·기간 조합의 계산은 점수
		배치(worker-scores)가 하루 한 번 하고 여기로는 오지 않는다 — 1개월 창 하나가
		운영 DB에서 수 초를 먹으므로 요청 경로에 둘 수 없다
		(`service/stock_score_rankings.go` 머리글의 실측 참조).
	*/
	scoreRankingRepo := repository.NewStockScoreRankingRepo(mysql.DB())
	scoreRankingSvc := service.NewScoreRankingService(scoreRankingRepo, scoreSvc)

	/*
		AI 요약 배치의 대상은 세 층에서 온다 (service/llm_batch_targets.go 머리글).

		1순위 레이더만 있던 동안에는 목록의 머리가 고정이라 매일 같은 종목이 다시
		구워졌다. 2순위(AI 추천 랭킹 회전)와 3순위(사용자가 담은 종목 중 요약 없음)를
		여기서 붙인다 — 둘 다 저장분을 읽을 뿐이라 외부 쿼터는 그대로다.
	*/
	stockSummarySvc.SetScoreRankings(scoreRankingRepo)
	stockSummarySvc.SetUserInterest(userHoldingsRepo, watchlistRepo)

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
	/*
		크래시가 남긴 running 행을 닫는다. **6시간 넘은 것만** 건드린다.

		예전에는 조건이 없었고, 그래서 API를 다시 올릴 때마다 다른 워커 컨테이너에서
		한창 돌고 있는 실행까지 실패로 기록됐다(2026-09-09 실측: 분기 재무 국내).
		잡은 각자 다른 프로세스에서 돌고 원장만 공유한다 — 이 프로세스의 재기동이
		남의 실행에 대해 말해 줄 수 있는 것은 아무것도 없다.
	*/
	if err := batchRunRepo.MarkStaleRunsFailed(bgCtx); err != nil {
		log.Printf("batch runs: clear stale: %v", err)
	}
	/*
		API 프로세스 안에서 도는 예약 배치도 **원장에 남긴다**.

		AI 요약과 뉴스 브리핑은 전용 워커가 없어 여기서 돈다. 그래서 오래도록 수동
		실행만 `batch_job_runs`에 남았고, 관리자 화면은 AI 요약의 마지막 실행을
		08-20으로 알고 있었다 — 그 사이 매일 돌고 있었는데도. 감시가 원장을 읽는 이상,
		원장에 없는 잡은 감시되지 않는다.
	*/
	stockSummarySvc.SetBatchRuns(batchRunRepo)
	newsBriefingSvc.SetBatchRuns(batchRunRepo)
	go newsBriefingSvc.Run(bgCtx)

	batchHealthSvc := service.NewBatchHealthService(batchRunRepo, repository.NewBatchHealthRepo(mysql.DB()))
	// api 쪽은 판정을 화면에 내주는 역할이지만, 사건 원장은 같은 배선을 쓴다 —
	// 관리자 화면이 「지금 열린 사건」을 읽는 자리가 여기다.
	opsIncidentRepo := repository.NewOpsIncidentRepo(mysql.DB())
	batchHealthSvc.SetIncidents(opsIncidentRepo)
	quotaIncidentReporter := service.NewQuotaIncidentReporter(apiBudget, opsIncidentRepo)
	quotaIncidentReporter.Install()
	batchHealthSvc.SetQuotaReporter(quotaIncidentReporter)
	batchAdminSvc := service.NewBatchAdminService(batchRunRepo, dailyCloseSvc, scoreSvc, stockSummarySvc)
	batchAdminSvc.SetHealth(batchHealthSvc)
	// 판정은 워커의 워치독이 하지만, `/metrics`는 이 프로세스에 있다. 관리자가
	// 화면을 열 때만 갱신하면 아무도 안 열었을 때 게이지가 얼어붙는다.
	safego.GoCtx(bgCtx, "batch-health-metrics", batchHealthSvc.RunMetricsLoop)
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
	/*
		실시간 시세 구독. **기본값은 off**이고 `TOSS_STREAM_ENABLED=true`로 켠다.

		켜려면 SECONDARY 자격증명이 있어야 한다 — 웹소켓은 그것을 **전용**으로 쓴다.
		REST와 공유하면 WS 재연결이 REST 토큰을 죽인다(`toss/stream.go` 헤더의 실측).
		`StreamReady`가 그 조건을 담고 있어서, 조건이 안 맞으면 조용히 폴링으로 남는다.

		폴러보다 **먼저** 붙인다 — 폴러가 첫 틱에서 구독 대상을 넘기기 때문이다.
	*/
	if cfg.Toss.StreamReady() && tossClient != nil {
		/*
			**REST 클라이언트의 토큰 관리자를 그대로 넘긴다.**

			전에는 구독 전용 관리자를 따로 만들어 SECONDARY를 독점하게 했다.
			근거는 「WS 재연결이 REST 토큰을 죽인다」였는데 실측으로 반증됐다
			(`docs/TOSS_TOKEN_DESIGN.md`). 공유하면 REST가 failover를 되찾고,
			구독은 두 앱을 다 써서 슬롯이 200 → 400이 된다.

			관리자를 새로 만들지 **않는** 것이 핵심이다. 같은 client_id에 발급
			주체가 둘이 되면 발급이 파괴적이라 서로의 토큰을 죽인다.
		*/
		auths := tossClient.TokenManagers()
		stream := service.NewQuoteStreamService(quotesSvc, auths)
		quotesSvc.SetStream(stream)
		safego.GoCtx(bgCtx, "toss-quote-stream", stream.Run)
		log.Printf("toss stream: websocket quotes enabled (%d credentials, %d connections, %d slots)",
			len(auths), len(auths)*service.StreamConnections,
			len(auths)*service.StreamConnections*service.StreamSlotsPerConn)
	} else if cfg.Toss.StreamEnabled {
		log.Println("toss stream: requested but Toss credentials missing — staying on polling")
	}

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
		sample:           handler.NewSampleHandler(quotesSvc, redis),
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
		scores:           handler.NewScoreHandler(scoreSvc, indexScoreSvc, scoreRankingSvc),
		news:             handler.NewNewsHandler(newsStore, newsReader, newsBriefingSvc),
		clientError:      handler.NewClientErrorHandler(),
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

	// 관리자 화면이 사건 원장을 읽는 자리. 생성자 인자가 아니라 세터인 이유는
	// AdminHandler 머리글에 있다.
	s.admin.SetIncidents(opsIncidentRepo)
	// 종목 별칭 관리. stockRepo는 「없는 심볼을 거부한다」에만 쓴다 —
	// 그 검사가 없으면 오타가 조용히 저장되고 그 별칭은 영영 아무 데도 안 붙는다.
	s.admin.SetStockAliases(repository.NewStockAliasRepo(mysql.DB()), stockRepo)

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
