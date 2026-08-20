import type {
  MarketCountry,
  MarketRadarParams,
  RankingDuration,
  RankingType,
} from "@/lib/market-radar-types";

export const MARKET_RADAR_DEFAULT_COUNT = 100;

/** FE fallback refresh interval before `market_data_settings` loads; actual cadence is server-configured (see useMarketDataSettings). */
export const MARKET_RADAR_POLL_MS = 60_000;

/** Quote overlay for list price/change — same API path as detail panel (2s). */
export const MARKET_RADAR_QUOTE_POLL_MS = 2_000;

/** List sparklines cost ~1 candle API each — off by default for faster load. */
export const MARKET_RADAR_SHOW_SPARKLINES = false;

export const RANKING_TYPE_OPTIONS: { id: RankingType; label: string }[] = [
  { id: "MARKET_TRADING_AMOUNT", label: "거래대금" },
  { id: "MARKET_TRADING_VOLUME", label: "거래량" },
];

export const RANKING_DURATION_OPTIONS: { id: RankingDuration; label: string }[] = [
  { id: "realtime", label: "실시간" },
  { id: "1d", label: "1일" },
  { id: "1w", label: "1주" },
  { id: "1mo", label: "1개월" },
  { id: "3mo", label: "3개월" },
  { id: "6mo", label: "6개월" },
  { id: "1y", label: "1년" },
];

const RANKING_DURATION_SET = new Set(RANKING_DURATION_OPTIONS.map((o) => o.id));

export function parseMarketRadarParams(searchParams: URLSearchParams): MarketRadarParams {
  const market = searchParams.get("market") === "US" ? "US" : "KR";
  const type = parseRankingType(searchParams.get("type"));
  const duration = parseRankingDuration(searchParams.get("duration"));

  return { market, type, duration, count: MARKET_RADAR_DEFAULT_COUNT };
}

export function marketRadarSearchParams(params: MarketRadarParams): URLSearchParams {
  const q = new URLSearchParams();
  q.set("market", params.market);
  q.set("type", params.type);
  if (params.duration !== "realtime") {
    q.set("duration", params.duration);
  }
  return q;
}

function parseRankingType(value: string | null): RankingType {
  if (value && RANKING_TYPE_OPTIONS.some((o) => o.id === value)) {
    return value as RankingType;
  }
  return "MARKET_TRADING_AMOUNT";
}

function parseRankingDuration(value: string | null): RankingDuration {
  if (value && RANKING_DURATION_SET.has(value as RankingDuration)) {
    return value as RankingDuration;
  }
  return "realtime";
}

export function rankingMetricLabel(type: RankingType): "amount" | "volume" {
  if (type.includes("VOLUME")) return "volume";
  return "amount";
}

export function changeRateHint(duration: RankingDuration): string {
  if (duration === "realtime") {
    return "전일 대비 등락률 · 실시간 거래대금/거래량 순";
  }
  const label = RANKING_DURATION_OPTIONS.find((o) => o.id === duration)?.label ?? duration;
  return `${label} 누적 거래대금/거래량 순 · 등락률은 전일 대비`;
}

export function rankingRefreshHint(duration: RankingDuration, realtimeMinutes = 5): string {
  if (duration === "realtime") return `${realtimeMinutes}분 주기`;
  if (duration === "1d" || duration === "1w") return "15분 주기";
  return "1시간 주기";
}

export function parseRankingNumber(raw: string | null | undefined): number {
  if (!raw) return 0;
  const n = Number.parseFloat(raw.replace(/,/g, ""));
  return Number.isFinite(n) ? n : 0;
}

/** Toss ranking changeRate uses basePrice (전일 정규장 기준가). Never trust raw changeRate alone. */
export function parseRankingChangeRate(
  raw: string | null | undefined,
  lastPrice?: string | null,
  basePrice?: string | null,
): number {
  const last = parseRankingNumber(lastPrice ?? undefined);
  const base = parseRankingNumber(basePrice ?? undefined);
  if (last > 0 && base > 0) {
    return ((last - base) / base) * 100;
  }
  return 0;
}

export function marketCountryToMarketField(country: MarketCountry): string {
  return country === "US" ? "US" : "KR";
}

export function marketRadarParamsKey(params: MarketRadarParams): string {
  return `${params.market}|${params.type}|${params.duration}`;
}

export function parseMarketRadarSymbol(searchParams: URLSearchParams): string {
  return searchParams.get("symbol")?.trim() ?? "";
}

/** Guess KR/US for market radar toolbar from stock metadata. */
export function inferMarketCountryFromStock(stock: {
  currency?: string;
  market?: string;
}): MarketCountry {
  if (stock.currency === "USD") return "US";
  const market = (stock.market ?? "").toUpperCase();
  if (
    market.includes("NASDAQ") ||
    market.includes("NYSE") ||
    market.includes("AMEX") ||
    market.includes("US") ||
    market.includes("ADR")
  ) {
    return "US";
  }
  return "KR";
}

export function marketRadarHref(
  params: MarketRadarParams,
  options?: { symbol?: string },
): string {
  const q = marketRadarSearchParams(params);
  if (options?.symbol) q.set("symbol", options.symbol);
  return `/market?${q.toString()}`;
}

export function marketRadarSymbolHref(symbol: string, market?: MarketCountry): string {
  return marketRadarHref(
    {
      market: market ?? "KR",
      type: "MARKET_TRADING_AMOUNT",
      duration: "realtime",
      count: MARKET_RADAR_DEFAULT_COUNT,
    },
    { symbol },
  );
}
