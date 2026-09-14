// Command dry-run-stock-summary calls the LLM once for a symbol and prints the
// raw completion plus what would be stored — without writing Redis or MySQL.
//
//	go run ./cmd/dry-run-stock-summary 000660
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
)

func main() {
	flag.Parse()
	symbol := "000660"
	if flag.NArg() > 0 {
		symbol = flag.Arg(0)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
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
	candleBarRepo := repository.NewCandleBarRepo(mysql.DB())
	fundamentalsRepo := repository.NewFundamentalsRepo(mysql.DB())
	fundamentalsSvc := service.NewFundamentalsService(nil, nil, nil, stockRepo, fundamentalsRepo)

	financialPeriodRepo := repository.NewFinancialPeriodRepo(mysql.DB())
	syncStateRepo := repository.NewSyncStateRepo(mysql.DB())
	financialHistorySvc := service.NewFinancialHistoryService(financialPeriodRepo, syncStateRepo, stockRepo)
	financialHistorySvc.SetValuationSources(candleBarRepo, fundamentalsRepo)

	earningsTabSvc := service.NewEarningsTabService(stockRepo, financialPeriodRepo, repository.NewDisclosureRepo(mysql.DB()))
	insiderTabSvc := service.NewInsiderTabService(repository.NewInsiderRepo(mysql.DB()), syncStateRepo, stockRepo)

	newsStore := service.NewNewsStore(cfg.News, nil, nil, nil, nil, nil, nil,
		repository.NewNewsRepo(mysql.DB()),
		repository.NewNewsKeywordRepo(mysql.DB()),
		repository.NewNewsPollSettingsRepo(mysql.DB()),
		stockRepo, repository.NewWatchlistRepo(mysql.DB()), repository.NewUserHoldingsRepo(mysql.DB()),
		redisConn.Client(),
	)

	llmSettings := service.NewLLMSettingsStore(repository.NewLLMSettingsRepo(mysql.DB()))
	summarySvc := service.NewStockSummaryService(
		fundamentalsSvc, financialHistorySvc, earningsTabSvc, insiderTabSvc,
		newsStore, candleBarRepo, stockRepo, llmPool, redisConn.Client(), llmSettings,
	)
	summarySvc.SetNarratives(repository.NewStockSummaryNarrativeRepo(mysql.DB()))
	scoreSettings := service.NewScoreSettingsStore(repository.NewScoreSettingsRepo(mysql.DB()))
	summarySvc.SetScores(service.NewScoreService(mysql.DB(), repository.NewStockScoreRepo(mysql.DB()), scoreSettings))

	out, err := summarySvc.DryRunLLM(ctx, symbol)
	if err != nil {
		log.Fatalf("dry-run %s: %v", symbol, err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		log.Fatalf("encode: %v", err)
	}
	fmt.Fprintf(os.Stderr, "\n[dry-run] %s tokens=%d — nothing was stored\n", symbol, out.Tokens)
}
