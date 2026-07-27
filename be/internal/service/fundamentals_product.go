package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

func fundamentalsNotApplicableMessage(st model.Stock) string {
	return productLabel(st) + " 상품은 개별 재무제표를 제공하지 않습니다."
}

func productLabel(st model.Stock) string {
	sec := strings.ToUpper(strings.TrimSpace(st.SecurityType))
	if st.AssetType == model.AssetETF || strings.Contains(sec, "ETF") {
		return "ETF"
	}
	if strings.Contains(sec, "ETN") {
		return "ETN"
	}
	return "해당"
}

func (s *FundamentalsService) resolveStock(ctx context.Context, symbol string) (*model.Stock, error) {
	st, err := s.stocks.GetBySymbol(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if st != nil {
		return st, nil
	}
	if s.toss == nil || !s.toss.Enabled() {
		return nil, nil
	}
	info, err := s.toss.GetStock(ctx, symbol)
	if err != nil || info == nil {
		return nil, err
	}
	merged := stockFromTossInfo(*info)
	if err := s.stocks.Upsert(ctx, merged); err != nil {
		return &merged, nil
	}
	return &merged, nil
}
