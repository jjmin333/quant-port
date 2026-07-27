import type { ApiError, ApiResponse } from "@/lib/portfolio-types";
import { PortfolioFetchError, getApiBaseUrl, mergeFetchInit, parseJson } from "@/lib/api-client";
import { sessionFetch } from "@/lib/session-fetch";
import type { BriefingEvent } from "@/lib/insights-types";
import type {
  BrokerAccount,
  OrdersListResponse,
} from "@/lib/orders-types";
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

export { PortfolioFetchError, getApiBaseUrl };

async function apiGet<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await sessionFetch(path, mergeFetchInit({ cache: "no-store", ...init }));
  if (!res.ok) {
    const body = await parseJson<ApiError>(res).catch(() => ({ error: res.statusText }));
    throw new PortfolioFetchError(body.error || "API request failed", res.status);
  }
  const body = await parseJson<ApiResponse<T>>(res);
  return body.data;
}

async function apiPost<T>(path: string, payload?: unknown): Promise<T> {
  const res = await sessionFetch(path, mergeFetchInit({
    method: "POST",
    cache: "no-store",
    headers: { "Content-Type": "application/json" },
    body: payload ? JSON.stringify(payload) : undefined,
  }));
  if (!res.ok) {
    const body = await parseJson<ApiError>(res).catch(() => ({ error: res.statusText }));
    throw new PortfolioFetchError(body.error || "API request failed", res.status);
  }
  const body = await parseJson<ApiResponse<T>>(res);
  return body.data;
}

async function apiDelete<T>(path: string): Promise<T> {
  const res = await sessionFetch(path, mergeFetchInit({ method: "DELETE", cache: "no-store" }));
  if (!res.ok) {
    const body = await parseJson<ApiError>(res).catch(() => ({ error: res.statusText }));
    throw new PortfolioFetchError(body.error || "API request failed", res.status);
  }
  const body = await parseJson<ApiResponse<T>>(res);
  return body.data;
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
    if (!res.ok) throw new PortfolioFetchError("Failed to load indices", res.status);
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
  const res = await fetch(`${getApiBaseUrl()}/api/v1/quotes?${params}`, { cache: "no-store" });
  if (!res.ok) throw new PortfolioFetchError("Failed to load quotes", res.status);
  const body = await parseJson<{ data: QuoteSummary[] | null }>(res);
  return body.data ?? [];
}

export async function fetchExchangeRateClient(): Promise<number> {
  const res = await fetch(
    `${getApiBaseUrl()}/api/v1/exchange-rate?baseCurrency=USD&quoteCurrency=KRW`,
    { cache: "no-store" },
  );
  if (!res.ok) throw new PortfolioFetchError("Failed to load exchange rate", res.status);
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
  if (!res.ok) throw new PortfolioFetchError("Failed to search stocks", res.status);
  const body = await parseJson<{ data: StockInfo[] | null; total?: number }>(res);
  return { items: body.data ?? [], total: body.total ?? 0 };
}

export async function fetchQuoteDetailClient(symbol: string): Promise<QuoteDetail> {
  return apiGet<QuoteDetail>(`/api/v1/quotes/${encodeURIComponent(symbol)}`);
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
  if (!res.ok) throw new PortfolioFetchError("Failed to load candle snapshot", res.status);
  const body = await parseJson<{ data: CandleSnapshot; source?: string }>(res);
  return { ...body.data, source: body.data.source ?? body.source ?? "unknown" };
}

export function quoteStreamUrl(symbol: string, interval: CandleInterval = "5m"): string {
  const params = new URLSearchParams({ interval });
  return `${getApiBaseUrl()}/api/v1/quotes/${encodeURIComponent(symbol)}/stream?${params}`;
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
  if (!res.ok) throw new PortfolioFetchError("Failed to load candles", res.status);
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

// Accounts & order history (read-only)
export async function fetchAccountsClient(): Promise<BrokerAccount[]> {
  return apiGet<BrokerAccount[]>("/api/v1/accounts");
}

export async function fetchOrdersClient(
  accountSeq: number,
  status: "OPEN" | "CLOSED" = "OPEN",
): Promise<OrdersListResponse> {
  return apiGet<OrdersListResponse>(
    `/api/v1/orders?accountSeq=${accountSeq}&status=${status}&limit=50`,
  );
}

// Market / disclosures
export async function fetchStockClient(symbol: string): Promise<StockInfo> {
  return apiGet<StockInfo>(`/api/v1/stocks/${encodeURIComponent(symbol)}`);
}

export async function fetchStocksBatchClient(symbols: string[]): Promise<StockInfo[]> {
  if (symbols.length === 0) return [];
  const params = new URLSearchParams({ symbols: symbols.join(",") });
  const res = await fetch(`${getApiBaseUrl()}/api/v1/stocks?${params}`, { cache: "no-store" });
  if (!res.ok) throw new PortfolioFetchError("Failed to load stocks", res.status);
  const body = await parseJson<{ data: StockInfo[] | null }>(res);
  return body.data ?? [];
}

export async function fetchDisclosuresTodayClient(): Promise<BriefingEvent[]> {
  return apiGet<BriefingEvent[]>("/api/v1/disclosures/today");
}

export async function fetchPortfolioSummaryClient() {
  const { fetchPortfolioSummaryClient: fn } = await import("@/lib/api-client");
  return fn();
}

export async function fetchDashboardInsightsClient() {
  const { fetchDashboardInsightsClient: fn } = await import("@/lib/api-client");
  return fn();
}
