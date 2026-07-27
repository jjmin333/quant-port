package service

import (
	"context"
	"fmt"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

type MarketService struct {
	toss        *toss.Client
	fx          *ExchangeRateService
	stocks      *repository.StockRepo
	marketStore *MarketDataStore
}

func NewMarketService(tossClient *toss.Client, fx *ExchangeRateService, stockRepo *repository.StockRepo) *MarketService {
	return &MarketService{toss: tossClient, fx: fx, stocks: stockRepo}
}

func (s *MarketService) SetMarketStore(store *MarketDataStore) {
	s.marketStore = store
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

func (s *MarketService) Rankings(ctx context.Context, p toss.GetRankingsParams) (*toss.RankingResponse, error) {
	client, err := s.requireToss()
	if err != nil {
		return nil, err
	}
	return client.GetRankings(ctx, p)
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

func (s *MarketService) StockWarnings(ctx context.Context, symbol string) ([]toss.StockWarning, error) {
	if s.marketStore != nil {
		warnings, ok := s.marketStore.EnsureStockWarnings(ctx, symbol)
		if ok {
			return warnings, nil
		}
	}
	return nil, nil
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
