import type { MarketIndex } from "@/lib/quotes-types";

/** Mirrors BE marketIndexCatalog — used for instant UI before fetch completes. */
export const MARKET_INDEX_PLACEHOLDERS: MarketIndex[] = [
  { id: "kospi", name: "KOSPI", value: 0, change: 0, changeRate: 0, currency: "KRW" },
  { id: "kosdaq", name: "KOSDAQ", value: 0, change: 0, changeRate: 0, currency: "KRW" },
  { id: "usd-krw", name: "USD/KRW", value: 0, change: 0, changeRate: 0, currency: "KRW" },
  { id: "nasdaq", name: "나스닥", value: 0, change: 0, changeRate: 0, currency: "USD" },
  { id: "nasdaq100", name: "나스닥 100", value: 0, change: 0, changeRate: 0, currency: "USD" },
  { id: "nq100", name: "나스닥100 선물", value: 0, change: 0, changeRate: 0, currency: "USD", badge: "QQQ" },
  { id: "sp500", name: "S&P 500", value: 0, change: 0, changeRate: 0, currency: "USD" },
  { id: "dow", name: "다우존스", value: 0, change: 0, changeRate: 0, currency: "USD" },
  { id: "vix", name: "VIX", value: 0, change: 0, changeRate: 0, currency: "USD" },
  { id: "us10y", name: "미 국채 10년", value: 0, change: 0, changeRate: 0, currency: "USD" },
  { id: "sox", name: "필라델피아 반도체", value: 0, change: 0, changeRate: 0, currency: "USD", badge: "SOXX" },
  { id: "btc", name: "비트코인", value: 0, change: 0, changeRate: 0, currency: "KRW" },
  { id: "gold", name: "금", value: 0, change: 0, changeRate: 0, currency: "USD" },
];

export function mergeMarketIndices(fetched: MarketIndex[]): MarketIndex[] {
  if (fetched.length === 0) return MARKET_INDEX_PLACEHOLDERS;
  const byId = new Map(fetched.map((i) => [i.id, i]));
  return MARKET_INDEX_PLACEHOLDERS.map((placeholder) => byId.get(placeholder.id) ?? placeholder);
}
