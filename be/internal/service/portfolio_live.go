package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

func portfolioLiveCacheKey(userID int64) string {
	return fmt.Sprintf("portfolio:live:v4:user:%d", userID)
}

// GetLive refreshes prices + FX on top of cached holdings (MARKET_DATA + MARKET_INFO only).
func (s *PortfolioService) GetLive(ctx context.Context) (*model.PortfolioSummary, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}

	base, err := s.getBaseSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	client, err := s.tossReg.ClientForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	cals := s.loadMarketCalendars(ctx, client)
	policy := computePollingPolicy(base.Holdings, cals, now)

	if policy.Mode == "idle" {
		return s.getLiveIdle(ctx, userID, client, base, cals, now, policy)
	}

	s.invalidateIdleCache(ctx, userID)
	return s.getLiveActive(ctx, userID, client, base, cals, now, policy)
}

func (s *PortfolioService) getLiveActive(
	ctx context.Context,
	userID int64,
	client *toss.Client,
	base *model.PortfolioSummary,
	cals marketCalendars,
	now time.Time,
	policy pollingPolicy,
) (*model.PortfolioSummary, error) {
	liveKey := portfolioLiveCacheKey(userID)
	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, liveKey).Bytes(); err == nil && len(cached) > 0 {
			var summary model.PortfolioSummary
			if json.Unmarshal(cached, &summary) == nil {
				return &summary, nil
			}
		}
	}

	symbols := make([]string, 0, len(base.Holdings))
	for _, h := range base.Holdings {
		symbols = append(symbols, h.Symbol)
	}

	var prices []toss.PriceResponse
	if len(symbols) > 0 && s.marketStore != nil {
		priceMap := s.marketStore.EnsurePrices(ctx, symbols)
		prices = make([]toss.PriceResponse, 0, len(priceMap))
		for _, sym := range symbols {
			up := strings.ToUpper(strings.TrimSpace(sym))
			if p, ok := priceMap[up]; ok {
				prices = append(prices, p)
			}
		}
	}

	fxMid, fxBuy, err := s.getLiveFX(ctx, client)
	if err != nil {
		return nil, err
	}

	live := s.applyLivePrices(ctx, userID, client, base, prices, fxMid, fxBuy, cals, now)
	applyPollingPolicy(live, policy)

	if s.redis != nil {
		if payload, err := json.Marshal(live); err == nil {
			_ = s.redis.Set(ctx, liveKey, payload, time.Duration(model.PortfolioLiveCacheSec)*time.Second).Err()
		}
	}
	return live, nil
}

func (s *PortfolioService) getLiveIdle(
	ctx context.Context,
	userID int64,
	client *toss.Client,
	base *model.PortfolioSummary,
	cals marketCalendars,
	now time.Time,
	policy pollingPolicy,
) (*model.PortfolioSummary, error) {
	frozen, err := s.loadIdleHoldings(ctx, userID, client, base, cals, now, policy)
	if err != nil {
		return nil, err
	}

	fxMid, fxBuy, err := s.getLiveFX(ctx, client)
	if err != nil {
		return nil, err
	}

	return applyFXToSummary(base, frozen, fxMid, fxBuy, now, policy), nil
}

func (s *PortfolioService) getBaseSnapshot(ctx context.Context, userID int64) (*model.PortfolioSummary, error) {
	base := s.ensureBaseSummary(authctx.WithUserID(ctx, userID), userID)
	return base, nil
}

func (s *PortfolioService) renderLiveSummary(
	ctx context.Context,
	userID int64,
	client *toss.Client,
	base *model.PortfolioSummary,
	cals marketCalendars,
	now time.Time,
	policy pollingPolicy,
) (*model.PortfolioSummary, error) {
	if policy.Mode == "idle" {
		return s.getLiveIdle(ctx, userID, client, base, cals, now, policy)
	}
	s.invalidateIdleCache(ctx, userID)
	return s.getLiveActive(ctx, userID, client, base, cals, now, policy)
}

// getLiveFX reads cached USD/KRW — never calls Toss on the request path.
func (s *PortfolioService) getLiveFX(ctx context.Context, _ *toss.Client) (mid, buy float64, err error) {
	mid, buy, ok := s.fxMidBuy(ctx)
	if !ok {
		return 0, 0, fmt.Errorf("exchange rate unavailable")
	}
	return mid, buy, nil
}

func (s *PortfolioService) applyLivePrices(
	ctx context.Context,
	userID int64,
	client *toss.Client,
	base *model.PortfolioSummary,
	prices []toss.PriceResponse,
	fxMid, fxBuy float64,
	cals marketCalendars,
	now time.Time,
) *model.PortfolioSummary {
	out := *base
	out.Holdings = make([]model.UnifiedHolding, len(base.Holdings))
	copy(out.Holdings, base.Holdings)

	priceMap := make(map[string]float64, len(prices))
	for _, p := range prices {
		priceMap[p.Symbol] = parseDecimal(p.LastPrice)
	}

	var totalMarketKRW float64
	var totalPurchaseKRW float64
	var totalProfitKRW float64
	var statsRefresh []string

	statsLoader := func(symbol string) holdingStatsCache {
		return s.loadHoldingStatsCache(ctx, symbol)
	}

	for i := range out.Holdings {
		h := &out.Holdings[i]
		prevMarket := h.MarketValueNative
		if price, ok := priceMap[h.Symbol]; ok && price > 0 {
			h.CurrentPrice = resolvePerSharePrice(price, prevMarket, h.TotalShares)
		}

		marketNative := h.TotalShares * h.CurrentPrice
		h.MarketValueNative = marketNative

		if h.Currency == "USD" {
			h.MarketValueKRW = marketNative * fxMid
			h.PurchaseAmountKRW = h.PurchaseAmountNative * h.PurchaseFxRate
			h.ProfitLossKRW = h.MarketValueKRW - h.PurchaseAmountKRW
			if h.PurchaseAmountNative > 0 {
				h.ReturnRateNative = ((marketNative - h.PurchaseAmountNative) / h.PurchaseAmountNative) * 100
			}
			if h.PurchaseAmountKRW > 0 {
				h.ReturnRateKRW = (h.ProfitLossKRW / h.PurchaseAmountKRW) * 100
			}
		} else {
			h.MarketValueKRW = marketNative
			h.PurchaseAmountKRW = h.PurchaseAmountNative
			h.ProfitLossKRW = marketNative - h.PurchaseAmountNative
			if h.PurchaseAmountNative > 0 {
				rate := (h.ProfitLossKRW / h.PurchaseAmountNative) * 100
				h.ReturnRateNative = rate
				h.ReturnRateKRW = rate
			}
		}

		totalMarketKRW += h.MarketValueKRW
		totalPurchaseKRW += h.PurchaseAmountKRW
		totalProfitKRW += h.ProfitLossKRW

		refreshHoldingSession(ctx, s.redis, h, cals, now, statsLoader, func(symbol string) {
			statsRefresh = append(statsRefresh, symbol)
		})
		updateSessionBreakdown(h)
		updateSinceHoldExtremes(h)
	}

	if len(statsRefresh) > 0 {
		s.triggerHoldingStatsRefresh(userID, statsRefresh)
	}

	out.ExchangeRate = fxMid
	out.ExchangeBuyRate = fxBuy
	out.TotalAssetsKRW = totalMarketKRW + out.TotalCashKRW
	out.TotalProfitKRW = totalProfitKRW
	if totalPurchaseKRW > 0 {
		out.TotalReturnRate = (totalProfitKRW / totalPurchaseKRW) * 100
	}

	accounts := make([]model.BrokerAccountSummary, len(out.Accounts))
	copy(accounts, out.Accounts)
	if len(accounts) == 1 {
		accounts[0].BalanceKRW = out.TotalAssetsKRW
	}
	out.Accounts = accounts

	out.Source.FetchedAt = now.Format(time.RFC3339)
	out.IsLive = true
	return &out
}

func finalizeSummary(summary *model.PortfolioSummary, isLive bool) {
	summary.LiveRefreshIntervalSec = model.PortfolioLiveRefreshIntervalSec
	summary.FullRefreshIntervalSec = model.PortfolioFullRefreshIntervalSec
	summary.RefreshIntervalSec = model.PortfolioFullRefreshIntervalSec
	summary.PollingMode = "active"
	summary.IsLive = isLive
}

func finalizeSummaryWithPolicy(summary *model.PortfolioSummary, policy pollingPolicy, isLive bool) {
	applyPollingPolicy(summary, policy)
	summary.IsLive = isLive
}
