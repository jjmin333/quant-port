package main

import (
	"context"
	"flag"
	"log"
	"path/filepath"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/seed"
)

const batchSize = 500

func main() {
	seedDir := flag.String("dir", "data/seed", "directory containing seed files")
	krxPath := flag.String("krx", "", "KRX CSV path (default: {dir}/krx_list.csv)")
	nasdaqPath := flag.String("nasdaq", "", "NASDAQ JSON path (default: {dir}/nasdaq.json)")
	nysePath := flag.String("nyse", "", "NYSE JSON path (default: {dir}/nyse.json)")
	amexPath := flag.String("amex", "", "AMEX JSON path (default: {dir}/amex.json)")
	dryRun := flag.Bool("dry-run", false, "parse files only, do not write to DB")
	flag.Parse()

	resolve := func(explicit, name string) string {
		if explicit != "" {
			return explicit
		}
		return filepath.Join(*seedDir, name)
	}

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

	var all []model.Stock
	for _, src := range sources {
		items, err := src.loader(src.path)
		if err != nil {
			log.Fatalf("%s: load %s: %v", src.label, src.path, err)
		}
		log.Printf("%s: parsed %d rows from %s", src.label, len(items), src.path)
		all = append(all, items...)
	}
	log.Printf("total parsed: %d rows", len(all))

	if *dryRun {
		log.Println("dry-run complete")
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	mysql, err := repository.NewMySQLForCLI(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defer mysql.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	migrationsDir := filepath.Join("migrations")
	if err := repository.EnsureSchema(ctx, mysql.DB(), migrationsDir); err != nil {
		log.Fatalf("schema: %v", err)
	}

	repo := repository.NewStockRepo(mysql.DB())
	for i := 0; i < len(all); i += batchSize {
		end := min(i+batchSize, len(all))
		batch := all[i:end]
		if err := repo.UpsertBatch(ctx, batch); err != nil {
			log.Fatalf("upsert batch %d-%d: %v", i, end, err)
		}
		log.Printf("upserted %d / %d", end, len(all))
	}

	total, err := repo.TotalCount(ctx)
	if err != nil {
		log.Fatalf("count: %v", err)
	}
	log.Printf("seed complete — stocks table row count: %d", total)
}
