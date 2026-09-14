export type QuoteCurrency = "KRW" | "USD";
export type QuoteAssetType = "domestic" | "us" | "etf";
export type QuoteFilter = "all" | "domestic" | "us" | "watchlist" | "held";
export type CandleInterval = "1m" | "5m" | "10m" | "30m" | "1d" | "1w" | "1M";
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
  /**
   * 마감 세션에서만 오는 두 시각(ISO). 서버가 거래소 달력에서 계산한다
   * (`be/internal/service/market_session_clock.go`).
   *
   * `sessionClosedAt` — **이 가격이 멈춘 시각.** 화면은 마감 중에 `updatedAt`(응답을
   * 받은 시각) 대신 이것을 찍는다. 그러지 않으면 새벽 0시 33분에 열어 본 화면이
   * 「오전 12:33:08 기준」이라고 적는데, 그 값은 전날 20:00에 멈춘 것이다.
   *
   * `nextSessionAt` — 다음 창(국내 프리마켓 08:00 등)이 열리는 시각. 화면은 그때까지
   * 시세 폴링을 멈춘다(`use-quote-detail`).
   */
  sessionClosedAt?: string;
  nextSessionAt?: string;
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

/**
 * Baseline price used for displayed change / changeRate.
 *
 * **언제나 전일 종가다.** 예전에는 마감·시간외 세션에서 「당일 정규장 종가」로
 * 갈아탔는데, 현재가는 시간외까지 반영한 최종가라 그 둘을 빼면 하루 등락률이
 * 아니라 **시간외 등락률**이 나온다(실측 2026-09-01 삼성전자: 최종가 255,000 ·
 * 당일 정규장 종가 260,500 → −2.11%로 보였지만 실제 그날 움직임은 전일 258,000
 * 대비 −1.16%). 서버도 같은 규칙으로 계산한다(`computeQuoteChange`) — 여기서만
 * 다른 기준을 쓰면 같은 값의 등락률과 기준선이 어긋난다.
 *
 * `regularClose`는 「정규장 종가」 줄로 따로 보여 준다.
 */
export function quoteChangeBaseline(quote: Pick<QuoteDetail, "prevClose" | "regularClose" | "marketSession">): number {
  return quote.prevClose;
}

/**
 * 등락률이 무엇에 대한 값인지 적는 한 줄.
 *
 * 예전에는 세션에 따라 「정규장 종가 대비」와 「전일 정규장 종가 대비」로 갈렸다.
 * 기준을 전일 종가로 통일한 뒤로는 한 문장뿐이라 상수로 둔다 — 세션을 받아
 * 분기하는 함수로 남겨 두면 「세션마다 다를 수 있다」는 잘못된 신호가 된다.
 */
export const QUOTE_CHANGE_BASIS_LABEL = "전일 종가 대비";

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
