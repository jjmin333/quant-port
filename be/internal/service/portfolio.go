package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

func portfolioSummaryCacheKey(userID int64) string {
	return fmt.Sprintf("portfolio:summary:v8:user:%d", userID)
}

type PortfolioService struct {
	tossReg        *TossRegistry
	redis          *redis.Client
	snapshots      *repository.SnapshotRepo
	portfolioCache *repository.PortfolioCacheRepo
	brokerCreds    *repository.BrokerCredentialsRepo
	fx             *ExchangeRateService
	marketStore    *MarketDataStore
	syncMu         sync.Mutex
	syncingUsers   map[int64]bool
}

func NewPortfolioService(
	tossReg *TossRegistry,
	redisClient *redis.Client,
	snapshots *repository.SnapshotRepo,
	portfolioCache *repository.PortfolioCacheRepo,
	brokerCreds *repository.BrokerCredentialsRepo,
	fx *ExchangeRateService,
) *PortfolioService {
	s := &PortfolioService{
		tossReg:        tossReg,
		redis:          redisClient,
		snapshots:      snapshots,
		portfolioCache: portfolioCache,
		brokerCreds:    brokerCreds,
		fx:             fx,
		syncingUsers:   make(map[int64]bool),
	}
	go s.RunSyncScheduler(context.Background())
	return s
}

func (s *PortfolioService) SetMarketStore(store *MarketDataStore) {
	s.marketStore = store
}

func (s *PortfolioService) fxMidBuy(ctx context.Context) (mid, buy float64, ok bool) {
	if s.fx != nil {
		return s.fx.MidBuy(ctx)
	}
	return 0, 0, false
}

func (s *PortfolioService) clientForCtx(ctx context.Context) (*toss.Client, int64, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, 0, err
	}
	client, err := s.tossReg.ClientForUser(ctx, userID)
	if err != nil {
		return nil, userID, err
	}
	return client, userID, nil
}

func (s *PortfolioService) GetSummary(ctx context.Context) (*model.PortfolioSummary, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}

	base := s.ensureBaseSummary(ctx, userID)

	client, err := s.tossReg.ClientForUser(ctx, userID)
	if err != nil {
		finalizeSummary(base, false)
		return base, nil
	}

	cals := s.loadMarketCalendars(ctx, client)
	policy := computePollingPolicy(base.Holdings, cals, time.Now())

	live, err := s.renderLiveSummary(ctx, userID, client, base, cals, time.Now(), policy)
	if err != nil {
		finalizeSummaryWithPolicy(base, policy, false)
		return base, nil
	}
	return live, nil
}

func (s *PortfolioService) fetchSummary(ctx context.Context) (*model.PortfolioSummary, error) {
	client, _, err := s.clientForCtx(ctx)
	if err != nil {
		return nil, err
	}

	accounts, err := client.GetAccounts(ctx)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		empty := &model.PortfolioSummary{
			Source: model.PortfolioSource{
				Provider:  "toss",
				FetchedAt: time.Now().Format(time.RFC3339),
			},
		}
		finalizeSummary(empty, false)
		return empty, nil
	}

	fxMid, fxBuy, ok := s.fxMidBuy(ctx)
	if !ok {
		return nil, fmt.Errorf("exchange rate unavailable")
	}

	type mergeState struct {
		item           toss.HoldingsItem
		totalQty       float64
		totalPurchase  float64
		totalMarket    float64
		accountEntries []model.AccountHoldingBreakdown
	}

	merged := map[string]*mergeState{}
	var (
		totalMarketKRW    float64
		totalCashKRW      float64
		totalProfitKRW    float64
		totalTodayKRW     float64
		totalReturnRate   float64
		totalTodayRate    float64
		totalUsdPurchase  float64
		totalKrwPurchase  float64
		impliedPurchase   float64
		brokerAccounts    []model.BrokerAccountSummary
		accountCount      int
	)

	for _, acc := range accounts {
		holdings, err := client.GetHoldings(ctx, acc.AccountSeq, "")
		if err != nil {
			return nil, fmt.Errorf("holdings accountSeq=%d: %w", acc.AccountSeq, err)
		}

		krwCash, err := client.GetBuyingPower(ctx, acc.AccountSeq, toss.CurrencyKRW)
		if err != nil {
			return nil, fmt.Errorf("buying power KRW accountSeq=%d: %w", acc.AccountSeq, err)
		}
		usdCash, err := client.GetBuyingPower(ctx, acc.AccountSeq, toss.CurrencyUSD)
		if err != nil {
			return nil, fmt.Errorf("buying power USD accountSeq=%d: %w", acc.AccountSeq, err)
		}

		// Toss 앱과 동일: amountAfterCost + 매매기준율(midRate) 환산
		accountMarketKRW := currencyPriceToKRW(holdings.MarketValue.AmountAfterCost, fxMid)
		accountCashKRW := parseDecimal(krwCash.CashBuyingPower)
		if usdCash != nil {
			accountCashKRW += parseDecimal(usdCash.CashBuyingPower) * fxMid
		}

		accountProfitKRW := currencyPriceToKRW(holdings.ProfitLoss.AmountAfterCost, fxMid)
		accountTodayKRW := currencyPriceToKRW(holdings.DailyProfitLoss.Amount, fxMid)

		brokerName := accountDisplayName(acc)
		brokerAccounts = append(brokerAccounts, model.BrokerAccountSummary{
			ID:          fmt.Sprintf("toss-%d", acc.AccountSeq),
			Name:        brokerName,
			AccountSeq:  acc.AccountSeq,
			AccountNo:   acc.AccountNo,
			AccountType: string(acc.AccountType),
			BalanceKRW:  accountMarketKRW + accountCashKRW,
			ChangeRate:  parseDecimal(holdings.DailyProfitLoss.Rate) * 100,
		})

		totalMarketKRW += accountMarketKRW
		totalCashKRW += accountCashKRW
		totalProfitKRW += accountProfitKRW
		totalTodayKRW += accountTodayKRW
		totalReturnRate += parseDecimal(holdings.ProfitLoss.RateAfterCost) * 100
		totalTodayRate += parseDecimal(holdings.DailyProfitLoss.Rate) * 100
		totalUsdPurchase += parseDecimalUSD(holdings.TotalPurchaseAmount.USD)
		totalKrwPurchase += parseDecimal(holdings.TotalPurchaseAmount.KRW)
		impliedPurchase += impliedPurchaseKRW(holdings, fxMid)
		accountCount++

		for _, item := range holdings.Items {
			qty := parseDecimal(item.Quantity)
			purchase := parseDecimal(item.MarketValue.PurchaseAmount)
			market := parseDecimal(item.MarketValue.AmountAfterCost)
			if market == 0 {
				market = parseDecimal(item.MarketValue.Amount)
			}

			symKey := strings.ToUpper(strings.TrimSpace(item.Symbol))
			if symKey == "" {
				continue
			}
			state, ok := merged[symKey]
			if !ok {
				merged[symKey] = &mergeState{
					item:           item,
					totalQty:       qty,
					totalPurchase:  purchase,
					totalMarket:    market,
					accountEntries: []model.AccountHoldingBreakdown{{Broker: brokerName, Shares: qty}},
				}
				continue
			}
			state.totalQty += qty
			state.totalPurchase += purchase
			state.totalMarket += market
			state.accountEntries = append(state.accountEntries, model.AccountHoldingBreakdown{
				Broker: brokerName,
				Shares: qty,
			})
		}
	}

	avgPurchaseFx := fxMid
	if totalUsdPurchase > 0 {
		derived := (impliedPurchase - totalKrwPurchase) / totalUsdPurchase
		if derived > 0 && !math.IsNaN(derived) && !math.IsInf(derived, 0) {
			avgPurchaseFx = derived
		}
	}

	if accountCount > 0 {
		totalReturnRate /= float64(accountCount)
		totalTodayRate /= float64(accountCount)
	}

	holdingsOut := make([]model.UnifiedHolding, 0, len(merged))
	for _, state := range merged {
		item := state.item
		avgPrice := resolvePerSharePrice(
			parseDecimal(item.AveragePurchasePrice),
			state.totalPurchase,
			state.totalQty,
		)
		if avgPrice <= 0 && state.totalQty == 0 {
			avgPrice = parseDecimal(item.AveragePurchasePrice)
		}
		currentPrice := resolvePerSharePrice(
			parseDecimal(item.LastPrice),
			state.totalMarket,
			state.totalQty,
		)
		returnRateNative := parseDecimal(item.ProfitLoss.RateAfterCost) * 100
		if returnRateNative == 0 {
			returnRateNative = parseDecimal(item.ProfitLoss.Rate) * 100
		}

		purchaseNative := state.totalPurchase
		marketNative := state.totalMarket
		if marketNative == 0 {
			marketNative = state.totalQty * currentPrice
		}

		purchaseFx := 1.0
		var purchaseKRW, marketKRW, profitKRW, returnRateKRW float64

		if item.Currency == toss.CurrencyUSD {
			purchaseFx = avgPurchaseFx
			purchaseKRW = purchaseNative * purchaseFx
			marketKRW = marketNative * fxMid
			profitKRW = marketKRW - purchaseKRW
			if purchaseKRW > 0 {
				returnRateKRW = (profitKRW / purchaseKRW) * 100
			}
		} else {
			purchaseKRW = purchaseNative
			marketKRW = marketNative
			profitKRW = marketKRW - purchaseKRW
			returnRateKRW = returnRateNative
		}

		holdingsOut = append(holdingsOut, model.UnifiedHolding{
			Symbol:               item.Symbol,
			Name:                 item.Name,
			TotalShares:          state.totalQty,
			AvgPrice:             avgPrice,
			CurrentPrice:         currentPrice,
			AssetType:            assetTypeFromItem(item),
			Currency:             string(item.Currency),
			ReturnRateNative:     returnRateNative,
			PurchaseAmountNative: purchaseNative,
			MarketValueNative:    marketNative,
			PurchaseFxRate:       purchaseFx,
			ReturnRateKRW:        returnRateKRW,
			PurchaseAmountKRW:    purchaseKRW,
			MarketValueKRW:       marketKRW,
			ProfitLossKRW:        profitKRW,
			Accounts:             state.accountEntries,
		})
	}

	return buildPortfolioSummary(
		fxMid, fxBuy, avgPurchaseFx,
		totalMarketKRW, totalCashKRW, totalProfitKRW, totalReturnRate, totalTodayKRW, totalTodayRate,
		brokerAccounts, holdingsOut,
	), nil
}

func buildPortfolioSummary(
	fxMid, fxBuy, avgPurchaseFx float64,
	totalMarketKRW, totalCashKRW, totalProfitKRW, totalReturnRate, totalTodayKRW, totalTodayRate float64,
	brokerAccounts []model.BrokerAccountSummary,
	holdingsOut []model.UnifiedHolding,
) *model.PortfolioSummary {
	summary := &model.PortfolioSummary{
		Source: model.PortfolioSource{
			Provider:  "toss",
			FetchedAt: time.Now().Format(time.RFC3339),
		},
		ExchangeRate:      fxMid,
		ExchangeBuyRate:   fxBuy,
		AvgPurchaseFxRate: avgPurchaseFx,
		TotalAssetsKRW:    totalMarketKRW + totalCashKRW,
		TotalCashKRW:      totalCashKRW,
		TotalProfitKRW:    totalProfitKRW,
		TotalReturnRate:   totalReturnRate,
		TodayProfitKRW:    totalTodayKRW,
		TodayChangeRate:   totalTodayRate,
		Accounts:          brokerAccounts,
		Holdings:          holdingsOut,
	}
	finalizeSummary(summary, false)
	return summary
}

func impliedPurchaseKRW(h *toss.HoldingsOverview, fxMid float64) float64 {
	evalKRW := currencyPriceToKRW(h.MarketValue.AmountAfterCost, fxMid)
	rate := parseDecimal(h.ProfitLoss.RateAfterCost)
	if rate == 0 {
		rate = parseDecimal(h.ProfitLoss.Rate)
	}
	if rate <= -1 {
		return parseDecimal(h.TotalPurchaseAmount.KRW) + parseDecimalUSD(h.TotalPurchaseAmount.USD)*fxMid
	}
	return evalKRW / (1 + rate)
}

func parseDecimalUSD(v *string) float64 {
	if v == nil {
		return 0
	}
	return parseDecimal(*v)
}

func currencyPriceToKRW(p toss.CurrencyPrice, fxMid float64) float64 {
	total := parseDecimal(p.KRW)
	if p.USD != nil {
		total += parseDecimal(*p.USD) * fxMid
	}
	return total
}

// Diagnostics and helpers below unchanged from prior implementation...

func (s *PortfolioService) Diagnostics(ctx context.Context) *model.PortfolioDiagnostics {
	d := &model.PortfolioDiagnostics{
		CheckedAt: time.Now().Format(time.RFC3339),
	}

	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		d.LastError = err.Error()
		d.Hints = []string{"로그인이 필요합니다."}
		return d
	}

	client, err := s.tossReg.ClientForUser(ctx, userID)
	if err != nil {
		d.LastError = err.Error()
		d.Hints = []string{
			"설정 → 거래 탭에서 토스증권 Open API 키를 연동하세요.",
			"또는 be/.env 에 AUTH_ALLOW_GLOBAL_TOSS=true 와 TOSS_CLIENT_ID/SECRET 을 설정할 수 있습니다.",
		}
		return d
	}

	d.TossEnabled = client.Enabled()
	if !d.TossEnabled {
		d.LastError = "toss client not enabled"
		return d
	}

	accounts, err := client.GetAccounts(ctx)
	if err != nil {
		d.LastError = err.Error()
		d.Hints = diagnoseTossError(err)
		if apiErr, ok := err.(*toss.Error); ok {
			d.ErrorCode = apiErr.Code
		}
		return d
	}

	d.TokenOK = true
	d.AccountsOK = true
	d.AccountsCount = len(accounts)

	if len(accounts) == 0 {
		d.Hints = []string{"토스증권 계좌가 연결되어 있지 않거나 조회 가능한 계좌가 없습니다."}
		return d
	}

	_, err = client.GetHoldings(ctx, accounts[0].AccountSeq, "")
	if err != nil {
		d.LastError = err.Error()
		d.Hints = diagnoseTossError(err)
		if apiErr, ok := err.(*toss.Error); ok {
			d.ErrorCode = apiErr.Code
		}
		return d
	}

	d.HoldingsOK = true
	summary, err := s.GetSummary(ctx)
	if err == nil {
		d.HoldingsCount = len(summary.Holdings)
	}
	return d
}

func accountDisplayName(acc toss.Account) string {
	suffix := acc.AccountNo
	if len(suffix) > 4 {
		suffix = suffix[len(suffix)-4:]
	}
	switch acc.AccountType {
	case toss.AccountPensionSavings:
		return fmt.Sprintf("토스증권 연금(%s)", suffix)
	case toss.AccountOverseasDerivatives:
		return fmt.Sprintf("토스증권 해외파생(%s)", suffix)
	default:
		return fmt.Sprintf("토스증권(%s)", suffix)
	}
}

func assetTypeFromItem(item toss.HoldingsItem) string {
	if item.MarketCountry == toss.MarketUS {
		name := strings.ToUpper(item.Name)
		if strings.Contains(name, "ETF") || strings.Contains(name, "ETN") {
			return "etf"
		}
		return "us"
	}
	return "domestic"
}

func diagnoseTossError(err error) []string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "403") || strings.Contains(msg, "forbidden") || strings.Contains(msg, "ip"):
		return []string{
			"IP 허용 목록 문제일 가능성이 높습니다.",
			"토스증권 WTS → [고객센터] → [Open API] → IP 허용 목록에 현재 서버 공인 IP를 등록하세요.",
			"로컬 개발 시 https://ifconfig.me 등으로 공인 IP를 확인한 뒤 등록합니다.",
			"IP 등록 후 몇 분 기다린 뒤 API를 다시 호출하세요.",
		}
	case strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "invalid_client"):
		return []string{
			"OAuth 클라이언트 ID/Secret이 올바르지 않습니다.",
			"be/.env 의 TOSS_CLIENT_ID, TOSS_CLIENT_SECRET 값을 WTS에서 재확인하세요.",
		}
	case strings.Contains(msg, "429") || strings.Contains(msg, "rate limit"):
		return []string{
			"Toss API Rate Limit에 걸렸습니다.",
			"Retry-After 헤더 시간만큼 대기 후 다시 시도하세요.",
		}
	case strings.Contains(msg, "connection") || strings.Contains(msg, "timeout"):
		return []string{
			"Toss API 서버에 연결할 수 없습니다.",
			"네트워크 연결과 TOSS_API_BASE_URL 설정을 확인하세요.",
		}
	default:
		return []string{
			"be/docs/toss-openapi.json 스펙과 be/.env 설정을 확인하세요.",
			"GET /api/v1/portfolio/diagnostics 로 상세 상태를 확인할 수 있습니다.",
		}
	}
}
