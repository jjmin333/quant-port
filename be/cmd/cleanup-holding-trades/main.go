package main

import (
	"context"
	"flag"
	"log"
	"path/filepath"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/repository"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "report mismatches only; do not delete")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()

	mysql, err := repository.NewMySQL(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defer mysql.Close()

	migrationsDir := filepath.Join("migrations")
	if err := repository.EnsureSchema(ctx, mysql.DB(), migrationsDir); err != nil {
		log.Fatalf("schema: %v", err)
	}

	result, err := repository.CleanupHoldingTradesPriceMismatch(ctx, mysql.DB(), *dryRun)
	if err != nil {
		log.Fatalf("cleanup: %v", err)
	}

	mode := "deleted"
	if result.DryRun {
		mode = "would delete"
	}
	log.Printf("holding_trades price-mismatch cleanup: pairs=%d %s_trades=%d holdings_reset=%d dry_run=%v",
		result.AffectedPairs, mode, result.DeletedTrades, result.ResetHoldings, result.DryRun)
}
