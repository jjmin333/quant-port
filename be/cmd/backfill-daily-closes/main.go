// Command backfill-daily-closes loads the 일별 종가 store from the daily bars

// already in candle_bars, then reports coverage.

//

// Safe overseas backfill (avoid MySQL overload):

//

//	go run ./cmd/backfill-daily-closes -status -market=us -since=2026-01-01

//	./scripts/backfill-us-safe.ps1

//

// Manual chunked fetch (no heavy projection):

//

//	go run ./cmd/backfill-daily-closes -skip-projection -market=us -since=2026-01-01 \

//	  -fetch-history -fetch-limit=100 -fetch-log-every=10 -timeout-min=60

//

// Manual chunked projection (one month at a time):

//

//	go run ./cmd/backfill-daily-closes -projection-only -market=us \

//	  -since=2026-01-01 -until=2026-01-31 -timeout-min=15

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



func main() {

	since := flag.String("since", service.DailyCloseBackfillStart, "first trade date to load (YYYY-MM-DD)")

	until := flag.String("until", "", "last trade date to load inclusive (YYYY-MM-DD); empty = no upper bound")

	market := flag.String("market", "all", "market scope: all, domestic, or us (us+etf)")

	timeoutMin := flag.Int("timeout-min", 30, "job timeout in minutes")

	skipProjection := flag.Bool("skip-projection", false, "skip candle_bars→stock_daily_closes SQL (use before fetch-only batches)")

	projectionOnly := flag.Bool("projection-only", false, "only run projection; ignore fetch flags")

	statusOnly := flag.Bool("status", false, "print incomplete/missing counts and exit (no writes)")

	fetchMissing := flag.Bool("fetch-missing", false, "fetch daily candles from Toss for symbols with no stored close")

	fetchHistory := flag.Bool("fetch-history", false, "fetch older daily candles until -since for symbols whose history starts too late")

	fetchStale := flag.String("fetch-stale", "", "fetch symbols whose newest stored close predates this date (YYYY-MM-DD)")

	fetchLimit := flag.Int("fetch-limit", 200, "max symbols to fetch in one -fetch-missing/-fetch-stale run")

	fetchDelayMs := flag.Int("fetch-delay-ms", 350, "pause between Toss candle requests")

	fetchLogEvery := flag.Int("fetch-log-every", 0, "log fetch progress every N symbols (0=summary only)")

	flag.Parse()



	if _, err := time.Parse("2006-01-02", *since); err != nil {

		log.Fatalf("invalid -since %q: %v", *since, err)

	}

	if *until != "" {

		if _, err := time.Parse("2006-01-02", *until); err != nil {

			log.Fatalf("invalid -until %q: %v", *until, err)

		}

	}

	if *fetchStale != "" {

		if _, err := time.Parse("2006-01-02", *fetchStale); err != nil {

			log.Fatalf("invalid -fetch-stale %q: %v", *fetchStale, err)

		}

	}

	if *skipProjection && *projectionOnly {

		log.Fatal("-skip-projection and -projection-only are mutually exclusive")

	}



	assetTypes, err := service.ParseDailyCloseMarket(*market)

	if err != nil {

		log.Fatal(err)

	}



	cfg, err := config.Load()

	if err != nil {

		log.Fatalf("config: %v", err)

	}



	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutMin)*time.Minute)

	defer cancel()



	mysql, err := repository.NewMySQL(cfg.MySQLDSN)

	if err != nil {

		log.Fatalf("mysql: %v", err)

	}

	defer mysql.Close()



	if err := repository.MigrateDailyClosesIfNeeded(ctx, mysql.DB(), filepath.Join("migrations")); err != nil {

		log.Fatalf("migrate: %v", err)

	}



	stockRepo := repository.NewStockDailyCloseRepo(mysql.DB())

	svc := service.NewDailyCloseService(

		mysql.DB(),

		stockRepo,

		repository.NewIndexDailyCloseRepo(mysql.DB()),

		repository.NewStockRepo(mysql.DB()),

		nil,

	)



	if *statusOnly {

		printStatus(ctx, svc, *since, assetTypes, *market)

		return

	}



	if pruned, err := svc.PruneWeekendRows(ctx); err != nil {

		log.Printf("prune weekend rows: %v", err)

	} else if pruned > 0 {

		log.Printf("pruned %d weekend-dated rows (bad upstream bars)", pruned)

	}



	runFetch := !*projectionOnly && (*fetchMissing || *fetchStale != "" || *fetchHistory)



	if !*skipProjection {

		window := *since

		if *until != "" {

			window += " … " + *until

		}

		log.Printf("projection %s (market=%s) …", window, *market)

		start := time.Now()

		affected, err := svc.BackfillStocksFromCandleBarsRange(ctx, *since, *until, assetTypes...)

		if err != nil {

			log.Fatalf("backfill: %v", err)

		}

		log.Printf("projection done in %s (%d affected rows; MySQL counts an updated row twice)",

			time.Since(start).Round(time.Millisecond), affected)

	} else {

		log.Printf("projection skipped (-skip-projection)")

	}



	if runFetch {

		if !cfg.Toss.Enabled() {

			log.Fatal("fetch-missing/-fetch-stale/-fetch-history requires Toss credentials")

		}

		redis, err := repository.NewRedis(cfg.RedisURL)

		if err != nil {

			log.Fatalf("redis: %v", err)

		}

		defer redis.Close()

		tossClient, err := toss.NewClient(cfg.Toss, toss.NewTokenStore(redis.Client()))

		if err != nil {

			log.Fatalf("toss: %v", err)

		}



		barRepo := repository.NewCandleBarRepo(mysql.DB())

		delay := time.Duration(*fetchDelayMs) * time.Millisecond



		if *fetchStale != "" {

			res, err := svc.FetchStaleCloses(ctx, tossClient, barRepo, *fetchStale, *since, *fetchLimit, delay, *fetchLogEvery, assetTypes...)

			if err != nil {

				log.Printf("fetch-stale: %v", err)

			}

			log.Printf("fetch-stale(<%s): attempted=%d filled=%d no-data=%d failed=%d (was %d symbols stale before this run)",

				*fetchStale, res.Attempted, res.Filled, res.Empty, res.Failed, res.Remaining)

		}



		if *fetchMissing {

			res, err := svc.FetchMissingCloses(ctx, tossClient, barRepo, *since, *fetchLimit, delay, *fetchLogEvery, assetTypes...)

			if err != nil {

				log.Printf("fetch-missing: %v", err)

			}

			log.Printf("fetch-missing: attempted=%d filled=%d no-data=%d failed=%d (was %d symbols missing before this run)",

				res.Attempted, res.Filled, res.Empty, res.Failed, res.Remaining)

		}



		if *fetchHistory {

			res, err := svc.FetchHistoryCloses(ctx, tossClient, barRepo, *since, *fetchLimit, delay, *fetchLogEvery, assetTypes...)

			if err != nil {

				log.Printf("fetch-history: %v", err)

			}

			log.Printf("fetch-history(>%s): attempted=%d filled=%d no-data=%d failed=%d (was %d symbols with incomplete history before this run)",

				*since, res.Attempted, res.Filled, res.Empty, res.Failed, res.Remaining)

		}

	}



	printCoverage(ctx, stockRepo, mysql, *since, assetTypes, *market)

}



func printStatus(ctx context.Context, svc *service.DailyCloseService, since string, assetTypes []model.QuoteAssetType, market string) {

	incomplete, err := svc.HistoryIncompleteCount(ctx, since, assetTypes...)

	if err != nil {

		log.Fatalf("status: %v", err)

	}

	_, missing, err := svc.SymbolsWithoutCloses(ctx, since, 0, assetTypes...)

	if err != nil {

		log.Fatalf("status: %v", err)

	}

	log.Printf("status market=%s since=%s · history_incomplete=%d · no_close_at_all=%d",

		market, since, incomplete, missing)

}



func printCoverage(ctx context.Context, stockRepo *repository.StockDailyCloseRepo, mysql *repository.MySQL, since string, assetTypes []model.QuoteAssetType, market string) {

	rows, symbols, minDate, maxDate, err := stockRepo.Stats(ctx, since)

	if err != nil {

		log.Fatalf("stats: %v", err)

	}

	log.Printf("stock_daily_closes since %s: %d rows across %d symbols (%s … %s)",

		since, rows, symbols, minDate, maxDate)



	filter, filterArgs := assetTypeFilterForMaster(assetTypes)

	var total int

	countSQL := `SELECT COUNT(*) FROM stocks WHERE status = 'NORMAL'` + strings.Replace(filter, "st.", "", -1)

	if err := mysql.DB().QueryRowContext(ctx, countSQL, filterArgs...).Scan(&total); err != nil {

		log.Printf("coverage: %v", err)

		return

	}

	log.Printf("coverage: %d/%d active master symbols have closes since %s (market=%s)",

		symbols, total, since, market)

}



func assetTypeFilterForMaster(assetTypes []model.QuoteAssetType) (string, []any) {

	if len(assetTypes) == 0 {

		return "", nil

	}

	placeholders := make([]string, len(assetTypes))

	args := make([]any, len(assetTypes))

	for i, t := range assetTypes {

		placeholders[i] = "?"

		args[i] = string(t)

	}

	return " AND asset_type IN (" + strings.Join(placeholders, ",") + ")", args

}


