package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/enact/quant-port/be/internal/catalog"
	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/stocklogo"
)

func main() {
	catalogPhase := flag.Bool("catalog", true, "sync missing ETP rows into stocks (insert-only)")
	logosPhase := flag.Bool("logos", true, "migrate logos for ETP rows missing manifest/PNG")
	krxCSV := flag.String("krx-etp-csv", "", "fallback domestic ETP CSV (same columns as krx_list.csv)")
	basDD := flag.String("bas-dd", "", "KRX basDd YYYYMMDD (default: latest KST weekday)")
	outDir := flag.String("out", "../fe/public/logos", "logo output dir (manifest.json + kr/*.png)")
	naverDelay := flag.Duration("naver-delay", 150*time.Millisecond, "delay between Naver logo fetches")
	fmpWorkers := flag.Int("fmp-workers", 4, "parallel FMP HEAD workers (conservative default)")
	dryRun := flag.Bool("dry-run", false, "fetch and diff only; do not write DB or logos")
	repairNames := flag.Bool("repair-names", false, "fix corrupted domestic ETP names from KRX UTF-8 catalog")
	flag.Parse()

	if !*catalogPhase && !*logosPhase && !*repairNames {
		log.Fatal("nothing to do: enable -catalog, -logos, and/or -repair-names")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Hour)
	defer cancel()

	var budget *apibudget.Manager
	redis, err := repository.NewRedis(cfg.RedisURL)
	if err != nil {
		log.Printf("warning: redis unavailable (%v) — apibudget in-memory only", err)
	} else {
		defer redis.Close()
		budget = apibudget.NewManager(redis.Client())
	}

	httpClient := &http.Client{Timeout: 45 * time.Second}
	krxClient := marketdata.NewKRXClient(cfg.KRX.APIKey, httpClient, budget)
	finnhubClient := marketdata.NewFinnhubClient(cfg.Finnhub.APIKey, httpClient, budget)

	source := &catalog.FileETPSource{
		DomesticPath: *krxCSV,
		Finnhub:      finnhubClient,
		KRX:          krxClient,
		HTTPClient:   httpClient,
		BasDD:        *basDD,
	}

	if *catalogPhase {
		if *dryRun {
			runCatalogDryRun(ctx, source)
		} else {
			runCatalog(ctx, cfg, source)
		}
	}

	if *repairNames {
		if *dryRun {
			log.Println("repair-names: skipped in -dry-run")
		} else {
			runRepairNames(ctx, cfg, krxClient, *basDD)
		}
	}

	if *logosPhase {
		if *dryRun {
			log.Println("logos: skipped in -dry-run")
			return
		}
		runLogos(ctx, cfg, *outDir, *naverDelay, *fmpWorkers)
	}
}

func runCatalogDryRun(ctx context.Context, source catalog.ETPSource) {
	var domesticN, usN int
	if domestic, err := source.DomesticETP(ctx); err != nil {
		log.Printf("dry-run domestic: %v (skipped)", err)
	} else {
		domesticN = len(domestic)
	}
	if us, err := source.US_ETP(ctx); err != nil {
		log.Printf("dry-run us: %v (skipped)", err)
	} else {
		usN = len(us)
	}
	if domesticN == 0 && usN == 0 {
		log.Fatal("dry-run: no catalog rows fetched — set KRX_API_KEY or -krx-etp-csv, and FINNHUB_API_KEY")
	}
	log.Printf("dry-run catalog: domestic=%d us=%d (no DB writes)", domesticN, usN)
}

func runCatalog(ctx context.Context, cfg *config.Config, source catalog.ETPSource) {
	mysql, err := repository.NewMySQLForCLI(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defer mysql.Close()

	migrationsDir := filepath.Join("migrations")
	if err := repository.EnsureSchema(ctx, mysql.DB(), migrationsDir); err != nil {
		log.Fatalf("schema: %v", err)
	}

	repo := repository.NewStockRepo(mysql.DB())
	sync := &catalog.SyncETPDelta{Stocks: repo, Source: source}
	res, err := sync.Run(ctx)
	if err != nil {
		log.Fatalf("catalog sync: %v", err)
	}
	fmt.Println()
	fmt.Println("=== ETP catalog sync (insert-only) ===")
	fmt.Printf("domestic fetched: %d\n", res.DomesticFetched)
	fmt.Printf("us fetched:       %d\n", res.USFetched)
	fmt.Printf("unique incoming:  %d\n", res.UniqueIncoming)
	fmt.Printf("already in DB:    %d\n", res.AlreadyInDB)
	fmt.Printf("inserted:         %d\n", res.Inserted)
	if res.DomesticError != "" {
		fmt.Printf("domestic warning: %s\n", res.DomesticError)
	}
	if res.USError != "" {
		fmt.Printf("us warning:       %s\n", res.USError)
	}
	fmt.Println()
}

func runRepairNames(ctx context.Context, cfg *config.Config, krx *marketdata.KRXClient, basDD string) {
	mysql, err := repository.NewMySQLForCLI(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defer mysql.Close()

	repo := repository.NewStockRepo(mysql.DB())
	n, err := catalog.RepairDomesticETPNames(ctx, repo, krx, basDD)
	if err != nil {
		log.Fatalf("repair names: %v", err)
	}
	fmt.Printf("repaired domestic ETP names: %d rows\n", n)
}

func runLogos(ctx context.Context, cfg *config.Config, outDir string, naverDelay time.Duration, fmpWorkers int) {
	mysql, err := repository.NewMySQLForCLI(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defer mysql.Close()

	repo := repository.NewStockRepo(mysql.DB())
	stocks, err := repo.ListETPStocks(ctx)
	if err != nil {
		log.Fatalf("list etp stocks: %v", err)
	}
	log.Printf("logos: %d ETP rows in DB", len(stocks))

	manifestPath := filepath.Join(outDir, "manifest.json")
	if err := stocklogo.LoadManifest(manifestPath); err != nil {
		log.Fatalf("load manifest: %v", err)
	}
	existing := stocklogo.GetManifest()
	if existing == nil {
		existing = &stocklogo.Manifest{Entries: map[string]stocklogo.ManifestEntry{}}
	}

	before := len(stocks)
	stocks = stocklogo.FilterDeltaLogoStocks(stocks, existing, filepath.Join(outDir, "kr"), filepath.Join(outDir, "us"), true, false)
	log.Printf("logos: %d rows need migration (skipped %d already done)", len(stocks), before-len(stocks))

	manifest, err := stocklogo.RunMigrate(ctx, stocklogo.MigrateOptions{
		OutDir:     outDir,
		Stocks:     stocks,
		NaverDelay: naverDelay,
		FMPWorkers: fmpWorkers,
		DeltaOnly:  true,
		Existing:   existing,
	})
	if err != nil {
		log.Fatalf("logo migrate: %v", err)
	}
	if err := stocklogo.SaveManifest(manifestPath, manifest); err != nil {
		log.Fatalf("save manifest: %v", err)
	}
	log.Printf("logos: manifest saved to %s", manifestPath)
}
