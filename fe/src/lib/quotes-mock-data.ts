export type QuoteCurrency = "KRW" | "USD";
export type QuoteAssetType = "domestic" | "us" | "etf";
export type QuoteMarket = "KRX" | "KOSDAQ" | "NYSE" | "NASDAQ" | "AMEX";
export type CandleInterval = "1m" | "5m" | "10m" | "30m" | "1d" | "1w" | "1M";
export type TradeSide = "buy" | "sell" | "neutral";

export type MarketIndex = {
  id: string;
  name: string;
  value: number;
  change: number;
  changeRate: number;
  currency: QuoteCurrency;
};

export type QuoteSummary = {
  symbol: string;
  name: string;
  market: QuoteMarket;
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

export const marketIndices: MarketIndex[] = [
  { id: "kospi", name: "KOSPI", value: 2_687.42, change: 18.35, changeRate: 0.69, currency: "KRW" },
  { id: "kosdaq", name: "KOSDAQ", value: 842.18, change: -4.22, changeRate: -0.5, currency: "KRW" },
  { id: "sp500", name: "S&P 500", value: 5_487.03, change: 32.14, changeRate: 0.59, currency: "USD" },
  { id: "nasdaq", name: "NASDAQ", value: 17_832.96, change: 98.42, changeRate: 0.55, currency: "USD" },
  { id: "usd-krw", name: "USD/KRW", value: 1_382.5, change: 3.2, changeRate: 0.23, currency: "KRW" },
];

export const quoteSummaries: QuoteSummary[] = [
  {
    symbol: "005930",
    name: "삼성전자",
    market: "KRX",
    currency: "KRW",
    assetType: "domestic",
    currentPrice: 62_100,
    change: 1_400,
    changeRate: 2.31,
    volume: 12_450_000,
    tradingValue: 768_000_000_000,
    isWatchlisted: true,
    isHeld: true,
  },
  {
    symbol: "000660",
    name: "SK하이닉스",
    market: "KRX",
    currency: "KRW",
    assetType: "domestic",
    currentPrice: 195_200,
    change: 3_500,
    changeRate: 1.85,
    volume: 3_820_000,
    tradingValue: 742_000_000_000,
    isWatchlisted: true,
    isHeld: true,
  },
  {
    symbol: "035420",
    name: "NAVER",
    market: "KRX",
    currency: "KRW",
    assetType: "domestic",
    currentPrice: 218_500,
    change: -2_800,
    changeRate: -1.27,
    volume: 890_000,
    tradingValue: 194_000_000_000,
    isWatchlisted: false,
    isHeld: false,
  },
  {
    symbol: "373220",
    name: "LG에너지솔루션",
    market: "KRX",
    currency: "KRW",
    assetType: "domestic",
    currentPrice: 382_000,
    change: 5_500,
    changeRate: 1.46,
    volume: 420_000,
    tradingValue: 159_000_000_000,
    isWatchlisted: false,
    isHeld: false,
  },
  {
    symbol: "NVDA",
    name: "NVIDIA",
    market: "NASDAQ",
    currency: "USD",
    assetType: "us",
    currentPrice: 128.4,
    change: 1.82,
    changeRate: 1.44,
    volume: 48_200_000,
    tradingValue: 6_180_000_000,
    isWatchlisted: true,
    isHeld: false,
  },
  {
    symbol: "TSLA",
    name: "Tesla Inc.",
    market: "NASDAQ",
    currency: "USD",
    assetType: "us",
    currentPrice: 261.5,
    change: 7.92,
    changeRate: 3.12,
    volume: 82_400_000,
    tradingValue: 21_500_000_000,
    isWatchlisted: true,
    isHeld: true,
  },
  {
    symbol: "SOXL",
    name: "SOXL",
    market: "NYSE",
    currency: "USD",
    assetType: "etf",
    currentPrice: 35.8,
    change: -0.33,
    changeRate: -0.92,
    volume: 28_600_000,
    tradingValue: 1_020_000_000,
    isWatchlisted: true,
    isHeld: true,
  },
  {
    symbol: "AAPL",
    name: "Apple Inc.",
    market: "NASDAQ",
    currency: "USD",
    assetType: "us",
    currentPrice: 214.28,
    change: 0.86,
    changeRate: 0.4,
    volume: 35_100_000,
    tradingValue: 7_520_000_000,
    isWatchlisted: false,
    isHeld: false,
  },
];

const quoteDetailsMap: Record<string, Omit<QuoteDetail, keyof QuoteSummary>> = {
  "005930": {
    open: 60_800,
    high: 62_400,
    low: 60_500,
    prevClose: 60_700,
    week52High: 89_500,
    week52Low: 48_200,
    marketCap: 371_000_000_000_000,
    per: 12.3,
    pbr: 1.18,
    updatedAt: "2026-07-14T18:42:15+09:00",
  },
  "000660": {
    open: 192_000,
    high: 196_800,
    low: 191_500,
    prevClose: 191_700,
    week52High: 218_000,
    week52Low: 118_500,
    marketCap: 142_000_000_000_000,
    per: 14.8,
    pbr: 2.42,
    updatedAt: "2026-07-14T18:42:12+09:00",
  },
  "035420": {
    open: 221_000,
    high: 222_500,
    low: 217_800,
    prevClose: 221_300,
    week52High: 285_000,
    week52Low: 178_000,
    marketCap: 35_800_000_000_000,
    per: 22.1,
    pbr: 1.05,
    updatedAt: "2026-07-14T18:41:58+09:00",
  },
  "373220": {
    open: 378_500,
    high: 384_000,
    low: 376_000,
    prevClose: 376_500,
    week52High: 498_000,
    week52Low: 312_000,
    marketCap: 89_200_000_000_000,
    per: null,
    pbr: 3.82,
    updatedAt: "2026-07-14T18:42:01+09:00",
  },
  NVDA: {
    open: 126.8,
    high: 129.2,
    low: 126.1,
    prevClose: 126.58,
    week52High: 140.76,
    week52Low: 47.32,
    marketCap: 3_150_000_000_000,
    per: 65.2,
    pbr: 48.5,
    updatedAt: "2026-07-14T05:42:08-04:00",
  },
  TSLA: {
    open: 254.2,
    high: 263.8,
    low: 252.4,
    prevClose: 253.58,
    week52High: 278.98,
    week52Low: 138.8,
    marketCap: 832_000_000_000,
    per: 58.4,
    pbr: 9.2,
    updatedAt: "2026-07-14T05:42:05-04:00",
  },
  SOXL: {
    open: 36.2,
    high: 36.45,
    low: 35.4,
    prevClose: 36.13,
    week52High: 48.72,
    week52Low: 18.24,
    marketCap: 8_200_000_000,
    per: null,
    pbr: null,
    updatedAt: "2026-07-14T05:41:52-04:00",
  },
  AAPL: {
    open: 213.5,
    high: 215.1,
    low: 212.8,
    prevClose: 213.42,
    week52High: 237.23,
    week52Low: 164.08,
    marketCap: 3_280_000_000_000,
    per: 32.8,
    pbr: 45.2,
    updatedAt: "2026-07-14T05:42:00-04:00",
  },
};

export function getQuoteDetail(symbol: string): QuoteDetail | null {
  const summary = quoteSummaries.find((q) => q.symbol === symbol);
  const detail = quoteDetailsMap[symbol];
  if (!summary || !detail) return null;
  return { ...summary, ...detail };
}

const orderBooksMap: Record<string, OrderBook> = {
  "005930": {
    symbol: "005930",
    timestamp: "2026-07-14T18:42:15+09:00",
    asks: [
      { price: 62_200, quantity: 4_820, orderCount: 128 },
      { price: 62_150, quantity: 8_340, orderCount: 214 },
      { price: 62_120, quantity: 12_600, orderCount: 302 },
      { price: 62_100, quantity: 18_200, orderCount: 456 },
      { price: 62_050, quantity: 6_900, orderCount: 189 },
    ],
    bids: [
      { price: 62_000, quantity: 15_400, orderCount: 388 },
      { price: 61_950, quantity: 9_800, orderCount: 245 },
      { price: 61_900, quantity: 11_200, orderCount: 278 },
      { price: 61_850, quantity: 7_600, orderCount: 198 },
      { price: 61_800, quantity: 5_200, orderCount: 142 },
    ],
  },
  TSLA: {
    symbol: "TSLA",
    timestamp: "2026-07-14T05:42:05-04:00",
    asks: [
      { price: 261.75, quantity: 420, orderCount: 38 },
      { price: 261.6, quantity: 680, orderCount: 52 },
      { price: 261.55, quantity: 920, orderCount: 64 },
      { price: 261.5, quantity: 1_240, orderCount: 88 },
      { price: 261.45, quantity: 560, orderCount: 41 },
    ],
    bids: [
      { price: 261.4, quantity: 980, orderCount: 72 },
      { price: 261.35, quantity: 740, orderCount: 58 },
      { price: 261.3, quantity: 1_120, orderCount: 84 },
      { price: 261.25, quantity: 640, orderCount: 46 },
      { price: 261.2, quantity: 480, orderCount: 35 },
    ],
  },
};

export function getOrderBook(symbol: string): OrderBook {
  const book = orderBooksMap[symbol];
  if (book) return book;

  const detail = getQuoteDetail(symbol);
  const price = detail?.currentPrice ?? 100;
  const step = detail?.currency === "USD" ? 0.05 : 50;

  return {
    symbol,
    timestamp: new Date().toISOString(),
    asks: Array.from({ length: 5 }, (_, i) => ({
      price: price + step * (5 - i),
      quantity: Math.round(1000 + Math.random() * 5000),
      orderCount: Math.round(50 + Math.random() * 200),
    })),
    bids: Array.from({ length: 5 }, (_, i) => ({
      price: price - step * (i + 1),
      quantity: Math.round(1000 + Math.random() * 5000),
      orderCount: Math.round(50 + Math.random() * 200),
    })),
  };
}

const tradesMap: Record<string, TradeTick[]> = {
  "005930": [
    { id: "1", symbol: "005930", time: "18:42:15", price: 62_100, quantity: 120, side: "buy" },
    { id: "2", symbol: "005930", time: "18:42:14", price: 62_050, quantity: 450, side: "sell" },
    { id: "3", symbol: "005930", time: "18:42:12", price: 62_100, quantity: 80, side: "buy" },
    { id: "4", symbol: "005930", time: "18:42:10", price: 62_000, quantity: 320, side: "sell" },
    { id: "5", symbol: "005930", time: "18:42:08", price: 62_050, quantity: 200, side: "buy" },
    { id: "6", symbol: "005930", time: "18:42:05", price: 62_100, quantity: 560, side: "buy" },
    { id: "7", symbol: "005930", time: "18:42:03", price: 62_000, quantity: 140, side: "neutral" },
    { id: "8", symbol: "005930", time: "18:42:01", price: 61_950, quantity: 890, side: "sell" },
  ],
  TSLA: [
    { id: "1", symbol: "TSLA", time: "05:42:05", price: 261.5, quantity: 40, side: "buy" },
    { id: "2", symbol: "TSLA", time: "05:42:04", price: 261.45, quantity: 120, side: "sell" },
    { id: "3", symbol: "TSLA", time: "05:42:02", price: 261.5, quantity: 85, side: "buy" },
    { id: "4", symbol: "TSLA", time: "05:42:00", price: 261.4, quantity: 200, side: "sell" },
    { id: "5", symbol: "TSLA", time: "05:41:58", price: 261.55, quantity: 60, side: "buy" },
  ],
};

export function getRecentTrades(symbol: string): TradeTick[] {
  if (tradesMap[symbol]) return tradesMap[symbol];

  const detail = getQuoteDetail(symbol);
  const price = detail?.currentPrice ?? 100;
  return Array.from({ length: 8 }, (_, i) => ({
    id: String(i + 1),
    symbol,
    time: `18:4${i}:${10 + i * 2}`,
    price: price + (i % 2 === 0 ? 0 : -(detail?.currency === "USD" ? 0.05 : 50)),
    quantity: Math.round(50 + Math.random() * 400),
    side: (["buy", "sell", "neutral"] as TradeSide[])[i % 3],
  }));
}

function generateCandles(basePrice: number, count: number, volatility: number): Candle[] {
  let price = basePrice;
  const candles: Candle[] = [];

  for (let i = 0; i < count; i++) {
    const open = price;
    const delta = (Math.random() - 0.48) * volatility;
    const close = open + delta;
    const high = Math.max(open, close) + Math.random() * volatility * 0.4;
    const low = Math.min(open, close) - Math.random() * volatility * 0.4;
    const hour = 9 + Math.floor(i / 12);
    const minute = (i * 5) % 60;

    candles.push({
      time: `${String(hour).padStart(2, "0")}:${String(minute).padStart(2, "0")}`,
      open,
      high,
      low,
      close,
      volume: Math.round(100_000 + Math.random() * 800_000),
    });
    price = close;
  }

  return candles;
}

const candlesMap: Record<string, Record<CandleInterval, Candle[]>> = {
  "005930": {
    "1m": generateCandles(60_800, 60, 200),
    "5m": generateCandles(60_800, 48, 400),
    "10m": generateCandles(60_800, 36, 600),
    "30m": generateCandles(60_800, 24, 900),
    "1d": generateCandles(60_800, 30, 1_200),
    "1w": generateCandles(60_800, 24, 2_500),
    "1M": generateCandles(60_800, 12, 5_000),
  },
  TSLA: {
    "1m": generateCandles(254.2, 60, 0.8),
    "5m": generateCandles(254.2, 48, 1.5),
    "10m": generateCandles(254.2, 36, 2),
    "30m": generateCandles(254.2, 24, 3),
    "1d": generateCandles(254.2, 30, 4),
    "1w": generateCandles(254.2, 24, 8),
    "1M": generateCandles(254.2, 12, 15),
  },
};

export function getCandles(symbol: string, interval: CandleInterval): Candle[] {
  const stored = candlesMap[symbol]?.[interval];
  if (stored) return stored;

  const detail = getQuoteDetail(symbol);
  const base = detail?.currentPrice ?? 100;
  const volatility = detail?.currency === "USD" ? base * 0.008 : base * 0.003;
  return generateCandles(base, interval === "1M" ? 12 : 48, volatility);
}

export type QuoteFilter = "all" | "domestic" | "us" | "watchlist" | "held";

export function filterQuotes(filter: QuoteFilter): QuoteSummary[] {
  switch (filter) {
    case "domestic":
      return quoteSummaries.filter((q) => q.assetType === "domestic");
    case "us":
      return quoteSummaries.filter((q) => q.assetType === "us" || q.assetType === "etf");
    case "watchlist":
      return quoteSummaries.filter((q) => q.isWatchlisted);
    case "held":
      return quoteSummaries.filter((q) => q.isHeld);
    default:
      return quoteSummaries;
  }
}
