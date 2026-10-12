import type { ApiResponse } from "@/lib/api-types";
import type {
  NewsArticle,
  NewsAssetTag,
  NewsListFilter,
  NewsTopic,
  NewsTopicFilter,
  NewsPreviewMode,
  NewsReaderContent,
  NewsSourceId,
} from "@/lib/news-types";
import { NEWS_SOURCES } from "@/lib/mock-news";
import { publisherById, resolvePublisherFromUrl } from "@/lib/news-publishers";
import { parseNewsSearchSymbol } from "@/lib/header-search-config";
import { getApiBaseUrl, mergeFetchInit, parseJson } from "@/lib/api-client";
import { cachedGet } from "@/lib/fetch-cache";
import { fetchWithTimeout } from "@/lib/fetch-timeout";

/**
 * 「오늘의 키워드」·「많이 언급된 종목」 집계의 클라이언트 캐시 TTL.
 *
 * 화면 갱신 주기(`NEWS_REFRESH_MS`)가 이 값보다 **길어야** 회차가 실제로 왕복한다.
 * 그 관계는 `screen-refresh.test.ts`가 잠근다 — 값에 이름을 붙인 이유가 그것이다.
 */
export const NEWS_AGGREGATE_TTL = 60_000;

type NewsArticleDTO = {
  id: string;
  market: "domestic" | "us";
  headline: string;
  headlineKo?: string;
  summary?: string;
  summaryKo?: string;
  sourceId: NewsSourceId;
  publisherId?: string;
  publisherName?: string;
  tags: string[];
  symbols?: string[];
  keywords?: string[];
  publishedAt: string;
  viewCount: number;
  url?: string;
  originalUrl?: string;
  portalUrl?: string;
  imageUrl?: string;
  previewMode?: NewsPreviewMode;
  embedPolicy?: "ok" | "blocked" | "unknown";
  readerMode?: boolean;
  highlighted?: boolean;
  topic?: NewsTopic;
  assetTags?: NewsAssetTag[];
};

type NewsListResponse = ApiResponse<NewsArticleDTO[]> & {
  meta?: { count?: number; totalCount?: number; updatedAt?: string };
};

type DashboardNewsResponse = ApiResponse<NewsArticleDTO[]> & {
  meta?: { count?: number; fullCount?: number; updatedAt?: string };
};

export type DomesticNewsResult = {
  articles: NewsArticle[];
  totalCount: number;
};

export type UsNewsResult = DomesticNewsResult;

/**
 * 홈 대시보드용 기사 묶음.
 *
 * `articles`는 앞 `fullCount`건만 **온전한 기사**이고, 그 뒤는 제목·태그·키워드·
 * 발행시각만 남긴 요약본이다. 홈이 실제로 그리는 건 여덟 줄인데 「오늘의 키워드」와
 * 속보 개수는 100건 전체를 세야 해서, 세지 못할 만큼 줄일 수도 없고 100건을 통째로
 * 받을 필요도 없다.
 *
 * 그래서 **모달을 여는 대상은 앞 `fullCount`건으로 제한해야 한다** — 뒤쪽 항목에는
 * 본문 링크가 없어 창이 비어 뜬다.
 */
export type DashboardNewsResult = {
  articles: NewsArticle[];
  fullCount: number;
};

function formatPublisherDisplayName(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed) return "기타";
  // Already Korean
  if (/[\uAC00-\uD7A3]/.test(trimmed)) return trimmed;
  return trimmed;
}

function mapArticle(dto: NewsArticleDTO): NewsArticle {
  const source =
    NEWS_SOURCES.find((s) => s.id === dto.sourceId && s.market === dto.market) ??
    NEWS_SOURCES.find((s) => s.id === dto.sourceId) ??
    NEWS_SOURCES.find((s) => s.id === "rss")!;
  const link = dto.originalUrl?.trim() || dto.url?.trim();
  const publisher = dto.publisherId
    ? publisherById(dto.publisherId, dto.publisherName)
    : resolvePublisherFromUrl(link, dto.publisherName);
  if (dto.publisherName && publisher.id === "unknown") {
    publisher.name = formatPublisherDisplayName(dto.publisherName);
  }
  return {
    id: dto.id,
    market: dto.market,
    headline: dto.headline,
    headlineKo: dto.headlineKo,
    summary: dto.summary,
    summaryKo: dto.summaryKo,
    publisher,
    source,
    tags: dto.tags ?? [],
    symbols: dto.symbols,
    keywords: dto.keywords?.filter(Boolean),
    publishedAt: dto.publishedAt,
    viewCount: dto.viewCount ?? 0,
    url: link,
    originalUrl: dto.originalUrl ?? link,
    portalUrl: dto.portalUrl,
    imageUrl: dto.imageUrl,
    previewMode: dto.previewMode,
    embedPolicy: dto.embedPolicy ?? "unknown",
    readerMode: dto.readerMode,
    highlighted: dto.highlighted,
    topic: dto.topic,
    assetTags: dto.assetTags,
  };
}

export async function fetchUsNews(
  filter: NewsListFilter,
  /**
   * 서버 렌더용 캐시 지정. 안 넘기면 예전 그대로 `no-store`(브라우저 폴링 경로).
   * 서버에서 크롤러에게 보여 줄 첫 화면을 그릴 때는 `next: { revalidate }`를 넘겨
   * 크롤러가 몇 번을 오든 BE 호출이 주기당 한 번이 되게 한다.
   */
  init?: RequestInit,
): Promise<UsNewsResult> {
  const params = new URLSearchParams();
  params.set("sort", filter.sort);
  if (filter.sourceId && filter.sourceId !== "all") {
    params.set("source", filter.sourceId);
  }
  if (filter.symbol?.trim()) {
    params.set("symbol", filter.symbol.trim());
  }
  const query = filter.query?.trim();
  if (query) {
    params.set("q", query);
    const symbol = parseNewsSearchSymbol(query, "us");
    if (symbol) {
      params.set("symbol", symbol);
    }
  }

  const res = await fetchWithTimeout(`${getApiBaseUrl()}/api/v1/news/us?${params}`, mergeFetchInit(init ?? {
    cache: "no-store",
  }));

  if (!res.ok) {
    const text = await res.text();
    throw new Error(text.slice(0, 200) || "Failed to load US news");
  }

  const body = await parseJson<NewsListResponse>(res);
  return {
    articles: (body.data ?? []).map(mapArticle),
    totalCount: body.meta?.totalCount ?? body.data?.length ?? 0,
  };
}

export async function fetchDomesticNews(
  filter: NewsListFilter,
  /** `fetchUsNews`의 같은 인자 참조. */
  init?: RequestInit,
): Promise<DomesticNewsResult> {
  const params = new URLSearchParams();
  params.set("sort", filter.sort);
  if (filter.sourceId && filter.sourceId !== "all") {
    params.set("source", filter.sourceId);
  }
  if (filter.symbol?.trim()) {
    params.set("symbol", filter.symbol.trim());
  }
  if (filter.topic) {
    params.set("topic", filter.topic);
  }
  const query = filter.query?.trim();
  if (query) {
    params.set("q", query);
    const symbol = parseNewsSearchSymbol(query, "domestic");
    if (symbol) {
      params.set("symbol", symbol);
    }
  }

  const res = await fetchWithTimeout(`${getApiBaseUrl()}/api/v1/news/domestic?${params}`, mergeFetchInit(init ?? {
    cache: "no-store",
  }));

  if (!res.ok) {
    const text = await res.text();
    throw new Error(text.slice(0, 200) || "Failed to load news");
  }

  const body = await parseJson<NewsListResponse>(res);
  return {
    articles: (body.data ?? []).map(mapArticle),
    totalCount: body.meta?.totalCount ?? body.data?.length ?? 0,
  };
}

export async function fetchDashboardNews(
  market: "domestic" | "us" = "domestic",
  /**
   * 본문까지 온전히 받을 기사 수.
   *
   * 홈 뉴스 카드는 옆의 브리핑 카드 높이를 채워야 해서 몇 줄을 그릴지가 화면 폭에 따라
   * 달라진다. 뒤쪽 요약본은 모달을 열 수 없으므로(본문·링크가 없다) **그릴 수 있는 줄
   * 수만큼은 온전한 기사여야 한다.** 서버 상한은 30이다.
   */
  full = 8,
  /** 국내 카테고리 칩(2026-10-09). 해외는 무시. */
  topic: NewsTopicFilter = "",
): Promise<DashboardNewsResult> {
  const path = `/api/v1/news/${market}/dashboard?full=${full}${market === "domestic" && topic ? `&topic=${topic}` : ""}`;
  // 서버도 이 목록을 15초 캐시한다. 여기 캐시는 그 위에서 **왕복 자체를** 없앤다 —
  // 홈을 떠났다 돌아오는 30초 안에는 회선을 아예 쓰지 않는다.
  return cachedGet(path, async () => {
    const res = await fetchWithTimeout(
      `${getApiBaseUrl()}${path}`,
      mergeFetchInit({ cache: "no-store" }),
    );
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text.slice(0, 200) || "Failed to load news");
    }
    const body = await parseJson<DashboardNewsResponse>(res);
    return {
      articles: (body.data ?? []).map(mapArticle),
      fullCount: body.meta?.fullCount ?? body.data?.length ?? 0,
    };
  });
}

/** Stock detail — domestic symbol match or US ticker/name keyword match in headlines. */
export async function fetchStockNews(params: {
  symbol: string;
  name: string;
  market: "domestic" | "us";
  limit?: number;
}): Promise<NewsArticle[]> {
  const symbol = params.symbol.trim();
  const name = params.name.trim();
  const limit = params.limit ?? 12;

  if (params.market === "domestic") {
    const { articles } = await fetchDomesticNews({
      market: "domestic",
      sort: "latest",
      symbol,
    });
    return articles.slice(0, limit);
  }

  if (params.market === "us") {
    const { articles } = await fetchUsNews({
      market: "us",
      sort: "latest",
      symbol,
    });
    return articles.slice(0, limit);
  }

  return [];
}

type UsDemoDTO = NewsArticleDTO & { readerMode?: boolean };

export async function fetchUsNewsDemo(): Promise<NewsArticle> {
  const res = await fetchWithTimeout(`${getApiBaseUrl()}/api/v1/news/us/demo`, mergeFetchInit({
    cache: "no-store",
  }));
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text.slice(0, 200) || "Failed to load US demo article");
  }
  const body = await parseJson<ApiResponse<UsDemoDTO>>(res);
  return mapArticle(body.data);
}

export async function fetchNewsReader(
  url: string,
  opts?: { market?: NewsArticle["market"]; articleId?: string },
): Promise<NewsReaderContent> {
  const params = new URLSearchParams({ url });
  if (opts?.market) {
    params.set("market", opts.market);
  }
  if (opts?.articleId) {
    params.set("articleId", opts.articleId);
  }
  const res = await fetchWithTimeout(`${getApiBaseUrl()}/api/v1/news/reader?${params}`, mergeFetchInit({
    cache: "no-store",
  }));
  if (!res.ok) {
    const body = await parseJson<{ error?: string }>(res).catch(() => ({ error: res.statusText }));
    throw new Error(body.error || "Failed to extract article");
  }
  const body = await parseJson<ApiResponse<NewsReaderContent>>(res);
  return body.data;
}

const readerPrefetch = new Map<string, Promise<NewsReaderContent>>();

function readerPrefetchKey(url: string, market?: NewsArticle["market"], articleId?: string): string {
  return `${market ?? ""}:${articleId ?? ""}:${url.trim()}`;
}

export function prefetchNewsReader(
  url: string,
  opts?: { market?: NewsArticle["market"]; articleId?: string },
): void {
  const trimmed = url.trim();
  const key = readerPrefetchKey(trimmed, opts?.market, opts?.articleId);
  if (!trimmed || trimmed === "#" || readerPrefetch.has(key)) {
    return;
  }
  const promise = fetchNewsReader(trimmed, opts).catch(() => {
    readerPrefetch.delete(key);
    throw new Error("prefetch failed");
  });
  readerPrefetch.set(key, promise);
}

export async function recordNewsView(id: string): Promise<void> {
  const params = new URLSearchParams({ id });
  await fetchWithTimeout(`${getApiBaseUrl()}/api/v1/news/view?${params}`, mergeFetchInit({
    method: "POST",
    cache: "no-store",
  })).catch(() => {
    // non-blocking
  });
}

/* ── 오늘의 키워드 ─────────────────────────────────────────────────────────── */

export type NewsKeywordCount = { keyword: string; count: number };

export type NewsTopKeywords = {
  keywords: NewsKeywordCount[];
  /** 실제로 센 기사 수. 요청한 표본보다 적을 수 있다(쌓인 게 그것뿐인 시장). */
  articleCount: number;
  /** 목록에 들어가려면 최소 몇 건에 등장해야 하는지. 각주 문구가 쓴다. */
  minMentions: number;
};

/**
 * 「오늘의 키워드」를 **집계된 채로** 받는다.
 *
 * 예전에는 홈이 기사 목록(100건)을 통째로 받아 브라우저에서 셌다. 표본을 1,000건으로
 * 넓히면서 서버로 옮겼다 — 요약본 1,000건은 300KB가 넘고, 그 전에 서버의 국내 200건
 * 상한에 걸려 **1,000을 달라고 해도 200이 왔다**. 집계 규칙(한 기사당 1회 · 두 글자
 * 이상 · 2건 이상)은 이제 서버 한 곳에만 있다.
 *
 * 서버도 5분 캐시하지만, 여기 캐시는 그 위에서 **왕복 자체를** 없앤다.
 */
export async function fetchTopKeywords(
  market: "domestic" | "us",
  limit = 14,
): Promise<NewsTopKeywords> {
  const path = `/api/v1/news/${market}/keywords?limit=${limit}`;
  return cachedGet(
    path,
    async () => {
      const res = await fetchWithTimeout(
        `${getApiBaseUrl()}${path}`,
        mergeFetchInit({ cache: "no-store" }),
      );
      if (!res.ok) throw new Error("Failed to load keywords");
      const body = await parseJson<ApiResponse<NewsTopKeywords | null>>(res);
      return {
        keywords: body.data?.keywords ?? [],
        articleCount: body.data?.articleCount ?? 0,
        minMentions: body.data?.minMentions ?? 2,
      };
    },
    NEWS_AGGREGATE_TTL,
  );
}

/* ── 많이 언급된 종목 ──────────────────────────────────────────────────────── */

export type NewsSymbolCount = {
  symbol: string;
  /** 서버가 `stocks`에서 붙여 준 종목명. 화면은 이걸 그대로 쓴다. */
  name: string;
  count: number;
};

export type NewsTopSymbols = {
  symbols: NewsSymbolCount[];
  /** 실제로 센 기사 수. 요청한 표본보다 적을 수 있다. */
  articleCount: number;
  minMentions: number;
};

/**
 * 「많이 언급된 종목」을 **집계된 채로** 받는다 — 「오늘의 키워드」와 같은 짝이다.
 *
 * 예전에는 화면이 목록(100건)을 세었다. 그런데 그 100건이 덮는 시간이 **34분**이라
 * (실측 2026-09-01: 16:14~16:47) 「오늘 많이 언급된 종목」이 실은 「지난 30분」이었고,
 * 새로고침마다 순위가 뒤집혔다. 서버는 최근 1,000건을 본다.
 *
 * 응답에 **종목명이 함께 온다.** 그래서 시세 배치가 돌아오기 전에도 화면이
 * 종목코드를 대신 그릴 일이 없다. 국내 시장이면 서버가 해외 티커를 걸러 주므로
 * 국내 뉴스 필터에 NVDA가 오르지 않는다 — 기사 하단 관련 종목 칩은 이 집계를 쓰지
 * 않으니 거기서는 그대로 보인다.
 */
export async function fetchTopSymbols(
  market: "domestic" | "us",
  limit = 6,
): Promise<NewsTopSymbols> {
  const path = `/api/v1/news/${market}/symbols?limit=${limit}`;
  return cachedGet(
    path,
    async () => {
      const res = await fetchWithTimeout(
        `${getApiBaseUrl()}${path}`,
        mergeFetchInit({ cache: "no-store" }),
      );
      if (!res.ok) throw new Error("Failed to load top symbols");
      const body = await parseJson<ApiResponse<NewsTopSymbols | null>>(res);
      return {
        symbols: body.data?.symbols ?? [],
        articleCount: body.data?.articleCount ?? 0,
        minMentions: body.data?.minMentions ?? 2,
      };
    },
    NEWS_AGGREGATE_TTL,
  );
}
