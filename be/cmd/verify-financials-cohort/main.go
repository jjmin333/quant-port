package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"path/filepath"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	mysql, err := repository.NewMySQLForCLI(cfg.MySQLDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer mysql.Close()
	if err := repository.EnsureSchema(ctx, mysql.DB(), filepath.Join("migrations")); err != nil {
		log.Fatal(err)
	}

	db := mysql.DB()
	stockRepo := repository.NewStockRepo(db)
	periodRepo := repository.NewFinancialPeriodRepo(db)
	fundRepo := repository.NewFundamentalsRepo(db)

	stockOnly := ` AND s.asset_type <> 'etf' AND UPPER(COALESCE(s.security_type,'')) NOT LIKE '%ETF%' AND UPPER(COALESCE(s.security_type,'')) NOT LIKE '%ETN%' `
	bothEmpty := ` NOT EXISTS (SELECT 1 FROM financial_periods fp WHERE fp.symbol=s.symbol) AND (fund.symbol IS NULL OR fund.calc_status IN ('failed','pending') OR (fund.calc_status='partial' AND fund.revenue IS NULL AND fund.net_income IS NULL AND fund.total_equity IS NULL AND fund.roe IS NULL)) `

	fmt.Println("=== financial migration audit ===")
	for _, asset := range []string{"domestic", "us"} {
		q := fmt.Sprintf(`SELECT COUNT(*) FROM stocks s LEFT JOIN stock_fundamentals fund ON fund.symbol=s.symbol WHERE s.status='NORMAL' AND s.asset_type='%s' %s AND %s`, asset, stockOnly, bothEmpty)
		count(ctx, db, asset+" both_empty", q)
	}
	count(ctx, db, "domestic val_ok no_periods",
		`SELECT COUNT(*) FROM stocks s JOIN stock_fundamentals fund ON fund.symbol=s.symbol WHERE s.status='NORMAL' AND s.asset_type='domestic'`+stockOnly+` AND fund.calc_status='ok' AND NOT EXISTS (SELECT 1 FROM financial_periods fp WHERE fp.symbol=s.symbol)`)
	count(ctx, db, "us val_ok no_periods",
		`SELECT COUNT(*) FROM stocks s JOIN stock_fundamentals fund ON fund.symbol=s.symbol WHERE s.status='NORMAL' AND s.asset_type='us'`+stockOnly+` AND fund.calc_status='ok' AND NOT EXISTS (SELECT 1 FROM financial_periods fp WHERE fp.symbol=s.symbol)`)

	fmt.Println("\n=== backfill CLI targets ===")
	dom, _ := stockRepo.ListBothEmptyFinancialSymbols(ctx, repository.FinancialBackfillFilter{AssetType: "domestic"})
	us, _ := stockRepo.ListBothEmptyFinancialSymbols(ctx, repository.FinancialBackfillFilter{AssetType: "us", ExcludeNoCIK: true})
	domP, _ := stockRepo.ListValuationOkNoPeriodsSymbols(ctx, repository.FinancialBackfillFilter{AssetType: "domestic"})
	usP, _ := stockRepo.ListValuationOkNoPeriodsSymbols(ctx, repository.FinancialBackfillFilter{AssetType: "us"})
	fmt.Printf("%-36s %6d\n", "domestic both_empty", len(dom))
	fmt.Printf("%-36s %6d\n", "us both_empty (no CIK excl)", len(us))
	fmt.Printf("%-36s %6d\n", "domestic periods-only", len(domP))
	fmt.Printf("%-36s %6d\n", "us periods-only", len(usP))

	fmt.Println("\n=== spot checks ===")
	spots := []string{"403870", "0105P0", "000400", "059180", "AAPL", "YSWY", "CRWD"}
	for _, sym := range spots {
		auditSymbol(ctx, sym, fundRepo, periodRepo)
	}
}

func count(ctx context.Context, db *sql.DB, label, sqlText string) {
	var n int
	if err := db.QueryRowContext(ctx, sqlText).Scan(&n); err != nil {
		log.Fatal(label, err)
	}
	fmt.Printf("%-36s %6d\n", label, n)
}

func auditSymbol(ctx context.Context, symbol string, fund *repository.FundamentalsRepo, periods *repository.FinancialPeriodRepo) {
	row, _ := fund.GetBySymbol(ctx, symbol)
	hasP, _ := periods.HasPeriods(ctx, symbol)
	status := "missing"
	tier := model.CoverageTierUnavailable
	if row != nil {
		status = string(row.CalcStatus)
		tier = model.ComputeCoverageTier(*row, nil)
	}
	fmt.Printf("  %s  fund=%s tier=%s periods=%v\n", symbol, status, tier, hasP)
}
