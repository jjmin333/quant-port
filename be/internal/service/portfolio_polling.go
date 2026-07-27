package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

func portfolioIdleCacheKey(userID int64) string {
	return fmt.Sprintf("portfolio:idle:v1:user:%d", userID)
}

type pollingPolicy struct {
	Mode                   string
	LiveRefreshIntervalSec int
	FullRefreshIntervalSec int
	IdleUntil              time.Time
	HasUSDHoldings         bool
}

type idleHoldingsSnapshot struct {
	FrozenAt string                  `json:"frozenAt"`
	Holdings []model.UnifiedHolding  `json:"holdings"`
}

func holdingsMarketFlags(holdings []model.UnifiedHolding) (hasUS, hasKR bool) {
	for _, h := range holdings {
		if h.AssetType == "us" || h.AssetType == "etf" || h.Currency == "USD" {
			hasUS = true
		} else {
			hasKR = true
		}
	}
	return hasUS, hasKR
}

func isAnyHeldMarketActive(cals marketCalendars, hasUS, hasKR bool, now time.Time) bool {
	if hasUS && cals.us != nil {
		if detectUsSession(cals.us.Today, now) != "closed" {
			return true
		}
	}
	if hasKR && cals.kr != nil && cals.kr.Today.Integrated != nil {
		if detectKrSession(*cals.kr.Today.Integrated, now) != "closed" {
			return true
		}
	}
	return false
}

func computePollingPolicy(holdings []model.UnifiedHolding, cals marketCalendars, now time.Time) pollingPolicy {
	hasUS, hasKR := holdingsMarketFlags(holdings)
	active := isAnyHeldMarketActive(cals, hasUS, hasKR, now)

	if active {
		return pollingPolicy{
			Mode:                   "active",
			LiveRefreshIntervalSec: model.PortfolioLiveRefreshIntervalSec,
			FullRefreshIntervalSec: model.PortfolioFullRefreshIntervalSec,
			HasUSDHoldings:         hasUS,
		}
	}

	return pollingPolicy{
		Mode:                   "idle",
		LiveRefreshIntervalSec: model.PortfolioLiveRefreshIntervalSec,
		FullRefreshIntervalSec: model.PortfolioIdleFullRefreshIntervalSec,
		IdleUntil:              nextSessionStartForHoldings(cals, hasUS, hasKR, now),
		HasUSDHoldings:         hasUS,
	}
}

func nextSessionStartForHoldings(cals marketCalendars, hasUS, hasKR bool, now time.Time) time.Time {
	var candidates []time.Time
	if hasKR && cals.kr != nil {
		if t := nextKrSessionStart(cals.kr.Today, cals.kr.NextBusinessDay, now); !t.IsZero() {
			candidates = append(candidates, t)
		}
	}
	if hasUS && cals.us != nil {
		if t := nextUsSessionStart(cals.us.Today, cals.us.NextBusinessDay, now); !t.IsZero() {
			candidates = append(candidates, t)
		}
	}
	if len(candidates) == 0 {
		return now.Add(24 * time.Hour)
	}
	earliest := candidates[0]
	for _, t := range candidates[1:] {
		if t.Before(earliest) {
			earliest = t
		}
	}
	return earliest
}

func nextKrSessionStart(today, nextDay toss.KrMarketDay, now time.Time) time.Time {
	if today.Integrated != nil {
		if t := nextIntegratedSessionStart(*today.Integrated, now); !t.IsZero() {
			return t
		}
	}
	if nextDay.Integrated != nil {
		if t := firstIntegratedSessionStart(*nextDay.Integrated); !t.IsZero() {
			return t
		}
	}
	return time.Time{}
}

func nextUsSessionStart(today, nextDay toss.UsMarketDay, now time.Time) time.Time {
	if t := nextUsDaySessionStart(today, now); !t.IsZero() {
		return t
	}
	return firstUsDaySessionStart(nextDay)
}

func nextIntegratedSessionStart(integrated toss.IntegratedHour, now time.Time) time.Time {
	var starts []time.Time
	if integrated.PreMarket != nil {
		if t, ok := parseCandleTime(integrated.PreMarket.StartTime); ok {
			starts = append(starts, t)
		}
	}
	if integrated.RegularMarket != nil {
		if t, ok := parseCandleTime(integrated.RegularMarket.StartTime); ok {
			starts = append(starts, t)
		}
	}
	if integrated.AfterMarket != nil {
		if t, ok := parseCandleTime(integrated.AfterMarket.StartTime); ok {
			starts = append(starts, t)
		}
	}
	return earliestAfter(starts, now)
}

func firstIntegratedSessionStart(integrated toss.IntegratedHour) time.Time {
	var starts []time.Time
	if integrated.PreMarket != nil {
		if t, ok := parseCandleTime(integrated.PreMarket.StartTime); ok {
			starts = append(starts, t)
		}
	}
	if len(starts) > 0 {
		return starts[0]
	}
	if integrated.RegularMarket != nil {
		if t, ok := parseCandleTime(integrated.RegularMarket.StartTime); ok {
			return t
		}
	}
	return time.Time{}
}

func nextUsDaySessionStart(day toss.UsMarketDay, now time.Time) time.Time {
	var starts []time.Time
	if day.PreMarket != nil {
		if t, ok := parseCandleTime(day.PreMarket.StartTime); ok {
			starts = append(starts, t)
		}
	}
	if day.DayMarket != nil {
		if t, ok := parseCandleTime(day.DayMarket.StartTime); ok {
			starts = append(starts, t)
		}
	}
	if day.RegularMarket != nil {
		if t, ok := parseCandleTime(day.RegularMarket.StartTime); ok {
			starts = append(starts, t)
		}
	}
	if day.AfterMarket != nil {
		if t, ok := parseCandleTime(day.AfterMarket.StartTime); ok {
			starts = append(starts, t)
		}
	}
	return earliestAfter(starts, now)
}

func firstUsDaySessionStart(day toss.UsMarketDay) time.Time {
	if day.PreMarket != nil {
		if t, ok := parseCandleTime(day.PreMarket.StartTime); ok {
			return t
		}
	}
	if day.DayMarket != nil {
		if t, ok := parseCandleTime(day.DayMarket.StartTime); ok {
			return t
		}
	}
	if day.RegularMarket != nil {
		if t, ok := parseCandleTime(day.RegularMarket.StartTime); ok {
			return t
		}
	}
	return time.Time{}
}

func earliestAfter(times []time.Time, now time.Time) time.Time {
	var best time.Time
	for _, t := range times {
		if !t.After(now) {
			continue
		}
		if best.IsZero() || t.Before(best) {
			best = t
		}
	}
	return best
}

func applyPollingPolicy(summary *model.PortfolioSummary, policy pollingPolicy) {
	summary.PollingMode = policy.Mode
	summary.LiveRefreshIntervalSec = policy.LiveRefreshIntervalSec
	summary.FullRefreshIntervalSec = policy.FullRefreshIntervalSec
	summary.RefreshIntervalSec = policy.FullRefreshIntervalSec
}

func (s *PortfolioService) loadIdleHoldings(
	ctx context.Context,
	userID int64,
	client *toss.Client,
	base *model.PortfolioSummary,
	cals marketCalendars,
	now time.Time,
	policy pollingPolicy,
) ([]model.UnifiedHolding, error) {
	idleKey := portfolioIdleCacheKey(userID)
	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, idleKey).Bytes(); err == nil && len(cached) > 0 {
			var snap idleHoldingsSnapshot
			if json.Unmarshal(cached, &snap) == nil && len(snap.Holdings) > 0 {
				return snap.Holdings, nil
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
	} else if len(symbols) > 0 {
		var err error
		prices, err = client.GetPrices(ctx, symbols)
		if err != nil {
			return nil, err
		}
	}

	frozen := s.applyLivePrices(ctx, userID, client, base, prices, base.ExchangeRate, base.ExchangeBuyRate, cals, now)
	for i := range frozen.Holdings {
		frozen.Holdings[i].MarketSession = "closed"
		updateSessionBreakdown(&frozen.Holdings[i])
	}

	ttl := time.Until(policy.IdleUntil)
	if ttl < time.Minute {
		ttl = time.Minute
	}
	if s.redis != nil {
		payload, err := json.Marshal(idleHoldingsSnapshot{
			FrozenAt: now.Format(time.RFC3339),
			Holdings: frozen.Holdings,
		})
		if err == nil {
			_ = s.redis.Set(ctx, idleKey, payload, ttl).Err()
		}
	}
	return frozen.Holdings, nil
}

func applyFXToSummary(base *model.PortfolioSummary, holdings []model.UnifiedHolding, fxMid, fxBuy float64, now time.Time, policy pollingPolicy) *model.PortfolioSummary {
	out := *base
	out.Holdings = make([]model.UnifiedHolding, len(holdings))
	copy(out.Holdings, holdings)

	var totalMarketKRW float64
	var totalPurchaseKRW float64
	var totalProfitKRW float64

	for i := range out.Holdings {
		h := &out.Holdings[i]
		if h.Currency == "USD" {
			h.MarketValueKRW = h.MarketValueNative * fxMid
			h.PurchaseAmountKRW = h.PurchaseAmountNative * h.PurchaseFxRate
			h.ProfitLossKRW = h.MarketValueKRW - h.PurchaseAmountKRW
			if h.PurchaseAmountKRW > 0 {
				h.ReturnRateKRW = (h.ProfitLossKRW / h.PurchaseAmountKRW) * 100
			}
		} else {
			h.MarketValueKRW = h.MarketValueNative
			h.PurchaseAmountKRW = h.PurchaseAmountNative
			h.ProfitLossKRW = h.MarketValueNative - h.PurchaseAmountNative
			if h.PurchaseAmountNative > 0 {
				rate := (h.ProfitLossKRW / h.PurchaseAmountNative) * 100
				h.ReturnRateNative = rate
				h.ReturnRateKRW = rate
			}
		}
		totalMarketKRW += h.MarketValueKRW
		totalPurchaseKRW += h.PurchaseAmountKRW
		totalProfitKRW += h.ProfitLossKRW
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
	applyPollingPolicy(&out, policy)
	return &out
}

func (s *PortfolioService) invalidateIdleCache(ctx context.Context, userID int64) {
	if s.redis != nil {
		_ = s.redis.Del(ctx, portfolioIdleCacheKey(userID)).Err()
	}
}
