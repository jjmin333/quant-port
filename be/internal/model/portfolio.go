package model

type PortfolioSource struct {
	Provider  string `json:"provider"`
	AccountNo string `json:"accountNo,omitempty"`
	FetchedAt string `json:"fetchedAt"`
}

type BrokerAccountSummary struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	AccountSeq  int64   `json:"accountSeq"`
	AccountNo   string  `json:"accountNo"`
	AccountType string  `json:"accountType"`
	BalanceKRW  float64 `json:"balanceKRW"`
	ChangeRate  float64 `json:"changeRate"`
}

type AccountHoldingBreakdown struct {
	Broker string  `json:"broker"`
	Shares float64 `json:"shares"`
}

type UnifiedHolding struct {
	Symbol       string  `json:"symbol"`
	Name         string  `json:"name"`
	TotalShares  float64 `json:"totalShares"`
	AvgPrice     float64 `json:"avgPrice"`
	CurrentPrice float64 `json:"currentPrice"`
	AssetType    string  `json:"assetType"`
	Currency     string  `json:"currency"`

	// Native currency (USD for US stocks, KRW for domestic)
	ReturnRateNative     float64 `json:"returnRateNative"`
	PurchaseAmountNative float64 `json:"purchaseAmountNative"`
	MarketValueNative    float64 `json:"marketValueNative"`

	// KRW view — current FX uses exchangeRate (midRate), purchase uses purchaseFxRate
	PurchaseFxRate        float64 `json:"purchaseFxRate"`
	ReturnRateKRW         float64 `json:"returnRateKRW"`
	PurchaseAmountKRW     float64 `json:"purchaseAmountKRW"`
	MarketValueKRW        float64 `json:"marketValueKRW"`
	ProfitLossKRW         float64 `json:"profitLossKRW"`

	// Previous regular session (전일 정규장 종가 대비)
	PrevClose         float64 `json:"prevClose,omitempty"`
	RegularClose      float64 `json:"regularClose,omitempty"`      // 당일 정규장 종가
	RegularChange     float64 `json:"regularChange,omitempty"`     // 정규장 구간 변동 (금액)
	RegularChangeRate float64 `json:"regularChangeRate,omitempty"` // 전일 정규장 종가 대비 %
	ExtendedChange    float64 `json:"extendedChange,omitempty"`    // 시간외 구간 변동 (금액)
	ExtendedChangeRate float64 `json:"extendedChangeRate,omitempty"` // 정규장 종가 대비 %
	MarketSession     string  `json:"marketSession,omitempty"`     // regular | after | pre | day | closed
	SessionChange     float64 `json:"sessionChange,omitempty"`     // legacy: 전일 종가 대비 (합산)
	SessionChangeRate float64 `json:"sessionChangeRate,omitempty"`

	// Since first purchase (매입 이후 고/저)
	PurchaseDate          string  `json:"purchaseDate,omitempty"`
	SincePurchaseHigh     float64 `json:"sincePurchaseHigh,omitempty"`
	SincePurchaseHighDate string  `json:"sincePurchaseHighDate,omitempty"`
	SincePurchaseLow      float64 `json:"sincePurchaseLow,omitempty"`
	SincePurchaseLowDate  string  `json:"sincePurchaseLowDate,omitempty"`

	Accounts []AccountHoldingBreakdown `json:"accounts"`
}

type PortfolioSummary struct {
	Source            PortfolioSource        `json:"source"`
	ExchangeRate      float64                `json:"exchangeRate"`      // midRate (매매기준율) — Toss 앱 표시 기준
	ExchangeBuyRate   float64                `json:"exchangeBuyRate"`   // rate (매수환율)
	AvgPurchaseFxRate float64                `json:"avgPurchaseFxRate"` // USD 보유 종목 가중 평균 매입 환율
	TotalAssetsKRW    float64                `json:"totalAssetsKRW"`
	TotalCashKRW      float64                `json:"totalCashKRW"`
	TotalProfitKRW    float64                `json:"totalProfitKRW"`
	TotalReturnRate   float64                `json:"totalReturnRate"`
	TodayProfitKRW    float64                `json:"todayProfitKRW"`
	TodayChangeRate   float64                `json:"todayChangeRate"`
	Accounts          []BrokerAccountSummary `json:"accounts"`
	Holdings          []UnifiedHolding       `json:"holdings"`
	RefreshIntervalSec     int `json:"refreshIntervalSec"`     // deprecated: use fullRefreshIntervalSec
	LiveRefreshIntervalSec int `json:"liveRefreshIntervalSec"` // 시세+환율 (MARKET_DATA)
	FullRefreshIntervalSec int `json:"fullRefreshIntervalSec"` // 보유종목 (ASSET)
	PollingMode            string `json:"pollingMode,omitempty"` // active | idle
	IsLive                 bool `json:"isLive,omitempty"`
}

type PortfolioDiagnostics struct {
	TossEnabled   bool     `json:"tossEnabled"`
	TokenOK       bool     `json:"tokenOk"`
	AccountsOK    bool     `json:"accountsOk"`
	AccountsCount int      `json:"accountsCount"`
	HoldingsOK    bool     `json:"holdingsOk"`
	HoldingsCount int      `json:"holdingsCount"`
	LastError     string   `json:"lastError,omitempty"`
	ErrorCode     string   `json:"errorCode,omitempty"`
	CheckedAt     string   `json:"checkedAt"`
	Hints         []string `json:"hints,omitempty"`
}

const (
	PortfolioFullRefreshIntervalSec       = 60   // ASSET — 보유 수량·매입가
	PortfolioLiveRefreshIntervalSec       = 5    // MARKET_DATA — 현재가 (burst ~10/s, 5s = 안전)
	PortfolioIdleFullRefreshIntervalSec   = 3600 // idle — 보유종목 (다음 장 시작 전까지)
	PortfolioFXCacheSec                   = 2    // live 환율 dedupe (최대 호출 빈도 유지)
	PortfolioLiveCacheSec                 = 2    // active live dedupe
	PortfolioStatsCacheSec                = 3600 // 일봉·매입일 통계
)

// PortfolioRefreshIntervalSec is kept for backward compatibility.
const PortfolioRefreshIntervalSec = PortfolioFullRefreshIntervalSec
