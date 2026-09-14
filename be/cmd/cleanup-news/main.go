package main

import (
	"context"
	"flag"
	"log"
	"path/filepath"
	"strings"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/redis/go-redis/v9"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "scan only; do not delete rows")
	market := flag.String("market", "all", "market to clean: domestic, us, or all")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()

	mysql, err := repository.NewMySQLForCLI(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defer mysql.Close()

	migrationsDir := filepath.Join("migrations")
	if err := repository.EnsureSchema(ctx, mysql.DB(), migrationsDir); err != nil {
		log.Fatalf("schema: %v", err)
	}

	var redisClient *redis.Client
	redis, err := repository.NewRedis(cfg.RedisURL)
	if err != nil {
		log.Printf("warning: redis unavailable (%v) — cache refresh skipped", err)
	} else {
		defer redis.Close()
		redisClient = redis.Client()
	}

	newsRepo := repository.NewNewsRepo(mysql.DB())
	stockRepo := repository.NewStockRepo(mysql.DB())
	watchlistRepo := repository.NewWatchlistRepo(mysql.DB())
	newsStore := service.NewNewsStore(cfg.News, nil, nil, nil, nil, nil, nil, newsRepo, nil, nil, stockRepo, watchlistRepo, nil, redisClient)

	for _, m := range marketsToClean(*market) {
		result, err := newsStore.CleanupIrrelevantArticles(ctx, m, *dryRun)
		if err != nil {
			log.Fatalf("cleanup (%s): %v", m, err)
		}

		mode := "deleted"
		if result.DryRun {
			mode = "would delete"
		}
		log.Printf("news cleanup (%s): total=%d kept=%d %s=%d dry_run=%v",
			m, result.Total, result.Kept, mode, result.Removed, result.DryRun)
	}
}

func marketsToClean(market string) []string {
	switch strings.ToLower(strings.TrimSpace(market)) {
	case "domestic":
		return []string{"domestic"}
	case "us":
		return []string{"us"}
	case "all", "":
		return []string{"domestic", "us"}
	default:
		log.Fatalf("unknown market %q (use domestic, us, or all)", market)
		return nil
	}
}
