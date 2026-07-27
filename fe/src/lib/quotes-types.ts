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
  currentPrice: number;
  change: number;
  changeRate: number;
  volume: number;
  tradingValue: number;
  isWatchlisted: boolean;
  isHeld: boolean;
};

export type QuoteDetail = QuoteSummary & {
  open: number;
  high: number;
  low: number;
  prevClose: number;
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

export type WatchlistItem = {
  id: number;
  symbol: string;
  createdAt: string;
};

export type StockInfo = {
  symbol: string;
  name: string;
  englishName?: string;
  market: string;
  currency: QuoteCurrency;
  securityType?: string;
  listDate?: string;
  sharesOutstanding?: string;
};
