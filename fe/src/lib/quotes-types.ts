export type QuoteCurrency = "KRW" | "USD";
export type QuoteAssetType = "domestic" | "us" | "etf";
export type QuoteFilter = "all" | "domestic" | "us" | "watchlist" | "held";
export type CandleInterval = "1m" | "5m" | "10m" | "30m" | "60m" | "1d" | "1w" | "1M";
export type ChartStyle = "bar" | "candle" | "area";
export type TradeSide = "buy" | "sell" | "neutral" | "unknown";

export type IndexDataKind = "live" | "eod";

export type IndexGroupMeta = {
  updatedAt: string;
  source: string;
};

export type MarketIndex = {
  id: string;
  name: string;
  value: number;
  change: number;
  changeRate: number;
  currency: QuoteCurrency;
  badge?: string;
  sparkline?: number[];
  updatedAt?: string;
  dataKind?: IndexDataKind;
};

export type QuoteSummary = {
  symbol: string;
  name: string;
  market: string;
  currency: QuoteCurrency;
  assetType: QuoteAssetType;
  logoUrl?: string;
  currentPrice: number;
  change: number;
  changeRate: number;
  volume: number;
  tradingValue: number;
  isWatchlisted: boolean;
  isHeld: boolean;
  watchlistMetrics?: WatchlistMetrics;
};

export type WatchlistMetrics = {
  addedAt: string;
  changeRatePrevClose: number;
  prevClose?: number;
  changeRatePrevWeek?: number | null;
  prevWeekClose?: number;
  changeRatePrevMonth?: number | null;
  prevMonthClose?: number;
  week52High?: number;
  week52Low?: number;
  changeRateWeek52High?: number | null;
  changeRateWeek52Low?: number | null;
};

export type QuoteDetail = QuoteSummary & {
  open: number;
  high: number;
  low: number;
  prevClose: number;
  regularClose?: number;
  marketSession?: string;
  week52High: number;
  week52Low: number;
  marketCap: number;
  per: number | null;
  pbr: number | null;
  updatedAt: string;
};

export type OrderBookLevel = {
  price: number;
  quantity: number;
  orderCount: number;
};

export type OrderBook = {
  symbol: string;
  asks: OrderBookLevel[];
  bids: OrderBookLevel[];
  timestamp: string;
};

export type TradeTick = {
  id: string;
  symbol: string;
  time: string;
  price: number;
  quantity: number;
  side: TradeSide;
};

export type Candle = {
  time: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

export type CandlesPage = {
  candles: Candle[];
  nextBefore: string | null;
};

export type CandleSnapshot = {
  symbol: string;
  interval: CandleInterval;
  completed: Candle[];
  forming?: Candle | null;
  nextBefore?: string | null;
  rev: number;
  source: string;
};

export type QuoteTerminalSnapshot = {
  detail: QuoteDetail;
  orderBook: OrderBook;
  trades: TradeTick[];
  source: string;
};

export type StreamEvent = {
  type: "snapshot" | "bar_update" | "bar_close" | "snapshot_bars";
  symbol: string;
  interval?: CandleInterval;
  bar?: Candle;
  forming?: Candle;
  rev: number;
  completed?: Candle[];
};

export type QuoteStreamEvent = {
  type: "quote_update";
  symbol: string;
  currentPrice: number;
  change: number;
  changeRate: number;
  prevClose: number;
  regularClose?: number;
  marketSession?: string;
  updatedAt: string;
};

/** Baseline price used for displayed change / changeRate. */
export function quoteChangeBaseline(quote: Pick<QuoteDetail, "prevClose" | "regularClose" | "marketSession">): number {
  if (
    (quote.marketSession === "after" || quote.marketSession === "closed") &&
    quote.regularClose != null &&
    quote.regularClose > 0
  ) {
    return quote.regularClose;
  }
  return quote.prevClose;
}

export function quoteChangeBasisLabel(quote: Pick<QuoteDetail, "prevClose" | "regularClose" | "marketSession">): string {
  const baseline = quoteChangeBaseline(quote);
  if (
    baseline > 0 &&
    quote.regularClose != null &&
    quote.regularClose > 0 &&
    baseline === quote.regularClose
  ) {
    return "정규장 종가 대비";
  }
  return "전일 정규장 종가 대비";
}

export type WatchlistItem = {
  id: number;
  symbol: string;
  createdAt: string;
  addedPrice?: number;
};

export type StockInfo = {
  symbol: string;
  name: string;
  englishName?: string;
  market: string;
  currency: QuoteCurrency;
  assetType?: QuoteAssetType;
  logoUrl?: string;
  securityType?: string;
  listDate?: string;
  sharesOutstanding?: string;
};
