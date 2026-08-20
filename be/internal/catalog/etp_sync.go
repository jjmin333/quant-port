package catalog

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/seed"
)

const insertBatchSize = 500

// ETPSource fetches domestic and US exchange-traded product catalog rows.
type ETPSource interface {
	DomesticETP(ctx context.Context) ([]model.Stock, error)
	US_ETP(ctx context.Context) ([]model.Stock, error)
}

// SyncETPDelta inserts catalog rows that are not yet in stocks (insert-only, no overwrite).
type SyncETPDelta struct {
	Stocks *repository.StockRepo
	Source ETPSource
}

type SyncETPResult struct {
	DomesticFetched int
	USFetched       int
	UniqueIncoming  int
	AlreadyInDB     int
	Inserted        int
	DomesticError   string
	USError         string
}

func (s *SyncETPDelta) Run(ctx context.Context) (SyncETPResult, error) {
	var res SyncETPResult
	if s == nil || s.Stocks == nil || s.Source == nil {
		return res, fmt.Errorf("catalog sync not configured")
	}

	domestic, domesticErr := s.Source.DomesticETP(ctx)
	if domesticErr != nil {
		res.DomesticError = domesticErr.Error()
	}
	res.DomesticFetched = len(domestic)

	us, usErr := s.Source.US_ETP(ctx)
	if usErr != nil {
		res.USError = usErr.Error()
	}
	res.USFetched = len(us)

	if domesticErr != nil && usErr != nil {
		return res, fmt.Errorf("domestic: %v; us: %v", domesticErr, usErr)
	}

	merged := mergeStocks(append(domestic, us...))
	res.UniqueIncoming = len(merged)
	if len(merged) == 0 {
		return res, nil
	}

	existing, err := s.Stocks.ExistingSymbolSet(ctx)
	if err != nil {
		return res, err
	}

	var toInsert []model.Stock
	for _, st := range merged {
		sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
		if sym == "" {
			continue
		}
		if _, ok := existing[sym]; ok {
			res.AlreadyInDB++
			continue
		}
		toInsert = append(toInsert, st)
	}

	for i := 0; i < len(toInsert); i += insertBatchSize {
		end := i + insertBatchSize
		if end > len(toInsert) {
			end = len(toInsert)
		}
		if err := s.Stocks.UpsertBatch(ctx, toInsert[i:end]); err != nil {
			return res, fmt.Errorf("insert batch: %w", err)
		}
		res.Inserted += end - i
	}
	return res, nil
}

func mergeStocks(items []model.Stock) []model.Stock {
	seen := map[string]struct{}{}
	out := make([]model.Stock, 0, len(items))
	for _, st := range items {
		sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
		if sym == "" {
			continue
		}
		if _, dup := seen[sym]; dup {
			continue
		}
		seen[sym] = struct{}{}
		st.Symbol = sym
		out = append(out, st)
	}
	return out
}

// FileETPSource loads domestic rows from CSV (same schema as krx_list) when API is unavailable.
type FileETPSource struct {
	DomesticPath string
	Finnhub      *marketdata.FinnhubClient
	KRX          *marketdata.KRXClient
	HTTPClient   *http.Client
	BasDD        string
}

func (f *FileETPSource) DomesticETP(ctx context.Context) ([]model.Stock, error) {
	if f.KRX != nil && f.KRX.Enabled() {
		basDd := strings.TrimSpace(f.BasDD)
		if basDd == "" {
			basDd = marketdata.KRXTtradeDate(time.Now())
		}
		rows, err := f.KRX.ListDomesticETP(ctx, basDd)
		if err == nil {
			return rows, nil
		}
		if marketdata.IsKRXUnauthorized(err) {
			return marketdata.ListDomesticETPFromNaver(ctx, f.HTTPClient)
		}
		return nil, err
	}
	path := strings.TrimSpace(f.DomesticPath)
	if path != "" {
		all, err := seed.LoadKRXCSV(path)
		if err != nil {
			return nil, err
		}
		out := make([]model.Stock, 0, len(all))
		for _, st := range all {
			sec := strings.ToUpper(st.SecurityType)
			if strings.Contains(sec, "ETF") || strings.Contains(sec, "ETN") {
				st.AssetType = model.AssetETF
				out = append(out, st)
			}
		}
		return out, nil
	}
	return marketdata.ListDomesticETPFromNaver(ctx, f.HTTPClient)
}

func (f *FileETPSource) US_ETP(ctx context.Context) ([]model.Stock, error) {
	if f.Finnhub == nil || !f.Finnhub.Enabled() {
		return nil, fmt.Errorf("finnhub: FINNHUB_API_KEY required for US ETP catalog")
	}
	return f.Finnhub.ListUSExchangeTradedProducts(ctx)
}
