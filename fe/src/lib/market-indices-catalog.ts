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
  // SPY·DIA(지수 추종 ETF, 10-01 추가)는 뺐다 (2026-10-07 사용자 요청 5 — 지수와 같은 시장이 두 번 보였다). BE 카탈로그에서도 뺐다.
  { id: "vix", name: "VIX", value: 0, change: 0, changeRate: 0, currency: "USD" },
  { id: "us10y", name: "미 국채 10년", value: 0, change: 0, changeRate: 0, currency: "USD" },
  // 국내 금리(2026-10-01, 한국은행 ECOS — 영업일 종가). 값은 % 수익률.
  { id: "kr-base", name: "한국 기준금리", value: 0, change: 0, changeRate: 0, currency: "KRW" },
  { id: "ktb3", name: "국고채 3년", value: 0, change: 0, changeRate: 0, currency: "KRW" },
  { id: "ktb10", name: "국고채 10년", value: 0, change: 0, changeRate: 0, currency: "KRW" },
  { id: "sox", name: "필라델피아 반도체", value: 0, change: 0, changeRate: 0, currency: "USD", badge: "SOXX" },
  { id: "btc", name: "비트코인", value: 0, change: 0, changeRate: 0, currency: "KRW" },
  { id: "gold", name: "금", value: 0, change: 0, changeRate: 0, currency: "USD" },
];

export function mergeMarketIndices(fetched: MarketIndex[]): MarketIndex[] {
  if (fetched.length === 0) return MARKET_INDEX_PLACEHOLDERS;
  const byId = new Map(fetched.map((i) => [i.id, i]));
  return MARKET_INDEX_PLACEHOLDERS.map((placeholder) => byId.get(placeholder.id) ?? placeholder);
}

/**
 * 지수별 출처·권리자 표기 (2026-10-01 결정, 10-07부터 홈 띠 밑이 아니라 **그 지수의 모달에서만** 보인다).
 * FRED 시리즈 노트의 권리자를 짧게 옮긴 것이다. 없는 지수는 표기하지 않는다.
 */
export const INDEX_ATTRIBUTION: Record<string, string> = {
  sp500: "FRED 제공 · © S&P Dow Jones Indices",
  dow: "FRED 제공 · © S&P Dow Jones Indices",
  nasdaq: "FRED 제공 · © Nasdaq",
  nasdaq100: "FRED 제공 · © Nasdaq",
  vix: "FRED 제공 · © Cboe",
  us10y: "FRED 제공",
};

/** 값이 금리(%)인 지표 — 등락을 %가 아니라 bp(0.01%p)로 읽는다. */
export const RATE_INDEX_IDS = new Set(["us10y", "kr-base", "ktb3", "ktb10"]);

export function formatRateChange(change: number): string {
  const bp = Math.round(change * 100);
  return bp === 0 ? "0bp" : `${bp > 0 ? "+" : ""}${bp}bp`;
}
