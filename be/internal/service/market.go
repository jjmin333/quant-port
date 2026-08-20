package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

type MarketService struct {
	toss             *toss.Client
	redis            *redis.Client
	fx               *ExchangeRateService
	stocks           *repository.StockRepo
	marketStore      *MarketDataStore
	rankingSnapshots *repository.RankingSnapshotRepo
	rankingsRepo     *repository.MarketRankingsRepo
	dataSettings     *MarketDataSettingsStore
	quotes           *QuotesService
	// warnings — 종목 지정 상태 표. 배치가 채우고 여기서는 읽기만 한다.
	warnings *repository.WarningRepo
}

// SetWarnings wires the 종목 경고 reader.
func (s *MarketService) SetWarnings(r *repository.WarningRepo) { s.warnings = r }

func NewMarketService(
	tossClient *toss.Client,
	redisClient *redis.Client,
	fx *ExchangeRateService,
	stockRepo *repository.StockRepo,
	rankingRepo *repository.RankingSnapshotRepo,
	rankingsRepo *repository.MarketRankingsRepo,
) *MarketService {
	return &MarketService{
		toss:             tossClient,
		redis:            redisClient,
		fx:               fx,
		stocks:           stockRepo,
		rankingSnapshots: rankingRepo,
		rankingsRepo:     rankingsRepo,
	}
}

func (s *MarketService) SetMarketStore(store *MarketDataStore) {
	s.marketStore = store
}

func (s *MarketService) SetQuotesService(q *QuotesService) {
	s.quotes = q
}

func (s *MarketService) SetDataSettings(store *MarketDataSettingsStore) {
	s.dataSettings = store
}

func (s *MarketService) requireToss() (*toss.Client, error) {
	if s.toss == nil || !s.toss.Enabled() {
		return nil, fmt.Errorf("toss api is not configured")
	}
	return s.toss, nil
}

func (s *MarketService) ExchangeRate(ctx context.Context, p toss.GetExchangeRateParams) (*toss.ExchangeRateResponse, error) {
	if p.BaseCurrency == "" {
		p.BaseCurrency = toss.CurrencyUSD
	}
	if p.QuoteCurrency == "" {
		p.QuoteCurrency = toss.CurrencyKRW
	}
	if p.BaseCurrency == toss.CurrencyUSD && p.QuoteCurrency == toss.CurrencyKRW && s.fx != nil {
		if resp, ok := s.fx.ToResponse(ctx); ok {
			return resp, nil
		}
		return nil, fmt.Errorf("exchange rate not available yet")
	}
	return nil, fmt.Errorf("unsupported currency pair")
}

func (s *MarketService) KrCalendar(ctx context.Context, date string) (*toss.KrMarketCalendarResponse, error) {
	client, err := s.requireToss()
	if err != nil {
		return nil, err
	}
	return client.GetKrMarketCalendar(ctx, date)
}

func (s *MarketService) UsCalendar(ctx context.Context, date string) (*toss.UsMarketCalendarResponse, error) {
	client, err := s.requireToss()
	if err != nil {
		return nil, err
	}
	return client.GetUsMarketCalendar(ctx, date)
}

func (s *MarketService) Rankings(ctx context.Context, p toss.GetRankingsParams) (*RankingsAPIResult, error) {
	if !isSupportedRankingPollParams(p) {
		return &RankingsAPIResult{Rankings: []toss.RankingItem{}}, nil
	}

	storeParams := p
	storeParams.Count = rankingsPollFetchCount

	stored, ok := s.loadRankingsFromStore(ctx, storeParams)
	if !ok {
		return &RankingsAPIResult{Rankings: []toss.RankingItem{}}, nil
	}
	result := stored.toAPIResult(rankingsAPIResultCount(p.Count))
	if s.quotes != nil && len(result.Rankings) > 0 {
		result.Rankings = s.quotes.EnrichRankingPrices(ctx, result.Rankings)
	}
	return result, nil
}

func rankingsAPIResultCount(requested int) int {
	if requested <= 0 || requested > rankingsPollFetchCount {
		return rankingsPollFetchCount
	}
	return requested
}

func (s *MarketService) Stock(ctx context.Context, symbol string) (*toss.StockInfo, error) {
	items, err := s.Stocks(ctx, []string{symbol})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (s *MarketService) Stocks(ctx context.Context, symbols []string) ([]toss.StockInfo, error) {
	order := normalizeStockSymbols(symbols)
	if len(order) == 0 {
		return nil, nil
	}

	dbBySymbol := make(map[string]model.Stock, len(order))
	if s.stocks != nil {
		fromDB, err := s.stocks.ListBySymbols(ctx, order)
		if err != nil {
			return nil, err
		}
		for _, st := range fromDB {
			dbBySymbol[st.Symbol] = st
		}
	}

	needToss := make([]string, 0, len(order))
	for _, sym := range order {
		st, ok := dbBySymbol[sym]
		if !ok || stockNeedsTossReference(st) {
			needToss = append(needToss, sym)
		}
	}

	responseBySymbol := make(map[string]toss.StockInfo, len(order))
	for sym, st := range dbBySymbol {
		if !stockNeedsTossReference(st) {
			responseBySymbol[sym] = catalogStockToTossInfo(st)
		}
	}

	if len(needToss) > 0 {
		client, err := s.requireToss()
		if err != nil {
			if len(responseBySymbol) == 0 {
				return nil, err
			}
			return orderStockInfos(valuesFromMap(responseBySymbol), order), nil
		}

		for i := 0; i < len(needToss); i += 200 {
			end := min(i+200, len(needToss))
			fromToss, err := client.GetStocks(ctx, needToss[i:end])
			if err != nil {
				if len(responseBySymbol) == 0 {
					return nil, fmt.Errorf("toss get stocks: %w", err)
				}
				break
			}
			for _, info := range fromToss {
				sym := normalizeStockSymbols([]string{info.Symbol})[0]
				if sym == "" {
					continue
				}
				existing, hasExisting := dbBySymbol[sym]
				merged := s.persistTossReference(ctx, existing, hasExisting, info)
				dbBySymbol[sym] = merged
				responseBySymbol[sym] = catalogStockToTossInfo(merged)
			}
		}

		for _, sym := range needToss {
			if _, ok := responseBySymbol[sym]; ok {
				continue
			}
			if st, ok := dbBySymbol[sym]; ok {
				responseBySymbol[sym] = catalogStockToTossInfo(st)
			}
		}
	}

	if len(responseBySymbol) == 0 {
		return nil, nil
	}
	return orderStockInfos(valuesFromMap(responseBySymbol), order), nil
}

// StockWarnings reads 지정 상태 from the table the warnings batch fills.
//
// 예전에는 캐시가 비면 여기서 Toss를 직접 불렀다 — 종목 수에 비례해 호출이
// 늘어나는 경로였다. 경고는 하루 단위로 붙고 풀리므로 배치가 **전 종목 매일**
// 갱신한다(reference_ingest.go IngestWarnings).
func (s *MarketService) StockWarnings(ctx context.Context, symbol string) ([]toss.StockWarning, error) {
	if s.warnings == nil {
		return nil, nil
	}
	rows, err := s.warnings.ListForSymbol(ctx, symbol)
	if err != nil {
		return nil, err
	}
	out := make([]toss.StockWarning, 0, len(rows))
	for _, w := range rows {
		out = append(out, toss.StockWarning{
			WarningType: w.WarningType,
			Exchange:    w.Exchange,
			StartDate:   w.StartDate,
			EndDate:     w.EndDate,
		})
	}
	return out, nil
}

func (s *MarketService) MarketIndicatorPrices(ctx context.Context, symbols []string) ([]toss.MarketIndicatorPriceResponse, error) {
	client, err := s.requireToss()
	if err != nil {
		return nil, err
	}
	return client.GetMarketIndicatorPrices(ctx, symbols)
}

func valuesFromMap(m map[string]toss.StockInfo) []toss.StockInfo {
	out := make([]toss.StockInfo, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

const sharesRefreshBatch = 200
const sharesRefreshPause = 150 * time.Millisecond

// RepairDomesticAssetTypes fixes KRW/KOSPI catalog rows mis-tagged as us (Toss market=KOSPI).
func (s *MarketService) RepairDomesticAssetTypes(ctx context.Context) (int, error) {
	if s.stocks == nil {
		return 0, nil
	}
	symbols, err := s.stocks.ListSymbols(ctx, "", 10000)
	if err != nil {
		return 0, err
	}
	repaired := 0
	for _, sym := range symbols {
		stock, err := s.stocks.GetBySymbol(ctx, sym)
		if err != nil || stock == nil {
			continue
		}
		want := assetTypeFromMarket(stock.Market, stock.SecurityType)
		if stock.Currency == model.CurrencyKRW || isDomesticKRSymbol(stock.Symbol) {
			want = model.AssetDomestic
		}
		if stock.AssetType == want {
			continue
		}
		stock.AssetType = want
		if err := s.stocks.Upsert(ctx, *stock); err != nil {
			continue
		}
		repaired++
	}
	return repaired, nil
}

// RefreshAllSharesOutstanding re-fetches Toss reference data (shares, list date) for catalog symbols.
func (s *MarketService) RefreshAllSharesOutstanding(ctx context.Context) (int, error) {
	if s.stocks == nil {
		return 0, nil
	}
	if n, err := s.RepairDomesticAssetTypes(ctx); err != nil {
		return 0, err
	} else if n > 0 {
		log.Printf("market: repaired domestic asset types for %d symbols", n)
	}
	client, err := s.requireToss()
	if err != nil {
		return 0, err
	}
	symbols, err := s.stocks.ListSymbols(ctx, "", 10000)
	if err != nil {
		return 0, err
	}
	updated := 0
	for i := 0; i < len(symbols); i += sharesRefreshBatch {
		end := min(i+sharesRefreshBatch, len(symbols))
		batch := symbols[i:end]
		fromToss, err := client.GetStocks(ctx, batch)
		if err != nil {
			return updated, err
		}
		for _, info := range fromToss {
			sym := strings.ToUpper(strings.TrimSpace(info.Symbol))
			if sym == "" {
				continue
			}
			existing, _ := s.stocks.GetBySymbol(ctx, sym)
			hasExisting := existing != nil
			var base model.Stock
			if hasExisting {
				base = *existing
			}
			_ = s.persistTossReference(ctx, base, hasExisting, info)
			updated++
		}
		if end < len(symbols) {
			select {
			case <-ctx.Done():
				return updated, ctx.Err()
			case <-time.After(sharesRefreshPause):
			}
		}
	}
	return updated, nil
}
