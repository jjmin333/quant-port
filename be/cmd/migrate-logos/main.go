package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/seed"
	"github.com/enact/quant-port/be/internal/stocklogo"
)

func main() {
	seedDir := flag.String("dir", "data/seed", "directory containing seed files")
	krxPath := flag.String("krx", "", "KRX CSV path (default: {dir}/krx_list.csv)")
	nasdaqPath := flag.String("nasdaq", "", "NASDAQ JSON path")
	nysePath := flag.String("nyse", "", "NYSE JSON path")
	amexPath := flag.String("amex", "", "AMEX JSON path")
	fromDB := flag.Bool("from-db", false, "load symbols from MySQL stocks table")
	useSeed := flag.Bool("seed", true, "load symbols from KRX CSV + US seed JSON")
	outDir := flag.String("out", "../fe/public/logos", "output directory for manifest.json and kr/*.png")
	usMax := flag.Int("us-max", 0, "max US symbols to FMP-probe (0 = all)")
	skipDomestic := flag.Bool("skip-domestic", false, "skip Naver download; keep existing kr/*.png and domestic manifest entries")
	deltaOnly := flag.Bool("delta-only", false, "skip symbols that already have manifest + local PNG (domestic) or FMP probe (US)")
	etpOnly := flag.Bool("etp-only", false, "with -from-db, load ETF/ETN rows only")
	naverDelay := flag.Duration("naver-delay", 120*time.Millisecond, "delay between Naver logo fetches")
	fmpWorkers := flag.Int("fmp-workers", 12, "parallel FMP HEAD workers")
	flag.Parse()

	if *deltaOnly {
		if *naverDelay == 120*time.Millisecond {
			*naverDelay = 150 * time.Millisecond
		}
		if *fmpWorkers == 12 {
			*fmpWorkers = 4
		}
	}

	resolve := func(explicit, name string) string {
		if explicit != "" {
			return explicit
		}
		return filepath.Join(*seedDir, name)
	}

	var stocks []model.Stock
	seen := map[string]struct{}{}

	if *fromDB {
		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("config: %v", err)
		}
		mysql, err := repository.NewMySQL(cfg.MySQLDSN)
		if err != nil {
			log.Fatalf("mysql: %v", err)
		}
		defer mysql.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		repo := repository.NewStockRepo(mysql.DB())
		var rows []model.Stock
		if *etpOnly {
			rows, err = repo.ListETPStocks(ctx)
		} else {
			rows, err = repo.ListLogoStocks(ctx)
		}
		if err != nil {
			log.Fatalf("list logo stocks: %v", err)
		}
		log.Printf("db: loaded %d symbols", len(rows))
		for _, s := range rows {
			addStock(seen, &stocks, s)
		}
	}

	if *useSeed {
		sources := []struct {
			label  string
			path   string
			loader func(string) ([]model.Stock, error)
		}{
			{"krx", resolve(*krxPath, "krx_list.csv"), seed.LoadKRXCSV},
			{"nasdaq", resolve(*nasdaqPath, "nasdaq.json"), func(p string) ([]model.Stock, error) {
				return seed.LoadUSJSON(p, "NASDAQ")
			}},
			{"nyse", resolve(*nysePath, "nyse.json"), func(p string) ([]model.Stock, error) {
				return seed.LoadUSJSON(p, "NYSE")
			}},
			{"amex", resolve(*amexPath, "amex.json"), func(p string) ([]model.Stock, error) {
				return seed.LoadUSJSON(p, "AMEX")
			}},
		}
		for _, src := range sources {
			items, err := src.loader(src.path)
			if err != nil {
				if os.IsNotExist(err) {
					log.Printf("%s: skip missing file %s", src.label, src.path)
					continue
				}
				log.Fatalf("%s: load %s: %v", src.label, src.path, err)
			}
			log.Printf("%s: parsed %d rows from %s", src.label, len(items), src.path)
			for _, s := range items {
				addStock(seen, &stocks, s)
			}
		}
	}

	if len(stocks) == 0 {
		log.Fatal("no symbols to migrate — use -from-db and/or -seed with valid data files")
	}

	if *usMax > 0 {
		stocks = capUS(stocks, *usMax)
		log.Printf("us-max: capped catalog to %d symbols", len(stocks))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	defer cancel()

	manifestPath := filepath.Join(*outDir, "manifest.json")
	var existing *stocklogo.Manifest
	if *skipDomestic || *deltaOnly {
		if err := stocklogo.LoadManifest(manifestPath); err != nil {
			log.Fatalf("load existing manifest: %v", err)
		}
		existing = stocklogo.GetManifest()
		if existing == nil {
			existing = &stocklogo.Manifest{Entries: map[string]stocklogo.ManifestEntry{}}
		}
	}
	if *skipDomestic {
		stocks = filterUSOnly(stocks)
		log.Printf("skip-domestic: probing %d US symbols only", len(stocks))
	}
	if *deltaOnly {
		before := len(stocks)
		stocks = stocklogo.FilterDeltaLogoStocks(stocks, existing, filepath.Join(*outDir, "kr"), true)
		log.Printf("delta-only: %d symbols need migration (skipped %d)", len(stocks), before-len(stocks))
	}

	manifest, err := stocklogo.RunMigrate(ctx, stocklogo.MigrateOptions{
		OutDir:       *outDir,
		Stocks:       stocks,
		NaverDelay:   *naverDelay,
		FMPWorkers:   *fmpWorkers,
		SkipDomestic: *skipDomestic,
		DeltaOnly:    *deltaOnly,
		Existing:     existing,
	})
	if err != nil {
		log.Fatalf("migrate: %v", err)
	}

	if err := stocklogo.SaveManifest(manifestPath, manifest); err != nil {
		log.Fatalf("save manifest: %v", err)
	}

	printSummary(manifest)
	log.Printf("manifest: %s", manifestPath)
}

func addStock(seen map[string]struct{}, dst *[]model.Stock, s model.Stock) {
	sym := strings.ToUpper(strings.TrimSpace(s.Symbol))
	if sym == "" {
		return
	}
	if _, ok := seen[sym]; ok {
		return
	}
	seen[sym] = struct{}{}
	s.Symbol = sym
	*dst = append(*dst, s)
}

func capUS(stocks []model.Stock, max int) []model.Stock {
	if max <= 0 {
		return stocks
	}
	var out []model.Stock
	us := 0
	for _, s := range stocks {
		if stocklogo.IsDomestic(s.Currency, s.AssetType) {
			out = append(out, s)
			continue
		}
		if us >= max {
			continue
		}
		out = append(out, s)
		us++
	}
	return out
}

func filterUSOnly(stocks []model.Stock) []model.Stock {
	var out []model.Stock
	for _, s := range stocks {
		if !stocklogo.IsDomestic(s.Currency, s.AssetType) {
			out = append(out, s)
		}
	}
	return out
}

func printSummary(m *stocklogo.Manifest) {
	d := m.Summary.Domestic
	u := m.Summary.US
	fmt.Println()
	fmt.Println("=== Logo migration summary ===")
	fmt.Printf("국내주식: total=%d naver성공=%d naver실패=%d (실패→FMP)=%d\n",
		d.Total, d.NaverSuccess, d.NaverFail, d.FMPFallback)
	fmt.Printf("미국주식: total=%d naver스킵=%d FMP성공=%d FMP실패=%d\n",
		u.Total, u.NaverSkipped, u.FMPAvailable, u.FMPUnavailable)
	fmt.Println()
}
