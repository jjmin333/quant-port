package main

import (
	"context"
	"flag"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/internal/toss"
)

const batchSize = 100

func main() {
	assetType := flag.String("asset-type", "us", "asset filter: us (includes ETF) | domestic | all")
	symbolsFlag := flag.String("symbols", "", "comma-separated symbols (overrides -asset-type list)")
	missingOnly := flag.Bool("missing-only", false, "sync only symbols without a Korean display name")
	limit := flag.Int("limit", 0, "max symbols (0 = all)")
	dryRun := flag.Bool("dry-run", false, "fetch from Toss only, do not write")
	timeoutHours := flag.Int("timeout-hours", 4, "job timeout in hours (0 = no limit)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	if *timeoutHours > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*timeoutHours)*time.Hour)
		defer cancel()
	}

	mysql, err := repository.NewMySQL(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defer mysql.Close()

	migrationsDir := filepath.Join("migrations")
	if err := repository.EnsureSchema(ctx, mysql.DB(), migrationsDir); err != nil {
		log.Fatalf("schema: %v", err)
	}

	if !cfg.Toss.Enabled() {
		log.Fatal("toss api is not configured (TOSS_CLIENT_ID / TOSS_CLIENT_SECRET)")
	}

	var tokenStore toss.TokenStore
	redis, err := repository.NewRedis(cfg.RedisURL)
	if err != nil {
		log.Printf("warning: redis unavailable (%v) — using in-memory token cache", err)
	} else {
		pingCtx, pingCancel := context.WithTimeout(ctx, 3*time.Second)
		pingErr := redis.Client().Ping(pingCtx).Err()
		pingCancel()
		if pingErr != nil {
			log.Printf("warning: redis ping failed (%v) — using in-memory token cache", pingErr)
			_ = redis.Close()
		} else {
			defer redis.Close()
			tokenStore = toss.NewTokenStore(redis.Client())
		}
	}

	tossClient, err := toss.NewClient(cfg.Toss, tokenStore)
	if err != nil {
		log.Fatalf("toss: %v", err)
	}

	stockRepo := repository.NewStockRepo(mysql.DB())

	res, err := mysql.DB().ExecContext(ctx, `
		UPDATE stocks SET status = 'NORMAL' WHERE status IN ('ACTIVE', 'TRADING')
	`)
	if err != nil {
		log.Fatalf("normalize status: %v", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		log.Printf("normalized catalog status for %d rows", n)
	}

	var symbols []string
	if strings.TrimSpace(*symbolsFlag) != "" {
		for _, sym := range strings.Split(*symbolsFlag, ",") {
			sym = strings.ToUpper(strings.TrimSpace(sym))
			if sym != "" && isSyncableSymbol(sym) {
				symbols = append(symbols, sym)
			}
		}
	} else {
		filter := *assetType
		if filter == "" {
			filter = "us"
		}
		var all []string
		var err error
		if *missingOnly {
			all, err = stockRepo.ListSymbolsMissingKoreanName(ctx, filter, *limit)
		} else {
			all, err = stockRepo.ListSymbols(ctx, filter, *limit)
		}
		if err != nil {
			log.Fatalf("load symbols: %v", err)
		}
		for _, sym := range all {
			if isSyncableSymbol(sym) {
				symbols = append(symbols, sym)
			}
		}
	}
	if *missingOnly {
		log.Printf("syncing missing korean display names for %d symbols", len(symbols))
	} else {
		log.Printf("syncing korean display names for %d symbols", len(symbols))
	}

	updated := 0
	for i := 0; i < len(symbols); i += batchSize {
		end := min(i+batchSize, len(symbols))
		batch := symbols[i:end]

		items := fetchStocksResilient(ctx, tossClient, batch)

		stocks := make([]model.Stock, 0, len(items))
		for _, info := range items {
			stocks = append(stocks, service.StockFromTossInfo(info))
		}

		if *dryRun {
			log.Printf("dry-run batch %d-%d: would upsert %d rows", i, end, len(stocks))
			updated += len(stocks)
			continue
		}

		if len(stocks) == 0 {
			continue
		}

		if err := stockRepo.UpsertBatch(ctx, stocks); err != nil {
			log.Fatalf("upsert batch %d-%d: %v", i, end, err)
		}
		updated += len(stocks)
		log.Printf("upserted %d / %d", end, len(symbols))
		time.Sleep(300 * time.Millisecond)
	}

	log.Printf("sync complete — updated %d stock names", updated)
}

func isSyncableSymbol(sym string) bool {
	return sym != "" && !strings.ContainsAny(sym, "^")
}

func fetchStocksResilient(ctx context.Context, client *toss.Client, symbols []string) []toss.StockInfo {
	return fetchStocksResilientRetry(ctx, client, symbols, 0)
}

func fetchStocksResilientRetry(ctx context.Context, client *toss.Client, symbols []string, attempt int) []toss.StockInfo {
	if len(symbols) == 0 {
		return nil
	}
	items, err := client.GetStocks(ctx, symbols)
	if err == nil {
		return items
	}
	if isTossRateLimit(err) && attempt < 5 {
		wait := time.Duration(attempt+1) * time.Second
		log.Printf("rate limited (%d symbols), retry in %s", len(symbols), wait)
		time.Sleep(wait)
		return fetchStocksResilientRetry(ctx, client, symbols, attempt+1)
	}
	if len(symbols) == 1 {
		log.Printf("skip %s: %v", symbols[0], err)
		return nil
	}
	mid := len(symbols) / 2
	left := fetchStocksResilientRetry(ctx, client, symbols[:mid], 0)
	right := fetchStocksResilientRetry(ctx, client, symbols[mid:], 0)
	return append(left, right...)
}

func isTossRateLimit(err error) bool {
	if err == nil {
		return false
	}
	if te, ok := err.(*toss.Error); ok {
		return te.StatusCode == 429 || te.Code == "rate-limit-exceeded"
	}
	return strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "rate-limit")
}
