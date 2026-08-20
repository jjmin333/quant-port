import {
  PortfolioFetchError,
  apiRequest,
  ensureOk,
  getApiBaseUrl,
  parseJson,
} from "@/lib/api-client";
import { cachedGet } from "@/lib/fetch-cache";
import type {
  Candle,
  CandleInterval,
  CandleSnapshot,
  CandlesPage,
  IndexGroupMeta,
  MarketIndex,
  OrderBook,
  QuoteDetail,
  QuoteFilter,
  QuoteSummary,
  QuoteTerminalSnapshot,
  StockInfo,
  TradeTick,
  WatchlistItem,
} from "@/lib/quotes-types";
import type {
  MarketCountry,
  RankingDuration,
  RankingsResponse,
  RankingType,
} from "@/lib/market-radar-types";

export { PortfolioFetchError, getApiBaseUrl };

function apiGet<T>(path: string, init?: RequestInit): Promise<T> {
  return apiRequest<T>(path, init);
}

function jsonBody(payload?: unknown): RequestInit {
  return {
    headers: { "Content-Type": "application/json" },
    body: payload ? JSON.stringify(payload) : undefined,
  };
}

function apiPost<T>(path: string, payload?: unknown): Promise<T> {
  return apiRequest<T>(path, { method: "POST", ...jsonBody(payload) });
}

function apiPut<T>(path: string, payload?: unknown): Promise<T> {
  return apiRequest<T>(path, { method: "PUT", ...jsonBody(payload) });
}

function apiDelete<T>(path: string): Promise<T> {
  return apiRequest<T>(path, { method: "DELETE" });
}

// Quotes
export async function fetchMarketIndicesClient(): Promise<{
  indices: MarketIndex[];
  updatedAt: string | null;
  groups: Record<string, IndexGroupMeta>;
}> {
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), 8_000);
  try {
    const res = await fetch(`${getApiBaseUrl()}/api/v1/quotes/indices`, {
      cache: "no-store",
      signal: controller.signal,
    });
    await ensureOk(res, "Failed to load indices");
    const body = await parseJson<{
      data: MarketIndex[] | null;
      updatedAt?: string;
      groups?: Record<string, IndexGroupMeta>;
    }>(res);
    return {
      indices: body.data ?? [],
      updatedAt: body.updatedAt ?? null,
      groups: body.groups ?? {},
    };
  } finally {
    window.clearTimeout(timeout);
  }
}

export async function fetchQuotesClient(
  filter: QuoteFilter = "all",
  q?: string,
  limit = 100,
  offset = 0,
): Promise<QuoteSummary[]> {
  const params = new URLSearchParams({ filter, limit: String(limit), offset: String(offset) });
  if (q?.trim()) params.set("q", q.trim());
  const data = await apiRequest<QuoteSummary[] | null>(
    `/api/v1/quotes?${params}`,
    undefined,
    "Failed to load quotes",
  );
  return data ?? [];
}

export async function fetchExchangeRateClient(): Promise<number> {
  const res = await fetch(
    `${getApiBaseUrl()}/api/v1/exchange-rate?baseCurrency=USD&quoteCurrency=KRW`,
    { cache: "no-store" },
  );
  await ensureOk(res, "Failed to load exchange rate");
  const body = await parseJson<{ data: { midRate?: string; rate?: string } | null }>(res);
  const raw = body.data?.midRate ?? body.data?.rate ?? "0";
  const rate = Number.parseFloat(raw);
  return Number.isFinite(rate) ? rate : 0;
}

export async function searchStocksCatalogClient(
  q: string,
  market?: "domestic" | "us" | "",
  limit = 50,
  offset = 0,
): Promise<{ items: StockInfo[]; total: number }> {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (q.trim()) params.set("q", q.trim());
  if (market) params.set("market", market);
  const res = await fetch(`${getApiBaseUrl()}/api/v1/stocks/search?${params}`, { cache: "no-store" });
  await ensureOk(res, "Failed to search stocks");
  const body = await parseJson<{ data: StockInfo[] | null; total?: number }>(res);
  return { items: body.data ?? [], total: body.total ?? 0 };
}

export async function fetchQuoteDetailClient(symbol: string): Promise<QuoteDetail> {
  return apiGet<QuoteDetail>(`/api/v1/quotes/${encodeURIComponent(symbol)}`);
}

/**
 * 여러 종목의 시세를 한 번에.
 *
 * `ttlMs`는 기본 5초다 — 시세는 캐시하면 안 되는 값이지만, **같은 목록을 그리는
 * 카드 둘이 같은 순간에 각자 부르는 것**은 막아야 한다(이 함수의 중복 제거가 그
 * 일을 한다). 5초는 눈이 알아채지 못하면서 그 중복만 걸러내는 폭이다.
 * 실시간 갱신이 필요한 화면은 폴링 주기를 따로 두므로 그때마다 새로 받는다.
 */
export async function fetchQuotesBatchClient(
  symbols: string[],
  ttlMs = 5_000,
): Promise<QuoteSummary[]> {
  if (symbols.length === 0) return [];
  const params = new URLSearchParams({ symbols: symbols.join(",") });
  const path = `/api/v1/quotes/batch?${params}`;
  return cachedGet(
    path,
    async () => {
      const res = await fetch(`${getApiBaseUrl()}${path}`, { cache: "no-store" });
      await ensureOk(res, "Failed to load batch quotes");
      const body = await parseJson<{ data: QuoteSummary[] | null }>(res);
      return body.data ?? [];
    },
    ttlMs,
  );
}

export async function fetchOrderBookClient(symbol: string): Promise<OrderBook> {
  return apiGet<OrderBook>(`/api/v1/quotes/${encodeURIComponent(symbol)}/orderbook`);
}

export async function fetchTradesClient(symbol: string): Promise<TradeTick[]> {
  return apiGet<TradeTick[]>(`/api/v1/quotes/${encodeURIComponent(symbol)}/trades`);
}

export async function fetchQuoteTerminalSnapshotClient(
  symbol: string,
): Promise<QuoteTerminalSnapshot> {
  return apiGet<QuoteTerminalSnapshot>(
    `/api/v1/quotes/${encodeURIComponent(symbol)}/terminal-snapshot`,
  );
}

export async function fetchCandleSnapshotClient(
  symbol: string,
  interval: CandleInterval = "5m",
): Promise<CandleSnapshot> {
  const params = new URLSearchParams({ interval });
  const res = await fetch(
    `${getApiBaseUrl()}/api/v1/quotes/${encodeURIComponent(symbol)}/candles/snapshot?${params}`,
    { cache: "no-store" },
  );
  await ensureOk(res, "Failed to load candle snapshot");
  const body = await parseJson<{ data: CandleSnapshot; source?: string }>(res);
  return { ...body.data, source: body.data.source ?? body.source ?? "unknown" };
}

export function quoteStreamUrl(symbol: string, interval: CandleInterval = "5m"): string {
  const params = new URLSearchParams({ interval });
  return `${getApiBaseUrl()}/api/v1/quotes/${encodeURIComponent(symbol)}/stream?${params}`;
}

export function quotePriceStreamUrl(symbol: string): string {
  return `${getApiBaseUrl()}/api/v1/quotes/${encodeURIComponent(symbol)}/quote-stream`;
}

export async function fetchCandlesClient(
  symbol: string,
  interval: CandleInterval = "5m",
): Promise<Candle[]> {
  const page = await fetchCandlesPageClient(symbol, interval);
  return page.candles;
}

export async function fetchCandlesPageClient(
  symbol: string,
  interval: CandleInterval = "5m",
  before?: string,
): Promise<CandlesPage & { source?: string }> {
  const params = new URLSearchParams({ interval });
  if (before) params.set("before", before);
  const res = await fetch(
    `${getApiBaseUrl()}/api/v1/quotes/${encodeURIComponent(symbol)}/candles?${params}`,
    { cache: "no-store" },
  );
  await ensureOk(res, "Failed to load candles");
  const body = await parseJson<{ data: Candle[]; nextBefore?: string | null; source?: string }>(res);
  return {
    candles: body.data ?? [],
    nextBefore: body.nextBefore ?? null,
    source: body.source,
  };
}

// Watchlist
export async function fetchWatchlistClient(): Promise<WatchlistItem[]> {
  const data = await apiGet<WatchlistItem[] | null>("/api/v1/watchlist");
  return data ?? [];
}

export async function addWatchlistClient(symbol: string): Promise<void> {
  await apiPost<{ symbol: string }>("/api/v1/watchlist", { symbol });
}

export async function removeWatchlistClient(symbol: string): Promise<void> {
  await apiDelete<{ symbol: string }>(`/api/v1/watchlist/${encodeURIComponent(symbol)}`);
}

export async function reorderWatchlistClient(symbols: string[]): Promise<void> {
  await apiPut<{ ok: boolean }>("/api/v1/watchlist/order", { symbols });
}

export async function fetchStockClient(symbol: string): Promise<StockInfo> {
  return apiGet<StockInfo>(`/api/v1/stocks/${encodeURIComponent(symbol)}`);
}

export async function fetchStocksBatchClient(symbols: string[]): Promise<StockInfo[]> {
  if (symbols.length === 0) return [];
  const params = new URLSearchParams({ symbols: symbols.join(",") });
  const res = await fetch(`${getApiBaseUrl()}/api/v1/stocks?${params}`, { cache: "no-store" });
  await ensureOk(res, "Failed to load stocks");
  const body = await parseJson<{ data: StockInfo[] | null }>(res);
  return body.data ?? [];
}

export type MarketDataSettingsResponse = {
  rankingsPollRealtimeMinutes: number;
  rankingsPollShortMinutes: number;
  rankingsPollLongMinutes: number;
  quoteOverlayPollSeconds: number;
  rankingsClientPollSeconds: number;
  sharesPollHours: number;
  fundamentalsPollHours: number;
  updatedAt?: string;
};

export async function fetchMarketDataSettingsClient(): Promise<MarketDataSettingsResponse> {
  const res = await fetch(`${getApiBaseUrl()}/api/v1/market/data-settings`, { cache: "no-store" });
  await ensureOk(res, "Failed to load market data settings");
  const body = await parseJson<{ data: MarketDataSettingsResponse }>(res);
  return body.data;
}

export async function fetchRankingsClient(params: {
  type: RankingType;
  marketCountry: MarketCountry;
  duration: RankingDuration;
  count?: number;
  excludeInvestmentCaution?: boolean;
}): Promise<RankingsResponse> {
  const q = new URLSearchParams({
    type: params.type,
    marketCountry: params.marketCountry,
    duration: params.duration,
  });
  if (params.count != null && params.count > 0) {
    q.set("count", String(params.count));
  }
  if (params.excludeInvestmentCaution) {
    q.set("excludeInvestmentCaution", "true");
  }
  // 랭킹 스냅샷은 서버가 주기적으로 갱신한다. 같은 파라미터로 잇달아 물어보는 건
  // (홈의 거래대금 카드 ↔ 마켓 화면) 같은 답을 두 번 받는 것뿐이다.
  return cachedGet(`/api/v1/rankings?${q}`, async () => {
    const res = await fetch(`${getApiBaseUrl()}/api/v1/rankings?${q}`, { cache: "no-store" });
    await ensureOk(res, "랭킹 조회 실패");
    const body = await parseJson<{ data: RankingsResponse }>(res);
    return body.data ?? { rankedAt: null, rankings: [] };
  });
}
