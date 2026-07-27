package service

import (
	"context"
	"fmt"
	"strings"
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

	switch stock.AssetType {
	case model.AssetDomestic:
		source = model.FundamentalsSourceDART
		current, prior, fetchErr = s.fetchDartSnapshots(ctx, stock.Symbol)
	default:
		source = model.FundamentalsSourceEDGAR
		var edgarShares *int64
		current, prior, edgarShares, fetchErr = s.edgar.FetchFinancialSnapshots(ctx, stock.Symbol)
		shares = s.resolveShares(ctx, stock, edgarShares)
	}
	if fetchErr != nil {
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

func intPtr(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}
