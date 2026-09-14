package main

import (
	"context"
	"database/sql"
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

func main() {
	symbolsFlag := flag.String("symbols", "", "comma-separated symbols (e.g. 005930,AVGO)")
	fromDB := flag.Bool("from-db", false, "load symbols from stocks table")
	assetType := flag.String("asset-type", "", "filter for --from-db: domestic | us | all")
	limit := flag.Int("limit", 0, "max symbols when using --from-db (0 = no limit)")
	skipExisting := flag.Bool("skip-existing", false, "skip symbols already present in stock_fundamentals")
	retryNotOK := flag.Bool("retry-not-ok", false, "refresh symbols where calc_status is not ok")
	recalcStatus := flag.Bool("recalc-status", false, "recalculate calc_status from stored metrics only (no refetch)")
	timeoutHours := flag.Int("timeout-hours", 8, "job timeout in hours (0 = no limit)")
	dryRun := flag.Bool("dry-run", false, "fetch and compute only, do not write to DB")
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

	mysql, err := repository.NewMySQLForCLI(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defer mysql.Close()

	migrationsDir := filepath.Join("migrations")
	if err := repository.EnsureSchema(ctx, mysql.DB(), migrationsDir); err != nil {
		log.Fatalf("schema: %v", err)
	}

	fundRepo := repository.NewFundamentalsRepo(mysql.DB())

	if *recalcStatus {
		updated, err := fundRepo.RecalcAllStatuses(ctx)
		if err != nil {
			log.Fatalf("recalc-status: %v", err)
		}
		log.Printf("recalc-status done — updated %d rows", updated)
		return
	}

	symbols := parseSymbols(*symbolsFlag)
	if *retryNotOK {
		notOK, err := fundRepo.ListSymbolsNotOK(ctx)
		if err != nil {
			log.Fatalf("retry-not-ok: %v", err)
		}
		symbols = append(symbols, notOK...)
		log.Printf("retry-not-ok: %d symbols to refresh", len(notOK))
	}
	if len(symbols) == 0 && !*fromDB {
		if !*retryNotOK {
			log.Fatal("provide --symbols, --from-db, or --retry-not-ok")
		}
	}
	if *fromDB {
		dbSymbols, err := loadSymbolsFromDB(ctx, mysql.DB(), *assetType, *limit)
		if err != nil {
			log.Fatalf("load symbols: %v", err)
		}
		symbols = append(symbols, dbSymbols...)
	}
	symbols = dedupeSymbols(symbols)
	if *skipExisting {
		existing, err := fundRepo.ListSymbols(ctx)
		if err != nil {
			log.Fatalf("load existing fundamentals: %v", err)
		}
		before := len(symbols)
		symbols = filterNewSymbols(symbols, existing)
		log.Printf("skip-existing: %d already seeded, %d remaining", before-len(symbols), len(symbols))
	}
	if len(symbols) == 0 {
		log.Println("nothing to process")
		return
	}

	dartClient := service.NewDartClient(service.DartConfig{APIKey: cfg.Dart.APIKey}, nil)
	edgarClient := service.NewEdgarClient(service.EdgarConfig{UserAgent: cfg.Edgar.UserAgent}, nil)
	stockRepo := repository.NewStockRepo(mysql.DB())

	var tossClient *toss.Client
	if cfg.Toss.Enabled() {
		redis, err := repository.NewRedis(cfg.RedisURL)
		if err != nil {
			log.Fatalf("redis: %v", err)
		}
		defer redis.Close()
		tossClient, err = toss.NewClient(cfg.Toss, toss.NewTokenStore(redis.Client()))
		if err != nil {
			log.Fatalf("toss: %v", err)
		}
	} else {
		log.Println("warning: Toss not configured — shares enrichment from Toss unavailable")
	}

	fundSvc := service.NewFundamentalsService(
		dartClient,
		edgarClient,
		tossClient,
		stockRepo,
		fundRepo,
	)

	ok, partial, failed := 0, 0, 0
	for i, symbol := range symbols {
		log.Printf("[%d/%d] %s", i+1, len(symbols), symbol)
		if *dryRun {
			stock, err := stockRepo.GetBySymbol(ctx, symbol)
			if err != nil || stock == nil {
				log.Printf("  skip: not in stocks table")
				failed++
				time.Sleep(120 * time.Millisecond)
				continue
			}
			if err := dryRunSymbol(ctx, dartClient, edgarClient, stock); err != nil {
				log.Printf("  failed: %v", err)
				failed++
			} else {
				ok++
			}
			time.Sleep(120 * time.Millisecond)
			continue
		}

		row, err := fundSvc.RefreshSymbol(ctx, symbol)
		if err != nil {
			log.Printf("  error: %v", err)
			failed++
			continue
		}
		switch row.CalcStatus {
		case model.FundamentalsStatusOK:
			ok++
			log.Printf("  ok fiscal_year=%v roe=%v", derefInt(row.FiscalYear), derefFloat(row.ROE))
		case model.FundamentalsStatusPartial:
			partial++
			log.Printf("  partial missing=%s", row.CalcError)
		default:
			failed++
			log.Printf("  failed: %s", row.CalcError)
		}
		time.Sleep(120 * time.Millisecond)
	}

	log.Printf("done — ok=%d partial=%d failed=%d total=%d", ok, partial, failed, len(symbols))
}

func dryRunSymbol(ctx context.Context, dart *service.DartClient, edgar *service.EdgarClient, stock *model.Stock) error {
	switch stock.AssetType {
	case model.AssetDomestic:
		cur, prior, err := dart.FindLatestAnnualSnapshots(ctx, stock.Symbol)
		if err != nil {
			return err
		}
		computed := service.ComputeStoredFundamentals(cur, prior, stock.SharesOutstanding)
		log.Printf("  dart FY=%d roe=%v rev_growth=%v", cur.FiscalYear, derefFloat(computed.ROE), derefFloat(computed.RevenueGrowthYoY))
	default:
		cur, prior, shares, err := edgar.FetchFinancialSnapshots(ctx, stock.Symbol)
		if err != nil {
			return err
		}
		sharesVal := shares
		if sharesVal == nil {
			sharesVal = stock.SharesOutstanding
		}
		computed := service.ComputeStoredFundamentals(cur, prior, sharesVal)
		log.Printf("  edgar FY=%d roe=%v rev_growth=%v", cur.FiscalYear, derefFloat(computed.ROE), derefFloat(computed.RevenueGrowthYoY))
	}
	return nil
}

func loadSymbolsFromDB(ctx context.Context, db *sql.DB, assetType string, limit int) ([]string, error) {
	return repository.NewStockRepo(db).ListSymbols(ctx, assetType, limit)
}

func parseSymbols(raw string) []string {
	parts := strings.Split(raw, ",")
	var out []string
	for _, p := range parts {
		p = strings.ToUpper(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func filterNewSymbols(symbols []string, existing map[string]struct{}) []string {
	var out []string
	for _, s := range symbols {
		if _, ok := existing[strings.ToUpper(strings.TrimSpace(s))]; ok {
			continue
		}
		out = append(out, s)
	}
	return out
}

func dedupeSymbols(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func derefInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func derefFloat(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}
