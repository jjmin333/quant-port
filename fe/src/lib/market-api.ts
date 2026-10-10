import {
  PortfolioFetchError,
  apiRequest,
  ensureOk,
  getApiBaseUrl,
  parseJson,
} from "@/lib/api-client";
import { cachedGet } from "@/lib/fetch-cache";
import { batchQuotes } from "@/lib/quote-batch";
import { fetchWithTimeout } from "@/lib/fetch-timeout";

/*
  # 같은 값을 두 번 묻지 않는다 (2026-09-03 사용자 지적)

  사용자가 종목을 검색했다가 「요청이 너무 잦습니다」를 받았다. 트래픽 때문이 아니라
  **한 번의 화면 진입이 1초에 서른 건**을 쏘아 사용자 버스트(20)를 넘긴 것이었다.
  서버 로그를 세어 보니 그중 절반이 같은 URL의 반복이었다 — data-settings 6회,
  stamps/state 4회, 종목·요약·점수·재무 각 2회. 카드마다 자기 fetch를 하는 구조
  자체는 옳지만(하나가 죽어도 나머지가 뜬다), 같은 순간에 같은 것을 묻는 것은 낭비다.

  아래 세 폭은 **값이 실제로 바뀌는 주기**에서 왔지, 어림이 아니다.
*/

/** 카탈로그·전역 설정 — 배치나 관리자가 하루 단위로 고치는 값. */
const STATIC_CATALOG_TTL_MS = 5 * 60_000;
/** 환율처럼 분 단위로도 거의 안 움직이는 값. */
const SLOW_MOVING_TTL_MS = 60_000;
/**
 * 시세처럼 **초 단위로 움직이는** 값. 여기서 캐시는 「재사용」이 아니라 **동시 요청
 * 합치기**다 — 같은 순간에 두 컴포넌트가 부르면 하나로 묶고, 폴링(2초)은 그대로
 * 매번 새로 받는다. 폴링 주기보다 짧아야 한다는 것이 이 값의 유일한 조건이다.
 */
const LIVE_COALESCE_TTL_MS = 900;
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
  RankingPriceRow,
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
  /** 서버가 마지막으로 갱신 회차를 끝낸 시각 — 값이 안 바뀌어도 움직인다. */
  checkedAt: string | null;
  groups: Record<string, IndexGroupMeta>;
}> {
  /*
    손으로 세우던 8초 AbortController를 걷어냈다 — 같은 상한이 이제 `fetchWithTimeout`
    안에 있고, 앱의 **모든** 요청이 그것을 지난다. 이 함수만 상한을 갖고 있던 것이
    2026-09-20 점검의 출발점이었다(`lib/fetch-timeout.ts` 머리글).

    `window.setTimeout`을 쓰던 자리라 이 함수는 서버에서 부르면 터졌었다. 이제는
    환경에 맞는 상한이 자동으로 걸려 **SSR 선반입에서도 그대로 쓸 수 있다.**
  */
  const res = await fetchWithTimeout(`${getApiBaseUrl()}/api/v1/quotes/indices`, {
    cache: "no-store",
  });
  await ensureOk(res, "Failed to load indices");
  const body = await parseJson<{
    data: MarketIndex[] | null;
    updatedAt?: string;
    checkedAt?: string;
    groups?: Record<string, IndexGroupMeta>;
  }>(res);
  return {
    indices: body.data ?? [],
    updatedAt: body.updatedAt ?? null,
    checkedAt: body.checkedAt ?? null,
    groups: body.groups ?? {},
  };
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

/**
 * 원/달러 환율 — 해외 종목 값을 원화로 바꿔 적는 화면이 전부 쓴다.
 *
 * 여러 카드가 각자 부르므로 짧게 재사용한다. 환율은 분 단위로도 거의 안 움직이고,
 * 화면이 쓰는 곳은 「대략 얼마인가」다.
 */
export async function fetchExchangeRateClient(): Promise<number> {
  const path = "/api/v1/exchange-rate?baseCurrency=USD&quoteCurrency=KRW";
  return cachedGet(
    path,
    async () => {
      const res = await fetchWithTimeout(`${getApiBaseUrl()}${path}`, { cache: "no-store" });
      await ensureOk(res, "Failed to load exchange rate");
      const body = await parseJson<{ data: { midRate?: string; rate?: string } | null }>(res);
      const raw = body.data?.midRate ?? body.data?.rate ?? "0";
      const rate = Number.parseFloat(raw);
      return Number.isFinite(rate) ? rate : 0;
    },
    SLOW_MOVING_TTL_MS,
  );
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
  const res = await fetchWithTimeout(`${getApiBaseUrl()}/api/v1/stocks/search?${params}`, { cache: "no-store" });
  await ensureOk(res, "Failed to search stocks");
  const body = await parseJson<{ data: StockInfo[] | null; total?: number }>(res);
  return { items: body.data ?? [], total: body.total ?? 0 };
}

/**
 * 종목 하나의 시세 상세. **초 단위로 움직이는 값**이라 캐시 폭은 겨우 한 번의
 * 「같은 순간」이다 — 같은 화면의 두 컴포넌트가 동시에 부르는 것만 합치고, 폴링은
 * 그대로 매번 새로 받는다(`LIVE_COALESCE_TTL_MS` 주석 참조).
 */
export async function fetchQuoteDetailClient(symbol: string): Promise<QuoteDetail> {
  const path = `/api/v1/quotes/${encodeURIComponent(symbol)}`;
  return cachedGet(path, () => apiGet<QuoteDetail>(path), LIVE_COALESCE_TTL_MS);
}

/**
 * 여러 종목의 시세를 한 번에.
 *
 * `ttlMs`는 기본 5초다 — 시세는 캐시하면 안 되는 값이지만, **같은 목록을 그리는
 * 카드 둘이 같은 순간에 각자 부르는 것**은 막아야 한다. 5초는 눈이 알아채지
 * 못하면서 그 중복만 걸러내는 폭이다. 실시간 갱신이 필요한 화면은 폴링 주기를
 * 따로 두므로 그때마다 새로 받는다.
 *
 * **목록이 서로 다른 카드들도 합쳐진다** (2026-09-05 사용자 지적 6). 예전에는 중복
 * 제거가 `cachedGet`의 URL 키였는데, 심볼 목록이 한 글자만 달라도 다른 키라 홈에서만
 * 다섯 번이 나갔다(그중 스무 종목은 서로 겹쳤다). 지금은 `lib/quote-batch`가 짧은
 * 창 동안 심볼을 모아 **한 번** 부르고, 받은 행을 **종목 단위로** 적어 둔다 —
 * 목록이 조금씩 다른 카드들이 서로의 결과를 쓴다.
 */
export async function fetchQuotesBatchClient(
  symbols: string[],
  ttlMs = 5_000,
): Promise<QuoteSummary[]> {
  if (symbols.length === 0) return [];
  return batchQuotes(symbols, fetchQuotesBatchRaw, ttlMs);
}

/** 실제 요청 한 번. 심볼 목록은 `batchQuotes`가 이미 합쳐 놓은 것이다. */
async function fetchQuotesBatchRaw(symbols: string[]): Promise<QuoteSummary[]> {
  const params = new URLSearchParams({ symbols: symbols.join(",") });
  const path = `/api/v1/quotes/batch?${params}`;
  const res = await fetchWithTimeout(`${getApiBaseUrl()}${path}`, { cache: "no-store" });
  await ensureOk(res, "Failed to load batch quotes");
  const body = await parseJson<{ data: QuoteSummary[] | null }>(res);
  return body.data ?? [];
}

export async function fetchOrderBookClient(symbol: string): Promise<OrderBook> {
  return apiGet<OrderBook>(`/api/v1/quotes/${encodeURIComponent(symbol)}/orderbook`);
}

export async function fetchTradesClient(symbol: string): Promise<TradeTick[]> {
  return apiGet<TradeTick[]>(`/api/v1/quotes/${encodeURIComponent(symbol)}/trades`);
}

/**
 * 종목 상세가 2초마다 읽는 스냅샷(현재가·호가·체결).
 *
 * `wantsOrderBook`은 **화면이 호가를 실제로 그리고 있을 때만** true다. 서버는 이
 * 신호로 웹소켓 호가 구독 대상을 정한다 — 상세 화면에 들어간 것만으로 호가까지
 * 구독하면 아무도 안 보는 프레임을 받게 된다(BE `marketPoller.book` 주석의 실측:
 * 호가 프레임이 체결의 4.3배였다).
 */
export async function fetchQuoteTerminalSnapshotClient(
  symbol: string,
  wantsOrderBook = false,
): Promise<QuoteTerminalSnapshot> {
  const q = wantsOrderBook ? "?book=1" : "";
  const path = `/api/v1/quotes/${encodeURIComponent(symbol)}/terminal-snapshot${q}`;
  return cachedGet(path, () => apiGet<QuoteTerminalSnapshot>(path), LIVE_COALESCE_TTL_MS);
}

export async function fetchCandleSnapshotClient(
  symbol: string,
  interval: CandleInterval = "5m",
): Promise<CandleSnapshot> {
  const params = new URLSearchParams({ interval });
  const path = `/api/v1/quotes/${encodeURIComponent(symbol)}/candles/snapshot?${params}`;
  return cachedGet(
    path,
    async () => {
      const res = await fetchWithTimeout(`${getApiBaseUrl()}${path}`, { cache: "no-store" });
      await ensureOk(res, "Failed to load candle snapshot");
      const body = await parseJson<{ data: CandleSnapshot; source?: string }>(res);
      return { ...body.data, source: body.data.source ?? body.source ?? "unknown" };
    },
    LIVE_COALESCE_TTL_MS,
  );
}

export function quoteStreamUrl(symbol: string, interval: CandleInterval = "5m"): string {
  const params = new URLSearchParams({ interval });
  return `${getApiBaseUrl()}/api/v1/quotes/${encodeURIComponent(symbol)}/stream?${params}`;
}

export function quotePriceStreamUrl(symbol: string): string {
  return `${getApiBaseUrl()}/api/v1/quotes/${encodeURIComponent(symbol)}/quote-stream`;
}

/**
 * 목록 화면 여러 줄의 가격을 **연결 하나로** 받는 SSE 주소.
 *
 * 위의 `quotePriceStreamUrl`은 상세 화면 한 종목의 것이다. 목록이 그 주소를 줄마다
 * 열면 스무 줄에 연결 스무 개다 — 그러라고 만든 경로가 아니다.
 *
 * `assetType`은 서버가 **어느 거래소 달력으로 마감을 판정할지** 정하는 값이다.
 * 목록은 시장 탭 하나를 그리므로 행들이 달력을 공유한다.
 */
export function listPriceStreamUrl(symbols: string[], assetType: string): string {
  const params = new URLSearchParams({ symbols: symbols.join(",") });
  if (assetType) params.set("assetType", assetType);
  return `${getApiBaseUrl()}/api/v1/quotes/prices/stream?${params}`;
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
  const res = await fetchWithTimeout(
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

/**
 * 관심종목에 담는다.
 *
 * `addedPrice` 는 **실행 취소로 되담을 때만** 넘긴다. 그러면 서버가 오늘 시세로
 * 덮지 않고 원래의 담은 가격을 그대로 되살린다 — 「담은 뒤로 얼마나 움직였나」의
 * 기준선을 잃지 않기 위해서다. 처음 담는 경로는 생략하고 서버가 현재가를 찍는다.
 */
export async function addWatchlistClient(symbol: string, addedPrice?: number): Promise<void> {
  await apiPost<{ symbol: string }>("/api/v1/watchlist", {
    symbol,
    ...(addedPrice && addedPrice > 0 ? { addedPrice } : {}),
  });
}

export async function removeWatchlistClient(symbol: string): Promise<void> {
  await apiDelete<{ symbol: string }>(`/api/v1/watchlist/${encodeURIComponent(symbol)}`);
}

export async function reorderWatchlistClient(symbols: string[]): Promise<void> {
  await apiPut<{ ok: boolean }>("/api/v1/watchlist/order", { symbols });
}

/**
 * 종목 카탈로그 한 줄(이름·시장·통화·자산군).
 *
 * **한 화면이 이걸 여러 번 묻는다** — 종목 워크스페이스가 뜰 때 헤더·개요·탭이
 * 같은 종목을 각자 필요로 하고, 개발 서버(React StrictMode)는 이펙트를 두 번 돌린다.
 * 실측 2026-09-03: 종목 하나를 여는 데 이 경로가 초당 2회 나갔다.
 *
 * 값 자체는 **배치가 하루 단위로** 고치는 것이라 5분 재사용이 넉넉하다.
 */
export async function fetchStockClient(symbol: string): Promise<StockInfo> {
  const path = `/api/v1/stocks/${encodeURIComponent(symbol)}`;
  return cachedGet(path, () => apiGet<StockInfo>(path), STATIC_CATALOG_TTL_MS);
}

export async function fetchStocksBatchClient(symbols: string[]): Promise<StockInfo[]> {
  if (symbols.length === 0) return [];
  const params = new URLSearchParams({ symbols: symbols.join(",") });
  const path = `/api/v1/stocks?${params}`;
  return cachedGet(
    path,
    async () => {
      const res = await fetchWithTimeout(`${getApiBaseUrl()}${path}`, { cache: "no-store" });
      await ensureOk(res, "Failed to load stocks");
      const body = await parseJson<{ data: StockInfo[] | null }>(res);
      return body.data ?? [];
    },
    STATIC_CATALOG_TTL_MS,
  );
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

/**
 * 폴링 주기 같은 **앱 전역 설정**. 관리자가 바꿀 때만 움직인다.
 *
 * 이걸 읽는 훅(`useMarketDataSettings`)을 여러 컴포넌트가 각자 부르는 바람에 화면
 * 하나가 뜰 때 같은 요청이 **여섯 번**까지 나갔다(실측 2026-09-03 서버 로그).
 * 값이 바뀌는 주기를 생각하면 5분 재사용도 짧은 편이다.
 */
export async function fetchMarketDataSettingsClient(): Promise<MarketDataSettingsResponse> {
  const path = "/api/v1/market/data-settings";
  return cachedGet(
    path,
    async () => {
      const res = await fetchWithTimeout(`${getApiBaseUrl()}${path}`, { cache: "no-store" });
      await ensureOk(res, "Failed to load market data settings");
      const body = await parseJson<{ data: MarketDataSettingsResponse }>(res);
      return body.data;
    },
    STATIC_CATALOG_TTL_MS,
  );
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
    const res = await fetchWithTimeout(`${getApiBaseUrl()}/api/v1/rankings?${q}`, { cache: "no-store" });
    await ensureOk(res, "랭킹 조회 실패");
    const body = await parseJson<{ data: RankingsResponse }>(res);
    return body.data ?? { rankedAt: null, rankings: [] };
  });
}

/**
 * 랭킹 목록의 **가격·등락률만** 다시 받는다 — 순위는 안 건드린다.
 *
 * 화면은 이걸 5초마다 부르고 **있는 행에 숫자만 덮어쓴다.** 순위 스냅샷 자체는
 * 서버가 5분 주기로 갱신하므로 `/rankings`를 5초마다 불러 봐야 같은 순서를 다시
 * 받을 뿐이고, 목록을 통째로 갈아 끼우면 스크롤 위치와 행 자리가 흔들린다.
 *
 * 장이 닫혀 있으면 서버가 빈 배열을 준다 — 그때는 확정 종가를 그대로 둔다.
 *
 * `cachedGet`을 쓰지 않는다: 그 캐시의 기본 TTL이 30초라 5초 폴링이 여섯 번 중
 * 다섯 번은 캐시에 막힌다.
 */
export async function fetchRankingPricesClient(params: {
  type: RankingType;
  marketCountry: MarketCountry;
  duration: RankingDuration;
  count?: number;
}): Promise<RankingPriceRow[]> {
  const q = new URLSearchParams({
    type: params.type,
    marketCountry: params.marketCountry,
    duration: params.duration,
  });
  if (params.count != null && params.count > 0) {
    q.set("count", String(params.count));
  }
  const res = await fetchWithTimeout(`${getApiBaseUrl()}/api/v1/rankings/prices?${q}`, { cache: "no-store" });
  await ensureOk(res, "랭킹 시세 조회 실패");
  const body = await parseJson<{ data: RankingPriceRow[] }>(res);
  return body.data ?? [];
}
