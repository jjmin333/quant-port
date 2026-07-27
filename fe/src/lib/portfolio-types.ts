export type PortfolioSource = {
  provider: string;
  accountNo?: string;
  fetchedAt: string;
};

export type BrokerAccountSummary = {
  id: string;
  name: string;
  accountSeq: number;
  accountNo: string;
  accountType: string;
  balanceKRW: number;
  changeRate: number;
};

export type AccountHoldingBreakdown = {
  broker: string;
  shares: number;
};

export type UnifiedHolding = {
  symbol: string;
  name: string;
  totalShares: number;
  avgPrice: number;
  currentPrice: number;
  assetType: "domestic" | "us" | "etf" | "bond" | "cash";
  currency: "KRW" | "USD";
  returnRateNative: number;
  purchaseAmountNative: number;
  marketValueNative: number;
  purchaseFxRate: number;
  returnRateKRW: number;
  purchaseAmountKRW: number;
  marketValueKRW: number;
  profitLossKRW: number;
  accounts: AccountHoldingBreakdown[];
  prevClose?: number;
  regularClose?: number;
  regularChange?: number;
  regularChangeRate?: number;
  extendedChange?: number;
  extendedChangeRate?: number;
  marketSession?: string;
  sessionChange?: number;
  sessionChangeRate?: number;
  purchaseDate?: string;
  sincePurchaseHigh?: number;
  sincePurchaseHighDate?: string;
  sincePurchaseLow?: number;
  sincePurchaseLowDate?: string;
};

export type PortfolioSummary = {
  source: PortfolioSource;
  exchangeRate: number;
  exchangeBuyRate: number;
  avgPurchaseFxRate: number;
  totalAssetsKRW: number;
  totalCashKRW: number;
  totalProfitKRW: number;
  totalReturnRate: number;
  todayProfitKRW: number;
  todayChangeRate: number;
  accounts: BrokerAccountSummary[];
  holdings: UnifiedHolding[];
  refreshIntervalSec?: number;
  liveRefreshIntervalSec?: number;
  fullRefreshIntervalSec?: number;
  pollingMode?: "active" | "idle";
  isLive?: boolean;
};

export type PortfolioDiagnostics = {
  tossEnabled: boolean;
  tokenOk: boolean;
  accountsOk: boolean;
  accountsCount: number;
  holdingsOk: boolean;
  holdingsCount: number;
  lastError?: string;
  errorCode?: string;
  checkedAt: string;
  hints?: string[];
};

export type ApiError = {
  error: string;
};

export type ApiResponse<T> = {
  data: T;
};

export type DisplayCurrency = "KRW" | "USD";
