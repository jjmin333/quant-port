package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

func catalogStockToTossInfo(s model.Stock) toss.StockInfo {
	var listDate *string
	if s.ListDate != nil {
		d := s.ListDate.Format("2006-01-02")
		listDate = &d
	}
	shares := ""
	if s.SharesOutstanding != nil {
		shares = strconv.FormatInt(*s.SharesOutstanding, 10)
	}
	return toss.StockInfo{
		Symbol:            s.Symbol,
		Name:              s.Name,
		EnglishName:       s.EnglishName,
		ISINCode:          s.ISINCode,
		Market:            s.Market,
		SecurityType:      s.SecurityType,
		Status:            s.Status,
		Currency:          toss.Currency(s.Currency),
		ListDate:          listDate,
		SharesOutstanding: shares,
	}
}

func normalizeStockSymbols(symbols []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}
	return out
}

func orderStockInfos(items []toss.StockInfo, order []string) []toss.StockInfo {
	bySymbol := make(map[string]toss.StockInfo, len(items))
	for _, item := range items {
		bySymbol[strings.ToUpper(strings.TrimSpace(item.Symbol))] = item
	}
	out := make([]toss.StockInfo, 0, len(order))
	for _, sym := range order {
		if item, ok := bySymbol[sym]; ok {
			out = append(out, item)
		}
	}
	return out
}

func stockNeedsTossReference(st model.Stock) bool {
	if st.ListDate == nil {
		return true
	}
	if st.SharesOutstanding == nil || *st.SharesOutstanding <= 0 {
		return true
	}
	return false
}

func mergeTossReference(existing model.Stock, info toss.StockInfo) model.Stock {
	out := existing
	if info.ListDate != nil && strings.TrimSpace(*info.ListDate) != "" {
		if t, err := repository.ParseListDate(*info.ListDate); err == nil && t != nil {
			out.ListDate = t
		}
	}
	if shares := parseDecimalInt(info.SharesOutstanding); shares > 0 {
		out.SharesOutstanding = &shares
	}
	return out
}

func stockFromTossInfo(info toss.StockInfo) model.Stock {
	symbol := strings.ToUpper(strings.TrimSpace(info.Symbol))
	var listDate *time.Time
	if info.ListDate != nil && strings.TrimSpace(*info.ListDate) != "" {
		listDate, _ = repository.ParseListDate(*info.ListDate)
	}
	var shares *int64
	if n := parseDecimalInt(info.SharesOutstanding); n > 0 {
		shares = &n
	}
	status := strings.TrimSpace(info.Status)
	switch strings.ToUpper(status) {
	case "", "ACTIVE", "TRADING":
		status = "NORMAL"
	}
	name := strings.TrimSpace(info.Name)
	if name == "" {
		name = symbol
	}
	englishName := strings.TrimSpace(info.EnglishName)
	if englishName == "" {
		englishName = name
	}
	return model.Stock{
		Symbol:            symbol,
		Name:              name,
		EnglishName:       englishName,
		ISINCode:          strings.TrimSpace(info.ISINCode),
		Market:            strings.TrimSpace(info.Market),
		SecurityType:      strings.TrimSpace(info.SecurityType),
		Currency:          model.QuoteCurrency(info.Currency),
		AssetType:         assetTypeFromMarket(info.Market, info.SecurityType),
		Status:            status,
		ListDate:          listDate,
		SharesOutstanding: shares,
	}
}

// StockFromTossInfo maps Toss StockInfo into the persisted catalog row (exported for batch jobs).
func StockFromTossInfo(info toss.StockInfo) model.Stock {
	return stockFromTossInfo(info)
}

func (s *MarketService) persistTossReference(ctx context.Context, existing model.Stock, hasExisting bool, info toss.StockInfo) model.Stock {
	var merged model.Stock
	if hasExisting {
		merged = mergeTossReference(existing, info)
	} else {
		merged = stockFromTossInfo(info)
	}
	if s.stocks != nil {
		_ = s.stocks.Upsert(ctx, merged)
	}
	return merged
}
