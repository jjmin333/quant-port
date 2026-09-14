// Command dry-run-news-briefing calls the LLM once per market and prints the
// raw completion plus what would be stored — without writing MySQL.
//
//	go run ./cmd/dry-run-news-briefing domestic
//	go run ./cmd/dry-run-news-briefing us
//	go run ./cmd/dry-run-news-briefing all
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/internal/toss"
)

func main() {
	flag.Parse()
	markets := []string{"domestic", "us"}
	if flag.NArg() > 0 && flag.Arg(0) != "all" {
		markets = []string{flag.Arg(0)}
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	mysql, err := repository.NewMySQLForCLI(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defer mysql.Close()

	redisConn, err := repository.NewRedis(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer redisConn.Close()

	apiBudget := apibudget.NewManager(redisConn.Client())
	llmPool := llm.NewPoolFromKeys(llm.Config{
		Provider: cfg.LLM.Provider,
		BaseURL:  cfg.LLM.BaseURL,
		Model:    cfg.LLM.Model,
	}, cfg.LLM.KeyKR, cfg.LLM.KeyUS, apiBudget)

	stockRepo := repository.NewStockRepo(mysql.DB())
	watchlistRepo := repository.NewWatchlistRepo(mysql.DB())
	holdingsRepo := repository.NewUserHoldingsRepo(mysql.DB())

	newsStore := service.NewNewsStore(cfg.News, nil, nil, nil, nil, nil, nil,
		repository.NewNewsRepo(mysql.DB()),
		repository.NewNewsKeywordRepo(mysql.DB()),
		repository.NewNewsPollSettingsRepo(mysql.DB()),
		stockRepo, watchlistRepo, holdingsRepo,
		redisConn.Client(),
	)

	indexStore := service.NewMarketIndexStore(nil, nil, nil, redisConn.Client(), repository.NewMarketIndexRepo(mysql.DB()))
	indexStore.SetDailyCloses(repository.NewIndexDailyCloseRepo(mysql.DB()))

	candleBarRepo := repository.NewCandleBarRepo(mysql.DB())
	quoteTerminalRepo := repository.NewQuoteTerminalRepo(mysql.DB())
	var tossClient *toss.Client
	if cfg.Toss.Enabled() {
		tossClient, err = toss.NewClient(cfg.Toss, toss.NewTokenStore(redisConn.Client()))
		if err != nil {
			log.Fatalf("toss: %v", err)
		}
	}
	fxSvc := service.NewExchangeRateService(tossClient, redisConn.Client(), repository.NewFxRateRepo(mysql.DB()))
	quotesSvc := service.NewQuotesService(tossClient, nil, fxSvc, watchlistRepo, stockRepo, redisConn.Client(), candleBarRepo, quoteTerminalRepo)

	earningsCalendarSvc := service.NewEarningsCalendarService(
		repository.NewEarningsEventRepo(mysql.DB()),
		repository.NewEarningsSurpriseHistoryRepo(mysql.DB()),
		redisConn.Client(),
	)

	briefingSvc := service.NewNewsBriefingService(
		newsStore, indexStore, quotesSvc,
		repository.NewNewsBriefingRepo(mysql.DB()),
	)
	briefingSvc.SetLLM(llmPool)
	briefingSvc.SetEarnings(earningsCalendarSvc)
	briefingSvc.SetToss(tossClient)

	results := make(map[string]any, len(markets))
	for i, market := range markets {
		if i > 0 {
			time.Sleep(6 * time.Second)
		}
		out, err := briefingSvc.DryRunBriefing(ctx, market)
		if err != nil {
			log.Fatalf("dry-run %s: %v", market, err)
		}
		results[market] = out
		fmt.Fprintf(os.Stderr, "[dry-run] %s tokens=%d — nothing was stored\n", market, out.Tokens)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if len(markets) == 1 {
		if err := enc.Encode(results[markets[0]]); err != nil {
			log.Fatalf("encode: %v", err)
		}
		return
	}
	if err := enc.Encode(results); err != nil {
		log.Fatalf("encode: %v", err)
	}
}
