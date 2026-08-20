package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

type FundamentalsService struct {
	dart        *DartClient
	edgar       *EdgarClient
	toss        *toss.Client
	stocks      *repository.StockRepo
	repo        *repository.FundamentalsRepo
	marketStore *MarketDataStore
}

func NewFundamentalsService(
	dart *DartClient,
	edgar *EdgarClient,
	tossClient *toss.Client,
	stocks *repository.StockRepo,
	repo *repository.FundamentalsRepo,
) *FundamentalsService {
	return &FundamentalsService{
		dart:   dart,
		edgar:  edgar,
		toss:   tossClient,
		stocks: stocks,
		repo:   repo,
	}
}

func (s *FundamentalsService) SetMarketStore(store *MarketDataStore) {
	s.marketStore = store
}

// fundamentalsAssetType resolves DART vs EDGAR source; guards against stale us mis-tags on KOSPI rows.
func (s *FundamentalsService) fundamentalsAssetType(stock *model.Stock) model.QuoteAssetType {
	return ResolveFundamentalsAssetType(stock)
}

// ResolveFundamentalsAssetType resolves DART vs EDGAR source; guards against
// stale us mis-tags on KOSPI rows. Shared by FundamentalsService and
// EarningsTabService — both need the same domestic/US split.
func ResolveFundamentalsAssetType(stock *model.Stock) model.QuoteAssetType {
	if stock == nil {
		return model.AssetUS
	}
	if at := assetTypeFromMarket(stock.Market, stock.SecurityType); at == model.AssetDomestic {
		return model.AssetDomestic
	}
	if stock.Currency == model.CurrencyKRW || isDomesticKRSymbol(stock.Symbol) {
		return model.AssetDomestic
	}
	return stock.AssetType
}

func (s *FundamentalsService) RefreshSymbol(ctx context.Context, symbol string) (*model.StockFundamentals, error) {
	stock, err := s.stocks.GetBySymbol(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if stock == nil {
		return nil, fmt.Errorf("fundamentals: unknown symbol %s", symbol)
	}

	shares := s.resolveShares(ctx, stock, nil)

	var current, prior model.FinancialSnapshot
	var source model.FundamentalsDataSource
	var fetchErr error

	switch s.fundamentalsAssetType(stock) {
	case model.AssetDomestic:
		source = model.FundamentalsSourceDART
		current, prior, fetchErr = s.fetchDartSnapshots(ctx, stock.Symbol)
		if fetchErr == nil {
			if edgarCurrent, edgarPrior, edgarShares, ok := s.tryFresherEdgarSnapshot(ctx, stock.Symbol, current); ok {
				current, prior = edgarCurrent, edgarPrior
				source = model.FundamentalsSourceEDGAR
				shares = s.resolveShares(ctx, stock, edgarShares)
			}
		}
	default:
		source = model.FundamentalsSourceEDGAR
		var edgarShares *int64
		current, prior, edgarShares, fetchErr = s.edgar.FetchFinancialSnapshots(ctx, stock.Symbol)
		shares = s.resolveShares(ctx, stock, edgarShares)
	}
	if fetchErr != nil {
		// Keep last good row when refresh fails (bulk repair must not wipe DART ok data).
		if existing, _ := s.repo.GetBySymbol(ctx, stock.Symbol); existing != nil &&
			existing.CalcStatus == model.FundamentalsStatusOK {
			return existing, nil
		}
		row := model.StockFundamentals{
			Symbol:     stock.Symbol,
			DataSource: source,
			CalcStatus: model.FundamentalsStatusFailed,
			CalcError:  fetchErr.Error(),
			FetchedAt:  nowPtr(),
		}
		if err := s.repo.Upsert(ctx, row); err != nil {
			return nil, err
		}
		return &row, nil
	}

	current = mergeSnapshot(current, shares)
	computed := ComputeStoredFundamentals(current, prior, shares)

	row := model.StockFundamentals{
		Symbol:             stock.Symbol,
		DataSource:         source,
		FiscalYear:         intPtr(current.FiscalYear),
		FiscalPeriod:       current.FiscalPeriod,
		ReportCode:         current.ReportCode,
		PeriodEnd:          current.PeriodEnd,
		FiledAt:            current.FiledAt,
		Revenue:            current.Revenue,
		OperatingIncome:    current.OperatingIncome,
		NetIncome:          current.NetIncome,
		TotalAssets:        current.TotalAssets,
		TotalEquity:        current.TotalEquity,
		TotalLiabilities:   current.TotalLiabilities,
		CurrentAssets:      current.CurrentAssets,
		CurrentLiabilities: current.CurrentLiabilities,
		EPSBasic:           current.EPSBasic,
		EPSDiluted:         current.EPSDiluted,
		SharesOutstanding:  shares,
		ROE:                computed.ROE,
		OperatingMargin:    computed.OperatingMargin,
		RevenueGrowthYoY:   computed.RevenueGrowthYoY,
		EPSGrowthYoY:       computed.EPSGrowthYoY,
		DebtRatio:          computed.DebtRatio,
		ROA:                computed.ROA,
		NetProfitMargin:    computed.NetProfitMargin,
		CurrentRatio:       computed.CurrentRatio,
		CalcStatus:         calcStatus(computed),
		FetchedAt:          nowPtr(),
	}
	if len(computed.Missing) > 0 {
		row.CalcError = strings.Join(computed.Missing, ",")
	}

	if err := s.repo.Upsert(ctx, row); err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *FundamentalsService) GetBySymbol(ctx context.Context, symbol string) (*model.StockFundamentals, error) {
	return s.repo.GetBySymbol(ctx, symbol)
}

func (s *FundamentalsService) LookupStock(ctx context.Context, symbol string) (*model.Stock, error) {
	return s.resolveStock(ctx, symbol)
}

func (s *FundamentalsService) GetWithValuation(ctx context.Context, symbol string) (*model.FundamentalsView, error) {
	stock, err := s.resolveStock(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if stock == nil {
		return nil, nil
	}

	if !model.IsFundamentalsApplicable(*stock) {
		return &model.FundamentalsView{
			StockFundamentals: model.StockFundamentals{
				Symbol: stock.Symbol,
			},
		}, nil
	}

	stored, err := s.repo.GetBySymbol(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return &model.FundamentalsView{
			StockFundamentals: model.StockFundamentals{
				Symbol: stock.Symbol,
			},
		}, nil
	}

	price := s.fetchPrice(ctx, symbol)
	view := &model.FundamentalsView{
		StockFundamentals: *stored,
		Price:             price,
	}
	if price == nil {
		return view, nil
	}

	snap := model.FinancialSnapshot{
		TotalEquity:       stored.TotalEquity,
		EPSBasic:          stored.EPSBasic,
		EPSDiluted:        stored.EPSDiluted,
		SharesOutstanding: stored.SharesOutstanding,
		NetIncome:         stored.NetIncome,
	}
	val := ComputeValuationMetrics(price, snap, stored.SharesOutstanding)
	view.PER = val.PER
	view.PBR = val.PBR
	return view, nil
}

// krxToEdgarTicker lists domestic (KRX) symbols confirmed — via SEC's own
// company_tickers.json — to also file 20-F with the SEC under a US ticker.
// Unsponsored ADRs (e.g. Samsung Electronics, SK Hynix) have no SEC filing
// obligation and are correctly absent here; symbols not in this map always
// use DART only, unchanged from before.
var krxToEdgarTicker = map[string]string{
	"017670": "SKM", // SK Telecom
	"105560": "KB",  // KB Financial Group
	"055550": "SHG", // Shinhan Financial Group
	"316140": "WF",  // Woori Financial Group
	"005490": "PKX", // POSCO Holdings
	"015760": "KEP", // Korea Electric Power (KEPCO)
	"034220": "LPL", // LG Display
}

// tryFresherEdgarSnapshot best-effort checks whether a dual-listed domestic
// symbol's EDGAR filing is more recent than what DART already returned.
// Returns ok=false (keep DART) on any missing mapping, lookup/fetch failure,
// or when EDGAR is not strictly newer — 20-F is annual-only, so in practice
// DART's quarterly cadence usually wins; this only matters right after a
// fresh 20-F filing outruns DART's latest annual report.
func (s *FundamentalsService) tryFresherEdgarSnapshot(
	ctx context.Context,
	symbol string,
	dartCurrent model.FinancialSnapshot,
) (current, prior model.FinancialSnapshot, shares *int64, ok bool) {
	if s.edgar == nil {
		return current, prior, nil, false
	}
	ticker, mapped := krxToEdgarTicker[symbol]
	if !mapped {
		return current, prior, nil, false
	}
	edgarCurrent, edgarPrior, edgarShares, err := s.edgar.FetchFinancialSnapshots(ctx, ticker)
	if err != nil {
		return current, prior, nil, false
	}
	if edgarCurrent.PeriodEnd == nil || dartCurrent.PeriodEnd == nil {
		return current, prior, nil, false
	}
	if !edgarCurrent.PeriodEnd.After(*dartCurrent.PeriodEnd) {
		return current, prior, nil, false // DART wins ties and when EDGAR is older
	}
	return edgarCurrent, edgarPrior, edgarShares, true
}

func (s *FundamentalsService) fetchDartSnapshots(ctx context.Context, symbol string) (current, prior model.FinancialSnapshot, err error) {
	current, prior, err = s.dart.FindLatestAnnualSnapshots(ctx, symbol)
	if err == nil {
		return current, prior, nil
	}
	if alt, ok := commonStockCandidate(symbol); ok {
		cur, pri, altErr := s.dart.FindLatestAnnualSnapshots(ctx, alt)
		if altErr == nil {
			return cur, pri, nil
		}
	}
	return current, prior, err
}

func commonStockCandidate(symbol string) (string, bool) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if len(symbol) != 6 {
		return "", false
	}
	last := rune(symbol[5])
	if last == '0' {
		return "", false
	}
	if unicode.IsDigit(last) || unicode.IsLetter(last) {
		return symbol[:5] + "0", true
	}
	return "", false
}

func (s *FundamentalsService) resolveShares(ctx context.Context, stock *model.Stock, edgarShares *int64) *int64 {
	if edgarShares != nil && *edgarShares > 0 {
		return edgarShares
	}
	if stock.SharesOutstanding != nil && *stock.SharesOutstanding > 0 {
		return stock.SharesOutstanding
	}
	if s.toss == nil || !s.toss.Enabled() {
		return nil
	}
	info, err := s.toss.GetStock(ctx, stock.Symbol)
	if err != nil || info == nil {
		return nil
	}
	if n := parseDecimalInt(info.SharesOutstanding); n > 0 {
		return &n
	}
	return nil
}

func (s *FundamentalsService) fetchPrice(ctx context.Context, symbol string) *float64 {
	if s.marketStore != nil {
		if v, ok := s.marketStore.EnsurePrice(ctx, symbol); ok {
			return &v
		}
	}
	return nil
}

const fundamentalsRefreshPause = 1100 * time.Millisecond // DART apibudget MinInterval 1s

// RefreshAllStored re-fetches DART/EDGAR fundamentals for symbols already in stock_fundamentals.
func (s *FundamentalsService) RefreshAllStored(ctx context.Context) (int, error) {
	if s.repo == nil {
		return 0, nil
	}
	symbols, err := s.repo.ListSymbolSlice(ctx)
	if err != nil {
		return 0, err
	}
	updated := 0
	for _, sym := range symbols {
		stock, err := s.stocks.GetBySymbol(ctx, sym)
		if err != nil || stock == nil || !model.IsFundamentalsApplicable(*stock) {
			continue
		}
		if _, err := s.RefreshSymbol(ctx, sym); err != nil {
			continue
		}
		updated++
		select {
		case <-ctx.Done():
			return updated, ctx.Err()
		case <-time.After(fundamentalsRefreshPause):
		}
	}
	return updated, nil
}

// RefreshFailedDomestic re-fetches DART fundamentals for domestic symbols stuck in failed state.
func (s *FundamentalsService) RefreshFailedDomestic(ctx context.Context) (int, error) {
	if s.repo == nil || s.stocks == nil {
		return 0, nil
	}
	symbols, err := s.repo.ListSymbolsNotOK(ctx)
	if err != nil {
		return 0, err
	}
	updated := 0
	for _, sym := range symbols {
		stock, err := s.stocks.GetBySymbol(ctx, sym)
		if err != nil || stock == nil {
			continue
		}
		if s.fundamentalsAssetType(stock) != model.AssetDomestic {
			continue
		}
		if _, err := s.RefreshSymbol(ctx, sym); err != nil {
			continue
		}
		updated++
		select {
		case <-ctx.Done():
			return updated, ctx.Err()
		case <-time.After(fundamentalsRefreshPause):
		}
	}
	return updated, nil
}

func intPtr(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}
