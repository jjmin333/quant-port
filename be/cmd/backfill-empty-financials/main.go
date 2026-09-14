package main

import (
	"context"
	"flag"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/internal/toss"
)

func main() {
	market := flag.String("market", "domestic", "domestic | us")
	symbolsFlag := flag.String("symbols", "", "comma-separated symbols (overrides cohort query)")
	trackedOnly := flag.Bool("tracked-only", false, "only watchlist + holdings")
	limit := flag.Int("limit", 0, "max symbols (0 = headroom cap)")
	dryRun := flag.Bool("dry-run", false, "list targets only, no external calls")
	periodsOnly := flag.Bool("periods-only", false, "ingest quarterly history only (valuation already ok)")
	holesOnly := flag.Bool("holes-only", false, "re-read symbols whose stored quarters have missing figures (parser-fix cohort)")
	dartReserve := flag.Int("dart-reserve", 5000, "DART daily calls to leave for scheduled jobs")
	timeoutHours := flag.Int("timeout-hours", 12, "job timeout (0 = none)")
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

	redis, err := repository.NewRedis(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer redis.Close()

	apiBudget := apibudget.NewManager(redis.Client())
	dartClient := service.NewDartClient(service.DartConfig{APIKey: cfg.Dart.APIKey}, apiBudget)
	edgarClient := service.NewEdgarClient(service.EdgarConfig{UserAgent: cfg.Edgar.UserAgent}, apiBudget).UseBatchBudget()
	stockRepo := repository.NewStockRepo(mysql.DB())
	fundRepo := repository.NewFundamentalsRepo(mysql.DB())

	var tossClient *toss.Client
	if cfg.Toss.Enabled() {
		tossClient, err = toss.NewClient(cfg.Toss, toss.NewTokenStore(redis.Client()))
		if err != nil {
			log.Fatalf("toss: %v", err)
		}
	}

	fundSvc := service.NewFundamentalsService(dartClient, edgarClient, tossClient, stockRepo, fundRepo)
	periodRepo := repository.NewFinancialPeriodRepo(mysql.DB())
	dartGate := service.NewProviderGate(apiBudget.LimitsFor(apibudget.ProviderDART).MinInterval)
	reference := service.NewReferenceIngestService(
		periodRepo,
		repository.NewDisclosureRepo(mysql.DB()),
		repository.NewWarningRepo(mysql.DB()),
		repository.NewSyncStateRepo(mysql.DB()),
		stockRepo,
		dartClient,
		edgarClient,
		tossClient,
		dartGate,
	)
	backfill := service.NewFinancialsBackfillService(stockRepo, fundSvc, fundRepo, periodRepo, reference, apiBudget)

	var asset model.QuoteAssetType
	switch *market {
	case "domestic", "kr":
		asset = model.AssetDomestic
	case "us", "overseas":
		asset = model.AssetUS
	default:
		log.Fatalf("unknown market %q", *market)
	}

	opts := service.FinancialsBackfillOptions{
		Market:      asset,
		TrackedOnly: *trackedOnly,
		Limit:       *limit,
		DryRun:      *dryRun,
		PeriodsOnly: *periodsOnly,
		HolesOnly:   *holesOnly,
		DARTReserve: *dartReserve,
	}.Normalized()

	extraSymbols := parseBackfillSymbols(*symbolsFlag)
	if len(extraSymbols) > 0 {
		if *dryRun {
			log.Printf("dry-run %s: %d explicit symbols", asset, len(extraSymbols))
			for i, sym := range extraSymbols {
				log.Printf("  [%d] %s", i+1, sym)
			}
			printHeadroom(ctx, apiBudget, asset)
			return
		}
		result := service.FinancialsBackfillResult{}
		for i, sym := range extraSymbols {
			if ctx.Err() != nil {
				break
			}
			result.Attempted++
			msg, ok := backfill.BackfillSymbol(ctx, sym, opts)
			if ok {
				result.OK++
			} else {
				result.Failed++
				if msg != "" {
					result.Details = append(result.Details, msg)
				}
			}
			log.Printf("financials backfill [%d/%d] %s ok=%v", i+1, len(extraSymbols), sym, ok)
			if i+1 < len(extraSymbols) {
				time.Sleep(opts.SymbolPause)
			}
		}
		log.Printf("done attempted=%d ok=%d failed=%d", result.Attempted, result.OK, result.Failed)
		printHeadroom(ctx, apiBudget, asset)
		return
	}

	if *dryRun {
		symbols, err := backfill.ListTargets(ctx, opts)
		if err != nil {
			log.Fatalf("list: %v", err)
		}
		mode := "full"
		if *holesOnly {
			mode = "holes-only"
		} else if *periodsOnly {
			mode = "periods-only"
		}
		log.Printf("dry-run %s (%s): %d symbols", asset, mode, len(symbols))
		for i, sym := range symbols {
			log.Printf("  [%d] %s", i+1, sym)
		}
		printHeadroom(ctx, apiBudget, asset)
		return
	}

	printHeadroom(ctx, apiBudget, asset)
	result, err := backfill.Run(ctx, opts)
	if err != nil {
		log.Fatalf("backfill: %v", err)
	}
	log.Printf("done attempted=%d ok=%d failed=%d skipped=%d stopped=%q",
		result.Attempted, result.OK, result.Failed, result.Skipped, result.Stopped)
	for _, d := range result.Details {
		log.Printf("  detail: %s", d)
	}
	printHeadroom(ctx, apiBudget, asset)
}

func parseBackfillSymbols(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToUpper(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func printHeadroom(ctx context.Context, b *apibudget.Manager, market model.QuoteAssetType) {
	p := apibudget.ProviderDART
	if market == model.AssetUS {
		p = apibudget.ProviderEdgarBulk
	}
	h := b.Headroom(ctx, p)
	u := b.Usage(ctx, p)
	log.Printf("quota %s: headroom_calls=%d used_day=%d", p, h.Calls, u.DayCount)
}
