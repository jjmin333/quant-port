package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/crypto"
	"github.com/enact/quant-port/be/internal/handler"
	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/internal/toss"
)

type Server struct {
	cfg   *config.Config
	http  *http.Server
	mysql *repository.MySQL
	redis *repository.Redis
	toss  *toss.Client
	auth  *service.AuthService

	health       *handler.HealthHandler
	authHandler  *handler.AuthHandler
	broker       *handler.BrokerHandler
	quotes       *handler.QuotesHandler
	accounts     *handler.AccountsHandler
	holdings     *handler.HoldingsHandler
	orders       *handler.OrdersHandler
	market       *handler.MarketHandler
	portfolio    *handler.PortfolioHandler
	insights     *handler.InsightsHandler
	watchlist    *handler.WatchlistHandler
	disclosures  *handler.DisclosuresHandler
	fundamentals *handler.FundamentalsHandler
	news         *handler.NewsHandler
}

func New(cfg *config.Config) (*Server, error) {
	mysql, err := repository.NewMySQL(cfg.MySQLDSN)
	if err != nil {
		return nil, fmt.Errorf("mysql: %w", err)
	}

	migrationsDir := filepath.Join("migrations")
	if err := repository.EnsureSchema(context.Background(), mysql.DB(), migrationsDir); err != nil {
		mysql.Close()
		return nil, fmt.Errorf("mysql schema: %w", err)
	}

	redis, err := repository.NewRedis(cfg.RedisURL)
	if err != nil {
		mysql.Close()
		return nil, fmt.Errorf("redis: %w", err)
	}

	box, err := crypto.NewBox(cfg.Auth.EncryptionMasterKey)
	if err != nil {
		mysql.Close()
		redis.Close()
		return nil, fmt.Errorf("encryption: %w", err)
	}

	userRepo := repository.NewUserRepo(mysql.DB())
	refreshRepo := repository.NewRefreshTokenRepo(mysql.DB())
	brokerCredsRepo := repository.NewBrokerCredentialsRepo(mysql.DB())
	brokerAccountRepo := repository.NewBrokerAccountRepo(mysql.DB())

	authSvc, err := service.NewAuthService(cfg.Auth, userRepo, refreshRepo, brokerCredsRepo)
	if err != nil {
		mysql.Close()
		redis.Close()
		return nil, fmt.Errorf("auth: %w", err)
	}

	var tossClient *toss.Client
	if cfg.Toss.Enabled() {
		tossClient, err = toss.NewClient(cfg.Toss, toss.NewTokenStore(redis.Client()))
		if err != nil {
			mysql.Close()
			redis.Close()
			return nil, fmt.Errorf("toss client: %w", err)
		}
		log.Println("toss open api global client enabled")
		tossClient.StartTokenRefresh(context.Background())
	} else {
		log.Println("toss open api global client disabled — per-user credentials or AUTH_ALLOW_GLOBAL_TOSS required")
	}

	tossReg := service.NewTossRegistry(cfg.Toss, tossClient, cfg.Auth.AllowGlobalToss, brokerCredsRepo, box, redis.Client())
	brokerSvc := service.NewBrokerService(brokerCredsRepo, brokerAccountRepo, box, tossReg)

	snapshotRepo := repository.NewSnapshotRepo(mysql.DB())
	portfolioCacheRepo := repository.NewPortfolioCacheRepo(mysql.DB())
	watchlistRepo := repository.NewWatchlistRepo(mysql.DB())
	candleBarRepo := repository.NewCandleBarRepo(mysql.DB())
	quoteTerminalRepo := repository.NewQuoteTerminalRepo(mysql.DB())
	stockRepo := repository.NewStockRepo(mysql.DB())

	fxRateRepo := repository.NewFxRateRepo(mysql.DB())
	fxSvc := service.NewExchangeRateService(tossClient, redis.Client(), fxRateRepo)
	go fxSvc.Run(context.Background())

	accountsSvc := service.NewAccountsService(tossReg)
	holdingsSvc := service.NewHoldingsService(tossReg)
	ordersSvc := service.NewOrdersService(tossReg)
	marketSvc := service.NewMarketService(tossClient, fxSvc, stockRepo)
	portfolioSvc := service.NewPortfolioService(tossReg, redis.Client(), snapshotRepo, portfolioCacheRepo, brokerCredsRepo, fxSvc)
	apiBudget := apibudget.NewManager(redis.Client())
	dartClient := service.NewDartClient(service.DartConfig{APIKey: cfg.Dart.APIKey}, apiBudget)
	earningsClient := service.NewEarningsClient(service.EarningsConfig{FinnhubAPIKey: cfg.Finnhub.APIKey}, apiBudget)
	marketStore := service.NewMarketDataStore(tossClient, redis.Client(), earningsClient, dartClient, portfolioSvc, watchlistRepo, quoteTerminalRepo)
	portfolioSvc.SetMarketStore(marketStore)
	go marketStore.Run(context.Background())

	marketClients := marketdata.NewClients(cfg.MarketData, apiBudget, redis.Client())
	indexRepo := repository.NewMarketIndexRepo(mysql.DB())
	indexStore := service.NewMarketIndexStore(tossClient, marketClients, fxSvc, redis.Client(), indexRepo)
	indexStore.SetMarketStore(marketStore)
	quotesSvc := service.NewQuotesService(tossClient, marketClients, fxSvc, watchlistRepo, portfolioSvc, stockRepo, redis.Client(), candleBarRepo, quoteTerminalRepo)
	quotesSvc.SetIndexStore(indexStore)
	go indexStore.Run(context.Background())
	quotesSvc.SetMarketStore(marketStore)
	marketSvc.SetMarketStore(marketStore)

	newsHTTP := &http.Client{Timeout: 15 * time.Second}
	naverNews := marketdata.NewNaverNewsClient(cfg.News.Naver.ClientID, cfg.News.Naver.ClientSecret, newsHTTP, apiBudget)
	kakaoNews := marketdata.NewKakaoNewsClient(cfg.News.Kakao.RestAPIKey, newsHTTP, apiBudget)
	rssNews := marketdata.NewRSSNewsClient(newsHTTP)
	newsRepo := repository.NewNewsRepo(mysql.DB())
	newsStore := service.NewNewsStore(cfg.News, naverNews, kakaoNews, rssNews, newsRepo, stockRepo, watchlistRepo, portfolioSvc, redis.Client())
	go newsStore.Run(context.Background())
	newsReader := service.NewNewsReaderService(redis.Client())

	watchlistSvc := service.NewWatchlistService(watchlistRepo)

	llmClient := llm.NewClient(llm.Config{
		APIKey:  cfg.LLM.APIKey,
		BaseURL: cfg.LLM.BaseURL,
		Model:   cfg.LLM.Model,
	})
	edgarClient := service.NewEdgarClient(service.EdgarConfig{UserAgent: cfg.Edgar.UserAgent}, apiBudget)
	fundamentalsRepo := repository.NewFundamentalsRepo(mysql.DB())
	fundamentalsSvc := service.NewFundamentalsService(dartClient, edgarClient, tossClient, stockRepo, fundamentalsRepo)
	fundamentalsSvc.SetMarketStore(marketStore)
	insightsSvc := service.NewInsightsService(portfolioSvc, marketSvc, llmClient, dartClient, earningsClient, redis.Client())
	insightsSvc.SetMarketStore(marketStore)

	go insightsSvc.RunScheduler(context.Background())

	s := &Server{
		cfg:          cfg,
		mysql:        mysql,
		redis:        redis,
		toss:         tossClient,
		auth:         authSvc,
		health:       handler.NewHealthHandler(mysql, redis, apiBudget),
		authHandler:  handler.NewAuthHandler(authSvc),
		broker:       handler.NewBrokerHandler(brokerSvc, portfolioSvc),
		quotes:       handler.NewQuotesHandler(quotesSvc),
		accounts:     handler.NewAccountsHandler(accountsSvc),
		holdings:     handler.NewHoldingsHandler(holdingsSvc),
		orders:       handler.NewOrdersHandler(ordersSvc),
		market:       handler.NewMarketHandler(marketSvc),
		portfolio:    handler.NewPortfolioHandler(portfolioSvc),
		insights:     handler.NewInsightsHandler(insightsSvc),
		watchlist:    handler.NewWatchlistHandler(watchlistSvc),
		disclosures:  handler.NewDisclosuresHandler(insightsSvc),
		fundamentals: handler.NewFundamentalsHandler(fundamentalsSvc),
		news:         handler.NewNewsHandler(newsStore, newsReader),
	}

	s.http = &http.Server{
		Addr:         cfg.Addr(),
		Handler:      s.routes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s, nil
}

func (s *Server) Run() error {
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.mysql != nil {
		_ = s.mysql.Close()
	}
	if s.redis != nil {
		_ = s.redis.Close()
	}
	return s.http.Shutdown(ctx)
}
